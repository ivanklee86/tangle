---
status: "accepted"
date: 2026-09-19
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Add component-level unit tests for Svelte views via `vitest-browser-svelte`

## Context and Problem Statement

[ADR 0004](0004-extract-shared-frontend-logic-into-lib.md) extracted the frontend's testable business logic into plain `.ts` modules under `web/src/lib/backend` and `web/src/lib/ui`, and deliberately deferred deciding on a component-testing library — betting instead on the (still not implemented) Playwright e2e suite from [ADR 0003](0003-svelte-e2e-testing-strategy.md) to cover view-level behavior. Two gaps remain after that work: `lib/backend/client.ts` (the `TangleAPIClient` class itself — its URL/method/body assembly — has no direct test) and `lib/backend/data.ts`'s `emptyApplicationResponseStore()` factory; and, more significantly, all five `.svelte` files under `lib/ui/components/` have zero test coverage of any kind, plain-function or otherwise.

## Decision Drivers

- The extraction work in ADR 0004 already pulled the *logic* out of components; what's left inside `.svelte` files is rendering and light glue (prop-driven branches, `onMount` fetch orchestration, `$app/stores` reads) — real behavior, just not previously reachable by vitest.
- ADR 0003's Playwright suite is still unimplemented, so today there is genuinely no automated coverage of any `.svelte` file, at any layer.
- Svelte's own current testing guidance for Svelte 5 favors `vitest-browser-svelte` (Vitest's Browser Mode, driving a real headless browser) over `@testing-library/svelte` + jsdom, since jsdom has known gaps simulating Svelte 5 runes/DOM behavior.
- A Playwright Chromium binary is already cached in this devcontainer (confirmed via `npx playwright --version` / `~/.cache/ms-playwright`), but the devcontainer's `Dockerfile` doesn't yet install the runtime/font apt packages that ADR-0003's plan already identified as required for correct text rendering — that installation step is a shared prerequisite for both this work and ADR-0003's eventual e2e suite.
- `vitest-browser-svelte` requires Vitest ≥5, which requires Vite ≥6.4 — both verified against the packages' published `peerDependencies` — so adopting it is a real dependency upgrade, not just an addition.
- Reading `.github/workflows/ci.yaml`'s `ts` job directly shows `task ts:test` (which runs `npm run test:unit`) is never invoked — unit tests, old or new, don't run in CI today regardless of this decision.

## Considered Options

- Skip component tests; keep unit tests scoped to `$lib` plain functions and rely solely on ADR-0003's (unimplemented) Playwright suite for view-level coverage.
- Add component tests via `@testing-library/svelte` + jsdom — no browser/font dependency, but weaker fidelity for Svelte 5 and a different testing idiom than Svelte's current guidance.
- Add component tests via `vitest-browser-svelte` (Vitest Browser Mode, Playwright-driven) — matches Svelte's current recommendation and reuses the same Playwright browser the devcontainer will need for ADR-0003 regardless.

## Decision Outcome

Chosen option: `vitest-browser-svelte`. It matches Svelte's own current testing guidance, gives higher-fidelity results than jsdom for Svelte 5's runes-based reactivity, and its browser/font prerequisites are work this repo needs to do anyway for ADR-0003's e2e suite — doing it once here means ADR-0003's implementation doesn't have to repeat it.

Scope is components only (`lib/ui/components/*.svelte`), not the route `+page.svelte` files. Full-page composition and interaction (tab switching across nested `Tabs`, the home page's redirect flows, `ApplicationsGrid`'s refresh polling as seen from a whole page) stays with ADR-0003's Playwright suite — the same reasoning ADR-0003 itself used to separate "UI layer" concerns from "does the real API agree with our types" concerns applies again here: testing full-route interaction at both the component-unit layer and the e2e layer would mean maintaining the same behavior twice.

### Consequences

- Good, because every `.svelte` component becomes reachable by a fast, deterministic test, with real DOM/CSS/event semantics (not jsdom's approximation).
- Good, because the devcontainer/font prerequisite work done here directly unblocks ADR-0003's e2e suite later — no duplicated investigation.
- Bad, because it requires bumping `vitest` (`^3.0.0` → `5.0.1`) and `vite` (currently resolving to `6.2.6` → pinned `6.4.3`) — a two-major-version jump for `vitest` that needs the existing six `$lib` spec files re-verified green before any new component tests are written, to isolate "broke from the bump" from "broke from new test code."
- Neutral, because `client.ts`/`data.ts`'s remaining `$lib` gaps are closed in the same pass but don't depend on any of the above — they're plain Node-environment vitest, landed first, independently useful even if the browser-mode work stalls.
- Neutral, because CI wiring (`task ts:test` was never called) is fixed as part of this work regardless of component tests — that gap existed before this decision and would need fixing either way.

## Pros and Cons of the Options

### Skip component tests, rely on ADR-0003 alone

- Good, because it requires no new tooling or version bumps.
- Bad, because ADR-0003 is itself unimplemented, leaving `.svelte` files with literally zero coverage for an indefinite period.

### `@testing-library/svelte` + jsdom

- Good, because it has no browser/font dependency at all — fully decoupled from ADR-0003's devcontainer work.
- Bad, because it's a weaker fidelity match for Svelte 5 runes than a real browser, and is no longer Svelte's primary recommended path.

### `vitest-browser-svelte` (chosen)

See Decision Outcome.

## More Information

- Implementation plan: [Svelte component unit tests](../agents/plans/svelte-component-unit-tests.md)
- Related: [ADR 0003](0003-svelte-e2e-testing-strategy.md), [ADR 0004](0004-extract-shared-frontend-logic-into-lib.md)
- Verified against source at implementation-planning time (2026-09-19): `vitest-browser-svelte`'s README (setup snippet), `vitest.dev`'s Browser Mode guide (multi-project `node`/`browser` split), and this repo's installed `vite`/`vitest` versions via `npm ls` / `npm view`.
