<script lang="ts">
	import {
		Alert,
		Button,
		Tabs,
		TabItem,
		Spinner,
		Table,
		TableBody,
		TableBodyCell,
		TableBodyRow,
		TableHead,
		TableHeadCell
	} from 'flowbite-svelte';

	import { onMount } from 'svelte';
	import { writable } from 'svelte/store';
	import { type ApplicationResponseStore, type ApplicationLinks } from '$lib/data';
	import { filterOutZeroResults } from '$lib/utils';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';
	import { ArgoCDHealthStatus, ArgoCDSyncStatus } from '$lib/components';

	interface Props {
		refresh: boolean;
		refreshPeriod: number;
	}

	let { refresh = false, refreshPeriod = 5 }: Props = $props();

	// flowbite-svelte 1.x dropped TableHeadCell's built-in `sort` prop/context, so
	// column sorting is reimplemented here (one independent sort state per tab).
	type SortKey = 'name' | 'health' | 'syncStatus';
	type SortState = { key: SortKey; direction: 'asc' | 'desc' };

	let sortState: Record<string, SortState | undefined> = $state({});

	function toggleSort(tabName: string, key: SortKey) {
		const current = sortState[tabName];
		sortState[tabName] =
			current?.key === key
				? { key, direction: current.direction === 'asc' ? 'desc' : 'asc' }
				: { key, direction: 'asc' };
	}

	function sortIndicator(tabName: string, key: SortKey): string {
		const current = sortState[tabName];
		if (current?.key !== key) return '';
		return current.direction === 'asc' ? ' ▲' : ' ▼';
	}

	function sortedApplications(
		tabName: string,
		applications: ApplicationLinks[]
	): ApplicationLinks[] {
		const state = sortState[tabName];
		if (!state) return applications;
		const sorted = [...applications].sort((a, b) => a[state.key].localeCompare(b[state.key]));
		return state.direction === 'asc' ? sorted : sorted.reverse();
	}

	const labels = $page.url.searchParams.get('labels');
	const excludeLabels = $page.url.searchParams.get('excludeLabels');

	const applicationsData = writable<ApplicationResponseStore>({
		response: { results: [] },
		errorResponse: { error: '' },
		error: false,
		loaded: false
	});

	var client = new TangleAPIClient();

	async function loadApplications() {
		// Reset the store
		applicationsData.set({
			response: { results: [] },
			errorResponse: { error: '' },
			error: false,
			loaded: false
		});

		// Get/Refresh status
		client.getApplications(labels, excludeLabels).then((result) => {
			applicationsData.set(result);
		});
	}

	onMount(() => {
		loadApplications();

		let interval: number;

		const startInterval = () => {
			clearInterval(interval); // Clear any existing interval
			interval = setInterval(() => {
				if (refresh) {
					loadApplications();
				}
			}, refreshPeriod * 1000);
		};

		startInterval(); // Start the interval initially

		$effect(() => {
			startInterval(); // Restart the interval whenever refreshPeriod changes
		});

		return () => clearInterval(interval);
	});
</script>

{#if $applicationsData.error}
	<Alert color="red" class="bg-red-500 text-white">
		<span class="font-medium">System error!</span>
		<br />
		{$applicationsData.errorResponse?.error}
	</Alert>
{:else if $applicationsData.loaded}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each filterOutZeroResults($applicationsData.response.results) as argoCDApplications, index (argoCDApplications.name)}
			<TabItem open={index === 0} disabled={argoCDApplications.applications.length === 0}>
				{#snippet titleSlot()}
					{argoCDApplications.name} ({argoCDApplications.applications.length})
				{/snippet}
				<Button href={argoCDApplications.link} target="_blank" class="mb-3">Take me there!</Button>
				<br />
				<Table hoverable={true}>
					<TableHead>
						<TableHeadCell
							class="cursor-pointer select-none"
							onclick={() => toggleSort(argoCDApplications.name, 'name')}
							>Applications{sortIndicator(argoCDApplications.name, 'name')}</TableHeadCell
						>
						<TableHeadCell
							class="cursor-pointer select-none"
							onclick={() => toggleSort(argoCDApplications.name, 'health')}
							>Health{sortIndicator(argoCDApplications.name, 'health')}</TableHeadCell
						>
						<TableHeadCell
							class="cursor-pointer select-none"
							onclick={() => toggleSort(argoCDApplications.name, 'syncStatus')}
							>Sync Status{sortIndicator(argoCDApplications.name, 'syncStatus')}</TableHeadCell
						>
					</TableHead>
					<TableBody class="divide-y">
						{#each sortedApplications(argoCDApplications.name, argoCDApplications.applications) as item (item.name)}
							<TableBodyRow>
								<TableBodyCell>
									<a href={item.url} target="_blank" rel="external" class="link-underline-primary"
										>{item.name}</a
									>
								</TableBodyCell>
								<TableBodyCell><ArgoCDHealthStatus healthStatus={item.health} /></TableBodyCell>
								<TableBodyCell><ArgoCDSyncStatus syncStatus={item.syncStatus} /></TableBodyCell>
							</TableBodyRow>
						{/each}
					</TableBody>
				</Table>
			</TabItem>
		{/each}
	</Tabs>
{:else}
	<br />
	<div class="text-center"><Spinner /></div>
{/if}
