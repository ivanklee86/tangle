package argocd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoServerApiClient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanklee86/tangle/internal/argocd"
	"github.com/ivanklee86/tangle/internal/argocd/argocdfakes"
)

func singleSourceApp() v1alpha1.Application {
	app := v1alpha1.Application{}
	app.Name = "single"
	app.Spec.Source = &v1alpha1.ApplicationSource{RepoURL: appsRepo, Path: "deploy", TargetRevision: "main"}
	return app
}

func twoRepoApp() v1alpha1.Application {
	app := v1alpha1.Application{}
	app.Name = "two-repos"
	app.Spec.Sources = v1alpha1.ApplicationSources{
		{RepoURL: appsRepo, Path: "deploy", TargetRevision: "v1.2"},
		{RepoURL: valuesRepo, Path: "values", TargetRevision: "main"},
	}
	return app
}

func newSourcesWrapper(t *testing.T, apps ...v1alpha1.Application) (argocd.IArgoCDWrapper, *argocdfakes.FakeClient) {
	t.Helper()

	client := argocdfakes.NewFakeClient(apps)
	client.ManifestsFunc = func(*application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error) {
		return &repoServerApiClient.ManifestResponse{Manifests: []string{`{"kind":"ConfigMap"}`}}, nil
	}

	wrapper, err := argocd.New(client, "test", &argocd.ArgoCDWrapperOptions{DoNotInstrumentWorkers: true})
	require.NoError(t, err)

	return wrapper, client
}

// hardRefreshes counts the Get calls that asked ArgoCD for a hard refresh.
func hardRefreshes(client *argocdfakes.FakeClient) int {
	count := 0
	for _, query := range client.RecordedGets() {
		if query.Refresh != nil && *query.Refresh == "hard" {
			count++
		}
	}
	return count
}

// Test cases (what reaches ArgoCD for each kind of diff):
//   - single source, no target repos: Revision on each side, exactly as
//     before, with no Revisions or SourcePositions (S1 compatibility)
//   - multi source, target repo given: Revisions and every SourcePosition on
//     both sides, the live side at each source's own revision and the target
//     side with only the matching source moved (M3)
//   - not affected and needs target repo: answered from a plain read, with no
//     hard refresh and no manifest requests (S3, M4)
//   - a failed read propagates as an error
//   - listing returns each application's sources
func TestArgoCDWrapperSources(t *testing.T) {
	t.Run("single source sends Revision as before", func(t *testing.T) {
		wrapper, client := newSourcesWrapper(t, singleSourceApp())

		response, err := wrapper.GetManifests(context.Background(), argocd.ManifestsRequest{ApplicationName: "single", LiveRef: "main", TargetRef: targetRef})
		require.NoError(t, err)

		assert.Equal(t, argocd.DiffStatusOK, response.Plan.Status)
		queries := client.RecordedManifestQueries()
		require.Len(t, queries, 2)
		revisions := []string{*queries[0].Revision, *queries[1].Revision}
		assert.ElementsMatch(t, []string{"main", targetRef}, revisions)
		for _, query := range queries {
			assert.Empty(t, query.Revisions)
			assert.Empty(t, query.SourcePositions)
		}
		assert.Equal(t, 1, hardRefreshes(client))
	})

	t.Run("multi source sends Revisions for every position", func(t *testing.T) {
		wrapper, client := newSourcesWrapper(t, twoRepoApp())

		response, err := wrapper.GetManifests(context.Background(), argocd.ManifestsRequest{ApplicationName: "two-repos", TargetRef: targetRef, TargetRepos: []string{"git@github.com:org/values.git"}})
		require.NoError(t, err)

		assert.Equal(t, argocd.DiffStatusOK, response.Plan.Status)
		queries := client.RecordedManifestQueries()
		require.Len(t, queries, 2)
		sent := [][]string{queries[0].Revisions, queries[1].Revisions}
		assert.ElementsMatch(t, [][]string{{"v1.2", "main"}, {"v1.2", targetRef}}, sent)
		for _, query := range queries {
			assert.Nil(t, query.Revision)
			assert.Equal(t, []int64{1, 2}, query.SourcePositions)
		}
	})

	t.Run("not affected skips the hard refresh and the render", func(t *testing.T) {
		wrapper, client := newSourcesWrapper(t, singleSourceApp())

		response, err := wrapper.GetManifests(context.Background(), argocd.ManifestsRequest{ApplicationName: "single", LiveRef: "main", TargetRef: targetRef, TargetRepos: []string{valuesRepo}})
		require.NoError(t, err)

		assert.Equal(t, argocd.DiffStatusNotAffected, response.Plan.Status)
		assert.Empty(t, response.LiveManifests)
		assert.Empty(t, response.TargetManifests)
		assert.Zero(t, hardRefreshes(client))
		assert.Empty(t, client.RecordedManifestQueries())
	})

	t.Run("needs target repo skips the hard refresh and the render", func(t *testing.T) {
		wrapper, client := newSourcesWrapper(t, twoRepoApp())

		response, err := wrapper.GetManifests(context.Background(), argocd.ManifestsRequest{ApplicationName: "two-repos", TargetRef: targetRef, InferTargetRepo: true})
		require.NoError(t, err)

		assert.Equal(t, argocd.DiffStatusNeedsTargetRepo, response.Plan.Status)
		assert.Equal(t, []string{"github.com/org/apps", "github.com/org/values"}, response.Plan.CandidateRepos)
		assert.Zero(t, hardRefreshes(client))
		assert.Empty(t, client.RecordedManifestQueries())
	})

	t.Run("a failed read is an error", func(t *testing.T) {
		wrapper, client := newSourcesWrapper(t, singleSourceApp())
		client.ErrOnGet["single"] = errors.New("simulated read error")

		_, err := wrapper.GetManifests(context.Background(), argocd.ManifestsRequest{ApplicationName: "single", LiveRef: "main", TargetRef: targetRef})
		assert.Error(t, err)
	})

	t.Run("listing returns sources", func(t *testing.T) {
		wrapper, _ := newSourcesWrapper(t, singleSourceApp(), twoRepoApp())

		results, err := wrapper.ListApplicationsByLabels(context.Background(), map[string]string{}, map[string]string{})
		require.NoError(t, err)

		byName := map[string]argocd.ListApplicationsResult{}
		for _, result := range results {
			byName[result.Name] = result
		}
		assert.Equal(t, "main", byName["single"].LiveRevision)
		assert.Len(t, byName["single"].Sources, 1)
		assert.Equal(t, "", byName["two-repos"].LiveRevision)
		assert.Len(t, byName["two-repos"].Sources, 2)
		assert.Equal(t, "v1.2", byName["two-repos"].Sources[0].TargetRevision)
	})
}
