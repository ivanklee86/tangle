package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
	"github.com/ivanklee86/tangle/internal/tangle"
)

// newFakeTangleServer mirrors internal/cli/tanglecli_test.go's helper of the
// same name — duplicated rather than shared because this file's package
// (main) and internal/cli are different packages, and the helper is small
// enough that a shared test-support package isn't worth it. See that file's
// comment for why test-3 is given a GetManifests error and what response
// shape that produces.
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

func TestCli(t *testing.T) {
	t.Run("root Command", func(t *testing.T) {
		b := bytes.NewBufferString("")

		command := NewRootCommand()
		command.SetOut(b)
		err := command.Execute()
		if err != nil {
			t.Fatal(err)
		}

		out, err := io.ReadAll(b)
		if err != nil {
			t.Fatal(err)
		}

		assert.Contains(t, string(out), "tangle-cli")
	})

	t.Run("generate manifests", func(t *testing.T) {
		server := newFakeTangleServer(t)

		b := bytes.NewBufferString("")
		tempDir, err := os.MkdirTemp("", "tangle")
		assert.NoError(t, err)

		command := NewRootCommand()
		command.SetOut(b)
		command.SetErr(b)
		command.SetArgs([]string{
			"generate-manifests",
			"--server-address", strings.TrimPrefix(server.URL, "http://"),
			"--insecure",
			"--folder", tempDir,
			"--target-ref", "test_gitops",
		})
		err = command.Execute()
		assert.NoError(t, err)

		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-prod-test-4.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-prod-test-4.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "error-prod-test-3.txt"))

		out, err := io.ReadAll(b)
		assert.NoError(t, err)
		assert.Contains(t, string(out), "Applications found: 4")
		assert.Contains(t, string(out), "True")
	})

	// --folder's help text promises "Defaults to current folder". CI jobs
	// commonly omit it, and it used to resolve to "" and write to the root.
	t.Run("generate manifests without --folder writes to the working directory", func(t *testing.T) {
		server := newFakeTangleServer(t)

		tempDir := t.TempDir()
		t.Chdir(tempDir)

		b := bytes.NewBufferString("")
		command := NewRootCommand()
		command.SetOut(b)
		command.SetErr(b)
		command.SetArgs([]string{
			"generate-manifests",
			"--server-address", strings.TrimPrefix(server.URL, "http://"),
			"--insecure",
			"--target-ref", "test_gitops",
		})
		assert.NoError(t, command.Execute())

		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "error-prod-test-3.txt"))
	})
}
