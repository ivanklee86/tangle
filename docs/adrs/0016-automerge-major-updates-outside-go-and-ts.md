---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Automerge major updates for everything except Go and TypeScript

## Context and Problem Statement

[ADR 0013](0013-renovate-weekly-grouped-updates.md) made `automerge: false` for `matchUpdateTypes: ["major"]` a repo-wide rule, on the grounds that a major-version bump carries real breaking-change risk across the board (it named Sveltekit, `argo-cd`/`traefik` chart majors, and Go major bumps specifically). In practice, most of that risk is concentrated in Go and TypeScript: a Go major bump can break compilation or runtime behavior across the whole module graph, and a Sveltekit/frontend-tooling major can break the build or the UI. Major bumps in the `k8s` group (Helm chart versions, `kubectl`/`helm`/`k9s`/`argocd` CLI pins, `gateway-api`), the `ci` group (`github-actions`), and the `devcontainer` group are comparatively low-risk and high-volume — GitHub Actions in particular version aggressively (e.g. `actions/checkout` v4→v5) and a major bump there is rarely a breaking change for how this repo uses them. Requiring manual review for all of these adds review load without a proportionate safety benefit, so majors outside Go/TS should automerge like every other update type, gated by the same 5-day `minimumReleaseAge` that already applies to everything.

## Decision Drivers

- Go and TypeScript majors carry compile/build/runtime breakage risk broad enough to justify keeping manual review; `k8s`/`ci`/`devcontainer` majors don't, in this repo's experience.
- `minimumReleaseAge: "5 days"` (ADR 0013, config root, applies to every manager) already provides a real safety buffer against a yanked or compromised release for every automerged update, major or not — it's not being removed here, only the additional manual-review gate for non-Go/TS majors.
- The existing `devcontainer` group already automerged unconditionally (including majors) before ADR 0013's repo-wide major rule tightened it; this decision effectively restores that original behavior for `devcontainer`, and extends the same policy to `k8s` and `ci`.

## Considered Options

- **Keep majors manual everywhere** (ADR 0013's original scope; rejected here — too broad given where the actual risk sits).
- **Automerge majors everywhere including Go/TS** (rejected — a Go major or a Sveltekit major automerging unattended is exactly the risk ADR 0013 was written to avoid; `minimumReleaseAge` alone doesn't mitigate a bad *design*, only a bad or yanked *release*).
- **Automerge majors for everything except Go and TypeScript** (chosen) — scope the manual-review requirement down to the two ecosystems where a major bump has historically meant real breakage, and automerge everywhere else.

## Decision Outcome

Restructure the repo-wide major-update rule in `renovate.json`: `matchUpdateTypes: ["major"]` now defaults to `automerge: true`/`platformAutomerge: true`, with three narrower rules layered after it (`matchDatasources: ["go"]`, `matchDatasources: ["npm"]`, and `matchManagers: ["dockerfile"]` + `matchPackageNames: ["golang", "ghcr.io/ivanklee86/devcontainer/go", "node"]`) putting `automerge: false` back for Go- and TypeScript-scoped majors specifically — the same matcher shapes already used by the `go`/`ts` groupName rules from ADR 0013, so the automerge scope now tracks the ecosystem grouping exactly. See [the implementation plan](../agents/plans/automerge-non-go-ts-majors.md) for the exact diff. This also reverts the `devcontainer` group's major-automerge back to unconditional (it's not Go/TS-scoped, so nothing overrides its own `automerge: true`), which ADR 0013 had tightened as a side effect of the old repo-wide rule.

### Consequences

- Good, because `k8s`/`ci`/`devcontainer` major bumps stop requiring manual merges, cutting review load for the highest-volume, comparatively low-risk update category (GitHub Actions majors especially).
- Good, because Go and TypeScript majors keep the manual-review gate ADR 0013 established for them specifically — this narrows that rule's scope rather than removing its rationale.
- Bad, because a breaking major bump in `k8s`/`ci`/`devcontainer` — a Helm chart with a breaking values-schema change, an Argo CD/Traefik chart major, a GitHub Action with a breaking input change — now merges unattended once 5 days old, with no human review before it lands. The Dependency Dashboard remains the way to catch and revert one after the fact, not before.

## Pros and Cons of the Options

### Keep majors manual everywhere

- Good, because it's the most conservative posture — nothing breaking lands without a human looking at it first.
- Bad, because it treats a `github-actions` major (frequent, usually low-risk) the same as a Go major (infrequent, often breaking), adding review load where the risk doesn't justify it.

### Automerge majors everywhere including Go/TS

- Good, because it's the simplest rule — no ecosystem-scoped carve-outs to maintain.
- Bad, because it removes the one manual gate ADR 0013 identified as actually load-bearing (Go/Sveltekit majors breaking the build), for no reviewed benefit.

## More Information

- Plan: [Automerge non-Go/TS majors](../agents/plans/automerge-non-go-ts-majors.md)
- [ADR 0013](0013-renovate-weekly-grouped-updates.md) (establishes the `go`/`ts`/`k8s`/`ci` groups this decision reuses the matchers from, and the original repo-wide major/automerge split this decision narrows)
- [ADR 0015](0015-restrict-argo-cd-go-module-updates-to-major-only.md) (unaffected — argo-cd's minor/patch updates stay disabled regardless of this decision's automerge changes, since `enabled: false` means the update is never proposed)
- Superseded by, if adopted later: none.
