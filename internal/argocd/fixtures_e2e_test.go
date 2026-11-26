//go:build e2e

package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_SourceFixtures checks the source-type fixture Applications
// (integration/kubernetes/sources/) against ArgoCD directly, not through
// Tangle. It proves each fixture renders the way the README of
// github.com/ivanklee86/tangle-integration-manifests says it does, at main
// and at test_gitops, so a Tangle test built on a fixture can trust it.
//
// It also records how ArgoCD's GetManifests picks revisions, which Tangle has
// to follow: a single-source app takes Revision, while a multi-source app
// ignores Revision and takes Revisions paired with 1-based SourcePositions.

const fixtureTargetRef = "test_gitops"

type renderedResources map[string]map[string]any

// renderFixture asks the sources ArgoCD for an application's manifests, with
// the query adjusted by revise (nil renders the live revisions).
func renderFixture(t *testing.T, applicationName string, revise func(*application.ApplicationManifestQuery)) (renderedResources, error) {
	t.Helper()

	client, err := NewArgoCDClient(&ArgoCDClientOptions{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_SOURCES_TOKEN",
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	query := &application.ApplicationManifestQuery{Name: &applicationName}
	if revise != nil {
		revise(query)
	}

	response, err := client.GetApplicationManifests(context.Background(), query)
	if err != nil {
		return nil, err
	}

	resources := renderedResources{}
	for _, manifest := range response.Manifests {
		resource := map[string]any{}
		require.NoError(t, json.Unmarshal([]byte(manifest), &resource))
		metadata, _ := resource["metadata"].(map[string]any)
		resources[fmt.Sprintf("%v/%v", resource["kind"], metadata["name"])] = resource
	}

	return resources, nil
}

func atRevision(revision string) func(*application.ApplicationManifestQuery) {
	return func(query *application.ApplicationManifestQuery) {
		query.Revision = &revision
	}
}

// atSourceRevisions sets the same revision on each 1-based source position.
func atSourceRevisions(revision string, positions ...int64) func(*application.ApplicationManifestQuery) {
	return func(query *application.ApplicationManifestQuery) {
		query.SourcePositions = positions
		for range positions {
			query.Revisions = append(query.Revisions, revision)
		}
	}
}

func (r renderedResources) configMapData(t *testing.T, name string) map[string]any {
	t.Helper()

	resource, found := r["ConfigMap/"+name]
	require.True(t, found, "expected ConfigMap %s in %v", name, r.keys())
	data, _ := resource["data"].(map[string]any)

	return data
}

// onlyReplicas returns spec.replicas of the one Deployment rendered.
func (r renderedResources) onlyReplicas(t *testing.T) float64 {
	t.Helper()

	var replicas []float64
	for _, resource := range r {
		if resource["kind"] == "Deployment" {
			spec, _ := resource["spec"].(map[string]any)
			value, _ := spec["replicas"].(float64)
			replicas = append(replicas, value)
		}
	}
	require.Len(t, replicas, 1, "expected one Deployment in %v", r.keys())

	return replicas[0]
}

func (r renderedResources) keys() []string {
	keys := []string{}
	for key := range r {
		keys = append(keys, key)
	}

	return keys
}

func TestE2E_SourceFixtures(t *testing.T) {
	setup(t)

	// Rendered by the Tanka config management plugin sidecar, at either ref.
	t.Run("tanka", func(t *testing.T) {
		live, err := renderFixture(t, "tanka", nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"greeting": "hello", "removed": "soon"}, live.configMapData(t, "tanka-fixture"))

		target, err := renderFixture(t, "tanka", atRevision(fixtureTargetRef))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"greeting": "hello, world"}, target.configMapData(t, "tanka-fixture"))
	})

	// A chart from a Helm repo: its revision is a chart version, so a git
	// branch name can't render it.
	t.Run("helm-repo", func(t *testing.T) {
		live, err := renderFixture(t, "helm-repo", nil)
		require.NoError(t, err)
		assert.Equal(t, float64(1), live.onlyReplicas(t))

		_, err = renderFixture(t, "helm-repo", atRevision(fixtureTargetRef))
		assert.Error(t, err)
	})

	t.Run("helm-repo-git-values", func(t *testing.T) {
		live, err := renderFixture(t, "helm-repo-git-values", nil)
		require.NoError(t, err)
		assert.Equal(t, float64(1), live.onlyReplicas(t))

		// The change is in the ref-only values source (position 2).
		target, err := renderFixture(t, "helm-repo-git-values", atSourceRevisions(fixtureTargetRef, 2))
		require.NoError(t, err)
		assert.Equal(t, float64(2), target.onlyReplicas(t))

		// Revision alone is ignored for a multi-source app: ArgoCD renders
		// the live revisions and reports no error.
		ignored, err := renderFixture(t, "helm-repo-git-values", atRevision(fixtureTargetRef))
		require.NoError(t, err)
		assert.Equal(t, live, ignored)
	})

	t.Run("git-chart-git-values", func(t *testing.T) {
		live, err := renderFixture(t, "git-chart-git-values", nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"greeting": "hello"}, live.configMapData(t, "git-chart-fixture"))
		assert.Equal(t, map[string]any{"extra": "a"}, live.configMapData(t, "git-chart-extra-fixture"))

		// Both sources are this repo, so both take the pushed ref.
		target, err := renderFixture(t, "git-chart-git-values", atSourceRevisions(fixtureTargetRef, 1, 2))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"greeting": "hello, world"}, target.configMapData(t, "git-chart-fixture"))
		assert.Equal(t, map[string]any{"extra": "b"}, target.configMapData(t, "git-chart-extra-fixture"))
	})

	t.Run("two-repos", func(t *testing.T) {
		live, err := renderFixture(t, "two-repos", nil)
		require.NoError(t, err)
		assert.Contains(t, live, "Deployment/podinfo")
		assert.Equal(t, map[string]any{"greeting": "hello"}, live.configMapData(t, "two-repos-fixture"))

		// Only this repo's source (position 2) has the branch.
		target, err := renderFixture(t, "two-repos", atSourceRevisions(fixtureTargetRef, 2))
		require.NoError(t, err)
		assert.Equal(t, live["Deployment/podinfo"], target["Deployment/podinfo"])
		assert.Equal(t, map[string]any{"greeting": "hello, world"}, target.configMapData(t, "two-repos-fixture"))

		// Sending the pushed ref to every source fails: stefanprodan/podinfo
		// has no test_gitops branch.
		_, err = renderFixture(t, "two-repos", atSourceRevisions(fixtureTargetRef, 1, 2))
		assert.Error(t, err)
	})
}
