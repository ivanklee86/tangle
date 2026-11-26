# SvelteKit `load()` migration

Status: proposed · 2026-09-19

Move `web/`'s two data-fetching routes (`/applications`, `/diffs`) from `onMount`-driven fetching to SvelteKit `load()` functions, per [ADR 0006](../../adrs/0006-sveltekit-load-functions.md). This is ADR-0004's deferred "workstream 7," now picked up with [ADR 0005](../../adrs/0005-svelte-component-unit-tests.md)'s test suite in place as a regression net for the logic layers — plus new route-level tests added here to cover the wiring logic this migration reintroduces into `.svelte` files.

Land the five workstreams below in order; each is independently useful and testable before moving to the next.

## 1. Injectable `fetch` on `TangleAPIClient`/`fetchEnvelope`

**`lib/backend/http.ts`**: `fetchEnvelope` gains a fourth parameter, defaulting to global `fetch`:

```ts
async function fetchEnvelope<T>(
	url: string,
	emptyResponse: T,
	options?: RequestInit,
	fetchImpl: typeof fetch = fetch
): Promise<Envelope<T>> {
	try {
		const response = await fetchImpl(url, options);
		// ...unchanged...
```

**`lib/backend/client.ts`**: constructor takes an optional `fetch`, stored and threaded through both methods:

```ts
class TangleAPIClient {
	baseUrl: string;
	private fetchImpl: typeof fetch;

	constructor(fetchImpl: typeof fetch = fetch) {
		this.baseUrl = PUBLIC_BASE_URL;
		this.fetchImpl = fetchImpl;
	}

	async getApplications(...): Promise<ApplicationResponseStore> {
		const url = ...;
		return fetchEnvelope<ApplicationsResponse>(url, EMPTY_APPLICATIONS_RESPONSE, undefined, this.fetchImpl);
	}

	async getApplicationDiff(...): Promise<ApplicationDiff> {
		const url = ...;
		const envelope = await fetchEnvelope<ApplicationDiffResponse>(url, EMPTY_APPLICATION_DIFF_RESPONSE, {...}, this.fetchImpl);
		return { ...envelope, requestDetails: { argoCD, applicationName } };
	}
}
```

Existing call sites (`new TangleAPIClient()` in `ApplicationsGrid.svelte`, `routes/diffs/+page.svelte`) are unaffected — the default parameter preserves current behavior exactly.

**Tests**: existing `http.spec.ts`/`client.spec.ts` keep using `vi.stubGlobal('fetch', ...)` (still valid — proves the default works). Add one new case to each: a custom `fetchImpl` passed explicitly is the one actually called, not global `fetch` (stub global fetch to throw/fail, pass a working mock as `fetchImpl`, assert success) — proves the injection point actually works, not just that the signature compiles.

**Rollback**: revert `http.ts`/`client.ts`/their spec additions; nothing else in this plan has landed yet to depend on it.

## 2. `/applications`: `+page.ts` + presentational `ApplicationsGrid`

**New file** `web/src/routes/applications/+page.ts`:

```ts
import TangleAPIClient from '$lib/backend/client';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const client = new TangleAPIClient(fetch);

	// Deliberately not awaited — streamed, so the page can show a loading
	// state via {#await} while it resolves, matching today's spinner UX.
	return {
		applications: client.getApplications(
			url.searchParams.get('labels'),
			url.searchParams.get('excludeLabels')
		)
	};
};
```

**`routes/applications/+page.svelte` changes**: gains `let { data }: PageProps = $props();` (SvelteKit's current `PageProps` convention for this kit version — confirm it's available at implementation time; fall back to `{ data }: { data: PageData }` from `./$types` if not), moves the refresh-interval logic up from `ApplicationsGrid.svelte` (same `onMount` + `setInterval` + `$effect`-on-period-change shape as today, just calling `invalidateAll()` from `$app/navigation` instead of a local `loadApplications()`), and replaces `<ApplicationGrid refresh={...} refreshPeriod={...} />` with:

```svelte
{#await data.applications}
	<br />
	<div class="text-center"><Spinner /></div>
{:then applications}
	<ApplicationGrid {applications} />
{/await}
```

(`Spinner` import moves from `ApplicationsGrid.svelte` to this file.)

