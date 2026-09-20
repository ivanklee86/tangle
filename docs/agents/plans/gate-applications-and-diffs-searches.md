# Gate Applications/Diffs behind an explicit search

Status: implemented · 2026-09-20

See [ADR 0008](../adrs/0008-gate-searches-behind-explicit-user-action.md) for the decision and rationale. This
document records what was actually implemented.

## Shared form components

New `lib/ui/components/ApplicationsForm.svelte` and `DiffsForm.svelte`, extracted from what was previously
inline markup on the home page (`routes/+page.svelte`):

- `ApplicationsForm`: `Labels`/`Exclude Labels` inputs, own internal `$state`, a `See applications` button
  disabled only while the label format is invalid (empty is valid — `isValidLabelFormat('')` is `true`), and an
  `onSubmit(labels, excludeLabels)` callback prop the parent route wires to its own navigation.
- `DiffsForm`: the same plus a `Target Ref` input; `initialLabels`/`initialExcludeLabels`/`initialTargetRef`
  props seed its internal state (via `$state(untrack(() => ...))`, since these are one-time seeds from a prop,
  not a live binding — Svelte's `state_referenced_locally` warning flags the un-`untrack`ed form). Its submit
  button stays disabled until `targetRef` is non-empty and both label fields are valid — the target-ref
  requirement is enforced by the disabled button, not a toast or explanatory paragraph.

Both components carry the gradient/`leading-none` button treatment described in the
[design consistency plan](flowbite-design-consistency.md).

## Diffs page

`routes/diffs/+page.svelte`'s `$effect` gains one guard before its `fetchDiffs()` call:

```ts
if (!currentTargetRef) {
	// No target ref means there's nothing to diff against — bail out
	// before fetchDiffs, which otherwise fans out one diff-generation
	// request per application to every ArgoCD instance (falling back
	// to comparing each app's live ref against itself).
	return;
}
```

`diffsLoaded`/`total` simply never become truthy, so the progress bar and tabs never render. The template adds
a branch between the error state and the `!diffsLoaded` state:

```svelte
{:else if !targetRef}
	<DiffsForm
		initialLabels={page.url.searchParams.get('labels') ?? ''}
		initialExcludeLabels={page.url.searchParams.get('excludeLabels') ?? ''}
		onSubmit={goToDiffs}
	/>
```

`goToDiffs` calls `goto(resolve(...))` (`$app/navigation` + `$app/paths`) with the new query string, which
re-triggers `+page.ts`'s `load()` and the `$effect` with the new `targetRef`. `+page.ts` itself is unchanged —
it already only ever read `labels`/`excludeLabels`, never `targetRef`, so its one `/api/applications` GET stays
unconditional (it's cheap; only the N-way diff fan-out needed gating).

One iteration correction: the first version of this also showed `ApplicationsForm` side-by-side with
`DiffsForm` on this route (mirroring the home page's two-up layout), on the theory that someone landing on an
empty diffs page might actually want to browse applications instead. Feedback was explicit that each route
should show only its own form — Diffs shows only `DiffsForm`. `ApplicationsForm` was removed from this route
entirely.

## Applications page

`routes/applications/+page.svelte` gains a `hasSearched` state, seeded once from the URL:

```ts
let hasSearched: boolean = $state(
	untrack(
		() =>
			page.url.searchParams.get('searched') === 'true' ||
			page.url.searchParams.get('labels') !== null ||
			page.url.searchParams.get('excludeLabels') !== null
	)
);
```

The `searched` marker exists because `labels`/`excludeLabels` are legitimately optional filters:
`buildQuery` (`lib/backend/url.ts`) drops empty/falsy values entirely, so a search submitted with both fields
blank produces a URL with no query string at all — indistinguishable from a bare nav-link click without an
explicit marker. This surfaced as a real bug during implementation: the home page's `ApplicationsForm`
originally navigated to `/applications${buildQuery({ labels, excludeLabels })}`, so submitting it with both
fields empty landed back on the gate instead of showing results. Fixed by always including `searched: 'true'`
in that `buildQuery` call (mirroring the page's own pre-existing `refresh=true` marker convention) — both the
home page's submit and the applications page's own re-submit (`search()`, which also does a `goto()` for
shareable/bookmarkable URLs) include it.

Template: `{#if !hasSearched}<ApplicationsForm onSubmit={search} />{:else}` wraps the existing refresh
controls, loading skeleton, and `ApplicationGrid` — all unchanged, just conditionally rendered. `+page.ts`'s
`load()` is untouched and still fetches unconditionally; the single GET this triggers even while the form is
showing is inexpensive and is the correct data to display immediately if the user does submit an empty search
(no re-fetch needed in that case, since `goto()` to an unchanged URL is a no-op navigation and the
already-resolved `data.applications` is exactly right).

## Tests

`routes/applications/page.svelte.test.ts` and `routes/diffs/page.svelte.test.ts` both moved from a static
`vi.mock('$app/state', ...)` to a `vi.hoisted` mutable `pageState` object, so individual tests can set the
mocked URL they need (e.g. with or without `targetRef`/`searched`) without re-mocking the module. New coverage
added for: the form rendering by default, the submit button's disabled state, navigation with the submitted
query string (including the all-empty-fields case for Applications), and the pre-existing "arrives with
filters already in the URL, skip straight to results" path.

## Verification

`npm run check`, `npm run lint`, `npm run test:unit -- --run` (82 tests), `npm run test:e2e`, plus manual
verification via `npm run dev` against the real local backend (`tangle-server` + the `task services` ArgoCD
environment) confirming: bare `/diffs/` shows the form and fires zero `POST .../diffs` requests; bare
`/applications/` shows the form; submitting either form with real or empty filters produces the expected
results and query string; arriving from the home page with real filters skips straight to results on both
routes.
