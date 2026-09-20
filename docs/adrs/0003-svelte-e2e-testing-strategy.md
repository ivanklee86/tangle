---
status: "accepted; cadence partially superseded by 0009"
date: 2026-09-19
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Svelte frontend e2e testing strategy: mocked by default, live stack as an opt-in smoke check

## Context and Problem Statement

`web/e2e/demo.test.ts` is the only Playwright spec in the repo, and it doesn't even pass against the current app (it waits for an `h1` that no page renders) — CI never runs it (`.github/workflows/ci.yaml`'s `ts` job only runs `task ts:install`/`ts:lint`/`ts:build`, never `task ts:test`/`test:e2e`). While manually verifying the Tailwind v4/Flowbite Svelte v1 upgrade in this session, real Playwright-driven browser checks (screenshots, click-through of tabs/sort/toast/accordion) caught two regressions that `svelte-check`/eslint/unit tests could not: an `@source` glob that silently dropped Flowbite's styling, and an `Input` left-icon overlapping placeholder text. That's strong evidence real e2e coverage is worth having. `TangleAPIClient` (`web/src/lib/client.ts`) talks to `tangle-server`'s REST API (`/api/applications`, `/api/argocd/:name/applications/:app/diffs`); a full end-to-end run needs that server backed by a real ArgoCD instance, which `task services:cicd` already stands up (k3d cluster + ArgoCD + the built `tangle-server` container) for the Go backend's own integration tests (`.github/workflows/ci.yaml`'s `go` job).

## Decision Drivers

- The `go` CI job already exercises the real ArgoCD-backed API surface (`internal/argocd`, `internal/tangle` handler tests, `task services:cicd`); frontend e2e's distinct value is validating the UI layer (rendering, interaction, dark mode, client-side sort) reacts correctly to known API shapes, not re-proving the backend works.
- `task services:cicd` brings up a full k3d cluster + ArgoCD + Docker image — correct but slow (multi-minute) and adds a Kubernetes/Docker dependency to whatever job runs it. Requiring that for every PR's frontend check would make the fast `ts` CI job slow and flaky in a way disproportionate to what frontend changes usually touch.
- ArgoCD-backed fixture data (application names, health/sync status) is real cluster state, not fixed test fixtures — asserting exact values against it makes tests brittle to whatever `integration/kubernetes/argocd` happens to contain at run time.

## Considered Options

- Mock the API boundary (Playwright route interception) for all e2e tests, real browser + real app code, fixture JSON standing in for `tangle-server`.
- Run all e2e tests against the live `task services:cicd` stack only.
- Mocked suite as the default/required check, plus a smaller opt-in suite against the live stack.

## Decision Outcome

Chosen option: "Mocked suite as the default/required check, plus a smaller opt-in suite against the live stack", because it keeps the fast, deterministic suite as the thing every PR must pass (real browser, real rendering and interaction logic, no flakiness from cluster state or bring-up time), while still keeping a live-stack smoke path available to catch drift between the fixtures and `tangle-server`'s actual response shape, without forcing every frontend PR to pay for a full cluster bring-up.

### Consequences

- Good, because the required `ts` CI job stays fast and hermetic — no Docker-in-Docker/k3d dependency for the check every PR needs to pass.
- Good, because the mocked suite can assert exact, meaningful content (specific application names, health states, sort order) instead of "a table exists," since the fixtures are authored to match `$lib/data.ts`'s types.
- Bad, because two response shapes now have to be kept in sync by hand: the e2e fixtures and whatever `tangle-server` actually returns. A live smoke run is the mitigation, not a fix — it only catches drift when someone remembers to run it (or a schedule triggers it).
- Bad, because the live smoke suite's assertions have to be structural ("at least one application row," "no console errors") rather than exact-value, since real cluster/ArgoCD state varies — it's a weaker check than the mocked suite, by design.

## Pros and Cons of the Options

### Mock the API boundary for everything

- Good, because it's the fastest and most deterministic option, and needs no Kubernetes/Docker tooling in the devcontainer or CI beyond Playwright's own browser install.
- Bad, because it never actually proves the frontend and the real `tangle-server` API agree on response shape — a backend change could silently break the frontend and no e2e run would catch it.

### Live stack only

- Good, because it's the most realistic check — it exercises the whole path from browser to ArgoCD.
- Bad, because it makes every frontend e2e run pay for a multi-minute cluster bring-up, and ties frontend test flakiness to cluster/ArgoCD availability instead of just app code.
- Bad, because assertions have to be loose (structural, not exact-value) to tolerate real cluster state, so it's a weaker regression check for UI logic like sort order or specific status rendering than a fixture-driven suite would be.

### Hybrid (chosen)

See Decision Outcome.

## More Information

- Implementation plan: [Svelte e2e testing](../agents/plans/svelte-e2e-testing.md)
- Related: [ADR 0002](0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md) (the upgrade whose manual Playwright verification motivated this ADR)
- Superseded by: [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md), which changes only the live-stack smoke suite's cadence (opt-in nightly/`workflow_dispatch` → folded into an always-run `e2e` job). The mocked-suite-as-required-check half of this decision is unchanged.
