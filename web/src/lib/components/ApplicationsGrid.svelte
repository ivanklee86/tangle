<script>
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
	import { apiData } from '$lib/data';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';
	import { ArgoCDHealthStatus, ArgoCDSyncStatus } from '$lib/components';

	const labels = $page.url.searchParams.get('labels');
	var client = new TangleAPIClient();

	let firstIndex = 0;
	onMount(() => {
		client.getApplications(labels).then(() => {
			if (!$apiData.error && $apiData.response.results.length > 0) {
				for (let i = 0; i < $apiData.response.results.length; i++) {
					if ($apiData.response.results[i].applications.length > 0) {
						firstIndex = i;
						break;
					}
				}
			}
		});
	});
</script>

{#if $apiData.error}
	<Alert color="none" class="bg-red-500 text-white">
		<span class="font-medium">System error!</span>
		<br />
		{$apiData.errorResponse?.error}
	</Alert>
{/if}

{#if apiData}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each $apiData.response.results as argoCDApplications, index}
			<TabItem
				title={argoCDApplications.name}
				open={index === firstIndex}
				disabled={argoCDApplications.applications.length === 0}
			>
				<span slot="title">{argoCDApplications.name}</span>
				<Button href={argoCDApplications.link} target="_blank" class="mb-3">Take me there!</Button>
				<br />
				<Table hoverable={true} items={argoCDApplications.applications}>
					<TableHead>
						<TableHeadCell sort={(a, b) => a.name.localeCompare(b.name)}>Applications</TableHeadCell
						>
						<TableHeadCell sort={(a, b) => a.health.localeCompare(b.health)}>Health</TableHeadCell>
						<TableHeadCell sort={(a, b) => a.syncStatus.localeCompare(b.syncStatus)}
							>Sync Status</TableHeadCell
						>
					</TableHead>
					<TableBody tableBodyClass="divide-y">
						<TableBodyRow slot="row" let:item>
							<TableBodyCell>
								<a href={item.url} target="_blank" class="link-underline-primary">{item.name}</a>
							</TableBodyCell>
							<TableBodyCell><ArgoCDHealthStatus healthStatus={item.health} /></TableBodyCell>
							<TableBodyCell><ArgoCDSyncStatus syncStatus={item.syncStatus} /></TableBodyCell>
						</TableBodyRow>
					</TableBody>
				</Table>
			</TabItem>
		{/each}
	</Tabs>
{:else}
	<Spinner />
{/if}
