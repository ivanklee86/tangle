//go:build e2e

package tangle

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newE2ETangle builds a real *Tangle from integration/tangle.yaml against
// the live ArgoCD `task services:cicd` brings up — the same config-loading
// path loader_test.go exercises, and the same "test"/"prod" ArgoCDs this
// package's mock-backed handler tests (handlers_test.go) used to build by
// hand before those moved to argocdfakes.
func newE2ETangle(t *testing.T) *Tangle {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatal(err)
	}

	config := koanf.New(".")
	loadedConfig, err := LoadConfig(config, LoadConfigOptions{Path: "../../integration/tangle.yaml"})
	if err != nil {
		t.Fatal(err)
	}

	// integration/tangle.yaml doesn't set doNotInstrument (real deployments
	// want the metrics) — but New() registers each ArgoCD's worker-pool
	// metrics on Prometheus's global registry, and constructing a fresh
	// Tangle per subtest (this file's own tests do exactly that) would
	// panic on the second registration of the same metric names. Disable
	// instrumentation for these short-lived test instances, same as
	// handlers_test.go's hand-rolled fake-backed config already does.
	loadedConfig.DoNotInstrument = true

	return New(loadedConfig, "testing")
}

// TestE2E_ServerApplications exercises applicationsHandler against a live
// ArgoCD seeded from integration/kubernetes/example/ — all four
// applications, split "default"/"my-project" across the "test"/"prod"
// ArgoCDs configured in integration/tangle.yaml, with the same
// label/exclude-label matrix TestHandlers asserts against fakes.
//
// Every top-level test in this file is prefixed TestE2E_ (matching
// internal/argocd/client_e2e_test.go's TestE2E_*) so `-run '^TestE2E_'`
// (tasks/go.yaml's test:e2e/test:e2e:ci) can select just these.
func TestE2E_ServerApplications(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		test_count int
		prod_count int
	}{
		{name: "no_tags", url: "/applications", test_count: 2, prod_count: 2},
		{name: "tags_match_all", url: "/applications?labels=foo:bar", test_count: 2, prod_count: 2},
		{name: "tags_match_one", url: "/applications?labels=env:test", test_count: 1, prod_count: 0},
		{name: "tags_match_none", url: "/applications?labels=env:foobar", test_count: 0, prod_count: 0},
		{name: "multiple_tags", url: "/applications?labels=env:test,bazz:buzz", test_count: 1, prod_count: 0},
		{name: "exclude", url: "/applications?labels=foo:bar&excludeLabels=env:test", test_count: 1, prod_count: 2},
		// #240: an exclude-only query used to be sent with no selector at
		// all, so ArgoCD listed every application (2 and 2 here). This is
		// the only layer where a real ArgoCD, rather than apimachinery,
		// interprets the selector.
		{name: "exclude_only", url: "/applications?excludeLabels=env:test", test_count: 1, prod_count: 2},
		// prod_count is 2 because the "my-project" applications carry no
		// "bazz" label, and a Kubernetes "!=" requirement matches an absent
		// key.
		{name: "exclude_only_multiple", url: "/applications?excludeLabels=env:test,bazz:buzz", test_count: 0, prod_count: 2},
		// The same key in both parameters with different values is
		// redundant but satisfiable, and stays a 200.
		{name: "cross_map_different_values", url: "/applications?labels=env:test&excludeLabels=env:prod", test_count: 1, prod_count: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newE2ETangle(t)

			req, _ := http.NewRequest("GET", test.url, nil)
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tangle.applicationsHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var result ApplicationsResponse
			err := json.NewDecoder(rr.Body).Decode(&result)
			assert.NoError(t, err)
			for _, argoCDResult := range result.Results {
				switch argoCDResult.Name {
				case "test":
					assert.Equal(t, test.test_count, len(argoCDResult.Applications))
				case "prod":
					assert.Equal(t, test.prod_count, len(argoCDResult.Applications))
				}
			}
		})
	}
}

