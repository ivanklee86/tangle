---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Restrict argo-cd's Go module updates to major-only

## Context and Problem Statement

[ADR 0013](0013-renovate-weekly-grouped-updates.md) grouped every `go` datasource dependency (the `gomod` manager plus the `go install` custom managers) into one weekly `go` PR, with minor/patch automerged and majors held for manual review. In practice, whenever `github.com/argoproj/argo-cd/v3` — the repo's one direct dependency on the full Argo CD codebase, which itself requires most of the Kubernetes ecosystem (`k8s.io/kubernetes`, its staging modules, `gitops-engine`, etc.) — has even a minor or patch release, Renovate's `go get`/`go mod tidy` update step recalculates the whole transitive graph to satisfy argo-cd's new constraints, not just argo-cd's own version line in `go.mod`. Go module indirect dependencies are already excluded from Renovate's own update proposals by default (`matchDepTypes: ["indirect"]` is disabled out of the box), so this isn't Renovate opening separate PRs for those transitive modules — it's `go mod tidy` mechanically sweeping dozens of indirect version bumps into the same commit as a routine argo-cd minor/patch bump, because argo-cd's own upstream `go.mod` shifts its dependency tree on every release. The result is a weekly `go` PR whose diff size is dominated by argo-cd churn regardless of which direct dependency actually triggered that week's update, making the PR expensive to review even though it's nominally "just a minor bump."

## Decision Drivers

- The huge diffs are a direct, mechanical consequence of `go mod tidy` reconciling argo-cd's transitive graph, not of Renovate mismanaging indirect dependencies — no existing Renovate default addresses this, since indirect deps are already excluded from independent proposals.
- Other direct Go dependencies in this repo (`go-chi/chi`, `spf13/cobra`, `spf13/viper`, `stretchr/testify`, etc.) have small, shallow trees and don't reproduce this problem; narrowing the fix to argo-cd specifically avoids losing minor/patch (including security-patch) automerge for everything else in the `go` group.
- Major-version review is already the norm for this dependency (`matchUpdateTypes: ["major"]` → `automerge: false`, repo-wide per ADR 0013); routing argo-cd's minor/patch bumps to the same manual-major-only path is a narrow extension of an existing pattern, not a new one.

## Considered Options

- **Disable minor/patch for all direct `gomod` dependencies** (rejected — solves the problem but also stops automatic minor/patch PRs, including security patches, for ~24 direct dependencies that don't have argo-cd's transitive footprint and aren't part of the problem being solved).
- **Scope the major-only restriction to `github.com/argoproj/argo-cd/v3` (and, implicitly, its already-indirect satellites `gitops-engine`/`pkg/v2`, which Renovate doesn't propose updates for on their own regardless) via `matchPackageNames`** (chosen — matches the diagnosis exactly, leaves every other direct Go dependency on today's minor/patch-automerge cadence).
- **Move argo-cd into its own Renovate group with a longer schedule/`minimumReleaseAge` instead of disabling minor/patch** (rejected — doesn't address the root cause; a scheduled argo-cd minor/patch PR would still carry the same oversized `go mod tidy` diff whenever it does fire, just less often).

## Decision Outcome

Add one `packageRule` to `renovate.json`, scoped to `github.com/argoproj/argo-cd/v3` via `matchPackageNames`, disabling minor and patch updates (`matchUpdateTypes: ["minor", "patch"]` → `enabled: false`) while leaving major updates flowing through the existing `go` group and its `automerge: false` major rule from ADR 0013. See [the implementation plan](../agents/plans/restrict-argocd-go-updates-to-major.md) for the exact diff.

### Consequences

- Good, because the weekly `go` PR stops absorbing argo-cd's full transitive-graph churn on routine minor/patch releases, shrinking its typical diff back down to the handful of other direct deps (and `go install` tool pins) that actually changed that week.
- Good, because every other direct Go dependency keeps today's minor/patch automerge behavior unchanged — this is a targeted fix for the one dependency causing the problem, not a blanket policy change.
- Bad, because argo-cd minor/patch releases — including any that carry security fixes — no longer get an automatic PR; they only surface when a major version bump also happens to be pending, or if someone checks the Renovate Dependency Dashboard and bumps it by hand. This is an explicit, accepted tradeoff for keeping the weekly PR reviewable.
- Neutral, because `gitops-engine`/`pkg/v2` need no rule of their own — they're already `// indirect` in `go.mod`, and Renovate's default `matchDepTypes: ["indirect"]` exclusion already keeps them out of independent proposals; only the mechanical `go mod tidy` sweep triggered by argo-cd's own version bump touches them, which this decision now suppresses for minor/patch.

## Pros and Cons of the Options

### Disable minor/patch for all direct `gomod` dependencies

- Good, because it's the simplest rule to write and reason about — one `matchManagers: ["gomod"]` clause with no package list to maintain.
- Bad, because it silently drops minor/patch (including security-patch) update PRs for every direct Go dependency, not just the one causing oversized diffs — an unrelated regression for `go-chi/chi`, `spf13/cobra`, `spf13/viper`, `stretchr/testify`, and the rest of the `require` block.

### Move argo-cd into its own group with a longer schedule

- Good, because it doesn't disable minor/patch updates outright — they'd still land, just less frequently.
- Bad, because it doesn't shrink the diff when the PR does fire; the `go mod tidy` sweep is a function of argo-cd's version delta, not of how often Renovate checks for one, so a less-frequent PR would eventually carry an even larger delta.

## More Information

- Plan: [Restrict argo-cd's Go module updates to major-only](../agents/plans/restrict-argocd-go-updates-to-major.md)
- [ADR 0013](0013-renovate-weekly-grouped-updates.md) (the `go` group and major-automerge-off precedent this decision extends)
- [Renovate: `matchDepTypes`](https://docs.renovatebot.com/configuration-options/#matchdeptypes) (gomod's `indirect` depType, excluded from independent updates by default)
- [Renovate: `matchPackageNames`](https://docs.renovatebot.com/configuration-options/#matchpackagenames)
- Superseded by, if adopted later: none.
