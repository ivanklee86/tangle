---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# CI pipeline restructure: mock-backed unit/integration Go tests, path-conditional jobs, and one always-run live e2e job

## Context and Problem Statement

`ci.yaml` today runs four jobs — `go`, `ts`, `format`, `docs` — unconditionally on every push to `main` and every pull request, with no `needs:` between them (see [docs/agents/ci.md](../agents/ci.md)). The `go` job is the slowest by a wide margin because every Go test file dials a real ArgoCD instance: `task services:cicd` tears down and rebuilds a k3d cluster, installs ArgoCD, builds the `tangle` Docker image, and runs it, before `go test ./...` (which includes `internal/argocd/client_test.go`, `internal/argocd/wrapper_test.go`, `internal/tangle/handlers_test.go`, `internal/cli/tanglecli_test.go`, and `cmd/tangle-cli/main_test.go`'s `generate-manifests` case) can even start. None of that cost is actually required by most of those tests' own logic — `internal/argocd.IArgoCDWrapper` and `internal/argocd.IArgoCDClient` are already interfaces (`internal/argocd/wrapper.go`, `internal/argocd/client.go`), and `tangle.Tangle.ArgoCDs` (`internal/tangle/server.go`) is an exported `map[string]argocd.IArgoCDWrapper` that tests can overwrite directly — the live-cluster dependency is a test-authoring choice, not an architectural necessity.

Separately, [ADR 0003](0003-svelte-e2e-testing-strategy.md) decided the frontend's live-stack Playwright suite should be opt-in only (`workflow_dispatch`/nightly), specifically to keep the required per-PR path fast and hermetic; [docs/agents/plans/svelte-e2e-testing.md](../agents/plans/svelte-e2e-testing.md) is the (unimplemented) plan built around that split. The current work session asks for something that doesn't fit ADR 0003's cadence: a single e2e job — covering both Go's live-ArgoCD tests and the frontend's live-stack Playwright suite — that always runs, on every push and PR, while the rest of the pipeline (Go unit/integration tests, TS checks) becomes conditional on whether relevant files changed.

This ADR also covers folding the standalone `format` job into `go` (it only lints Go files, so it's a Go concern, and today it provides no fail-fast benefit since all four jobs schedule together — already flagged in `docs/agents/ci.md`'s "Things worth revisiting"), and making `docs` conditional on `docs/**` changes excluding `docs/agents/**` (which `mkdocs.yml`'s `exclude_docs: agents/` already excludes from the built site, so it's not a new distinction, just wiring CI to match it).

## Decision Drivers

- `IArgoCDClient`/`IArgoCDWrapper` already exist as interfaces, and `Tangle.ArgoCDs` is already an overwritable exported map — mock-backed tests are a test-file rewrite, not a new abstraction layer.
- `task services:cicd` (k3d + ArgoCD + Docker image bring-up) is the single most expensive step in CI and today gates tests that don't need it.
- The Go live-ArgoCD tests and a frontend live-stack Playwright suite would both need the exact same bring-up; running them in one job means paying for it once instead of twice, and gives one place to look for "is the real stack healthy" failures.
- GitHub Actions required-status-checks behave better with one workflow whose jobs are conditionally skipped (`if:` on a job, driven by a cheap path-filter job) than with multiple workflows each gated by `on.push.paths`/`pull_request.paths` — a skipped job still reports a (green) status, so a required check stays satisfied; a workflow that never triggers at all can leave that same required check stuck pending forever on a PR that doesn't touch its paths.
- `mkdocs.yml` already excludes `docs/agents/` from the published site (`exclude_docs: | agents/`), so gating the `docs` job on `docs/**` (minus `docs/agents/**`) plus `mkdocs.yml`/`requirements.txt` matches what actually affects the build, not an arbitrary new boundary.
- Explicit product decision: overall CI wall-clock time is not a constraint here. The goal is a full test pyramid — fast unit tests, mock/fixture-backed integration tests, and real-infra e2e tests, for both Go and the frontend — and a slower `e2e` job is an accepted cost of that, not something to trim coverage to avoid.

## Considered Options

- Keep ADR 0003's cadence: Go tests split into mock-backed/live, but the live Go tests stay in the already-always-running `go` job, and the frontend live-stack suite stays opt-in/nightly as planned. "e2e always runs" is read as already true for Go and out of scope for the frontend.
- One combined `e2e` job, always run: fold both the live-Go tests and the frontend live-stack Playwright suite into a single job that always runs on push/PR, superseding ADR 0003's opt-in cadence for the frontend suite specifically.
- Path-conditional `go`/`ts`/`docs` jobs via separate workflow files with native `on.paths` triggers, vs. one workflow with a `dorny/paths-filter` job feeding `if:` conditions into `go`/`ts`/`docs`.

## Decision Outcome

Chosen options: "One combined `e2e` job, always run" and "One workflow with a `dorny/paths-filter` job feeding `if:` conditions," because together they let the fast, path-conditional jobs (`go` unit/integration, `ts`) stay hermetic and skip cleanly when irrelevant, while a single always-run `e2e` job is the one place that pays for and exercises the real ArgoCD-backed stack, for both backend and frontend.

This **partially supersedes [ADR 0003](0003-svelte-e2e-testing-strategy.md)**: the mocked-suite-is-the-default-check half of that decision is unchanged and still the reason the frontend's fixture-driven Playwright suite lives in the fast `ts` job, not here. What changes is the live-stack suite's cadence — from opt-in (`workflow_dispatch`/nightly) to always-run, folded into this ADR's `e2e` job. [docs/agents/plans/svelte-e2e-testing.md](../agents/plans/svelte-e2e-testing.md)'s workstreams 1–3 (mocked suite, devcontainer browser install, Task wiring) are unaffected; workstream 4's CI section is superseded by this ADR's `e2e` job design (see [the implementation plan](../agents/plans/ci-pipeline-restructure.md)).

### Consequences

- Good, because `go` and `ts` become fast, hermetic, path-conditional checks — a docs-only or web-only PR no longer waits on a full k3d/ArgoCD/Docker bring-up it can't possibly break.
- Good, because the `e2e` job's one bring-up serves both the Go live-ArgoCD tests and the frontend live-stack smoke suite, instead of paying for two separate cluster stand-ups (today's `go` job, plus a hypothetical separate frontend live job).
- Good, because skipped `go`/`ts`/`docs` jobs (via `if:` in one workflow) still report a status, so branch-protection required checks naming those jobs stay satisfiable on PRs that don't touch their paths.
- Accepted trade-off, not a defect: every PR now always pays for the `e2e` job's cluster bring-up (multi-minute, and now covering more ground than before — see [the implementation plan](../agents/plans/ci-pipeline-restructure.md)'s widened live-suite scope), even a docs-only or CSS-only change. This was previously true only for Go changes (via the always-running `go` job) and never true for frontend-only changes. Explicitly accepted per this session's direction that overall CI time is secondary to test-pyramid coverage; mitigated only in that it's the *one* job every PR always waits on — everything else (`go`, `ts`, `docs`) got faster and path-conditional.
- Bad, because it reverses part of an already-accepted ADR (0003) after roughly zero implementation time has passed — worth noting in review in case the "opt-in" cadence was also motivated by concerns beyond per-PR speed (e.g., cluster flakiness in CI) that this decision doesn't independently re-litigate.
- Bad, because mock-backed Go unit/integration tests and the real ArgoCD client are now tested in different files/jobs — a change to `IArgoCDClient`'s contract that both a fake and the real implementation must honor could pass the fast job's mocked tests while breaking the live `e2e` job's tests, or vice versa; there's no compile-time guarantee the fake and the real client stay behaviorally aligned beyond both satisfying the same Go interface signature.

