// External test package (not `package argocd`): argocdfakes imports argocd,
// so argocd's own in-package tests can't import argocdfakes without an
// import cycle. Every symbol used below is exported, so this needs no
// unexported access anyway.
package argocd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoServerApiClient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
)

// wrapperFixtureApplications scopes argocdfakes.ExampleApplications() down
// to test-1/test-2 — the "default" project subset a real ArgoCD server
// restricted the ARGOCD_TOKEN identity these tests authenticated as to,
// before this file moved off a live cluster. Keeping just that subset
// preserves every assertion below unchanged.
func wrapperFixtureApplications() []v1alpha1.Application {
	return argocdfakes.ExampleApplications()[:2]
}

// Test cases preserved from before this file moved to argocdfakes (byte-for-byte
// same subtests/assertions — only how the client dependency is provided changed):
//   - pool: labels{env=test} returns exactly test-1
//   - exclude: labels{foo=bar} minus excludeLabels{env=test} returns exactly test-2
//   - error: a client-level List failure propagates out of the wrapper
//   - get manifests from pool: a known application returns non-nil manifests
//   - not found getting manifests: an unknown application errors
func TestArgoCDWrapper(t *testing.T) {
	t.Run("pool", func(t *testing.T) {
		client := argocdfakes.NewFakeClient(wrapperFixtureApplications())

		wrapper, err := argocd.New(client, "test", &argocd.ArgoCDWrapperOptions{
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
		client := argocdfakes.NewFakeClient(wrapperFixtureApplications())

		wrapper, err := argocd.New(client, "test", &argocd.ArgoCDWrapperOptions{
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
		client := argocdfakes.NewFakeClient(wrapperFixtureApplications())
		client.ErrOnList = errors.New("simulated connection error")

		wrapper, err := argocd.New(client, "test", &argocd.ArgoCDWrapperOptions{
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
		client := argocdfakes.NewFakeClient(wrapperFixtureApplications())
		client.ManifestsByApp["test-1"] = &repoServerApiClient.ManifestResponse{
			Manifests: []string{"apiVersion: v1\nkind: ConfigMap\n"},
		}

		wrapper, err := argocd.New(client, "test", &argocd.ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.Nil(t, err)

		results, err := wrapper.GetManifests(context.Background(), "test-1", "main", "test_gitops")
		assert.Nil(t, err)
		assert.NotNil(t, results)
	})

	t.Run("not found getting manifests", func(t *testing.T) {
		client := argocdfakes.NewFakeClient(wrapperFixtureApplications())

		wrapper, err := argocd.New(client, "test", &argocd.ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: true,
		})
		assert.Nil(t, err)

		_, err = wrapper.GetManifests(context.Background(), "test-5", "main", "test_gitops")
		assert.NotNil(t, err)
	})
}
