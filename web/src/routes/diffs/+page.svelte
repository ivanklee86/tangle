<script lang="ts">
	import { A, Alert, Heading, List, Li, P, Tabs, TabItem, Spinner } from 'flowbite-svelte';

	import { ExclamationCircleSolid } from 'flowbite-svelte-icons';

	import { onMount } from 'svelte';
	import { writable } from 'svelte/store';
	import { type ApplicationsDiffsData, type ApplicationResponseStore } from '$lib/data';
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

	const diffData = writable<ApplicationsDiffsData>({});
	let loaded = false;

	onMount(() => {
		client
			.getApplications(labels)
			.then(async (result) => {
				applicationsData.set(result);

				const diffPromises = result.response.results.flatMap((argoCD) =>
					argoCD.applications.map((application) =>
						client.getApplicationDiff(
							argoCD.name,
							application.name,
							application.liveRef,
							targetRef ? targetRef : application.liveRef
						)
					)
				);

				const diffResults = await Promise.all(diffPromises);

				const diffDataMap: ApplicationsDiffsData = {};
				diffResults.forEach((result) => {
					const argoCDName = result.requestDetails.argoCD;
					const applicationName = result.requestDetails.applicationName;

					// Initialize the argoCDName object if it doesn't exist
					if (!diffDataMap[argoCDName]) {
						diffDataMap[argoCDName] = {};
					}

					diffDataMap[argoCDName][applicationName] = result;
				});

				diffData.set(diffDataMap);
			})
			.finally(() => {
				loaded = true;
			});
	});
</script>

{#if loaded}
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
								<AppManifests diffData={$diffData[argoCDApplications.name]?.[application.name]} />
							</TabItem>
						{/each}
					</Tabs>
				</TabItem>
			{/each}
		</Tabs>
	{/if}
{:else}
	<div class="flex justify-center m-2">
		<P italic>Loading Applications...</P>
	</div>
	<div class="text-center"><Spinner /></div>
{/if}
