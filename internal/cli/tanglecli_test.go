package cli

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCharacterCount(t *testing.T) {
	testString := "key=value"

	t.Run("Can count ='s correctly", func(t *testing.T) {
		assert.Equal(t, countCharacterOccurrences(testString, '='), 1)
	})
}

func TestTangleCLIHappyPaths(t *testing.T) {
	config := Config{
		ServerAddr:      "localhost:8081",
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
		assert.Equal(t, tangleCLI.Config.Labels, expectedMap)
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
