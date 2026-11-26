package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
	"github.com/ivanklee86/tangle/internal/tangle"
)

// newSourcesTangleServer serves one ArgoCD with two applications whose plans
// are canned: "single" is not affected and "multi" needs a target repo. The
// fake wrapper records the requests the CLI's flags turned into.
func newSourcesTangleServer(t *testing.T) (*httptest.Server, *argocdfakes.FakeWrapper) {
	t.Helper()

	config := tangle.TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		ArgoCDs:         map[string]tangle.TangleArgoCDConfig{},
		DoNotInstrument: true,
	}
	realTangle := tangle.New(&config, "testing")

	wrapper := argocdfakes.NewFakeWrapper([]argocdfakes.FakeApplication{
		{Name: "single", Project: "default", Namespace: "argocd", Labels: map[string]string{"team": "platform"}, LiveRevision: "main"},
		{Name: "multi", Project: "default", Namespace: "argocd", Labels: map[string]string{"team": "platform"}},
		{Name: "other", Project: "default", Namespace: "argocd", Labels: map[string]string{"team": "other"}, LiveRevision: "main"},
	})
	wrapper.ManifestsByApp["single"] = &argocd.GetManifestsResponse{Plan: argocd.RevisionPlan{Status: argocd.DiffStatusNotAffected}}
	wrapper.ManifestsByApp["other"] = &argocd.GetManifestsResponse{Plan: argocd.RevisionPlan{Status: argocd.DiffStatusNotAffected}}
	wrapper.ManifestsByApp["multi"] = &argocd.GetManifestsResponse{Plan: argocd.RevisionPlan{
		Status:         argocd.DiffStatusNeedsTargetRepo,
		MultiSource:    true,
		CandidateRepos: []string{"github.com/org/charts", "github.com/org/values"},
	}}
	realTangle.ArgoCDs = map[string]argocd.IArgoCDWrapper{"test": wrapper}

	server := httptest.NewServer(realTangle.Server.Handler)
	t.Cleanup(server.Close)

	return server, wrapper
}

// Test cases:
//   - a repeated --target-repo and --infer-target-repo reach every diff
//     request, in order, without being split on commas
//   - the results table names each status, and "needs target repo" writes an
//     error file (which is what --fail-on-error counts) while "not affected"
//     doesn't
//   - lists for target-repo and label in tangle.yaml arrive as separate
//     items, not as one "[a b]" string
//   - --target-ref alone sends no target repos and no inference, so
//     single-source diffs behave as before
//
// Not covered: the exit code under --fail-on-error. TangleCLI.Error calls
// os.Exit, which would end the test binary; the error file is the observable
// stand-in for "counted as a failure".
func TestCliTargetRepos(t *testing.T) {
	t.Run("target repo flags reach the server", func(t *testing.T) {
		server, wrapper := newSourcesTangleServer(t)
		tempDir := t.TempDir()

		b := bytes.NewBufferString("")
		command := NewRootCommand()
		command.SetOut(b)
		command.SetErr(b)
		command.SetArgs([]string{
			"generate-manifests",
			"--server-address", strings.TrimPrefix(server.URL, "http://"),
			"--insecure",
			"--folder", tempDir,
			"--target-ref", "feature-x",
			"--target-repo", "https://github.com/org/charts.git",
			"--target-repo", "git@github.com:org/values.git",
			"--infer-target-repo",
		})
		require.NoError(t, command.Execute())

		requests := wrapper.Requests()
		require.Len(t, requests, 3)
		for _, request := range requests {
			assert.Equal(t, "feature-x", request.TargetRef)
			assert.Equal(t, []string{"https://github.com/org/charts.git", "git@github.com:org/values.git"}, request.TargetRepos)
			assert.True(t, request.InferTargetRepo)
		}

		assert.FileExists(t, fmt.Sprintf("%s/error-test-multi.txt", tempDir))
		assert.NoFileExists(t, fmt.Sprintf("%s/error-test-single.txt", tempDir))

		out, err := io.ReadAll(b)
		require.NoError(t, err)
		assert.Contains(t, string(out), "Not affected")
		assert.Contains(t, string(out), "Needs target repo")
	})

	t.Run("lists in tangle.yaml reach list flags item by item", func(t *testing.T) {
		server, wrapper := newSourcesTangleServer(t)
		dir := t.TempDir()
		t.Chdir(dir)
		config := "target-repo:\n  - https://github.com/org/charts.git\n  - git@github.com:org/values.git\nlabel:\n  - team=platform\n"
		require.NoError(t, os.WriteFile("tangle.yaml", []byte(config), 0o644))

		b := bytes.NewBufferString("")
		command := NewRootCommand()
		command.SetOut(b)
		command.SetErr(b)
		command.SetArgs([]string{
			"generate-manifests",
			"--server-address", strings.TrimPrefix(server.URL, "http://"),
			"--insecure",
			"--folder", dir,
			"--target-ref", "feature-x",
		})
		require.NoError(t, command.Execute())

		// The label filter applied: "other" is team=other.
		requests := wrapper.Requests()
		require.Len(t, requests, 2)
		for _, request := range requests {
			assert.NotEqual(t, "other", request.ApplicationName)
			assert.Equal(t, []string{"https://github.com/org/charts.git", "git@github.com:org/values.git"}, request.TargetRepos)
		}
	})

	t.Run("a run with only OK and error diffs prints the table as before", func(t *testing.T) {
		// CI scripts may parse this table, so the Status column only appears
		// when a diff is one the Error column can't describe.
		server := newFakeTangleServer(t)

		b := bytes.NewBufferString("")
		command := NewRootCommand()
		command.SetOut(b)
		command.SetErr(b)
		command.SetArgs([]string{
			"generate-manifests",
			"--server-address", strings.TrimPrefix(server.URL, "http://"),
			"--insecure",
			"--folder", t.TempDir(),
			"--target-ref", "test_gitops",
		})
		require.NoError(t, command.Execute())

		raw, err := io.ReadAll(b)
		require.NoError(t, err)
		out := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(string(raw), "")
		assert.NotContains(t, out, "STATUS")
		assert.Regexp(t, `(?m)^\s*test\s+test-1\s+False ✅\s*$`, out)
		assert.Regexp(t, `(?m)^\s*prod\s+test-3\s+True 🔥\s*$`, out)
	})

	t.Run("target ref alone sends no target repos", func(t *testing.T) {
		server, wrapper := newSourcesTangleServer(t)

		command := NewRootCommand()
		command.SetOut(io.Discard)
		command.SetErr(io.Discard)
		command.SetArgs([]string{
			"generate-manifests",
			"--server-address", strings.TrimPrefix(server.URL, "http://"),
			"--insecure",
			"--folder", t.TempDir(),
			"--target-ref", "feature-x",
		})
		require.NoError(t, command.Execute())

		for _, request := range wrapper.Requests() {
			assert.Empty(t, request.TargetRepos)
			assert.False(t, request.InferTargetRepo)
		}
	})
}
