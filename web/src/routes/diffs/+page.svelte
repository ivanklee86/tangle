<script lang="ts">
	import { A, Alert, Heading, List, Li, P, Tabs, TabItem, Spinner } from 'flowbite-svelte';

	import { ExclamationCircleSolid } from 'flowbite-svelte-icons';

	import { onMount } from 'svelte';
	import { writable } from 'svelte/store';
	import { type ApplicationResponseStore } from '$lib/data';
	import { filterOutZeroResults } from '$lib/utils';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';
	import { AppManifests, ArgoCDHealthStatus, ArgoCDSyncStatus } from '$lib/components';

	const labels = $page.url.searchParams.get('labels');
	const targetRef = $page.url.searchParams.get('targetRef');
	var client = new TangleAPIClient();

	const applicationsData = writable<ApplicationResponseStore>({
		response: { results: [] },
		errorResponse: { error: '' },
		error: false,
		loaded: false
	});

	onMount(() => {
		client.getApplications(labels).then((result) => {
			applicationsData.set(result);
		});
	});
</script>

{#if $applicationsData.error}
	<Alert color="none" class="bg-red-500 text-white">
		<span class="font-medium">System error!</span>
		<br />
		{$applicationsData.errorResponse?.error}
	</Alert>
{/if}

{#if $applicationsData.loaded}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each filterOutZeroResults($applicationsData.response.results) as argoCDApplications, index}
			<TabItem
				title={argoCDApplications.name}
				open={index === 0}
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
