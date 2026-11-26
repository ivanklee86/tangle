package argocd_test

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
)

// Test cases:
//   - every spelling of one repo (https, ssh, scp-style, .git, trailing slash,
//     credentials, port, upper-case host, surrounding whitespace) normalizes
//     to the same value
//   - a bare host:port/path is a port; scp syntax (user@host:path) never is
//   - the path keeps its case, and different repos stay different
//   - blank input normalizes to "", so callers can drop it
func TestNormalizeRepoURL(t *testing.T) {
	sameRepo := []string{
		"https://github.com/org/repo",
		"https://github.com/org/repo.git",
		"https://github.com/org/repo/",
		"https://github.com/org/repo.git/",
		"https://GitHub.com/org/repo",
		"https://user:token@github.com/org/repo.git",
		"http://github.com/org/repo",
		"ssh://git@github.com/org/repo.git",
		"ssh://git@github.com:22/org/repo.git",
		"git@github.com:org/repo.git",
		"git@github.com:org/repo",
		"github.com/org/repo",
		"github.com:443/org/repo",
		"  https://github.com/org/repo.git  ",
	}

	for _, spelling := range sameRepo {
		t.Run(spelling, func(t *testing.T) {
			assert.Equal(t, "github.com/org/repo", argocd.NormalizeRepoURL(spelling))
		})
	}

	t.Run("a bare host:port matches the https URL with that port", func(t *testing.T) {
		assert.Equal(t, argocd.NormalizeRepoURL("https://git.example.com:8443/org/repo.git"), argocd.NormalizeRepoURL("git.example.com:8443/org/repo"))
	})

	t.Run("scp syntax with a numeric first segment keeps it as path", func(t *testing.T) {
		assert.Equal(t, "git.example.com/2024/repo", argocd.NormalizeRepoURL("git@git.example.com:2024/repo.git"))
	})

	t.Run("path case is kept", func(t *testing.T) {
		assert.Equal(t, "github.com/Org/Repo", argocd.NormalizeRepoURL("https://github.com/Org/Repo.git"))
	})

	t.Run("different repos stay different", func(t *testing.T) {
		assert.NotEqual(t, argocd.NormalizeRepoURL("https://github.com/org/charts"), argocd.NormalizeRepoURL("https://github.com/org/values"))
		assert.NotEqual(t, argocd.NormalizeRepoURL("https://github.com/org/repo"), argocd.NormalizeRepoURL("https://gitlab.com/org/repo"))
	})

	t.Run("blank", func(t *testing.T) {
		assert.Equal(t, "", argocd.NormalizeRepoURL(""))
		assert.Equal(t, "", argocd.NormalizeRepoURL("   "))
	})
}

// Test cases:
//   - a single-source Application comes back as one source at position 1
//   - a multi-source Application keeps ArgoCD's order and 1-based positions,
//     with each source's kind: a git path, a git `ref` source, a Helm repo
//     chart and an OCI artifact
//   - an Application with no source at all returns nothing
func TestSourcesOf(t *testing.T) {
	t.Run("single source", func(t *testing.T) {
		app := &v1alpha1.Application{}
		app.Spec.Source = &v1alpha1.ApplicationSource{RepoURL: "https://github.com/org/app.git", Path: "deploy", TargetRevision: "main"}

		assert.Equal(t, []argocd.Source{
			{Position: 1, RepoURL: "https://github.com/org/app.git", Path: "deploy", TargetRevision: "main", Kind: argocd.SourceKindGit},
		}, argocd.SourcesOf(app))
	})

	t.Run("multi source", func(t *testing.T) {
		app := &v1alpha1.Application{}
		app.Spec.Sources = v1alpha1.ApplicationSources{
			{RepoURL: "https://stefanprodan.github.io/podinfo", Chart: "podinfo", TargetRevision: "6.17.0"},
			{RepoURL: "https://github.com/org/values.git", Ref: "values", TargetRevision: "main"},
			{RepoURL: "oci://ghcr.io/org/manifests", Path: ".", TargetRevision: "1.0.0"},
			{RepoURL: "https://github.com/org/app.git", Path: "overlays/prod", TargetRevision: "v1.2"},
		}

		assert.Equal(t, []argocd.Source{
			{Position: 1, RepoURL: "https://stefanprodan.github.io/podinfo", Chart: "podinfo", TargetRevision: "6.17.0", Kind: argocd.SourceKindHelm},
			{Position: 2, RepoURL: "https://github.com/org/values.git", Ref: "values", TargetRevision: "main", Kind: argocd.SourceKindGit},
			{Position: 3, RepoURL: "oci://ghcr.io/org/manifests", Path: ".", TargetRevision: "1.0.0", Kind: argocd.SourceKindOCI},
			{Position: 4, RepoURL: "https://github.com/org/app.git", Path: "overlays/prod", TargetRevision: "v1.2", Kind: argocd.SourceKindGit},
		}, argocd.SourcesOf(app))
	})

	t.Run("no source", func(t *testing.T) {
		assert.Nil(t, argocd.SourcesOf(&v1alpha1.Application{}))
		assert.Nil(t, argocd.SourcesOf(nil))
	})
}

// Test cases (which sources read files through a ref):
//   - a Helm valueFiles entry `$values/...` uses the source whose ref is
//     values
//   - a Helm fileParameters path counts too
//   - several users are listed in position order
//   - a ref nothing reads has no users, and a source without a ref has none
//   - a lookalike prefix (`$valuesx/`) doesn't count
func TestSourcesOfRefUsers(t *testing.T) {
	app := &v1alpha1.Application{}
	app.Spec.Sources = v1alpha1.ApplicationSources{
		{RepoURL: "https://charts.example.com", Chart: "web", TargetRevision: "1.0.0", Helm: &v1alpha1.ApplicationSourceHelm{
			ValueFiles: []string{"values.yaml", "$values/web/values.yaml"},
		}},
		{RepoURL: "https://github.com/org/values.git", Ref: "values", TargetRevision: "main"},
		{RepoURL: "https://github.com/org/chart.git", Path: "chart", TargetRevision: "main", Helm: &v1alpha1.ApplicationSourceHelm{
			FileParameters: []v1alpha1.HelmFileParameter{{Name: "config", Path: "$values/chart/config.json"}},
			ValueFiles:     []string{"$valuesx/other.yaml"},
		}},
		{RepoURL: "https://github.com/org/unused.git", Ref: "unused", TargetRevision: "main"},
	}

	sources := argocd.SourcesOf(app)

	assert.Nil(t, sources[0].UsedBy, "a source without a ref")
	assert.Equal(t, []int64{1, 3}, sources[1].UsedBy)
	assert.Nil(t, sources[2].UsedBy)
	assert.Nil(t, sources[3].UsedBy, "a ref nothing reads")
}
