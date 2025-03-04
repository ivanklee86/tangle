package main

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

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
		b := bytes.NewBufferString("")
		tempDir, err := os.MkdirTemp("", "tangle")
		assert.NoError(t, err)

		command := NewRootCommand()
		command.SetOut(b)
		command.SetErr(b)
		command.SetArgs([]string{
			"generate-manifests",
			"--server-address", "localhost:8081",
			"--insecure",
			"--folder", tempDir,
			"--target-ref", "test_gitops",
		})
		err = command.Execute()
		assert.NoError(t, err)

		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "diff-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-1.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-test-test-2.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "manifests-prod-test-3.yaml"))
		assert.FileExists(t, fmt.Sprintf("%s/%s", tempDir, "error-prod-test-3.txt"))

		out, err := io.ReadAll(b)
		assert.NoError(t, err)
		assert.Contains(t, string(out), "Applications found: 4")
		assert.Contains(t, string(out), "True")
	})
}
