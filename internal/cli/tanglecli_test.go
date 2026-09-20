package cli

import (
	"bytes"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
	"github.com/ivanklee86/tangle/internal/tangle"
)

// newFakeTangleServer builds a real *tangle.Tangle (so its actual router,
// including chi's Recoverer middleware, is exercised end to end) with fake
// ArgoCD wrappers injected, and serves it via httptest — pkg/client is a
// plain net/http client against a domain string, so an httptest.Server's
// address works identically to a real deployed tangle-server for everything
// TangleCLI.GenerateManifests exercises. test-3 is deliberately given a
// GetManifests error: on the real cluster its target-branch helm chart has a
// broken values.yaml, and the resulting response — 200 with only
// manifestGenerationError set — is what these tests were originally written
// against (see internal/tangle/server_e2e_test.go for why that response
// shape is what it is).
func newFakeTangleServer(t *testing.T) *httptest.Server {
	t.Helper()

	argocdConfig := make(map[string]tangle.TangleArgoCDConfig)
	argocdConfig["test"] = tangle.TangleArgoCDConfig{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	}
	argocdConfig["prod"] = tangle.TangleArgoCDConfig{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_PROD_TOKEN",
	}

	config := tangle.TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		ArgoCDs:         argocdConfig,
		DoNotInstrument: true,
	}

	realTangle := tangle.New(&config, "testing")

	testWrapper := argocdfakes.NewFakeWrapper([]argocdfakes.FakeApplication{
		{Name: "test-1", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "test", "foo": "bar", "bazz": "buzz"}, LiveRevision: "main"},
		{Name: "test-2", Project: "default", Namespace: "argocd", Labels: map[string]string{"env": "preprod", "foo": "bar", "bazz": "buzz"}, LiveRevision: "main"},
	})
	testWrapper.ManifestsByApp["test-1"] = &argocd.GetManifestsResponse{
		LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-1\n"},
		TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-1\ndata:\n  updated: \"true\"\n"},
	}
	testWrapper.ManifestsByApp["test-2"] = &argocd.GetManifestsResponse{
		LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-2\n"},
		TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-2\ndata:\n  updated: \"true\"\n"},
	}

	prodWrapper := argocdfakes.NewFakeWrapper([]argocdfakes.FakeApplication{
		{Name: "test-3", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "prod", "foo": "bar"}, LiveRevision: "main"},
		{Name: "test-4", Project: "my-project", Namespace: "argocd", Labels: map[string]string{"env": "infra", "foo": "bar"}, LiveRevision: "main"},
	})
	prodWrapper.ErrOnGetManifests["test-3"] = errors.New("rpc error: code = Unknown desc = failed to execute helm template command: broken values.yaml")
	prodWrapper.ManifestsByApp["test-4"] = &argocd.GetManifestsResponse{
		LiveManifests:   []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-4\n"},
		TargetManifests: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-4\ndata:\n  updated: \"true\"\n"},
	}

	realTangle.ArgoCDs = map[string]argocd.IArgoCDWrapper{
		"test": testWrapper,
		"prod": prodWrapper,
	}

	server := httptest.NewServer(realTangle.Server.Handler)
	t.Cleanup(server.Close)

	return server
}

func TestCharacterCount(t *testing.T) {
	testString := "key=value"

	t.Run("Can count ='s correctly", func(t *testing.T) {
		assert.Equal(t, countCharacterOccurrences(testString, '='), 1)
	})
}

func TestTangleCLIHappyPaths(t *testing.T) {
	server := newFakeTangleServer(t)

	config := Config{
		ServerAddr:      strings.TrimPrefix(server.URL, "http://"),
		Insecure:        true,
		LabelsAsStrings: []string{"env=test"},
	}

	b := bytes.NewBufferString("")

	tangleCLI := NewWithConfig(config)
	tangleCLI.Out = b
	tangleCLI.Err = b

	t.Run("tangle-cli creation", func(t *testing.T) {
		expectedMap := make(map[string]string)
		expectedMap["env"] = "test"
		assert.Equal(t, tangleCLI.Labels, expectedMap)
	})

	t.Run("tangle-cli happy path", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "tangle")
		assert.NoError(t, err)

		tangleCLI.Labels = make(map[string]string)
		tangleCLI.Folder = tempDir
		tangleCLI.TargetRef = "test_gitops"
		tangleCLI.GenerateManifests()

		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "error-prod-test-3.txt"))
	})

	t.Run("tangle-cli happy path with retries", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "tangle")
		assert.NoError(t, err)

		tangleCLI.Labels = make(map[string]string)
		tangleCLI.Folder = tempDir
		tangleCLI.TargetRef = "test_gitops"
		tangleCLI.Retries = 3
		tangleCLI.GenerateManifests()

		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "error-prod-test-3.txt"))
	})
}
