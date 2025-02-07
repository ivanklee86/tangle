<script lang="ts">
	import { A, Alert, Heading, List, Li, P, Tabs, TabItem, Spinner } from 'flowbite-svelte';

	import { ExclamationCircleSolid } from 'flowbite-svelte-icons';

	import { onMount } from 'svelte';
	import { apiData } from '$lib/data';
	import { filterOutZeroResults } from '$lib/utils';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';
	import { AppManifests, ArgoCDHealthStatus, ArgoCDSyncStatus } from '$lib/components';

	const labels = $page.url.searchParams.get('labels');
	const targetRef = $page.url.searchParams.get('targetRef');
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

{#if $apiData.loaded}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each filterOutZeroResults($apiData.response.results) as argoCDApplications, index}
			<TabItem
				title={argoCDApplications.name}
				open={index === firstIndex}
				disabled={argoCDApplications.applications.length === 0}
			>
				<Tabs>
					{#each argoCDApplications.applications as application, appIndex}
						<TabItem title={application.name} open={appIndex === 0}>
							<div slot="title" class="flex items-center">
								{#if application.syncStatus === 'Unknown'}<ExclamationCircleSolid
										class="w-5 h-5 me-2 text-amber-500 dark:text-amber-400"
									/>{/if}
								{application.name}
							</div>
							<Heading tag="h3">Status</Heading>
							<List tag="ul" class="space-y-1 text-gray-500 dark:text-gray-400" list="none">
								<Li icon>
									<ArgoCDHealthStatus healthStatus={application.health} />
								</Li>
								<Li icon>
									<ArgoCDSyncStatus syncStatus={application.syncStatus} />
								</Li>
							</List>
							<br />
							<P>(<A href={application.url} aClass="xs">More Info</A>)</P>
							<br />
							<AppManifests
								argocd={argoCDApplications.name}
								appName={application.name}
								liveRef={application.liveRef}
								targetRef={targetRef ? targetRef : application.liveRef}
							/>
						</TabItem>
					{/each}
				</Tabs>
			</TabItem>
		{/each}
	</Tabs>
{:else}
	<br />
	<div class="text-center"><Spinner /></div>
{/if}
