# Svelte frontend `$lib` refactor

Status: proposed · 2026-09-19

Extract duplicated/untestable logic out of `web/`'s Svelte views and `TangleAPIClient` into plain `.ts` modules under `web/src/lib`, with a vitest unit test landing alongside each extraction. See [ADR 0004](../../adrs/0004-extract-shared-frontend-logic-into-lib.md) for why this is one PR of incremental extractions rather than a bigger rewrite, and why it doesn't touch SvelteKit `load()` functions (that's a follow-on).

Land the six workstreams below in order, one commit per workstream within a single PR, matching this repo's existing convention ([svelte-e2e-testing.md](svelte-e2e-testing.md)).

**Layout note**: the workstreams below describe each new module as `web/src/lib/<name>.ts` for readability. In the landed implementation these ended up sorted into `web/src/lib/backend/` (`client.ts`, `http.ts`, `data.ts`, `diffs.ts`, `url.ts`) and `web/src/lib/ui/` (`components/`, `sort.ts`, `validation.ts`, `utils.ts`), per [ADR 0004](../../adrs/0004-extract-shared-frontend-logic-into-lib.md)'s consequences section — read `$lib/http` below as `$lib/backend/http`, `$lib/sort` as `$lib/ui/sort`, and so on.

## 1. `lib/http.ts` — shared response-envelope helper

**Problem**: `client.ts`'s `getApplications` and `getApplicationDiff` each hand-build the same `{response, errorResponse, error, loaded}` envelope — fetch, parse JSON, branch on `response.status !== 200`, catch and rebuild the same shape on a thrown error — with the only real difference being the response type and what an "empty" response looks like. The catch blocks also mistype the error as `error as string` instead of extracting a message.

**New file** `web/src/lib/http.ts`:

```ts
import { type TangleError } from '$lib/data';

interface Envelope<T> {
	response: T;
	errorResponse: TangleError;
	error: boolean;
	loaded: boolean;
}

async function fetchEnvelope<T>(
	url: string,
	emptyResponse: T,
	options?: RequestInit
): Promise<Envelope<T>> {
	try {
		const response = await fetch(url, options);
		const data = await response.json();

		if (response.status !== 200) {
			return { response: emptyResponse, errorResponse: data as TangleError, error: true, loaded: true };
		}

		return { response: data as T, errorResponse: { error: '' }, error: false, loaded: true };
	} catch (error) {
		return {
			response: emptyResponse,
			errorResponse: { error: error instanceof Error ? error.message : String(error) },
			error: true,
			loaded: true
		};
	}
}

export { fetchEnvelope, type Envelope };
```

**`client.ts` changes**: both methods call `fetchEnvelope`, then attach the shape-specific extra field (`getApplicationDiff` spreads in `requestDetails`). Existing `console.error` calls on the catch path are dropped since `fetchEnvelope` already turns a thrown error into the same `errorResponse.error` shape the caller renders — no information is lost, just no longer duplicated per call site.

**Test** `web/src/lib/http.spec.ts`: mock global `fetch` (vitest `vi.stubGlobal('fetch', ...)`); cases — 200 response returns `{response: data, error: false, loaded: true}`; non-200 response returns `{response: emptyResponse, errorResponse: data, error: true}`; a rejected fetch returns `{error: true, errorResponse: {error: <message>}}`; a thrown non-`Error` value is stringified rather than producing `"[object Object]"` via a raw cast.

**Rollback**: revert `http.ts` and the `client.ts` hunk; nothing else depends on it yet.

## 2. `lib/url.ts` — shared query-string builder

**Problem**: query-string building with manual `?`/`&` separator tracking is implemented three times — `client.ts` (`labels`/`excludeLabels`) and twice in `routes/+page.svelte` (`redirectToApplications`, `redirectToDiff`, the latter also handling `targetRef`).

**New file** `web/src/lib/url.ts`:

```ts
function buildQuery(params: Record<string, string | null | undefined>): string {
	const search = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (value) search.set(key, value);
	}
	const query = search.toString();
	return query ? `?${query}` : '';
}

export { buildQuery };
```

Order of keys in the `params` object is preserved by `URLSearchParams`/object iteration, so callers control param order by the order they list keys (matches current output: `redirectToDiff` lists `targetRef` before `labels`/`excludeLabels`).

**Usage**: `client.ts` — `` `${this.baseUrl}${PATH_APPLICATIONS}${buildQuery({ labels, excludeLabels })}` ``. `routes/+page.svelte` — both redirect functions build their URL as `` `${BASE_URL}${buildQuery({ targetRef, labels, excludeLabels })}` `` (only `redirectToDiff` passes `targetRef`).

**Test** `web/src/lib/url.spec.ts`: empty params returns `''`; one param returns `?key=value`; multiple params joined with `&` in insertion order; `null`/`undefined`/`''` values are omitted, matching the current `.length > 0` guards.

**Rollback**: revert `url.ts` and its two call sites.

## 3. `lib/validation.ts` — label-format validation

**Problem**: `LABEL_CHECK` and the "only validate if non-empty" check are duplicated between `redirectToApplications` and `redirectToDiff` in `routes/+page.svelte`.

**New file** `web/src/lib/validation.ts`:

```ts
const LABEL_FORMAT: RegExp = /^[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*$/;

function isValidLabelFormat(value: string): boolean {
	return value.length === 0 || LABEL_FORMAT.test(value);
}

export { isValidLabelFormat };
```

**Usage**: both redirect functions replace `labels.length != 0 && !LABEL_CHECK.test(labels)` with `!isValidLabelFormat(labels)`.

**Test** `web/src/lib/validation.spec.ts`: empty string is valid; `foo:bar` valid; `foo:bar,baz:qux` valid; missing colon, empty segment, and stray comma are invalid.

**Rollback**: revert `validation.ts` and its two call sites in `+page.svelte`.

## 4. `lib/diffs.ts` — diff fan-out and aggregation

**Problem**: `routes/diffs/+page.svelte`'s `onMount` builds a flat list of `(argoCD, application)` pairs, fires `getApplicationDiff` for each, tracks `progress`/`total`, and folds the flat results into the nested `{argoCDName: {applicationName: ApplicationDiff}}` shape by hand, inline in the component. None of this is reachable by a unit test today.

**New file** `web/src/lib/diffs.ts`:

```ts
import { type ArgoCDApplicationResults, type ApplicationDiff, type ApplicationsDiffsData } from '$lib/data';
import type TangleAPIClient from '$lib/client';

interface DiffRequest {
	argoCDName: string;
	applicationName: string;
	liveRef: string;
	targetRef: string;
}

function buildDiffRequests(
	results: ArgoCDApplicationResults[],
	targetRef: string | null
): DiffRequest[] {
	return results.flatMap((argoCD) =>
		argoCD.applications.map((application) => ({
			argoCDName: argoCD.name,
			applicationName: application.name,
			liveRef: application.liveRef,
			targetRef: targetRef ? targetRef : application.liveRef
		}))
	);
}

function buildDiffMap(diffs: ApplicationDiff[]): ApplicationsDiffsData {
	const map: ApplicationsDiffsData = {};
	diffs.forEach((diff) => {
		const { argoCD, applicationName } = diff.requestDetails;
		if (!map[argoCD]) map[argoCD] = {};
		map[argoCD][applicationName] = diff;
	});
	return map;
}

interface FetchDiffsCallbacks {
	onTotal?: (total: number) => void;
	onProgress?: () => void;
}

async function fetchDiffs(
	client: TangleAPIClient,
	results: ArgoCDApplicationResults[],
	targetRef: string | null,
	callbacks: FetchDiffsCallbacks = {}
): Promise<ApplicationsDiffsData> {
	const requests = buildDiffRequests(results, targetRef);
	callbacks.onTotal?.(requests.length);

	const diffs = await Promise.all(
		requests.map((request) =>
			client
				.getApplicationDiff(request.argoCDName, request.applicationName, request.liveRef, request.targetRef)
				.then((result) => {
					callbacks.onProgress?.();
					return result;
				})
		)
	);

	return buildDiffMap(diffs);
}

export { buildDiffRequests, buildDiffMap, fetchDiffs, type DiffRequest, type FetchDiffsCallbacks };
```

**`routes/diffs/+page.svelte` changes**: `onMount`'s body becomes `applicationsData.set(result); diffData.set(await fetchDiffs(client, result.response.results, targetRef, { onTotal: (n) => (total = n), onProgress: () => (progress += 1) }));` — same reactive `progress`/`total` `$state` driving the existing `Progressbar`, no template changes. `reloadDiff` is unchanged (it's already a single `client.getApplicationDiff` call with no duplication to extract).

