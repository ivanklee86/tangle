<script lang="ts">
	import { Button, Pane, Progressbar, Search, SplitPane } from 'flowbite-svelte';
	import { RefreshOutline } from 'flowbite-svelte-icons';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import {
		type ApplicationDiff,
		type ApplicationResponseStore,
		type ApplicationsDiffsData
	} from '$lib/backend/data';
	import { fetchDiffs } from '$lib/backend/diffs';
	import TangleAPIClient from '$lib/backend/client';
	import {
		buildDiffRows,
		countOutcome,
		filterDiffRows,
		groupByInstance,
		loadedCount,
		resolveSelection,
		rowKey,
		type DiffFilter,
		type DiffRow
	} from '$lib/ui/diffs';
	import { diffsHref, type Query } from '$lib/ui/query';
	import { DiffDetail, ErrorAlert, QueryBar, QueryDrawer } from '$lib/ui/components';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let query = $derived(data.query);
	// Nothing to show without a ref to compare against, so the drawer is this
	// page's empty state too. Labels are not part of the condition: a ref with
	// no labels means "diff everything", which is a real request.
	let editing: boolean = $state(untrack(() => data.applications === undefined));

	// Separate from +page.ts's client — this one drives the diff fan-out and
	// reloadDiff, neither of which is part of the load() lifecycle.
	const client = new TangleAPIClient();

	let applications: ApplicationResponseStore | undefined = $state();
	let diffs: ApplicationsDiffsData = $state({});
	let filter: DiffFilter = $state('all');
	let nameFilter: string = $state('');
	let selectedKey: string | undefined = $state();

	// Bumped whenever the diffs on screen stop belonging to what's being
	// asked for: a new query, a reload of everything, or unmounting. Every
	// fetch captures it when it starts and drops its result if it has moved
	// on, so a slow request for an old target ref can't land under the new
	// one's heading. Deliberately not $state: nothing renders from it.
	let generation = 0;

	// A request was actually made. Without a target ref the load fetches
	// nothing, so there is nothing to wait for.
	let requested = $derived(data.applications !== undefined && data.query.targetRef.length > 0);

	function mergeDiff(diff: ApplicationDiff): void {
		const { argoCD, applicationName } = diff.requestDetails;
		diffs = { ...diffs, [argoCD]: { ...diffs[argoCD], [applicationName]: diff } };
	}

	// Each diff is merged as it lands, so the sidebar fills in while the
	// slowest requests are still running. That also makes the map complete
	// by the time the fan-out finishes, so there's no final wholesale
	// assignment to overwrite a single-diff reload that landed in between.
	function fanOut(results: ApplicationResponseStore['response']['results'], targetRef: string) {
		const current = ++generation;
		diffs = {};
		fetchDiffs(client, results, targetRef, {
			onProgress: (diff) => {
				if (current === generation) mergeDiff(diff);
			}
		});
	}

	// $effect rather than onMount: SvelteKit reuses this component across
	// client-side navigations (a labels or targetRef change) without
	// remounting, so onMount would only ever see the first query.
	$effect(() => {
		const pending = data.applications;
		const targetRef = data.query.targetRef;
		const current = ++generation;

		applications = undefined;
		diffs = {};

		if (!pending || targetRef.length === 0) return;

		pending.then((result) => {
			if (current !== generation) return;
			applications = result;
			if (!result.error) fanOut(result.response.results, targetRef);
		});

		return () => {
			generation += 1;
		};
	});

	function applyQuery(next: Query): void {
		goto(resolve(diffsHref(next) as '/diffs'));
	}

	async function reloadDiff(row: DiffRow): Promise<void> {
		const current = generation;
		const result = await client.getApplicationDiff(
			row.instance,
			row.name,
			row.liveRef,
			query.targetRef
		);
		if (current === generation) mergeDiff(result);
	}

	// Rows exist as soon as the applications call returns and are classified
	// pending until their own diff arrives, so the sidebar fills immediately
	// and progress is derived from the rows rather than counted separately.
	let rows = $derived(
		applications?.error ? [] : buildDiffRows(applications?.response.results ?? [], diffs)
	);
	let loaded = $derived(loadedCount(rows));
	let visible = $derived(filterDiffRows(rows, filter, nameFilter));
	let selected = $derived(resolveSelection(visible, selectedKey));

	function reloadAll(): void {
		if (!applications || applications.error) return;
		fanOut(applications.response.results, query.targetRef);
	}
