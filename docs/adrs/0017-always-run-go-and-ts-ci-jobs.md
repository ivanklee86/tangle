---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Always run the `go` and `ts` CI jobs; drop their path-conditional gating

## Context and Problem Statement

[ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) made `go` and `ts` (along with `docs`) conditional on a `dorny/paths-filter`-driven `filter` job, so a PR that doesn't touch Go or frontend files skips the corresponding job entirely, while a single `e2e` job (real k3d/ArgoCD, real browser) runs unconditionally on every push and PR because ADR 0009 wanted one place that always pays for and exercises the real stack. That combination means the path-conditional gating on `go`/`ts` no longer buys what it was built to buy: every PR's total CI wall-clock is already bounded below by `e2e`'s multi-minute cluster bring-up regardless of whether `go`/`ts` run, and `go`/`ts` run in parallel with `e2e` (no `needs` between them, only `filter`), so skipping either one saves runner-minutes/cost but does not shorten how long a PR author actually waits for CI to go green. That's a materially weaker justification than the one ADR 0009 and, before it, [ADR 0003](0003-svelte-e2e-testing-strategy.md) relied on — "keep the required per-PR path fast" — since the required per-PR path is no longer fast in the first place. Given that developer wait time is now dominated by an always-run job, always running the fast, hermetic `go`/`ts` jobs too — closing the coverage gap where a change lands without either language's own unit/integration suite having been asked to prove anything about it — costs nothing that wasn't already being spent.

## Decision Drivers

- `e2e` has no `if:` and always runs; since `go`/`ts` run in parallel with it rather than gating it, making `go`/`ts` unconditional adds no wall-clock time to the critical path a PR author actually waits on — that path is already set by `e2e`.
- The path-conditional design's original rationale (ADR 0003's "keep the required per-PR path fast and hermetic," carried into ADR 0009) is specifically about wall-clock time on the required path; once `e2e` dominates that path unconditionally, skipping `go`/`ts` no longer serves that rationale — it only trims runner-minutes, a cost this repo has not treated as a constraint (ADR 0009's own decision drivers state "overall CI wall-clock time is not a constraint here").
- Path-conditional gating is only as good as the filter's glob list; a change that doesn't match any listed path but still affects `go`/`ts` behavior (a new shared file neither `go:`/`ts:` filters list, a `Taskfile.yaml` include neither anticipated) can silently skip the one fast check that would have caught it. Always running removes that failure mode for `go`/`ts` outright, rather than relying on the filter list staying exhaustive.
- Scope: this reverses the path-conditional half of ADR 0009 for `go` and `ts` only. `docs` is left conditional — a docs-only build (`task python:test`) is a distinct, lower-risk toolchain (`uv`/mkdocs) unrelated to the correctness guarantees `go`/`ts` provide, and this decision's driver (skipping a fast, hermetic code-correctness check no longer buys real wait-time savings) doesn't extend to it without separately re-litigating whether mkdocs-build coverage should be unconditional too — left for a future decision if it comes up.

## Considered Options

- **Keep `go`/`ts` path-conditional as-is** (status quo per ADR 0009) — rejected: keeps optimizing for wall-clock savings that `e2e`'s always-run design already forecloses, in exchange for a real coverage gap.
- **Always run `go` and `ts`, leave `docs` conditional** (chosen) — matches this session's specific ask; keeps the `filter` job and its `docs` output, since `docs` still benefits from path-conditional gating on its own terms.
- **Always run `go`, `ts`, and `docs`; remove the `filter` job and `dorny/paths-filter` entirely** — rejected for now: extends the same reasoning to `docs`, which is plausible but out of scope for this decision (see Decision Drivers); would also mean re-deriving whether the `filter` job has any remaining purpose once nothing consumes its output.

## Decision Outcome

Chosen option: "Always run `go` and `ts`, leave `docs` conditional." Drop `needs: [filter]` and `if: needs.filter.outputs.go == 'true'` / `if: needs.filter.outputs.ts == 'true'` from the `go` and `ts` jobs in `ci.yaml`, so both run unconditionally on every push and PR, the same way `e2e` already does. Trim `filter`'s `dorny/paths-filter` configuration and job outputs down to `docs` only, since nothing reads the `go`/`ts` filter outputs anymore. `docs` keeps its existing `needs: [filter]` / `if: needs.filter.outputs.docs == 'true'` gating, unchanged. See [the implementation plan](../agents/plans/always-run-go-ts-ci-jobs.md) for the exact diff.

This **partially supersedes [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md)**: the test-taxonomy decision (mock-backed Go unit/integration tests, the always-run `e2e` job, folding `format` into `go`) is unchanged and still the reason those jobs are shaped the way they are. What changes is only the path-conditional-gating half of that ADR, and only for `go`/`ts` — `docs` keeps the `filter`-driven gating ADR 0009 established.

### Consequences

- Good, because a change to shared infrastructure that the path filter's glob list doesn't anticipate can no longer silently skip `go`/`ts`'s own fast, hermetic correctness checks.
- Good, because branch-protection reasoning for `go`/`ts` gets simpler — they're required checks that always run, with no "a skipped job still reports green" nuance left to explain for those two (that nuance still applies to `docs`).
- Good, because it costs no additional wall-clock time on the path a PR author actually waits on — `go`/`ts` already run in parallel with the always-run `e2e` job, which remains the long pole.
- Neutral, because `filter`'s job still exists (for `docs`) even though two of its three original outputs (`go`, `ts`) are gone — this is a smaller, single-purpose job now rather than removed outright, which is deliberate given this decision's scope (see Considered Options).
- Bad, because it spends more runner-minutes than the status quo on every PR that touches neither Go nor frontend files (e.g., a docs-only or CI-config-only change now also pays for `go`'s lint/test/coverage run and `ts`'s vitest/mocked-Playwright run) — accepted, since ADR 0009 already established that this repo doesn't treat overall CI cost/wall-clock as a binding constraint, and the same logic applies here.

## Pros and Cons of the Options

### Keep `go`/`ts` path-conditional as-is

- Good, because it changes nothing — zero implementation risk.
- Bad, because it keeps trading real coverage (a path-filter gap silently skipping `go`/`ts`) for a wall-clock savings that `e2e`'s always-run design already made moot.

### Always run `go` and `ts`, leave `docs` conditional (chosen)

See Decision Outcome.

### Always run `go`, `ts`, and `docs`; remove `filter` entirely

- Good, because it's the more thorough, consistent version of the same argument — if wall-clock no longer distinguishes conditional from unconditional jobs, no job needs conditional gating, and a workflow with no `filter` job is simpler to read.
- Bad, because it goes beyond what this decision was asked to cover, and `docs`'s own toolchain/risk profile hasn't been separately evaluated here — left for a future ADR if the same argument is extended there.

## More Information

- Implementation plan: [Always run go/ts CI jobs](../agents/plans/always-run-go-ts-ci-jobs.md)
- Related: [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) (partially superseded — see Decision Outcome; its test-taxonomy and always-run-`e2e` decisions are unchanged)
- Related: [ADR 0003](0003-svelte-e2e-testing-strategy.md) (its "keep the required path fast" rationale, already partially superseded by ADR 0009's `e2e` cadence change, is the same rationale this decision finds no longer load-bearing for `go`/`ts`)
- Current-state reference: [docs/agents/ci.md](../agents/ci.md)
- Superseded by, if adopted later: none.
