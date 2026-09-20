<script lang="ts">
	import {
		Select,
		Button,
		Tooltip,
		Heading,
		Table,
		TableBody,
		TableBodyCell,
		TableBodyRow
	} from 'flowbite-svelte';
	import { RefreshOutline } from 'flowbite-svelte-icons';
	import { page } from '$app/state';
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { buildQuery } from '$lib/backend/url';
	import { ApplicationGrid, ApplicationsForm } from '$lib/ui/components';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let selectedRefreshPeriod: number = $state(10);

	const refreshParam = page.url.searchParams.get('refresh');
	let refreshEnabled: boolean = $state(refreshParam === 'true');

	// Show the label/exclude-label form until the user explicitly searches —
	// same gate as the Diffs page's target ref, so landing on this tab
	// doesn't run an unfiltered search by default. Skipped entirely when
	// arriving with filters already in the URL (e.g. from the home page).
	// `searched` covers the case where labels/excludeLabels were submitted
	// but left empty — buildQuery drops empty values, so labels/excludeLabels
	// alone can't distinguish "submitted with no filters" from "never submitted".
	let hasSearched = $derived(
		page.url.searchParams.get('searched') === 'true' ||
			page.url.searchParams.get('labels') !== null ||
			page.url.searchParams.get('excludeLabels') !== null
	);

	function search(labels: string, excludeLabels: string): void {
		goto(
			resolve(
				`/applications${buildQuery({ labels, excludeLabels, searched: 'true' })}` as '/applications'
			)
		);
	}

	let refreshPeriods = [
		{ value: 1, name: '1s' },
		{ value: 5, name: '5s' },
		{ value: 10, name: '10s' },
		{ value: 15, name: '15s' }
	];

	onMount(() => {
		let interval: number;

		const startInterval = () => {
			clearInterval(interval); // Clear any existing interval
			interval = setInterval(() => {
				if (refreshEnabled) {
					invalidateAll();
				}
			}, selectedRefreshPeriod * 1000);
		};

		startInterval(); // Start the interval initially

		$effect(() => {
			startInterval(); // Restart the interval whenever refreshPeriod changes
		});

		return () => clearInterval(interval);
	});
</script>

<svelte:head>
	<title>Applications | Tangle</title>
</svelte:head>

<Heading tag="h1" class="sr-only">Applications</Heading>

{#if !hasSearched}
	<ApplicationsForm onSubmit={search} />
{:else}
	<div class="mt-4 flex justify-end gap-2">
		<Select class="w-fit" items={refreshPeriods} bind:value={selectedRefreshPeriod} />
		<Button
			color={refreshEnabled ? 'primary' : 'alternative'}
			aria-label="Toggle automatic refresh"
			onclick={() => (refreshEnabled = !refreshEnabled)}><RefreshOutline /></Button
		>
		<Tooltip>Toggle automatic refresh</Tooltip>
	</div>

	{#await data.applications}
		<div class="mt-4" role="status" aria-label="Loading applications">
			<Table hoverable={true}>
				<TableBody>
					{#each [0, 1, 2, 3] as rowIndex (rowIndex)}
						<TableBodyRow>
							<TableBodyCell>
								<div
									class="h-2.5 w-32 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"
								></div>
							</TableBodyCell>
							<TableBodyCell>
								<div
									class="h-2.5 w-20 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"
								></div>
							</TableBodyCell>
							<TableBodyCell>
								<div
									class="h-2.5 w-20 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"
								></div>
							</TableBodyCell>
						</TableBodyRow>
					{/each}
				</TableBody>
			</Table>
		</div>
	{:then applications}
		<ApplicationGrid {applications} />
	{/await}
{/if}