**Test** `web/src/lib/diffs.spec.ts`: `buildDiffRequests` — flattens two ArgoCDs × two applications into four requests, falls back to `liveRef` when `targetRef` is null/empty; `buildDiffMap` — groups a flat `ApplicationDiff[]` back into the nested per-ArgoCD/per-application shape; `fetchDiffs` — with a stubbed `TangleAPIClient` (object literal satisfying the one method used), asserts `onTotal` fires once with the request count before any diff resolves, `onProgress` fires once per resolved diff, and the returned map matches `buildDiffMap`'s output for the same inputs.

**Rollback**: revert `diffs.ts` and the `onMount` hunk in `routes/diffs/+page.svelte`.

## 5. `lib/sort.ts` — table sort comparator

**Problem**: `ApplicationsGrid.svelte` implements column sorting (`toggleSort`, `sortIndicator`, `ariaSort`, `sortedApplications`) inline, operating on a `Record<string, SortState | undefined>` keyed by tab name. The comparator and state-transition logic are plain, testable logic currently reachable only by mounting the component.

**New file** `web/src/lib/sort.ts`:

```ts
import { type ApplicationLinks } from '$lib/data';

type SortKey = 'name' | 'health' | 'syncStatus';
type SortDirection = 'asc' | 'desc';
type SortState = { key: SortKey; direction: SortDirection };

function nextSortState(current: SortState | undefined, key: SortKey): SortState {
	if (current?.key === key) {
		return { key, direction: current.direction === 'asc' ? 'desc' : 'asc' };
	}
	return { key, direction: 'asc' };
}

function sortIndicator(state: SortState | undefined, key: SortKey): string {
	if (state?.key !== key) return '';
	return state.direction === 'asc' ? ' ▲' : ' ▼';
}

function ariaSort(state: SortState | undefined, key: SortKey): 'ascending' | 'descending' | 'none' {
	if (state?.key !== key) return 'none';
	return state.direction === 'asc' ? 'ascending' : 'descending';
}

function sortApplications(
	applications: ApplicationLinks[],
	state: SortState | undefined
): ApplicationLinks[] {
	if (!state) return applications;
	const sorted = [...applications].sort((a, b) => a[state.key].localeCompare(b[state.key]));
	return state.direction === 'asc' ? sorted : sorted.reverse();
}

export { nextSortState, sortIndicator, ariaSort, sortApplications, type SortKey, type SortState };
```