</script>

<svelte:head>
	<title>Diffs | Tangle</title>
</svelte:head>

{#snippet outcomeDot(row: DiffRow)}
	<span
		aria-hidden="true"
		class="h-2.5 w-2.5 shrink-0 rounded-full {row.outcome === 'error'
			? 'bg-red-500'
			: row.outcome === 'changed'
				? 'bg-yellow-400'
				: row.outcome === 'pending'
					? 'bg-gray-400'
					: 'bg-green-500'}"
	></span>
{/snippet}

<QueryBar title="Diffs" {query} showTargetRef onEdit={() => (editing = true)}>
	{#snippet summary()}
		{#if !applications}
			{#if requested}
				Loading applications…
			{:else}
				Pick labels and a target ref to compare against.
			{/if}
		{:else}
			{rows.length}
			{rows.length === 1 ? 'application' : 'applications'} against
			<span class="font-mono text-gray-900 dark:text-white">{query.targetRef}</span>
			·
			<span class="text-yellow-600 dark:text-yellow-300"
				>{countOutcome(rows, 'changed')} changed</span
			>
			{#if countOutcome(rows, 'error') > 0}
				· <span class="text-red-600 dark:text-red-400"
					>{countOutcome(rows, 'error')}
					{countOutcome(rows, 'error') === 1 ? 'error' : 'errors'}</span
				>
			{/if}
			· {loaded} of {rows.length} loaded
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if applications && !applications.error}
			<Button size="sm" color="alternative" onclick={reloadAll}>
				<RefreshOutline class="me-1.5 h-4 w-4" />Reload all
			</Button>
		{/if}
	{/snippet}
</QueryBar>

{#if applications?.error}
	<div class="p-6"><ErrorAlert message={applications.errorResponse?.error} /></div>
{:else if rows.length > 0}
	<SplitPane class="h-[calc(100vh-11rem)]" initialSizes={[26, 74]} minSize={220}>
		<Pane class="flex min-h-0 flex-col bg-gray-50 dark:bg-gray-800">
			<div class="space-y-2.5 border-b border-gray-200 p-3 dark:border-gray-700">
				<Search
					size="md"
					placeholder="Filter by name"
					aria-label="Filter applications by name"
					bind:value={nameFilter}
				/>
				<div
					role="group"
					aria-label="Filter by diff outcome"
					class="inline-flex overflow-hidden rounded-lg border border-gray-300 dark:border-gray-600"
				>
					{#each [{ id: 'changed' as const, label: 'Changed', count: countOutcome(rows, 'changed') }, { id: 'errors' as const, label: 'Errors', count: countOutcome(rows, 'error') }, { id: 'all' as const, label: 'All', count: rows.length }] as option (option.id)}
						<button
							type="button"
							aria-pressed={filter === option.id}
							class="h-8 border-e border-gray-300 px-2.5 text-xs font-semibold last:border-e-0 dark:border-gray-600 {filter ===
							option.id
								? 'bg-primary-600 text-white'
								: 'bg-white text-gray-700 dark:bg-gray-900 dark:text-gray-300'}"
							onclick={() => (filter = option.id)}
						>
							{option.label}
							{option.count}
						</button>
					{/each}
				</div>
			</div>

			<nav aria-label="Applications" class="min-h-0 grow overflow-y-auto p-2">
				{#each groupByInstance(visible) as group (group.instance)}
					<div class="px-2 pt-2 pb-1">
						<span
							class="text-xs font-bold tracking-wider text-gray-500 uppercase dark:text-gray-400"
							>{group.instance}</span
						>
						<span class="ms-1.5 text-xs text-gray-500 dark:text-gray-400">
							· {group.rows.filter((row) => row.outcome === 'changed').length} changed of
							{group.rows.length}
						</span>
					</div>
					<ul class="list-none space-y-0.5 p-0">
						{#each group.rows as row (rowKey(row))}
							<li>
								<button
									type="button"
									aria-current={selected && rowKey(selected) === rowKey(row) ? 'true' : undefined}
									class="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-start text-sm {selected &&
									rowKey(selected) === rowKey(row)
										? 'bg-gray-200 dark:bg-gray-700'
										: 'hover:bg-gray-100 dark:hover:bg-gray-700/50'}"
									onclick={() => (selectedKey = rowKey(row))}
								>
									{@render outcomeDot(row)}
									<span class="grow truncate text-gray-900 dark:text-gray-100">{row.name}</span>
									<!--
										Each outcome reads differently, so the list can be
										scanned without stopping to parse every row: additions
										and removals in their own colours, a failure called a
										failure, and anything still running kept quiet so it
										doesn't compete with results that are in.
									-->
									<span class="shrink-0 font-mono text-xs">
										{#if row.outcome === 'pending'}
											<span class="text-gray-400 italic dark:text-gray-500">pending</span>
										{:else if row.outcome === 'error'}
											<span class="font-semibold text-red-600 dark:text-red-400">Error</span>
										{:else if row.outcome === 'changed'}
											<span class="text-green-600 dark:text-green-400">+{row.stats.added}</span>
											<span class="text-red-600 dark:text-red-400">−{row.stats.removed}</span>
										{:else}
											<span class="text-gray-400 dark:text-gray-500">No changes</span>
										{/if}
									</span>
								</button>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="p-4 text-sm text-gray-500 dark:text-gray-400">
						No applications match these filters.
					</p>
				{/each}
			</nav>

			{#if loaded < rows.length}
				<div class="border-t border-gray-200 p-3 dark:border-gray-700">
					<Progressbar
						progress={Math.round((loaded / rows.length) * 100)}
						labelOutside="Fetching diffs {loaded} / {rows.length}"
					/>
				</div>
			{/if}
		</Pane>

		<Pane class="min-h-0">
			{#if selected}
				<!--
					Keyed on the selection so the detail pane's tab state resets
					when you move to another application, instead of leaving you on
					"Live manifests" for a diff you just opened.
				-->
				{#key rowKey(selected)}
					<DiffDetail row={selected} targetRef={query.targetRef} onReload={reloadDiff} />
				{/key}
			{/if}
		</Pane>
	</SplitPane>
{:else if applications}
	<div class="px-6 py-16 text-center">
		<h2 class="text-lg font-bold">No applications match this query</h2>
		<p class="mx-auto mt-2 max-w-md text-sm text-gray-500 dark:text-gray-400">
			Nothing to diff against <span class="font-mono">{query.targetRef}</span>.
		</p>
		<Button class="mt-5" color="primary" onclick={() => (editing = true)}>Edit query</Button>
	</div>
{:else if requested}
	<div class="p-6" role="status">
		<span class="sr-only">Loading applications</span>
		<div class="space-y-2">
			{#each [0, 1, 2, 3, 4] as rowIndex (rowIndex)}
				<div class="h-10 animate-pulse rounded-lg bg-gray-200 dark:bg-gray-700"></div>
			{/each}
		</div>
	</div>
{:else if !editing}
	<!--
		The editor was closed without a target ref: nothing was fetched, so
		"loading" would never resolve. Say what's missing instead.
	-->
	<div class="px-6 py-16 text-center">
		<h2 class="text-lg font-bold">Nothing to compare yet</h2>
		<p class="mx-auto mt-2 max-w-md text-sm text-gray-500 dark:text-gray-400">
			Diffs need a target ref to compare each application's live manifests against.
		</p>
		<Button class="mt-5" color="primary" onclick={() => (editing = true)}>Edit query</Button>
	</div>
{/if}

<QueryDrawer
	bind:open={editing}
	title="Edit diff query"
	description="Applying re-renders every matching application."
	{query}
	targetRefMode="required"
	applyLabel="Apply and run diffs"
	hrefFor={diffsHref}
	showCli
	onApply={applyQuery}
/>
