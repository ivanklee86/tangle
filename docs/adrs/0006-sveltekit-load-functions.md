---
status: "accepted"
date: 2026-09-19
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Migrate route data-fetching from `onMount` to SvelteKit `load()` functions

## Context and Problem Statement

[ADR 0004](0004-extract-shared-frontend-logic-into-lib.md) extracted the frontend's fetch/aggregation/sort logic out of `.svelte` files into `$lib`, but deliberately left *where* that logic is invoked untouched — `lib/ui/components/ApplicationsGrid.svelte` and `routes/diffs/+page.svelte` still trigger their fetches from `onMount`, rather than SvelteKit's idiomatic `load()` functions (`+page.ts`), and deferred that migration as "workstream 7, a follow-on." With [ADR 0005](0005-svelte-component-unit-tests.md)'s component-test suite now in place (54 tests: `$lib` plain functions + presentational components), there's now a real regression net for the logic layers, making this a reasonable time to revisit.

## Decision Drivers

- `onMount`-driven fetching means data-fetching is invisible to SvelteKit's router — no automatic loading-state integration, no `invalidate`/`invalidateAll` hook for controlled re-fetching, and the fetch is tied to component lifecycle rather than navigation.
- The app runs with `ssr = false` (`routes/+layout.ts`) and `prerender = true` (adapter-static's documented SPA pattern: prerender a static shell, then the client-side router takes over). This means `load()` functions here are universal (`+page.ts`) and run entirely client-side, after hydration — there is no server/client serialization boundary to design around, which simplifies this migration considerably compared to an SSR-enabled app.
- `ApplicationsGrid.svelte` currently owns both data-fetching *and* presentation, with the refresh toggle/period threaded down as props from `routes/applications/+page.svelte` — an awkward split that `load()` + `invalidateAll()` naturally resolves by moving fetch ownership to the route and leaving the component purely presentational.
- `routes/diffs/+page.svelte`'s diff fan-out drives a granular progress bar (X of Y diffs loaded) — a UX detail worth preserving deliberately rather than losing to a coarser "loading" state as a side effect of the migration.
- `TangleAPIClient` currently hardcodes the global `fetch` inside `lib/backend/http.ts`'s `fetchEnvelope`; SvelteKit's `load()` functions receive their own `fetch` implementation and documentation recommends using it (request deduplication, credential handling) even in an SSR-disabled app, since the pattern is the same regardless.

## Considered Options

- Leave fetching in `onMount`, as ADR-0004 left it.
- Migrate both routes' initial data-fetch to `load()` by fully awaiting it (blocks page render until data resolves, no per-page spinner without extra `$navigating`-based UI).
- Migrate both routes' initial data-fetch to `load()` using SvelteKit's *streamed* (unawaited) promise pattern, consumed via `{#await}` in the page template — keeps a per-page loading state close to today's, with fetch ownership moved to the router layer.

## Decision Outcome

Chosen option: streamed `load()` for each route's initial "list of applications" fetch, consumed via `{#await}`. This is the one option that gets the architectural benefit (fetch ownership at the routing layer, `invalidateAll()` for refresh, testable `load()` functions) without a UX regression (the `{#await}` pending state reproduces today's spinner almost exactly) or losing the diffs page's progress bar (which stays as client-side orchestration reacting to the *resolved* applications data, not inside `load()` itself).

`TangleAPIClient`/`fetchEnvelope` gain an injectable `fetch` parameter (default: global `fetch`) so `load()` can pass its own, and component/unit tests can inject a mock directly instead of `vi.stubGlobal`.

### Consequences

- Good, because `ApplicationsGrid.svelte` becomes purely presentational (props in, markup out) — its component test loses the `$app/stores`/global-`fetch` mocking it currently needs, becoming meaningfully simpler.
- Good, because both routes' `load()` functions are plain exported async-ish functions, directly unit-testable (mock `{url, fetch}`, assert the returned promise's shape) without mounting anything.
- Good, because refresh-on-interval for `/applications` becomes `invalidateAll()` on a timer instead of a bespoke reset-and-refetch store dance — one fewer bespoke pattern in the codebase.
- Neutral, because the diffs page's diff-fan-out/progress-bar logic stays inside `routes/diffs/+page.svelte`'s template as nested `{#await}`/reactive state — real UI-wiring logic remains in a `.svelte` file, which looks like it cuts against ADR-0004's "move logic out of views" goal, but the actual fetch/aggregation logic (`fetchDiffs`/`buildDiffMap`) already lives in and stays in `lib/backend/diffs.ts`; what's left in the page is genuinely presentation-coupled (when to trigger, how to surface progress) and doesn't have a clean home outside the component.
- Bad, because this migration reintroduces real wiring logic into two route `.svelte` files that ADR-0005 explicitly scoped *out* of component-level testing (route composition was left to ADR-0003's still-unimplemented e2e suite). The existing 54-test suite is real regression coverage for the logic layers and presentational components, but does **not** automatically cover whether `{#await}` wiring or the refresh→`invalidateAll` behavior work correctly — that gap is addressed by adding minimal route-level component tests as part of this work (see the plan), not left fully to manual verification.

## Pros and Cons of the Options

### Leave as-is (`onMount`)

- Good, because it requires no change and no new risk.
- Bad, because it leaves the architectural awkwardness (fetch ownership split between route and component, no `invalidate` hook, no `load()` testability) unresolved indefinitely.

### Fully-awaited `load()`

- Good, because it's the simplest `load()` usage, no streaming semantics to reason about.
- Bad, because the page doesn't render at all until data resolves — no built-in per-page spinner without additional `$navigating`-based UI, a real UX change from today's immediate-render-then-spinner behavior.

### Streamed `load()` + `{#await}` (chosen)

See Decision Outcome.

## More Information

- Implementation plan: [SvelteKit load() migration](../agents/plans/sveltekit-load-migration.md)
- Related: [ADR 0004](0004-extract-shared-frontend-logic-into-lib.md) (deferred this as "workstream 7"), [ADR 0005](0005-svelte-component-unit-tests.md) (the test suite this migration is a follow-on to, and whose route-level scoping decision this ADR partially revisits)
