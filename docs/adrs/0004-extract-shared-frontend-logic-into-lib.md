---
status: "accepted"
date: 2026-09-19
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Extract shared logic out of Svelte view components into `$lib`, with unit tests added incrementally

## Context and Problem Statement

Following the Tailwind v4/Flowbite Svelte v1 upgrade ([ADR 0002](0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md)) and the e2e testing strategy ([ADR 0003](0003-svelte-e2e-testing-strategy.md)), the `web/` frontend's business logic — API response-envelope construction, query-string building, diff fan-out/aggregation, and table sorting — is duplicated across `web/src/lib/client.ts` and several `.svelte` route/component files, and none of it has unit-test coverage (`web/src/demo.spec.ts` is a placeholder "1 + 2 = 3" test). Because this logic lives directly inside `<script>` blocks of Svelte components, it cannot be unit tested without mounting the component; the only coverage possible today is the (not-yet-implemented) e2e suite from ADR 0003, which is deliberately loose/slow and not meant to catch logic-level regressions such as an off-by-one in diff aggregation or a broken sort comparator.

## Decision Drivers

- The same logic is duplicated 2-3 times: the try/catch/status-check/envelope-construction block in `client.ts`'s `getApplications` and `getApplicationDiff`; query-string building in `client.ts` and twice more in `routes/+page.svelte`'s two redirect functions; the "fetch into a loading/error writable store" pattern in `ApplicationsGrid.svelte` and `routes/diffs/+page.svelte`.
- Diff fan-out/aggregation (`routes/diffs/+page.svelte`) and table-row sorting (`ApplicationsGrid.svelte`) are real domain logic with no test seam today, because they're embedded in component `<script>` blocks.
- `lib/data.ts` exports two writable stores (`apiData`, `diffData`) that nothing imports — dead code left over from an earlier state-management pattern (confirmed via repo-wide grep).
- Unit tests should land alongside each extraction, not as a separate pass after an untested refactor.
- A larger move to SvelteKit `load()` functions, replacing all `onMount`-driven client-side fetching, is a bigger and separate architectural question and is deliberately out of scope here.

## Considered Options

- Leave the logic where it is and rely solely on the ADR-0003 e2e suite for regression coverage.
- Extract the duplicated/untestable logic into `$lib` modules incrementally, landing vitest unit tests alongside each extraction, as one PR (one commit per extraction).
- Do the same extraction and additionally migrate every route to SvelteKit `load()` functions in the same pass.

## Decision Outcome

Chosen option: "Extract the duplicated/untestable logic into `$lib` modules incrementally... as one PR", because it collapses the known duplication and gives the highest-value logic (HTTP envelope handling, query-string building, diff aggregation, sorting) direct unit-test coverage, while keeping the change reviewable as a single PR with one commit per extraction. Bundling in the `load()` migration would conflate two different kinds of risk — logic-correctness cleanup versus a framework-pattern change touching every route — so that migration is deferred to a follow-on plan instead.

Once the extraction produced enough standalone `$lib` modules to make the flat `web/src/lib/` directory hard to scan, those modules were additionally sorted into two folders along the same seam the extraction already implied: `lib/backend/` (`client.ts`, `http.ts`, `data.ts`, `diffs.ts`, `url.ts` — everything that speaks the `tangle-server` wire format or orchestrates fetches) and `lib/ui/` (`components/`, `sort.ts`, `validation.ts`, `utils.ts` — everything that only shapes data for rendering or reacts to user input). `url.ts` is used by both sides (`client.ts`'s query strings and `+page.svelte`'s internal nav links) but was placed in `backend/` since it exists to satisfy the wire API's query-string conventions; the UI's use of it for internal links is incidental.

### Consequences

- Good, because duplicated logic collapses to one implementation each: a generic HTTP envelope helper, a query-string builder, a diff-aggregation module, and a sort-comparator module.
- Good, because the new `$lib` modules are plain `.ts` functions, directly unit-testable with vitest and no component mounting required.
- Good, because the dead `apiData`/`diffData` exports in `data.ts` are removed as part of the same cleanup.
- Neutral, because view components still do their own `onMount`-driven fetching after this change — moving to SvelteKit `load()` functions is deferred to a follow-on plan, not decided here.
- Neutral, because whether to add a component-testing library (e.g. `@testing-library/svelte`) for view-level unit tests is deliberately not decided here — the extraction targets the logic that has no test seam at all today; remaining view-level behavior (rendering, interaction) is already the target of the ADR-0003 Playwright suite, so a component-testing library is only worth adding if a concrete untested view-level bug shows up that neither vitest-on-`$lib` nor Playwright would catch.

## Pros and Cons of the Options

### Leave as-is, rely on e2e only

- Good, because it requires no work now.
- Bad, because it leaves duplicated logic duplicated, and leaves diff-aggregation/sort logic with no fast, deterministic test coverage — only the slower, structurally-looser e2e suite once ADR 0003 is implemented.

### Extract into `$lib` incrementally, unit tests alongside (chosen)

See Decision Outcome.

### Extract and migrate to SvelteKit `load()` in the same pass

- Good, because it avoids touching the same route files twice across two separate changes.
- Bad, because it conflates a logic-correctness refactor with a framework-pattern migration that touches every route, making the PR harder to review and riskier to land in one step.

## More Information

- Implementation plan: [Svelte frontend `$lib` refactor](../agents/plans/svelte-frontend-lib-refactor.md)
- Related: [ADR 0002](0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md), [ADR 0003](0003-svelte-e2e-testing-strategy.md)
- Follow-on (not yet an ADR): migrating routes from `onMount`-driven fetching to SvelteKit `load()` functions.
