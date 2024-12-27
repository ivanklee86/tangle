package argocd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgoCDWrapper(t *testing.T) {
	setup(t)

	t.Run("pool", func(t *testing.T) {
		client, err := NewArgoCDClient(&ArgoCDClientOptions{
			Address:         "localhost:8080",
			Insecure:        true,
			AuthTokenEnvVar: "ARGOCD_TOKEN",
		})
		assert.Nil(t, err)

		wrapper, err := New(client, "test", &ArgoCDWrapperOptions{})
		assert.Nil(t, err)

		labels := make(map[string]string)
		labels["env"] = "test"

		results := wrapper.ListApplicationsByLabels(context.Background(), labels)
		assert.Len(t, results, 1)
		assert.Equal(t, "test-1", results[0].Name)
	})
}
