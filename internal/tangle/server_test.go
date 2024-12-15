package tangle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	config := TangleConfig{
		Name:                   "test-tangle",
		Domain:                 "localhost",
		Port:                   8081,
		DoNotInstrumentWorkers: true,
	}

	t.Run("Basic test", func(t *testing.T) {
		tangle := New(&config)

		assert.NotNil(t, tangle.ArgoCDs)
	})
}