## Pros and Cons of the Options

### Keep frontend e2e opt-in per ADR 0003

- Good, because it doesn't reverse an already-accepted decision, and avoids adding cluster bring-up cost to frontend-only PRs.
- Bad, because it does not satisfy this session's explicit ask for one e2e suite that always runs and covers "ArgoCD/server running" — it would leave "e2e" meaning "Go's live tests only," with the frontend's live-stack coverage still nightly/manual.

### Combined always-run `e2e` job (chosen)

See Decision Outcome.

### Separate workflow files with native `on.paths` triggers

- Good, because it needs no third-party action — pure GitHub Actions primitives.
- Bad, because a PR touching only `web/**` would never trigger a `ci-go.yaml` workflow at all; if branch protection requires a check from that workflow, the PR shows that check as perpetually pending rather than skipped, which is a known GitHub Actions rough edge with path-filtered workflow triggers (as opposed to path-filtered *jobs* inside one workflow).

### Single workflow + `dorny/paths-filter` (chosen)

See Decision Outcome.

## More Information

- Implementation plan: [CI pipeline restructure](../agents/plans/ci-pipeline-restructure.md)
- Related: [ADR 0003](0003-svelte-e2e-testing-strategy.md) (partially superseded — see Decision Outcome)
- Related: [ADR 0010](0010-replace-codecov-with-octocov.md) (a separate, independent coverage-tooling swap in the same `go` job this ADR restructures)
- Current-state reference: [docs/agents/ci.md](../agents/ci.md)