// TestE2E_ServerApplicationsBadRequest covers #241 against a Tangle built
// from integration/tangle.yaml through the real LoadConfig path, with real
// wrappers and the real router and middleware behind it.
//
// A rejected request never reaches ArgoCD, so unlike the cases above this
// isn't testing the live integration — the handler tests already pin the
// messages. What it adds is that the validation still holds when the server
// is assembled from real configuration rather than a hand-built test config,
// which is where a config-dependent difference (timeouts, CORS, the
// instrumentation middleware the unit tests disable) would surface.
func TestE2E_ServerApplicationsBadRequest(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "malformed_include", url: "/api/applications?labels=env"},
		{name: "malformed_exclude", url: "/api/applications?excludeLabels=env"},
		{name: "too_many_separators", url: "/api/applications?labels=env:test:extra"},
		{name: "malformed_alongside_valid", url: "/api/applications?labels=env,team:platform"},
		{name: "duplicate_include_key", url: "/api/applications?labels=env:test,env:prod"},
		{name: "duplicate_exclude_key", url: "/api/applications?excludeLabels=env:test,env:prod"},
		{name: "contradictory_pair", url: "/api/applications?labels=env:test&excludeLabels=env:test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newE2ETangle(t)

			server := httptest.NewServer(tangle.Server.Handler)
			defer server.Close()

			resp, err := http.Get(server.URL + test.url)
			assert.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var result ErrorResponse
			assert.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
			assert.NotEmpty(t, result.Error, "a 400 must say which label was wrong")
		})
	}
}

