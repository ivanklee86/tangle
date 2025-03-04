package cli

import (
	"bytes"
	"io"
	"testing"

	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/assert"
)

func TestOutputs(t *testing.T) {
	config := Config{
		ServerAddr:      "localhost:8081",
		Insecure:        true,
		LabelsAsStrings: []string{"env=test"},
	}

	b := bytes.NewBufferString("")

	tangleCLi := NewWithConfig(config)
	tangleCLi.Out = b
	tangleCLi.Err = b

	testPhrase := "I'm a little hamster."

	t.Run("outputs string", func(t *testing.T) {
		tangleCLi.Output(testPhrase)

		out, err := io.ReadAll(b)
		assert.Nil(t, err)
		assert.Equal(t, testPhrase+"\n", string(out))
	})

	t.Run("outputs header", func(t *testing.T) {
		tangleCLi.OutputHeading(testPhrase)

		out, err := io.ReadAll(b)
		assert.Nil(t, err)
		assert.Contains(t, stripansi.Strip(string(out)), testPhrase)
	})
}