**`ApplicationsGrid.svelte` changes**: keeps its own `sortState: Record<string, SortState | undefined> = $state({})` (the per-tab `$state` itself stays component-local — it's view state, not shared logic), but `toggleSort`, `sortIndicator`, `ariaSort`, and `sortedApplications` become thin wrappers calling into `lib/sort.ts`, e.g. `function toggleSort(tabName: string, key: SortKey) { sortState[tabName] = nextSortState(sortState[tabName], key); }`.

**Test** `web/src/lib/sort.spec.ts`: `nextSortState` — no prior state defaults to `asc`; same key toggles `asc`→`desc`→`asc`; different key resets to `asc`; `sortApplications` — ascending/descending by each of `name`/`health`/`syncStatus`, undefined state returns the input array unchanged (and doesn't mutate it); `sortIndicator`/`ariaSort` — correct glyph/ARIA value per state, `''`/`'none'` when the state is for a different key.

**Rollback**: revert `sort.ts` and the `ApplicationsGrid.svelte` hunk.

## 6. Cleanup: dead stores, `filterOutZeroResults` coverage, minor fixes

- **`data.ts`**: replace the unused `apiData`/`diffData` writable exports with plain factory functions `emptyApplicationResponseStore(): ApplicationResponseStore` and `emptyApplicationDiff(): ApplicationDiff`, returning the same default object literals those two stores were initialized with. Use these factories everywhere a fresh default value is currently written inline: `ApplicationsGrid.svelte`'s `writable<ApplicationResponseStore>({...})` (both the initial value and the reset in `loadApplications`) and `routes/diffs/+page.svelte`'s `applicationsData`/`diffData` initial values.
- **`ApplicationsGrid.svelte` / `routes/diffs/+page.svelte`**: `var client = new TangleAPIClient();` → `const client = new TangleAPIClient();` (both files).
- **`web/src/demo.spec.ts`**: delete — it's the placeholder "1 + 2 = 3" test; by the end of this plan there are real unit tests (`http.spec.ts`, `url.spec.ts`, `validation.spec.ts`, `diffs.spec.ts`, `sort.spec.ts`) proving the vitest setup works.
- **`web/src/lib/utils.spec.ts`** (new): `filterOutZeroResults` has no test today despite predating this plan — add one while touching this area: an `ArgoCDApplicationResults[]` mixing empty and non-empty `applications` arrays is filtered down to only the non-empty ones.

**Rollback**: revert the `data.ts`/`ApplicationsGrid.svelte`/`diffs/+page.svelte` hunks; restore `demo.spec.ts` if desired (its removal is independent of everything else in this workstream).

## Sequencing notes

- Workstreams 1-3 (`http.ts`, `url.ts`, `validation.ts`) are independent of each other and of 4-5; do them in any order, but numbered order keeps `client.ts` (touched by 1 and 2) settled before the route/component workstreams.
- Workstream 4 (`diffs.ts`) and workstream 5 (`sort.ts`) touch different files (`routes/diffs/+page.svelte` vs `ApplicationsGrid.svelte`) and don't depend on each other.
- Workstream 6 touches `data.ts`, which workstreams 4-5 don't depend on, so it can land last without blocking anything.
- After each workstream: `npm run check` (svelte-check + TS) and `npm run test:unit -- --run` should stay green; run `npm run lint` once at the end of the PR rather than after every commit.
- This plan doesn't touch `routes/+page.svelte`'s template, `ApplicationsGrid.svelte`'s template, or `routes/diffs/+page.svelte`'s template — only `<script>` block internals — so no visual/behavioral change is expected; the existing (not-yet-implemented) ADR-0003 e2e suite, once landed, is the right place to confirm that holds.