// TestServerE2EDiffs generates a real diff for one application per ArgoCD
// against the real cluster's "main"/"test_gitops" branches. test-4 (not
// test-3) is deliberately the "prod" case here: test-3's target-branch helm
// chart has a genuinely broken values.yaml (confirmed live — its diff
// response comes back 200 with only manifestGenerationError set, via a
// server-side error path in applicationManifestsHandler that has no `return`
// after the error branch and works only because chi's Recoverer middleware
// absorbs the resulting nil-pointer panic after the error JSON already
// flushed — see the CLI tests, which exercise that exact path through the
// full router). A new test shouldn't freshly depend on that accident when a
// clean example (test-4) makes the same point about real diff generation.
func TestE2E_ServerDiffs(t *testing.T) {
	tests := []struct {
		name   string
		argocd string
		app    string
	}{
		{name: "test_argocd", argocd: "test", app: "test-1"},
		{name: "prod_argocd", argocd: "prod", app: "test-4"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tangle := newE2ETangle(t)

			requestBody, _ := json.Marshal(DiffsRequest{LiveRef: "main", TargetRef: "test_gitops"})
			req, _ := http.NewRequest("POST", "/api/argocd/"+test.argocd+"/applications/"+test.app+"/diffs", bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("argocd", test.argocd)
			rctx.URLParams.Add("name", test.app)
			ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(tangle.applicationManifestsHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var result DiffsResponse
			err := json.NewDecoder(rr.Body).Decode(&result)
			assert.NoError(t, err)
			assert.Empty(t, result.ManifestGenerationError)
			assert.NotEmpty(t, result.TargetManifests)
		})
	}
}

// TestE2E_ServerSourceFixtures covers the source-type fixture Applications
// (integration/kubernetes/sources/) through Tangle's real router, the way the
// UI and CLI reach them: list the applications, then ask for each one's diff
// from its listed liveRef to the pushed ref. internal/argocd's
// TestE2E_SourceFixtures pins what each fixture renders at either ref, which
// is what the expected diffs below are built from.
func TestE2E_ServerSourceFixtures(t *testing.T) {
	tangle := newE2ETangle(t)
	server := httptest.NewServer(tangle.Server.Handler)
	defer server.Close()

	t.Run("applications", func(t *testing.T) {
		tests := []struct {
			name     string
			query    string
			expected []string
		}{
			{name: "suite", query: "labels=suite:sources", expected: []string{"git-chart-git-values", "helm-repo", "helm-repo-git-values", "multi-source-cmp", "same-repo-pinned", "tanka", "three-repos", "two-repos"}},
			{name: "multi_source", query: "labels=source:multi-source", expected: []string{"git-chart-git-values", "helm-repo-git-values", "multi-source-cmp", "same-repo-pinned", "three-repos", "two-repos"}},
			{name: "single_source", query: "labels=suite:sources&excludeLabels=source:multi-source", expected: []string{"helm-repo", "tanka"}},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				byArgoCD := listApplications(t, server.URL, test.query)

				// Only the sources token can see the sources project, so the
				// test and prod ArgoCDs return nothing for these queries.
				assert.Empty(t, byArgoCD["test"])
				assert.Empty(t, byArgoCD["prod"])
				assert.ElementsMatch(t, test.expected, applicationNames(byArgoCD["sources"]))
			})
		}
	})

	sources := listApplications(t, server.URL, "labels=suite:sources")["sources"]

	t.Run("live_refs", func(t *testing.T) {
		// A single-source app's liveRef is its targetRevision: a git branch,
		// or a chart version for a chart from a Helm repo.
		assert.Equal(t, "main", findApplication(t, sources, "tanka").LiveRef)
		assert.Equal(t, "6.17.0", findApplication(t, sources, "helm-repo").LiveRef)
	})

	t.Run("tanka", func(t *testing.T) {
		// Rendered by the Tanka config management plugin sidecar.
		result := postDiffs(t, server.URL, "tanka", findApplication(t, sources, "tanka").LiveRef, "test_gitops")

		assert.Empty(t, result.ManifestGenerationError)
		added, removed := changedLines(result.Diffs)
		assert.ElementsMatch(t, []string{"greeting: hello, world"}, added)
		assert.ElementsMatch(t, []string{"greeting: hello", "removed: soon"}, removed)
	})

	t.Run("helm_repo_chart_version", func(t *testing.T) {
		// For a chart from a Helm repo, the ref to compare to is a chart
		// version.
		result := postDiffs(t, server.URL, "helm-repo", findApplication(t, sources, "helm-repo").LiveRef, "6.16.0")

		assert.Empty(t, result.ManifestGenerationError)
		added, removed := changedLines(result.Diffs)
		assert.Contains(t, added, "helm.sh/chart: podinfo-6.16.0")
		assert.Contains(t, added, "image: ghcr.io/stefanprodan/podinfo:6.16.0")
		assert.Contains(t, removed, "helm.sh/chart: podinfo-6.17.0")
		assert.Contains(t, removed, "image: ghcr.io/stefanprodan/podinfo:6.17.0")
	})

	t.Run("helm_repo_git_ref", func(t *testing.T) {
		// A git branch isn't a chart version, so the render fails, and the
		// failure is reported rather than shown as an empty diff.
		result := postDiffs(t, server.URL, "helm-repo", findApplication(t, sources, "helm-repo").LiveRef, "test_gitops")

		assert.Contains(t, result.ManifestGenerationError, "test_gitops")
		assert.Empty(t, result.TargetManifests)
		assert.Empty(t, result.Diffs)
	})

	// Each multi-source fixture has a real change on test_gitops, in
	// tangle-integration-manifests. With that repo as the target repo, only
	// its sources move (ADR 0032), and both sides are rendered with explicit
	// Revisions, so render artifacts cancel out and the diff is exactly the
	// pushed change.
	//
	// Except for one artifact: an explicit render includes the podinfo
	// chart's Helm test-hook Pods, and the chart names them with
	// randAlphaNum, so every render names them differently and they show up
	// as changed on each side. helm-repo-git-values therefore checks that the
	// diff contains the change, not that it's only the change.
	multiSource := []struct {
		app            string
		added, removed []string
		randomHooks    bool
	}{
		{app: "helm-repo-git-values", added: []string{"replicas: 2"}, removed: []string{"replicas: 1"}, randomHooks: true},
		{app: "git-chart-git-values", added: []string{"greeting: hello, world", "extra: b"}, removed: []string{"greeting: hello", "extra: a"}},
		{app: "two-repos", added: []string{"greeting: hello, world"}, removed: []string{"greeting: hello"}},
		// The tag-pinned source moves too, because it's in the target repo
		// (D4). Its second source also spells the repo without .git.
		{app: "same-repo-pinned", added: []string{"greeting: hello, world", "greeting: hello, world"}, removed: []string{"greeting: hello", "greeting: hello"}},
		{app: "multi-source-cmp", added: []string{"greeting: hello, world"}, removed: []string{"greeting: hello"}},
		{app: "three-repos", added: []string{"greeting: hello, world"}, removed: []string{"greeting: hello"}},
	}
	for _, test := range multiSource {
		t.Run(test.app, func(t *testing.T) {
			result := postDiffsRequest(t, server.URL, test.app, DiffsRequest{TargetRef: "test_gitops", TargetRepos: []string{fixtureManifestsRepo}})

			assert.Equal(t, "ok", result.Status)
			assert.Empty(t, result.ManifestGenerationError)
			added, removed := changedLines(result.Diffs)
			if test.randomHooks {
				assert.Subset(t, added, test.added)
				assert.Subset(t, removed, test.removed)
				return
			}
			assert.ElementsMatch(t, test.added, added)
			assert.ElementsMatch(t, test.removed, removed)
		})
	}

	t.Run("multi_source_needs_target_repo", func(t *testing.T) {
		// two-repos spans podinfo and the manifests repo, so without a
		// target repo Tangle won't guess, even with inference on.
		result := postDiffsRequest(t, server.URL, "two-repos", DiffsRequest{TargetRef: "test_gitops", InferTargetRepo: true})

		assert.Equal(t, "needsTargetRepo", result.Status)
		assert.Equal(t, []string{"github.com/stefanprodan/podinfo", "github.com/ivanklee86/tangle-integration-manifests"}, result.CandidateRepos)
		assert.Contains(t, result.ManifestGenerationError, "needs target repo")
		assert.Empty(t, result.Diffs)
	})

	t.Run("ref_source_users", func(t *testing.T) {
		// helm-repo-git-values' chart reads $values/... through source 2.
		result := postDiffsRequest(t, server.URL, "helm-repo-git-values", DiffsRequest{TargetRef: "test_gitops", TargetRepos: []string{fixtureManifestsRepo}})

		require.Len(t, result.Sources, 2)
		assert.Equal(t, "values", result.Sources[1].Ref)
		assert.Equal(t, []int64{1}, result.Sources[1].UsedBy)
		assert.Equal(t, []int64{1}, findApplication(t, sources, "helm-repo-git-values").Sources[1].UsedBy)
	})

	t.Run("multi_source_inferred", func(t *testing.T) {
		// helm-repo-git-values' only git source is the manifests repo.
		result := postDiffsRequest(t, server.URL, "helm-repo-git-values", DiffsRequest{TargetRef: "test_gitops", InferTargetRepo: true})

		assert.Equal(t, "ok", result.Status)
		assert.True(t, result.InferredTargetRepo)
		// Contains, for the random test-hook names explained above.
		added, removed := changedLines(result.Diffs)
		assert.Contains(t, added, "replicas: 2")
		assert.Contains(t, removed, "replicas: 1")
	})

	t.Run("not_affected", func(t *testing.T) {
		// tanka has no source in podinfo's repo.
		result := postDiffsRequest(t, server.URL, "tanka", DiffsRequest{LiveRef: "main", TargetRef: "test_gitops", TargetRepos: []string{"https://github.com/stefanprodan/podinfo"}})

		assert.Equal(t, "notAffected", result.Status)
		assert.Empty(t, result.ManifestGenerationError)
		assert.Empty(t, result.Diffs)
	})

	t.Run("extra_target_repo_changes_nothing", func(t *testing.T) {
		// M12: a target repo two-repos doesn't use leaves the result as M3.
		result := postDiffsRequest(t, server.URL, "two-repos", DiffsRequest{TargetRef: "test_gitops", TargetRepos: []string{fixtureManifestsRepo, tangleRepo}})

		assert.Equal(t, "ok", result.Status)
		added, removed := changedLines(result.Diffs)
		assert.ElementsMatch(t, []string{"greeting: hello, world"}, added)
		assert.ElementsMatch(t, []string{"greeting: hello"}, removed)
	})

	t.Run("target_repo_without_the_ref", func(t *testing.T) {
		// M11 and M13: tangle has no test_gitops branch, so sending the ref
		// to its source fails, whether it's the only target repo or one of
		// two. The other sources rendering fine doesn't hide the error.
		for _, repos := range [][]string{{tangleRepo}, {fixtureManifestsRepo, tangleRepo}} {
			result := postDiffsRequest(t, server.URL, "three-repos", DiffsRequest{TargetRef: "test_gitops", TargetRepos: repos})

			assert.Equal(t, "error", result.Status, "target repos %v", repos)
			assert.Contains(t, result.ManifestGenerationError, "test_gitops")
			assert.Empty(t, result.Diffs)

			// #293: the error says which sources got the ref. Source 2 is
			// tangle, the one without the branch, and it always moved.
			require.Len(t, result.Sources, 3, "target repos %v", repos)
			assert.True(t, result.Sources[1].Moved, "target repos %v", repos)
			assert.Equal(t, "test_gitops", result.Sources[1].TargetRevision)
			assert.False(t, result.Sources[0].Moved, "podinfo is never a target repo here")
		}
	})

	t.Run("missing_target_ref", func(t *testing.T) {
		// M7: a ref the matched repo doesn't have is an error, never an
		// empty diff.
		result := postDiffsRequest(t, server.URL, "two-repos", DiffsRequest{TargetRef: "does-not-exist", TargetRepos: []string{fixtureManifestsRepo}})

		assert.Equal(t, "error", result.Status)
		assert.Contains(t, result.ManifestGenerationError, "does-not-exist")
	})

	t.Run("no_change_is_an_empty_diff", func(t *testing.T) {
		// L2: diffing against main renders both sides with explicit
		// Revisions at the same content, so nothing differs. (Not
		// helm-repo-git-values: its random Helm test-hook names always
		// differ, as explained above.)
		for _, app := range []string{"git-chart-git-values", "two-repos", "multi-source-cmp"} {
			result := postDiffsRequest(t, server.URL, app, DiffsRequest{TargetRef: "main", TargetRepos: []string{fixtureManifestsRepo}})

			assert.Equal(t, "ok", result.Status, app)
			assert.Empty(t, result.Diffs, app)
		}
	})

	// R1 to R6: the target ref spelled as a tag or a SHA gives the same
	// diff as the branch. The SHA is looked up at run time, because every
	// rebase of test_gitops changes it.
	targetSHA := fixtureTargetSHA(t)
	branchDiff := postDiffs(t, server.URL, "tanka", "main", "test_gitops").Diffs
	require.NotEmpty(t, branchDiff)

	t.Run("tag_and_sha_single_source", func(t *testing.T) {
		// R1 and R2.
		for _, ref := range []string{"fixture-target", targetSHA} {
			result := postDiffs(t, server.URL, "tanka", "main", ref)

			assert.Empty(t, result.ManifestGenerationError, ref)
			assert.Equal(t, diffBody(branchDiff), diffBody(result.Diffs), ref)
		}
	})

	t.Run("tag_and_sha_multi_source", func(t *testing.T) {
		// R3 and R4.
		for _, ref := range []string{"fixture-target", targetSHA} {
			result := postDiffsRequest(t, server.URL, "two-repos", DiffsRequest{TargetRef: ref, TargetRepos: []string{fixtureManifestsRepo}})

			assert.Equal(t, "ok", result.Status, ref)
			added, removed := changedLines(result.Diffs)
			assert.ElementsMatch(t, []string{"greeting: hello, world"}, added, ref)
			assert.ElementsMatch(t, []string{"greeting: hello"}, removed, ref)
		}
	})

	t.Run("abbreviated_sha", func(t *testing.T) {
		// R5: whatever ArgoCD makes of a short SHA, Tangle reports it, as
		// the same diff or as an error naming the ref. Never an empty diff.
		short := targetSHA[:7]
		result := postDiffs(t, server.URL, "tanka", "main", short)

		if result.ManifestGenerationError != "" {
			t.Logf("ArgoCD rejects an abbreviated SHA: %s", result.ManifestGenerationError)
			assert.Contains(t, result.ManifestGenerationError, short)
			return
		}
		t.Log("ArgoCD resolves an abbreviated SHA")
		assert.Equal(t, diffBody(branchDiff), diffBody(result.Diffs))
	})

	t.Run("sha_from_another_repo", func(t *testing.T) {
		// R6: a commit of tangle's isn't in the manifests repo.
		result := postDiffsRequest(t, server.URL, "two-repos", DiffsRequest{TargetRef: tangleFixtureCommit, TargetRepos: []string{fixtureManifestsRepo}})

		assert.Equal(t, "error", result.Status)
		assert.NotEmpty(t, result.ManifestGenerationError)
	})
}

