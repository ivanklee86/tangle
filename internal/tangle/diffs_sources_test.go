package tangle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoServerApiClient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
)

const (
	chartsRepoURL = "https://github.com/org/charts.git"
	valuesRepoURL = "https://github.com/org/values.git"
)

func sourceApplications() []v1alpha1.Application {
	single := v1alpha1.Application{}
	single.Name = "single"
	single.Namespace = "argocd"
	single.Spec.Source = &v1alpha1.ApplicationSource{RepoURL: chartsRepoURL, Path: "deploy", TargetRevision: "main"}

	multi := v1alpha1.Application{}
	multi.Name = "multi"
	multi.Namespace = "argocd"
	multi.Spec.Sources = v1alpha1.ApplicationSources{
		{RepoURL: chartsRepoURL, Path: "chart", TargetRevision: "main"},
		{RepoURL: valuesRepoURL, Ref: "values", TargetRevision: "main"},
	}

	return []v1alpha1.Application{single, multi}
}

// renderedRevisions answers every manifest query with one ConfigMap naming the
// revisions it was asked for, so a diff shows exactly which revisions changed.
func renderedRevisions(query *application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error) {
	revisions := strings.Join(query.Revisions, ",")
	if query.Revision != nil {
		revisions = *query.Revision
	}

	return &repoServerApiClient.ManifestResponse{Manifests: []string{
		fmt.Sprintf(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"rendered"},"data":{"revisions":%q}}`, revisions),
	}}, nil
}

func newSourcesServer(t *testing.T) (*httptest.Server, *argocdfakes.FakeClient) {
	t.Helper()

	return newSourcesServerWith(t, sourceApplications())
}

func newSourcesServerWith(t *testing.T, apps []v1alpha1.Application) (*httptest.Server, *argocdfakes.FakeClient) {
	t.Helper()

	tangle := newTestTangle()
	client := argocdfakes.NewFakeClient(apps)
	client.ManifestsFunc = renderedRevisions

	wrapper, err := argocd.New(client, "test", &argocd.ArgoCDWrapperOptions{DoNotInstrumentWorkers: true})
	require.NoError(t, err)
	tangle.ArgoCDs = map[string]argocd.IArgoCDWrapper{"test": wrapper}

	server := httptest.NewServer(tangle.Server.Handler)
	t.Cleanup(server.Close)

	return server, client
}

func postSourceDiffs(t *testing.T, server *httptest.Server, applicationName string, body string) (int, DiffsResponse) {
	t.Helper()

	resp, err := http.Post(server.URL+"/api/argocd/test/applications/"+applicationName+"/diffs", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result DiffsResponse
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &result), string(raw))
	}

	return resp.StatusCode, result
}

// Test cases (the diffs endpoint's promises for multi-source support):
//   - a request with only liveRef and targetRef diffs a single-source app as
//     before, now with status ok and its one source (S1 compatibility)
//   - a multi-source app with a target repo diffs only that repo's source and
//     reports which source moved (M3)
//   - inference picks the single git repo when asked (M1 inferred)
//   - a multi-source app with no target repo is "needs target repo", listing
//     the candidates, and also sets manifestGenerationError so older CLIs fail
//     (M4, D2)
//   - a target repo the app doesn't use is "not affected": no error and no
//     manifests (S3)
//   - a blank target repo is rejected with 400 before ArgoCD is called
//   - a failure reports status error
//   - the applications endpoint lists every app's sources
func TestDiffsSources(t *testing.T) {
	t.Run("single source without new fields behaves as before", func(t *testing.T) {
		server, _ := newSourcesServer(t)

		status, result := postSourceDiffs(t, server, "single", `{"liveRef":"main","targetRef":"feature-x"}`)

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "ok", result.Status)
		assert.Empty(t, result.ManifestGenerationError)
		assert.Contains(t, result.Diffs, "-    revisions: main\n")
		assert.Contains(t, result.Diffs, "+    revisions: feature-x\n")
		assert.Equal(t, []DiffSource{{Position: 1, Kind: "git", RepoURL: chartsRepoURL, Path: "deploy", LiveRevision: "main", TargetRevision: "feature-x", Moved: true}}, result.Sources)
	})

	t.Run("multi source with a target repo moves only that source", func(t *testing.T) {
		server, _ := newSourcesServer(t)

		status, result := postSourceDiffs(t, server, "multi", `{"targetRef":"feature-x","targetRepos":["https://github.com/org/values"]}`)

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "ok", result.Status)
		assert.Contains(t, result.Diffs, "-    revisions: main,main\n")
		assert.Contains(t, result.Diffs, "+    revisions: main,feature-x\n")
		assert.Equal(t, []string{"github.com/org/values"}, result.TargetRepos)
		assert.False(t, result.InferredTargetRepo)
		require.Len(t, result.Sources, 2)
		assert.False(t, result.Sources[0].Moved)
		assert.True(t, result.Sources[1].Moved)
		assert.Equal(t, "values", result.Sources[1].Ref)
	})

	t.Run("needs target repo", func(t *testing.T) {
		server, _ := newSourcesServer(t)

		status, result := postSourceDiffs(t, server, "multi", `{"targetRef":"feature-x","inferTargetRepo":true}`)

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "needsTargetRepo", result.Status)
		assert.Equal(t, []string{"github.com/org/charts", "github.com/org/values"}, result.CandidateRepos)
		assert.Contains(t, result.ManifestGenerationError, "needs target repo")
		assert.Contains(t, result.ManifestGenerationError, "github.com/org/charts, github.com/org/values")
		assert.Empty(t, result.Diffs)
	})

	t.Run("not affected", func(t *testing.T) {
		server, client := newSourcesServer(t)

		status, result := postSourceDiffs(t, server, "single", `{"liveRef":"main","targetRef":"feature-x","targetRepos":["https://github.com/org/values.git"]}`)

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "notAffected", result.Status)
		assert.Empty(t, result.ManifestGenerationError)
		assert.Empty(t, result.Diffs)
		assert.Empty(t, result.LiveManifests)
		assert.Empty(t, client.RecordedManifestQueries())
	})

	t.Run("inference picks the one git repo", func(t *testing.T) {
		// A Helm chart plus git values: its git sources are all in one repo.
		app := v1alpha1.Application{}
		app.Name = "helm-values"
		app.Spec.Sources = v1alpha1.ApplicationSources{
			{RepoURL: "https://stefanprodan.github.io/podinfo", Chart: "podinfo", TargetRevision: "6.17.0"},
			{RepoURL: valuesRepoURL, Ref: "values", TargetRevision: "main"},
		}
		server, _ := newSourcesServerWith(t, []v1alpha1.Application{app})

		status, result := postSourceDiffs(t, server, "helm-values", `{"targetRef":"feature-x","inferTargetRepo":true}`)

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "ok", result.Status)
		assert.True(t, result.InferredTargetRepo)
		assert.Equal(t, []string{"github.com/org/values"}, result.TargetRepos)
		assert.Contains(t, result.Diffs, "+    revisions: 6.17.0,feature-x\n")
	})

	t.Run("a blank target repo is rejected", func(t *testing.T) {
		server, client := newSourcesServer(t)

		status, _ := postSourceDiffs(t, server, "multi", `{"targetRef":"feature-x","targetRepos":[" "]}`)

		assert.Equal(t, http.StatusBadRequest, status)
		assert.Empty(t, client.RecordedGets())
	})

	t.Run("a failure reports status error", func(t *testing.T) {
		server, _ := newSourcesServer(t)

		status, result := postSourceDiffs(t, server, "missing", `{"liveRef":"main","targetRef":"feature-x"}`)

		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "error", result.Status)
		assert.NotEmpty(t, result.ManifestGenerationError)
	})

	t.Run("applications list their sources", func(t *testing.T) {
		server, _ := newSourcesServer(t)

		resp, err := http.Get(server.URL + "/api/applications")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		var result ApplicationsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.Len(t, result.Results, 1)

		byName := map[string]ApplicationLinks{}
		for _, app := range result.Results[0].Applications {
			byName[app.Name] = app
		}
		assert.Equal(t, []ApplicationSource{{Position: 1, Kind: "git", RepoURL: chartsRepoURL, Path: "deploy", TargetRevision: "main"}}, byName["single"].Sources)
		assert.Equal(t, []ApplicationSource{
			{Position: 1, Kind: "git", RepoURL: chartsRepoURL, Path: "chart", TargetRevision: "main"},
			{Position: 2, Kind: "git", RepoURL: valuesRepoURL, Ref: "values", TargetRevision: "main"},
		}, byName["multi"].Sources)
		assert.Equal(t, "", byName["multi"].LiveRef)
	})
}
