package argocd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgoCD(t *testing.T) {
	t.Run("pool", func(t *testing.T) {
		client := New(&ArgoCDClientOptions{})

		labels := make(map[string]string)
		labels["foo"] = "bar"
		labels["apple"] = "banana"

		results := client.ListApplicationsByLabels(labels)
		assert.Len(t, results, 2)
	})
}
