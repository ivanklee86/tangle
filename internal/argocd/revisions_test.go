package argocd_test

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"

	"github.com/ivanklee86/tangle/internal/argocd"
)

const (
	appsRepo   = "https://github.com/org/apps.git"
	valuesRepo = "https://github.com/org/values.git"
	thirdRepo  = "https://github.com/org/third.git"
	helmRepo   = "https://stefanprodan.github.io/podinfo"
	targetRef  = "feature-x"
)

func gitSource(position int64, repo string, revision string) argocd.Source {
	return argocd.Source{Position: position, RepoURL: repo, Path: "deploy", TargetRevision: revision, Kind: argocd.SourceKindGit}
}

func helmSource(position int64) argocd.Source {
	return argocd.Source{Position: position, RepoURL: helmRepo, Chart: "podinfo", TargetRevision: "6.17.0", Kind: argocd.SourceKindHelm}
}

// moved reports, per position, whether the plan sends the target ref there.
func moved(plan argocd.RevisionPlan) []bool {
	result := []bool{}
	for _, source := range plan.Sources {
		result = append(result, source.Moved)
	}
	return result
}

// Test cases, named after the use-case rows in
// docs/agents/plans/2026-11-26-multi-source-applications.md:
//
// Single source:
//   - S1: no target repos, git source: the source gets the target ref
//   - S4: no target repos, Helm source: still sent the ref, as today (ArgoCD
//     then reports the bad chart version)
//   - S2: a target repo that matches, in another spelling: the source moves
//   - S3: a target repo that doesn't match: not affected, nothing moves
//   - S4b: a target repo and a Helm source: not affected
//   - no source at all: planned as today, a single-source render
//
// Multi source:
//   - M1: Helm chart + git values, target repo given: only the git source moves
//   - M1 inferred: same, with no target repo and inference on
//   - M1b: no target repo and inference off: needs a target repo
//   - M2: two sources in one repo: both move
//   - M3/M12: two repos, one given (alone or with an unused extra): one moves
//   - M4: two repos, none given, inference on: needs a target repo, both
//     repos listed in source order
//   - M5: a target repo none of the sources use: not affected
//   - M6: one repo at two revisions: both move, each keeps its live revision
//   - M10: three repos, none given: three candidates in source order
//   - M13: two of three repos given: those two positions move
//   - only Helm sources and no target repo: needs a target repo, with no
//     candidates, because there's no git source the ref could belong to
//
// Ref kinds and inputs:
//   - R: a tag and a full SHA pass through byte-for-byte
//   - repeated and differently-spelled target repos collapse to one; blanks
//     are dropped
//   - Positions, LiveRevisions and TargetRevisions line up per position
func TestPlanRevisions(t *testing.T) {
	t.Run("S1 single git source, no target repos", func(t *testing.T) {
		plan := argocd.PlanRevisions([]argocd.Source{gitSource(1, appsRepo, "main")}, targetRef, nil, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.False(t, plan.MultiSource)
		assert.Equal(t, []bool{true}, moved(plan))
		assert.Equal(t, []string{targetRef}, plan.TargetRevisions())
	})

	t.Run("S4 single Helm source, no target repos", func(t *testing.T) {
		plan := argocd.PlanRevisions([]argocd.Source{helmSource(1)}, targetRef, nil, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.Equal(t, []string{targetRef}, plan.TargetRevisions())
	})

	t.Run("S2 single source, matching target repo", func(t *testing.T) {
		plan := argocd.PlanRevisions([]argocd.Source{gitSource(1, appsRepo, "main")}, targetRef, []string{"git@github.com:org/apps"}, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.Equal(t, []bool{true}, moved(plan))
	})

	t.Run("S3 single source, other target repo", func(t *testing.T) {
		plan := argocd.PlanRevisions([]argocd.Source{gitSource(1, appsRepo, "main")}, targetRef, []string{valuesRepo}, false)

		assert.Equal(t, argocd.DiffStatusNotAffected, plan.Status)
		assert.Equal(t, []bool{false}, moved(plan))
		assert.Equal(t, []string{"main"}, plan.TargetRevisions())
	})

	t.Run("S4b single Helm source, target repo given", func(t *testing.T) {
		plan := argocd.PlanRevisions([]argocd.Source{helmSource(1)}, targetRef, []string{helmRepo}, false)

		assert.Equal(t, argocd.DiffStatusNotAffected, plan.Status)
	})

	t.Run("no source", func(t *testing.T) {
		plan := argocd.PlanRevisions(nil, targetRef, nil, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.False(t, plan.MultiSource)
		assert.Empty(t, plan.Sources)
	})

	helmWithValues := []argocd.Source{helmSource(1), {Position: 2, RepoURL: valuesRepo, Ref: "values", TargetRevision: "main", Kind: argocd.SourceKindGit}}

	t.Run("M1 Helm chart with git values, target repo given", func(t *testing.T) {
		plan := argocd.PlanRevisions(helmWithValues, targetRef, []string{valuesRepo}, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.True(t, plan.MultiSource)
		assert.False(t, plan.Inferred)
		assert.Equal(t, []bool{false, true}, moved(plan))
		assert.Equal(t, []string{"6.17.0", targetRef}, plan.TargetRevisions())
	})

	t.Run("M1 inferred", func(t *testing.T) {
		plan := argocd.PlanRevisions(helmWithValues, targetRef, nil, true)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.True(t, plan.Inferred)
		assert.Equal(t, []string{"github.com/org/values"}, plan.TargetRepos)
		assert.Equal(t, []bool{false, true}, moved(plan))
	})

	t.Run("M1b no target repo, inference off", func(t *testing.T) {
		plan := argocd.PlanRevisions(helmWithValues, targetRef, nil, false)

		assert.Equal(t, argocd.DiffStatusNeedsTargetRepo, plan.Status)
		assert.Equal(t, []string{"github.com/org/values"}, plan.CandidateRepos)
		assert.Equal(t, []bool{false, false}, moved(plan))
	})

	t.Run("M2 two sources in one repo", func(t *testing.T) {
		sources := []argocd.Source{gitSource(1, appsRepo, "main"), gitSource(2, appsRepo, "main")}
		plan := argocd.PlanRevisions(sources, targetRef, []string{appsRepo}, false)

		assert.Equal(t, []bool{true, true}, moved(plan))
	})

	twoRepos := []argocd.Source{gitSource(1, appsRepo, "main"), gitSource(2, valuesRepo, "main")}

	t.Run("M3 two repos, one given", func(t *testing.T) {
		plan := argocd.PlanRevisions(twoRepos, targetRef, []string{valuesRepo}, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.Equal(t, []bool{false, true}, moved(plan))
	})

	t.Run("M12 an extra target repo the app doesn't use", func(t *testing.T) {
		plan := argocd.PlanRevisions(twoRepos, targetRef, []string{valuesRepo, thirdRepo}, false)

		assert.Equal(t, []bool{false, true}, moved(plan))
	})

	t.Run("M4 two repos, none given, inference on", func(t *testing.T) {
		plan := argocd.PlanRevisions(twoRepos, targetRef, nil, true)

		assert.Equal(t, argocd.DiffStatusNeedsTargetRepo, plan.Status)
		assert.Equal(t, []string{"github.com/org/apps", "github.com/org/values"}, plan.CandidateRepos)
	})

	t.Run("M5 target repo matches no source", func(t *testing.T) {
		plan := argocd.PlanRevisions(twoRepos, targetRef, []string{thirdRepo}, false)

		assert.Equal(t, argocd.DiffStatusNotAffected, plan.Status)
		assert.Equal(t, []bool{false, false}, moved(plan))
	})

	t.Run("M6 one repo at two revisions", func(t *testing.T) {
		sources := []argocd.Source{gitSource(1, appsRepo, "main"), gitSource(2, appsRepo, "v1.2")}
		plan := argocd.PlanRevisions(sources, targetRef, []string{appsRepo}, false)

		assert.Equal(t, []string{"main", "v1.2"}, plan.LiveRevisions())
		assert.Equal(t, []string{targetRef, targetRef}, plan.TargetRevisions())
	})

	threeRepos := []argocd.Source{gitSource(1, appsRepo, "main"), gitSource(2, valuesRepo, "main"), gitSource(3, thirdRepo, "main")}

	t.Run("M10 three repos, none given", func(t *testing.T) {
		plan := argocd.PlanRevisions(threeRepos, targetRef, nil, false)

		assert.Equal(t, argocd.DiffStatusNeedsTargetRepo, plan.Status)
		assert.Equal(t, []string{"github.com/org/apps", "github.com/org/values", "github.com/org/third"}, plan.CandidateRepos)
	})

	t.Run("M13 two of three repos given", func(t *testing.T) {
		plan := argocd.PlanRevisions(threeRepos, targetRef, []string{thirdRepo, valuesRepo}, false)

		assert.Equal(t, []bool{false, true, true}, moved(plan))
	})

	t.Run("only Helm sources, no target repo", func(t *testing.T) {
		plan := argocd.PlanRevisions([]argocd.Source{helmSource(1), helmSource(2)}, targetRef, nil, true)

		assert.Equal(t, argocd.DiffStatusNeedsTargetRepo, plan.Status)
		assert.Empty(t, plan.CandidateRepos)
	})

	t.Run("R tag and SHA pass through unchanged", func(t *testing.T) {
		for _, ref := range []string{"fixture-target", "3f9c2a17be04d1c8e6a5f0b9d27c4e81a3b6f5d0", "3f9c2a1"} {
			single := argocd.PlanRevisions([]argocd.Source{gitSource(1, appsRepo, "main")}, ref, nil, false)
			multi := argocd.PlanRevisions(twoRepos, ref, []string{valuesRepo}, false)

			assert.Equal(t, []string{ref}, single.TargetRevisions())
			assert.Equal(t, []string{"main", ref}, multi.TargetRevisions())
		}
	})

	t.Run("target repos are deduplicated and blanks dropped", func(t *testing.T) {
		plan := argocd.PlanRevisions(twoRepos, targetRef, []string{valuesRepo, "", "git@github.com:org/values", "https://github.com/org/values/"}, false)

		assert.Equal(t, []string{"github.com/org/values"}, plan.TargetRepos)
	})

	t.Run("blank-only target repos count as none", func(t *testing.T) {
		plan := argocd.PlanRevisions([]argocd.Source{gitSource(1, appsRepo, "main")}, targetRef, []string{"", " "}, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.Equal(t, []bool{true}, moved(plan))
	})

	t.Run("positions and revisions line up", func(t *testing.T) {
		plan := argocd.PlanRevisions(threeRepos, targetRef, []string{valuesRepo}, false)

		assert.Equal(t, []int64{1, 2, 3}, plan.Positions())
		assert.Equal(t, []string{"main", "main", "main"}, plan.LiveRevisions())
		assert.Equal(t, []string{"main", targetRef, "main"}, plan.TargetRevisions())
	})
}

// Test cases (choosing sources goes by count, rendering by how the spec is
// written):
//   - spec.source renders with Revision
//   - a one-entry spec.sources is chosen like a single source (no target repo
//     needed) but renders positionally, because ArgoCD ignores Revision for
//     any spec.sources
//   - a longer spec.sources renders positionally, as before
func TestPlanApplication(t *testing.T) {
	t.Run("spec.source", func(t *testing.T) {
		app := &v1alpha1.Application{}
		app.Spec.Source = &v1alpha1.ApplicationSource{RepoURL: appsRepo, Path: "deploy", TargetRevision: "main"}

		plan := argocd.PlanApplication(app, targetRef, nil, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.False(t, plan.Positional)
	})

	t.Run("one-entry spec.sources", func(t *testing.T) {
		app := &v1alpha1.Application{}
		app.Spec.Sources = v1alpha1.ApplicationSources{{RepoURL: appsRepo, Path: "deploy", TargetRevision: "main"}}

		plan := argocd.PlanApplication(app, targetRef, nil, false)

		assert.Equal(t, argocd.DiffStatusOK, plan.Status)
		assert.False(t, plan.MultiSource)
		assert.True(t, plan.Positional)
		assert.Equal(t, []int64{1}, plan.Positions())
		assert.Equal(t, []string{"main"}, plan.LiveRevisions())
		assert.Equal(t, []string{targetRef}, plan.TargetRevisions())
	})

	t.Run("longer spec.sources", func(t *testing.T) {
		app := &v1alpha1.Application{}
		app.Spec.Sources = v1alpha1.ApplicationSources{
			{RepoURL: appsRepo, Path: "deploy", TargetRevision: "main"},
			{RepoURL: valuesRepo, Path: "values", TargetRevision: "main"},
		}

		plan := argocd.PlanApplication(app, targetRef, []string{valuesRepo}, false)

		assert.True(t, plan.MultiSource)
		assert.True(t, plan.Positional)
	})
}
