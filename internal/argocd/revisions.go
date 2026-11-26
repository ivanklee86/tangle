package argocd

import (
	"slices"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// DiffStatus is the outcome of planning a diff, before anything is rendered.
type DiffStatus string

const (
	// DiffStatusOK means there's something to render.
	DiffStatusOK DiffStatus = "ok"
	// DiffStatusNotAffected means target repos were given and none of the
	// Application's sources are in them, so the change can't touch it.
	DiffStatusNotAffected DiffStatus = "notAffected"
	// DiffStatusNeedsTargetRepo means the Application has several sources and
	// Tangle can't tell which of them the target ref belongs to.
	DiffStatusNeedsTargetRepo DiffStatus = "needsTargetRepo"
)

// PlannedSource is one source with the revision each side of the diff uses.
type PlannedSource struct {
	Source
	LiveRevision   string
	TargetRevision string
	// Moved is true when this source gets the target ref.
	Moved bool
}

// RevisionPlan says how to render both sides of a diff (ADR 0032).
type RevisionPlan struct {
	Status DiffStatus
	// MultiSource is true when the Application has more than one source, so
	// choosing which of them get the target ref needs a target repo.
	MultiSource bool
	// Positional plans send Revisions/SourcePositions for every position on
	// both sides; the others send Revision, exactly as before. It follows how
	// the spec is written, not how many sources it has: ArgoCD ignores
	// Revision for any Application with spec.sources, even a one-entry list.
	Positional bool
	Sources    []PlannedSource
	// TargetRepos are the normalized repos the target ref was applied to:
	// the caller's list, or the one inferred.
	TargetRepos []string
	Inferred    bool
	// CandidateRepos lists the app's git repos, normalized and in source
	// order, when Status is DiffStatusNeedsTargetRepo.
	CandidateRepos []string
}

// PlanRevisions decides which sources of an Application get the target ref.
//
//   - Every source whose repo matches any of targetRepos gets targetRef; every
//     other source stays at its own targetRevision.
//   - With no targetRepos, a single-source Application behaves as it always
//     has: its one source gets targetRef, whatever kind of source it is.
//   - With no targetRepos, a multi-source Application whose git sources are
//     all in one repo uses that repo only when infer is set. Otherwise it
//     needs a target repo.
//   - With targetRepos, an Application none of whose sources match is not
//     affected. Helm and OCI sources never match.
//
// targetRef is passed through untouched, so a branch, a tag and a SHA all plan
// the same way.
func PlanRevisions(sources []Source, targetRef string, targetRepos []string, infer bool) RevisionPlan {
	wanted := normalizeAll(targetRepos)

	if len(sources) <= 1 {
		return planSingleSource(sources, targetRef, wanted)
	}

	plan := RevisionPlan{MultiSource: true, Positional: true}
	if len(wanted) == 0 {
		candidates := gitRepos(sources)
		if !infer || len(candidates) != 1 {
			plan.Status = DiffStatusNeedsTargetRepo
			plan.CandidateRepos = candidates
			plan.Sources = unmoved(sources)
			return plan
		}
		wanted = candidates
		plan.Inferred = true
	}

	plan.TargetRepos = wanted
	plan.Sources = make([]PlannedSource, 0, len(sources))
	moved := 0
	for _, source := range sources {
		planned := PlannedSource{Source: source, LiveRevision: source.TargetRevision, TargetRevision: source.TargetRevision}
		if matches(source, wanted) {
			planned.TargetRevision = targetRef
			planned.Moved = true
			moved++
		}
		plan.Sources = append(plan.Sources, planned)
	}

	plan.Status = DiffStatusOK
	if moved == 0 {
		plan.Status = DiffStatusNotAffected
	}

	return plan
}

// PlanApplication plans an Application's diff. Choosing sources goes by
// PlanRevisions; rendering goes by the spec, so an Application written with
// spec.sources is rendered positionally even when the list has one entry.
func PlanApplication(app *v1alpha1.Application, targetRef string, targetRepos []string, infer bool) RevisionPlan {
	plan := PlanRevisions(SourcesOf(app), targetRef, targetRepos, infer)
	if app != nil && app.Spec.HasMultipleSources() {
		plan.Positional = true
	}
	return plan
}

func planSingleSource(sources []Source, targetRef string, wanted []string) RevisionPlan {
	plan := RevisionPlan{Status: DiffStatusOK, TargetRepos: wanted}
	if len(sources) == 0 {
		return plan
	}

	source := sources[0]
	planned := PlannedSource{Source: source, LiveRevision: source.TargetRevision, TargetRevision: targetRef, Moved: true}
	if len(wanted) > 0 && !matches(source, wanted) {
		plan.Status = DiffStatusNotAffected
		planned.TargetRevision = source.TargetRevision
		planned.Moved = false
	}
	plan.Sources = []PlannedSource{planned}

	return plan
}

// Positions returns every source position, in order, for SourcePositions.
func (p RevisionPlan) Positions() []int64 {
	positions := make([]int64, 0, len(p.Sources))
	for _, source := range p.Sources {
		positions = append(positions, source.Position)
	}
	return positions
}

// LiveRevisions returns each position's live revision, matching Positions.
func (p RevisionPlan) LiveRevisions() []string {
	revisions := make([]string, 0, len(p.Sources))
	for _, source := range p.Sources {
		revisions = append(revisions, source.LiveRevision)
	}
	return revisions
}

// TargetRevisions returns each position's target revision, matching Positions.
func (p RevisionPlan) TargetRevisions() []string {
	revisions := make([]string, 0, len(p.Sources))
	for _, source := range p.Sources {
		revisions = append(revisions, source.TargetRevision)
	}
	return revisions
}

func matches(source Source, wanted []string) bool {
	return source.Kind == SourceKindGit && slices.Contains(wanted, NormalizeRepoURL(source.RepoURL))
}

// gitRepos returns the distinct git repos of sources, normalized, in source
// order.
func gitRepos(sources []Source) []string {
	repos := []string{}
	for _, source := range sources {
		if source.Kind != SourceKindGit {
			continue
		}
		repo := NormalizeRepoURL(source.RepoURL)
		if !slices.Contains(repos, repo) {
			repos = append(repos, repo)
		}
	}
	return repos
}

// normalizeAll normalizes repos, drops blanks and collapses spellings of the
// same repo, keeping the caller's order.
func normalizeAll(repos []string) []string {
	normalized := []string{}
	for _, repo := range repos {
		n := NormalizeRepoURL(repo)
		if n != "" && !slices.Contains(normalized, n) {
			normalized = append(normalized, n)
		}
	}
	return normalized
}

func unmoved(sources []Source) []PlannedSource {
	planned := make([]PlannedSource, 0, len(sources))
	for _, source := range sources {
		planned = append(planned, PlannedSource{Source: source, LiveRevision: source.TargetRevision, TargetRevision: source.TargetRevision})
	}
	return planned
}
