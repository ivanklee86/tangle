---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Hold `k8s.io/*` and the `gitops-engine` replace to manual argo-cd bumps

## Context and Problem Statement

[PR #216](https://github.com/ivanklee86/tangle/pull/216) ("Update go", the weekly grouped `go`
minor/patch PR) fails two ways, reproduced by testing each independently in a scratch worktree of
the PR branch:

1. `renovate/artifacts` failed regenerating `go.sum`. The PR advances `go.mod`'s `replace
   github.com/argoproj/argo-cd/gitops-engine => ...` pin — a pseudo-version tied to a specific
   commit, per the comment directly above it: "argo-cd v3.5.3 vendors gitops-engine into its own
   repo and only wires it up via a local replace in its own go.mod, which this module (as a
   consumer) doesn't inherit. Point it at a resolvable commit from the same v3.5.3 tag instead" —
   to a newer upstream commit. That commit's own `go.mod` has since renamed its module path to
   `github.com/argoproj/argo-cd/gitops-engine/v3`, which is invalid to reference from a non-`/v3`
   replace target: `go: errors parsing go.mod: replace ...: version "v0.0.0-...-c6760e808d4f"
   invalid: gitops-engine/go.mod has post-v0 module path ".../gitops-engine/v3" at revision
   c6760e808d4f`. The failed artifact step also left a malformed partial edit committed to the PR
   (a bare commit hash, missing the `v0.0.0-<timestamp>-` pseudo-version prefix `go mod edit` would
   normally produce).
2. Reverting *only* that line and keeping the PR's other change — `k8s.io/apimachinery v0.36.1 →
   v0.37.0`, plus the `replace (...)` block of ~30 other `k8s.io/*` packages that followed it in
   the same diff — **still fails to build**: `go mod tidy` errors that `k8s.io/api@v0.37.0` no
   longer contains `k8s.io/api/scheduling/v1alpha2`, which argo-cd v3.5.3's vendored
   `k8s.io/kubernetes` code still imports transitively through `gitops-engine`.

Both failures trace to the same fact: the `k8s.io/*` `replace (...)` block is, per its own comment,
"mirrored here from argo-cd v3.5.3's own go.mod replace block so those staging modules resolve to
their real v0.36.1 releases instead of the unresolvable v0.0.0 placeholder" — it is not an
independent set of dependencies this repo chooses versions for, it's a byte-for-byte copy of what
one exact argo-cd release (v3.5.3) needs internally. The `gitops-engine` replace is the same kind
of pin, one level up. Bumping either independently of a deliberate, matching argo-cd version bump
produces a `go.mod` that no longer describes a version argo-cd v3.5.3 can actually build against —
which is exactly the failure mode [ADR 0015](0015-restrict-argo-cd-go-module-updates-to-major-only.md)
already identified and restricted for `argo-cd/v3` itself ("a scheduled argo-cd minor/patch PR
would still carry the same oversized `go mod tidy` diff whenever it does fire"). ADR 0015 stopped
short of the packages argo-cd's own vendoring pins in lockstep, and this is exactly where that gap
shows up.

The sharper problem: `k8s.io/apimachinery v0.36.1 → v0.37.0` is a semver-**minor** bump (leading
`0` doesn't change), and `renovate.json`'s repo-wide rule auto-merges every minor/patch/pin/digest
update. Had `renovate/artifacts` not failed on the unrelated `gitops-engine` line first, **this
specific PR was on a path to auto-merge a change that doesn't build** — the artifact failure was
incidental luck, not a review gate doing its job.

## Decision Drivers

- The `k8s.io/*` replace block and the `gitops-engine` replace are companion pins to one exact,
  currently-fixed argo-cd version (`v3.5.3`, `go.mod`'s own `require`), not independently
  version-managed dependencies — the same category ADR 0015 already restricted for `argo-cd/v3`
  itself, for the same underlying reason.
- Confirmed by direct reproduction, not inference: reverting *only* the `gitops-engine` line still
  leaves a non-building `go.mod` from the `k8s.io/*` bump alone — this is not a single narrow fix
  like TypeScript/svelte-check (ADR 0019), it's the whole mirrored block.
- These bumps are currently eligible for `renovate.json`'s repo-wide minor/patch/pin/digest
  automerge rule, which is a real risk, not a hypothetical one — this PR was headed there.
- A future argo-cd version bump already requires a human to manually re-derive this entire mirrored
  block from the new version's own `go.mod` (there is no automated path for it today, per ADR
  0015's implementation) — disabling independent Renovate tracking of `k8s.io/*`/`gitops-engine`
  doesn't remove any automation that currently exists, it just stops Renovate from proposing a
  half-updated version of a block that only makes sense fully re-derived together.

## Considered Options

- **Fix only the `gitops-engine` line for this one PR, leave `k8s.io/*` alone** (rejected) —
  disproven by reproduction: the PR still fails to build with only that one line reverted, so this
  wouldn't actually unblock anything, and the next `k8s.io/*` PR reproduces the same class of
  failure on its own.
- **Restrict by update type only (minor/patch), mirroring ADR 0015's `argo-cd/v3` rule exactly**
  (rejected) — ADR 0015 kept majors flowing for `argo-cd/v3` because a major is the deliberate,
  reviewed re-pin event. `k8s.io/*` packages are all `v0.x`, so under semver they have no "major"
  update type to preserve; there's no analogous safe category to leave open here, so a narrower
  rule buys nothing.
- **Full `enabled: false` for `k8s.io/**` and the `gitops-engine` replace target** (chosen) — no
  update type for either is currently safe to automate; every correct change to this block is a
  manual, argo-cd-version-driven re-pin.

## Decision Outcome

Add one `packageRule` to `renovate.json`, immediately after ADR 0015's `argo-cd/v3` minor/patch
rule:

```json
{
  "description": "k8s.io/* and the gitops-engine replace mirror argo-cd v3.5.3's own vendoring pins; only a deliberate argo-cd bump should move them",
  "matchDatasources": ["go"],
  "matchPackageNames": ["k8s.io/**", "github.com/argoproj/argo-cd/gitops-engine"],
  "enabled": false
}
```

`k8s.io/**` (not `sigs.k8s.io/**`, which isn't part of the mirrored block and isn't implicated in
either failure) — verified directly against the `minimatch` matcher Renovate uses for
`matchPackageNames` glob patterns: it matches `k8s.io/apimachinery` and `k8s.io/api`, and correctly
excludes `sigs.k8s.io/yaml`. Schema-validated with `prek run renovate-config-validator --all-files`.
See [the implementation plan](../agents/plans/fix-gomplate-v5-and-hold-argocd-k8s-pins.md).

### Consequences

- Good, because Renovate stops proposing a `go.mod` state that cannot build against this repo's
  pinned argo-cd version — closing both the artifact-update failure and the standalone
  `k8s.io/apimachinery` build failure at the source, not just the one line that happened to fail
  first.
- Good, because it closes a real automerge exposure: without this rule, a future `k8s.io/*` minor
  bump reaching `renovate/artifacts` successfully (unlike this one) would auto-merge unreviewed and
  break the build.
- Neutral, because this doesn't change how a *real* argo-cd version bump gets handled — that was
  already a fully manual re-derivation of the mirrored block before this decision, and stays that
  way.
- Bad, because `k8s.io/*` security patches (if any land against v0.36.x specifically) won't surface
  as Renovate PRs either, the same accepted tradeoff ADR 0015 already made for `argo-cd/v3` itself.

## Pros and Cons of the Options

### Fix only the `gitops-engine` line

- Good, because it's the smallest possible diff.
- Bad, because it's disproven by direct testing — the PR still doesn't build with only that line
  reverted.

### Restrict by update type (minor/patch), not full disable

- Good, because it mirrors ADR 0015's `argo-cd/v3` rule shape exactly, one convention for both.
- Bad, because `k8s.io/*` packages are permanently `v0.x` — there's no "major" category actually
  reachable here to preserve, so the extra specificity has no real effect and adds a rule shape
  that looks intentional but isn't.

### Full `enabled: false` (chosen)

See Decision Outcome.

## More Information

- Blocked PR: [#216](https://github.com/ivanklee86/tangle/pull/216)
- Implementation plan: [Fix gomplate v5 and hold argo-cd/k8s.io pins](../agents/plans/fix-gomplate-v5-and-hold-argocd-k8s-pins.md)
- [ADR 0015](0015-restrict-argo-cd-go-module-updates-to-major-only.md) (the rule this decision
  extends, and the shared reasoning — a manually-managed vendoring companion pin, not an
  independently-versioned dependency)
- [ADR 0019](0019-hold-typescript-majors-until-svelte-check-supports-them.md) (a related but
  distinct case — a temporary, liftable version ceiling on a package with a real independent
  upstream release cycle, unlike this decision's permanent companion pins)
- Current-state reference: [docs/agents/ci.md](../agents/ci.md)
- Superseded by, if adopted later: a future ADR covering the next deliberate argo-cd version bump,
  which will need to re-derive this entire mirrored block by hand regardless of this rule.