// Repos and refs the source-type fixtures use.
const (
	// tangleRepo is this repo, the second source of three-repos. It has no
	// test_gitops branch.
	tangleRepo = "https://github.com/ivanklee86/tangle.git"
	// tangleFixtureCommit is the commit of this repo three-repos pins.
	tangleFixtureCommit = "309da004053082d138071fb5494d0b510bc23473"
)

// fixtureTargetSHA returns the commit test_gitops points at, and fails if the
// fixture-target tag has drifted from it, which happens when test_gitops is
// rebased and the tag isn't moved (see the manifests repo's README).
func fixtureTargetSHA(t *testing.T) string {
	t.Helper()

	output, err := exec.Command("git", "ls-remote", fixtureManifestsRepo, "refs/heads/test_gitops", "refs/tags/fixture-target", "refs/tags/fixture-target^{}").Output()
	require.NoError(t, err)

	refs := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		sha, ref, found := strings.Cut(line, "\t")
		if found {
			refs[ref] = sha
		}
	}

	branch := refs["refs/heads/test_gitops"]
	tag := refs["refs/tags/fixture-target^{}"]
	if tag == "" {
		tag = refs["refs/tags/fixture-target"]
	}
	require.NotEmpty(t, branch, "test_gitops not found in %s", fixtureManifestsRepo)
	require.Equal(t, branch, tag, "fixture-target must point at test_gitops; move it after a rebase (see the manifests repo's README)")

	return branch
}

