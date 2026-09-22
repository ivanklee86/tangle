<script lang="ts">
	import {
		Button,
		Search,
		Select,
		Table,
		TableBody,
		TableBodyCell,
		TableBodyRow,
		TableHead,
		TableHeadCell,
		Toggle
	} from 'flowbite-svelte';
	import { ArrowRightOutline, ArrowUpRightFromSquareOutline } from 'flowbite-svelte-icons';
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount, untrack } from 'svelte';
	import {
		attentionCount,
		countBy,
		emptyFacets,
		filterApplications,
		flattenApplications,
		hasActiveFacets,
		toggleFacet
	} from '$lib/ui/applications';
	import { applicationsHref, diffsHref, isEmptyQuery, type Query } from '$lib/ui/query';
	import {
		ariaSort as ariaSortFor,
		nextSortState,
		sortApplications,
		sortIndicator as sortIndicatorFor,
		type SortKey,
		type SortState
	} from '$lib/ui/sort';
	import {
		ArgoCDHealthStatus,
		ArgoCDSyncStatus,
		ErrorAlert,
		QueryBar,
		QueryDrawer
	} from '$lib/ui/components';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let query = $derived(data.query);
	// Open straight away when there's nothing to show — the drawer is the
	// page's empty state, so there's no second full-page form to maintain.
	// untrack: a one-time seed from the load data, not a live binding — a
	// later navigation re-runs load and remounts, so this doesn't need to
	// track. Without it Svelte warns that only the initial value is captured.
	let editing: boolean = $state(untrack(() => isEmptyQuery(data.query)));

	let facets = $state(emptyFacets());
	let sortState: SortState | undefined = $state({ key: 'health', direction: 'desc' });

	let refreshEnabled: boolean = $state(false);
	let refreshPeriod: number = $state(10);
	const refreshPeriods = [
		{ value: 5, name: '5 s' },
		{ value: 10, name: '10 s' },
		{ value: 30, name: '30 s' },
		{ value: 60, name: '60 s' }
	];

	function applyQuery(next: Query): void {
		goto(resolve(applicationsHref(next) as '/applications'));
	}

	function toggleSort(key: SortKey): void {
		sortState = nextSortState(sortState, key);
	}

	onMount(() => {
		let interval: ReturnType<typeof setInterval>;

		const start = () => {
			clearInterval(interval);
			interval = setInterval(() => {
				if (refreshEnabled) invalidateAll();
			}, refreshPeriod * 1000);
		};

		start();
		$effect(start);

		return () => clearInterval(interval);
	});
</script>

<svelte:head>
	<title>Applications | Tangle</title>
</svelte:head>

