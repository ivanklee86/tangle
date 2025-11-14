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

		wrapper, err := New(client, "test", &ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.Nil(t, err)

		labels := make(map[string]string)
		labels["env"] = "test"
		excludeLabels := make(map[string]string)

		results, err := wrapper.ListApplicationsByLabels(context.Background(), labels, excludeLabels)
		assert.Nil(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "test-1", results[0].Name)
	})

	t.Run("exclude", func(t *testing.T) {
		client, err := NewArgoCDClient(&ArgoCDClientOptions{
			Address:         "localhost:8080",
			Insecure:        true,
			AuthTokenEnvVar: "ARGOCD_TOKEN",
		})
		assert.Nil(t, err)

		wrapper, err := New(client, "test", &ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.Nil(t, err)

		labels := make(map[string]string)
		labels["foo"] = "bar"
		excludeLabels := make(map[string]string)
		excludeLabels["env"] = "test"

		results, err := wrapper.ListApplicationsByLabels(context.Background(), labels, excludeLabels)
		assert.Nil(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "test-2", results[0].Name)
	})

	t.Run("error", func(t *testing.T) {
		client, err := NewArgoCDClient(&ArgoCDClientOptions{
			Address:         "https://localhost:8080",
			Insecure:        true,
			AuthTokenEnvVar: "ARGOCD_TOKEN",
		})
		assert.Nil(t, err)

		wrapper, err := New(client, "test", &ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.Nil(t, err)

		labels := make(map[string]string)
		labels["env"] = "test"
		excludeLabels := make(map[string]string)

		_, err = wrapper.ListApplicationsByLabels(context.Background(), labels, excludeLabels)
		assert.NotNil(t, err)
	})

	t.Run("get manifests from pool", func(t *testing.T) {
		client, err := NewArgoCDClient(&ArgoCDClientOptions{
			Address:         "localhost:8080",
			Insecure:        true,
			AuthTokenEnvVar: "ARGOCD_TOKEN",
		})
		assert.Nil(t, err)

		wrapper, err := New(client, "test", &ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.Nil(t, err)

		results, err := wrapper.GetManifests(context.Background(), "test-1", "main", "test_gitops")
		assert.Nil(t, err)
		assert.NotNil(t, results)
	})

	t.Run("not found getting manifests", func(t *testing.T) {
		client, err := NewArgoCDClient(&ArgoCDClientOptions{
			Address:         "localhost:8080",
			Insecure:        true,
			AuthTokenEnvVar: "ARGOCD_TOKEN",
		})
		assert.Nil(t, err)

		wrapper, err := New(client, "test", &ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.Nil(t, err)

		_, err = wrapper.GetManifests(context.Background(), "test-5", "main", "test_gitops")
		assert.NotNil(t, err)
	})

}