// diffBody drops a unified diff's file headers, which name temp files and
// differ between runs.
func diffBody(diff string) string {
	lines := []string{}
	for line := range strings.SplitSeq(diff, "\n") {
		if !strings.HasPrefix(line, "--- ") && !strings.HasPrefix(line, "+++ ") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// fixtureManifestsRepo is the repo the source-type fixtures' test_gitops
// change is in, spelled without .git to show any spelling matches.
const fixtureManifestsRepo = "https://github.com/ivanklee86/tangle-integration-manifests"

// listApplications returns each ArgoCD's applications for a query string.
func listApplications(t *testing.T, serverURL string, query string) map[string][]ApplicationLinks {
	t.Helper()

	resp, err := http.Get(serverURL + "/api/applications?" + query)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result ApplicationsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	byArgoCD := map[string][]ApplicationLinks{}
	for _, argoCDResult := range result.Results {
		byArgoCD[argoCDResult.Name] = argoCDResult.Applications
	}

	return byArgoCD
}

func applicationNames(applications []ApplicationLinks) []string {
	names := []string{}
	for _, application := range applications {
		names = append(names, application.Name)
	}

	return names
}

func findApplication(t *testing.T, applications []ApplicationLinks, name string) ApplicationLinks {
	t.Helper()

	for _, application := range applications {
		if application.Name == name {
			return application
		}
	}
	require.FailNow(t, "application not listed", "%s not in %v", name, applicationNames(applications))

	return ApplicationLinks{}
}

// postDiffs asks the sources ArgoCD for an application's diff.
func postDiffs(t *testing.T, serverURL string, app string, liveRef string, targetRef string) DiffsResponse {
	t.Helper()

	return postDiffsRequest(t, serverURL, app, DiffsRequest{LiveRef: liveRef, TargetRef: targetRef})
}

// postDiffsRequest sends a whole diffs request to the sources ArgoCD.
func postDiffsRequest(t *testing.T, serverURL string, app string, request DiffsRequest) DiffsResponse {
	t.Helper()

	requestBody, err := json.Marshal(request)
	require.NoError(t, err)

	resp, err := http.Post(serverURL+"/api/argocd/sources/applications/"+app+"/diffs", "application/json", bytes.NewBuffer(requestBody))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result DiffsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	return result
}

// changedLines returns a unified diff's added and removed lines, without the
// file headers and with surrounding whitespace trimmed.
func changedLines(diff string) (added []string, removed []string) {
	added, removed = []string{}, []string{}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "), strings.HasPrefix(line, "--- "):
		case strings.HasPrefix(line, "+"):
			added = append(added, strings.TrimSpace(line[1:]))
		case strings.HasPrefix(line, "-"):
			removed = append(removed, strings.TrimSpace(line[1:]))
		}
	}

	return added, removed
}