**`lib/ui/components/ApplicationsGrid.svelte` changes**: becomes purely presentational. Removed entirely: `onMount`, the `writable` store + `emptyApplicationResponseStore` import, `TangleAPIClient` import/instantiation, `$app/stores` import, `loadApplications`, the refresh/refreshPeriod props and interval. New props: `{ applications: ApplicationResponseStore }`. Every `$applicationsData` reference in the template becomes `applications` (no store subscription needed — it's a plain prop now). Sort state/helpers (`sortState`, `toggleSort`, `sortIndicator`, `ariaSort`, `sortedApplications`) are unchanged — they never depended on fetching.

**`ApplicationsGrid.svelte.test.ts` rewrite**: this is where the migration pays for itself in test simplicity. Drop `vi.mock('$app/stores', ...)` and the `fetch` mocking entirely — tests become `render(ApplicationsGrid, { applications: <fixture> })` with no async fetch-resolution dance. Keep the same four cases from the current test (tabs-per-ArgoCD + count suffix + `filterOutZeroResults`, sort-by-column-header, error alert) minus the "spinner before fetch resolves" case, which moves to workstream 5's new route-level test (the spinner is now the route's responsibility, not the component's).

**Rollback**: revert `+page.ts`, `+page.svelte`, `ApplicationsGrid.svelte`, and its test; workstream 1 stands alone without this.

## 3. `/diffs`: `+page.ts` for the applications fetch, diff fan-out stays client-side

**New file** `web/src/routes/diffs/+page.ts`:

```ts
import TangleAPIClient from '$lib/backend/client';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url, fetch }) => {
	const client = new TangleAPIClient(fetch);

	return {
		applications: client.getApplications(
			url.searchParams.get('labels'),
			url.searchParams.get('excludeLabels')
		)
	};
};
```

**`routes/diffs/+page.svelte` changes**: the applications fetch moves out of `onMount`'s direct `client.getApplications()` call into `data.applications` (the `load()`-sourced promise); the diff fan-out (`fetchDiffs`, `progress`/`total` state) stays as client-side orchestration.

**Corrected during implementation (2026-09-19)**: the plan's original design called the diff fan-out directly as a nested `{#await loadDiffs(applications)}` expression. This does not work — Svelte 5 throws `state_unsafe_mutation` ("Updating state inside... a template expression is forbidden") because `loadDiffs` synchronously calls `fetchDiffs`, which synchronously invokes the `onTotal` callback (mutating `total`) *before* its first `await` — and that whole synchronous prefix runs while Svelte is still evaluating the `{#await EXPR}` block's `EXPR`, which Svelte treats as an unsafe context for state mutation. This is exactly the `$effect`-driven fallback the original plan flagged as a possibility, now confirmed as the only working option: trigger the fan-out from `onMount`, not from a template expression position. Structurally, what actually shipped:

```svelte
<script lang="ts">
	// ...same imports, plus onMount...
	let { data }: PageProps = $props();

	const targetRef = $page.url.searchParams.get('targetRef');
	const client = new TangleAPIClient(); // separate instance from +page.ts's — this one is for
	                                       // reloadDiff and the diff fan-out, not page-load data

	let applications: ApplicationResponseStore | undefined = $state();
	let progress: number = $state(0);
	let total: number = $state(0);
	let diffsLoaded: boolean = $state(false);
	const diffData = writable<ApplicationsDiffsData>({});

	onMount(() => {
		data.applications.then(async (result) => {
			applications = result;
			if (result.error) return;

			const diffDataMap = await fetchDiffs(client, result.response.results, targetRef, {
				onTotal: (n) => (total = n),
				onProgress: () => (progress += 1)
			});
			diffData.set(diffDataMap);
			diffsLoaded = true;
		});
	});

	function reloadDiff(...) { /* unchanged */ }
</script>

{#if !applications}
	<div class="flex justify-center m-10"><P italic>Loading Applications...</P></div>
{:else if applications.error}
	<Alert color="red" class="bg-red-500 text-white">...</Alert>
{:else if !diffsLoaded}
	<div class="flex justify-center m-10"><P italic>Loading Applications...</P></div>
	{#if total > 0}<Progressbar progress={Math.round((progress / total) * 100)} .../>{/if}
{:else}
	<!-- existing Tabs markup, using `applications.response.results` and `$diffData` — unchanged -->
{/if}
```

This is a three-state chained `{#if}` (not yet got applications / applications errored / applications ok but diffs pending / diffs loaded) instead of nested `{#await}` blocks — functionally identical to the original plan's intent (same loading text, same progress bar, same error alert), verified against the pre-migration behavior via a manual browser smoke check (both routes render their error state correctly against a stub server with no real API, no console errors, no crashes). The initial applications fetch still originates from `load()` (independently unit-tested in workstream 4) — only the diff fan-out's trigger mechanism differs from the original plan.

**`reloadDiff`**: unchanged — it was already a standalone imperative action independent of the initial fetch, not part of this migration's scope.

**Rollback**: revert `+page.ts` and the `+page.svelte` hunk; workstreams 1–2 are unaffected.

## 4. Unit tests for the new `load()` functions

**New file** `web/src/routes/applications/load.spec.ts`: SvelteKit reserves any filename starting with `+` under `src/routes` — `+page.spec.ts` fails to load with "Files prefixed with + are reserved" (confirmed during implementation; it also poisoned the shared dev-server with a Vite error overlay that broke an unrelated browser-mode component test running in the same process). Construct a minimal fake `LoadEvent` (only `url` and `fetch` are used — cast the rest away, e.g. `as Parameters<typeof load>[0]`), mock `fetch`, call `load(...)`, await the returned `applications` promise, assert: the URL passed to `fetch` includes `labels`/`excludeLabels` from the input `url`'s search params (reusing the same assertions style as `client.spec.ts`'s query-string tests); the resolved shape matches what `TangleAPIClient.getApplications` would produce for a given mock response.

