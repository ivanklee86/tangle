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
	<Alert color="none" class="bg-red-500 text-white">
		<span class="font-medium">System error!</span>
		<br />
		{$applicationsData.errorResponse?.error}
	</Alert>
{:else if $applicationsData.loaded}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each filterOutZeroResults($applicationsData.response.results) as argoCDApplications, index (argoCDApplications.name)}
			<TabItem
				title={argoCDApplications.name}
				open={index === 0}
				disabled={argoCDApplications.applications.length === 0}
			>
				<span slot="title"
					>{argoCDApplications.name} ({argoCDApplications.applications.length})</span
				>
				<Button href={argoCDApplications.link} target="_blank" class="mb-3">Take me there!</Button>
				<br />
				<Table hoverable={true} items={argoCDApplications.applications}>
					<TableHead>
						<TableHeadCell
							sort={(a: ApplicationLinks, b: ApplicationLinks) => a.name.localeCompare(b.name)}
							>Applications</TableHeadCell
						>
						<TableHeadCell
							sort={(a: ApplicationLinks, b: ApplicationLinks) => a.health.localeCompare(b.health)}
							>Health</TableHeadCell
						>
						<TableHeadCell
							sort={(a: ApplicationLinks, b: ApplicationLinks) =>
								a.syncStatus.localeCompare(b.syncStatus)}>Sync Status</TableHeadCell
						>
					</TableHead>
					<TableBody tableBodyClass="divide-y">
						<TableBodyRow slot="row" let:item>
							<TableBodyCell>
								{/* @ts-expect-error: Svelte doesn't correctly impute type of item and you can't set it. */ null}
								<a href={item.url} target="_blank" class="link-underline-primary">{item.name}</a>
							</TableBodyCell>
							{/* @ts-expect-error: See ^ */ null}
							<TableBodyCell><ArgoCDHealthStatus healthStatus={item.health} /></TableBodyCell>
							{/* @ts-expect-error: See % */ null}
							<TableBodyCell><ArgoCDSyncStatus syncStatus={item.syncStatus} /></TableBodyCell>
						</TableBodyRow>
					</TableBody>
				</Table>
			</TabItem>
		{/each}
	</Tabs>
{:else}
	<br />
	<div class="text-center"><Spinner /></div>
{/if}