{#await data.applications}
	<QueryBar title="Applications" {query} onEdit={() => (editing = true)}>
		{#snippet summary()}Loading…{/snippet}
	</QueryBar>
	<!--
		The header and toolbar come from the URL, so the query is readable
		before any data arrives; only the rows are placeholders. A spinner
		would replace the whole page with something that says less.
	-->
	<div class="p-6" role="status">
		<!--
			Real text, not just an aria-label: a live region announces its
			contents, so a label alone leaves a screen reader with nothing to
			read when the region appears.
		-->
		<span class="sr-only">Loading applications</span>
		<div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
			{#each [0, 1, 2, 3, 4, 5] as rowIndex (rowIndex)}
				<div
					class="flex items-center gap-6 border-b border-gray-200 px-4 py-3.5 last:border-0 dark:border-gray-700"
				>
					<div class="h-2.5 w-32 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
					<div class="h-5 w-11 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
					<div class="h-5 w-20 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
					<div class="h-5 w-20 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
				</div>
			{/each}
		</div>
	</div>
{:then applications}
	{@const rows = applications ? flattenApplications(applications.response.results) : []}
	{@const needing = attentionCount(rows)}
	{@const visible = sortApplications(filterApplications(rows, facets), sortState)}

	<QueryBar title="Applications" {query} onEdit={() => (editing = true)}>
		{#snippet summary()}
			{#if !applications}
				Pick labels to search every configured Argo CD instance.
			{:else}
				{rows.length}
				{rows.length === 1 ? 'application' : 'applications'} across
				{applications.response.results.length}
				{applications.response.results.length === 1 ? 'instance' : 'instances'}
				{#if needing > 0}
					· <span class="text-yellow-600 dark:text-yellow-300">{needing} need attention</span>
				{/if}
			{/if}
		{/snippet}
		{#snippet actions()}
			{#if applications && rows.length > 0}
				<!--
					Replaces the old per-row Diff button: diffing one application at a
					time was never the job, and this carries the whole query across so
					the Diffs page opens on the same set.
				-->
				<Button size="sm" color="primary" href={diffsHref(query)}>
					Diff these applications<ArrowRightOutline class="ms-1.5 h-4 w-4" />
				</Button>
			{/if}
		{/snippet}
	</QueryBar>

	{#if applications?.error}
		<div class="p-6"><ErrorAlert message={applications.errorResponse?.error} /></div>
	{:else if applications}
		<div class="flex flex-wrap items-center gap-3 px-6 pt-4 pb-3">
			<div
				class="inline-flex overflow-hidden rounded-lg border border-gray-300 dark:border-gray-600"
			>
				<button
					type="button"
					aria-pressed={facets.attentionOnly}
					class="h-9 border-e border-gray-300 px-3 text-sm font-semibold dark:border-gray-600 {facets.attentionOnly
						? 'bg-primary-600 text-white'
						: 'bg-white text-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
					onclick={() => (facets.attentionOnly = true)}
				>
					Needs attention {needing}
				</button>
				<button
					type="button"
					aria-pressed={!facets.attentionOnly}
					class="h-9 px-3 text-sm font-semibold {!facets.attentionOnly
						? 'bg-primary-600 text-white'
						: 'bg-white text-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
					onclick={() => (facets.attentionOnly = false)}
				>
					All {rows.length}
				</button>
			</div>

			{#each [{ label: 'Health', field: 'health' as const, set: 'health' as const }, { label: 'Sync', field: 'syncStatus' as const, set: 'sync' as const }] as facet (facet.set)}
				{@const options = countBy(rows, facet.field)}
				{#if options.length > 1}
					<div class="flex items-center gap-2">
						<span
							class="text-xs font-bold tracking-wider text-gray-500 uppercase dark:text-gray-400"
							>{facet.label}</span
						>
						<div
							role="group"
							aria-label="Filter by {facet.label.toLowerCase()}"
							class="inline-flex overflow-hidden rounded-lg border border-gray-300 dark:border-gray-600"
						>
							{#each options as option (option.value)}
								<button
									type="button"
									aria-pressed={facets[facet.set].has(option.value)}
									class="h-9 border-e border-gray-300 px-3 text-sm last:border-e-0 dark:border-gray-600 {facets[
										facet.set
									].has(option.value)
										? 'bg-primary-600 text-white'
										: 'bg-white text-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
									onclick={() => (facets[facet.set] = toggleFacet(facets[facet.set], option.value))}
								>
									{option.value}
									<span class="opacity-70">{option.count}</span>
								</button>
							{/each}
						</div>
					</div>
				{/if}
			{/each}

			<div class="grow"></div>

			<Search
				size="md"
				class="w-56"
				placeholder="Filter by name"
				aria-label="Filter by application name"
				bind:value={facets.name}
			/>
			<Toggle bind:checked={refreshEnabled}>Auto-refresh</Toggle>
			<Select
				class="w-24"
				aria-label="Refresh period"
				items={refreshPeriods}
				bind:value={refreshPeriod}
				disabled={!refreshEnabled}
			/>
		</div>

		<div class="px-6 pb-6">
			<div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
				<div class="max-h-[60vh] overflow-y-auto">
					<Table hoverable>
						<TableHead class="sticky top-0 z-10">
							{#each [{ label: 'Application', key: 'name' as const }, { label: 'Argo CD', key: 'instance' as const }, { label: 'Health', key: 'health' as const }, { label: 'Sync', key: 'syncStatus' as const }] as column (column.key)}
								<TableHeadCell aria-sort={ariaSortFor(sortState, column.key)}>
									<button
										type="button"
										class="cursor-pointer select-none"
										onclick={() => toggleSort(column.key)}
										>{column.label}{sortIndicatorFor(sortState, column.key)}</button
									>
								</TableHeadCell>
							{/each}
							<TableHeadCell>Live ref</TableHeadCell>
							<TableHeadCell class="text-right"><span class="sr-only">Actions</span></TableHeadCell>
						</TableHead>
						<TableBody class="divide-y">
							{#each visible as application (`${application.instance}/${application.name}`)}
								<TableBodyRow>
									<TableBodyCell class="font-semibold">{application.name}</TableBodyCell>
									<TableBodyCell>
										<span
											class="rounded bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700 dark:bg-gray-900 dark:text-gray-300"
											>{application.instance}</span
										>
									</TableBodyCell>
									<TableBodyCell
										><ArgoCDHealthStatus healthStatus={application.health} /></TableBodyCell
									>
									<TableBodyCell
										><ArgoCDSyncStatus syncStatus={application.syncStatus} /></TableBodyCell
									>
									<TableBodyCell class="font-mono text-xs text-gray-500 dark:text-gray-400"
										>{application.liveRef}</TableBodyCell
									>
									<TableBodyCell class="text-right whitespace-nowrap">
										<Button
											size="xs"
											color="alternative"
											href={application.url}
											target="_blank"
											rel="external"
										>
											Open in Argo CD<ArrowUpRightFromSquareOutline class="ms-1.5 h-3 w-3" />
										</Button>
									</TableBodyCell>
								</TableBodyRow>
							{/each}
						</TableBody>
					</Table>

					{#if visible.length === 0}
						<div class="px-6 py-12 text-center">
							{#if rows.length === 0}
								<h2 class="text-lg font-bold">No applications match this query</h2>
								<p class="mx-auto mt-2 max-w-md text-sm text-gray-500 dark:text-gray-400">
									Nothing across {applications.response.results.length} Argo CD
									{applications.response.results.length === 1 ? 'instance' : 'instances'} carries
									{#if query.labels}<code class="font-mono">{query.labels}</code>{:else}any label{/if}
									{#if query.excludeLabels}without
										<code class="font-mono">{query.excludeLabels}</code>{/if}.
								</p>
								<Button class="mt-5" color="primary" onclick={() => (editing = true)}
									>Edit query</Button
								>
							{:else}
								<!--
									Filters hiding every row is a different state from the
									query matching nothing: the query is fine, the toolbar
									is the thing to undo.
								-->
								<p class="text-sm text-gray-500 dark:text-gray-400">
									No applications match the filters in the toolbar.
								</p>
								<Button
									class="mt-4"
									size="sm"
									color="alternative"
									onclick={() => (facets = emptyFacets())}>Clear filters</Button
								>
							{/if}
						</div>
					{/if}
				</div>

				{#if rows.length > 0}
					<div
						class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 bg-gray-50 px-4 py-2.5 text-xs text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
					>
						<span>
							Showing <strong class="text-gray-900 dark:text-white">{visible.length}</strong> of
							{rows.length}
							<!--
								Only when there's still something on screen: with every row
								filtered out the empty state above already offers this, and
								two buttons with the same name is a worse answer than one.
							-->
							{#if hasActiveFacets(facets) && visible.length > 0}
								· <button type="button" class="underline" onclick={() => (facets = emptyFacets())}
									>Clear filters</button
								>
							{/if}
						</span>
						<span>Rows update in place on refresh; order changes only when you sort or filter</span>
					</div>
				{/if}
			</div>
		</div>
	{/if}
{/await}

<QueryDrawer
	bind:open={editing}
	title="Edit query"
	description="Applying searches every configured Argo CD instance."
	{query}
	targetRefMode="hidden"
	applyLabel="See applications"
	onApply={applyQuery}
/>
