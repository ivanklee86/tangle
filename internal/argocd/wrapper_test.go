package argocd

import (
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

		wrapper, err := New(&client, &ArgoCDWrapperOptions{})
		assert.Nil(t, err)

		labels := make(map[string]string)
		labels["foo"] = "bar"
		labels["apple"] = "banana"

		results := wrapper.ListApplicationsByLabels(labels)
		assert.Len(t, results, 2)
	})
}
