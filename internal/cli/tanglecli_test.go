package cli

import (
	"bytes"
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
		tangleCLI.GenerateManifests()
	})
}