**New file** `web/src/routes/diffs/load.spec.ts`: same shape, asserting `labels`/`excludeLabels` (not `targetRef` — that's read directly in the component, not in `load()`, since it's only needed for the diff fan-out step that stays client-side).

Both are plain `.spec.ts` files under `src/routes/`, matching the existing `node` Vitest project's `include: ['src/**/*.spec.{js,ts}']` glob from ADR-0005's config split — no new tooling needed.

**Rollback**: revert the two new spec files.

## 5. Route-level component tests for the new wiring

**Why this wasn't in ADR-0005's scope, and why it's back now**: ADR-0005 deliberately left route `.svelte` files untested, reasoning that full-page composition belonged to ADR-0003's e2e suite. This migration reintroduces real wiring logic into both route files (streamed-load consumption, refresh→`invalidateAll`, the diffs page's nested-await progress wiring) that has no test coverage anywhere once workstreams 2–3 land — the removed "spinner before fetch resolves" case from `ApplicationsGrid.svelte.test.ts` (workstream 2) is a concrete example of coverage that would otherwise just disappear. Adding minimal tests for exactly this new wiring, without attempting full page-composition coverage (tab switching, sort-through-the-whole-page, etc. — still deferred to ADR-0003), keeps the regression net honest without re-litigating ADR-0005's broader scoping call.

Same `+`-prefix naming restriction from workstream 4 applies to these too — named `page.svelte.test.ts` (no leading `+`), importing the component from `'./+page.svelte'`.

- **`web/src/routes/applications/page.svelte.test.ts`**: mock `$app/stores`/`$app/navigation` (`invalidateAll` as a `vi.fn()`) and pass `data: { applications: <pending or resolved promise> }` as a prop. Cases: a pending `applications` promise shows the spinner (`getByRole('status')`); a resolved one renders `ApplicationGrid`'s content; clicking the refresh toggle (located via `screen.container.querySelector('button')` — it has no accessible name, since it's icon-only, so role/text locators don't reach it) then advancing a mocked timer (`vi.useFakeTimers()` + `vi.advanceTimersByTimeAsync`) calls `invalidateAll()`.
- **`web/src/routes/diffs/page.svelte.test.ts`**: same mocking approach, plus stub global `fetch` for the diff fan-out step (consistent with how `client.spec.ts` already does it). Cases: pending applications shows the "Loading Applications..." text; resolved applications with an error shows the system-error alert; resolved applications with no error triggers the diff fan-out and eventually renders the tabs (assert final state, not the transient progress bar — that's already covered at the unit level by `diffs.spec.ts`'s `fetchDiffs` tests).

**Rollback**: revert the two new test files; workstreams 2–4 stand without them (at reduced coverage, per the "why this wasn't in scope" note above).

## Sequencing notes

- Do 1 → (2, 3 in either order, independent of each other) → 4 → 5.
- After 2 and after 3: run the full suite (`npm run test:unit -- --run`), `npm run check`, and manually exercise the route in a dev server (`npm run dev`) before moving on — per ADR-0006's consequences, the automated suite doesn't yet cover the route-wiring workstream 5 is about to add, so there's a real gap between "tests pass" and "this route actually works" until workstream 5 lands.
- Workstream 5 is what actually closes the regression-coverage gap this migration reopens — don't treat workstreads 2–4 as "done, tested" without it, even though the existing suite will report green.
- This plan doesn't touch `routes/+page.svelte` (home) or `routes/+layout.svelte` — neither fetches anything, so there's nothing to migrate.
- Commit each workstream separately, matching this repo's existing convention.
