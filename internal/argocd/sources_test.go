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
		"  https://github.com/org/repo.git  ",
	}

	for _, spelling := range sameRepo {
		t.Run(spelling, func(t *testing.T) {
			assert.Equal(t, "github.com/org/repo", argocd.NormalizeRepoURL(spelling))
		})
	}

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
