<script lang="ts">
	import {
		A,
		Alert,
		Heading,
		List,
		Li,
		P,
		Tabs,
		TabItem,
		Progressbar,
		GradientButton
	} from 'flowbite-svelte';

	import { ExclamationCircleSolid, RefreshOutline } from 'flowbite-svelte-icons';

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

	let progress: number = $state(0);
	let total: number = $state(0);

	const diffData = writable<ApplicationsDiffsData>({});
	let loaded: boolean = $state(false);

	onMount(() => {
		client
			.getApplications(labels)
			.then(async (result) => {
				applicationsData.set(result);

				const diffPromises = result.response.results.flatMap((argoCD) =>
					argoCD.applications.map((application) =>
						client
							.getApplicationDiff(
								argoCD.name,
								application.name,
								application.liveRef,
								targetRef ? targetRef : application.liveRef
							)
							.then((result) => {
								progress += 1;
								return result;
							})
					)
				);
				total = diffPromises.length;

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

	function reloadDiff(
		argoCDName: string,
		applicationName: string,
		liveRef: string,
		targetRef: string
	) {
		client
			.getApplicationDiff(argoCDName, applicationName, liveRef, targetRef ? targetRef : liveRef)
			.then((result) => {
				diffData.update((data) => {
					const newData = { ...data };

					newData[argoCDName][applicationName] = result;
					return newData;
				});
			});
	}
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
									{#if application.syncStatus === 'Unknown' || $diffData[argoCDApplications.name]?.[application.name].error || $diffData[argoCDApplications.name]?.[application.name].response.manifestGenerationError.length > 0}<ExclamationCircleSolid
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
								<div class="align-bottom">
									<P>(<A href={application.url} target="_blank" aClass="xs">More Info</A>)</P>
									<GradientButton
										class="absolute right-5"
										outline
										color="pinkToOrange"
										on:click={() =>
											reloadDiff(
												argoCDApplications.name,
												application.name,
												application.liveRef,
												targetRef ? targetRef : application.liveRef
											)}><RefreshOutline /></GradientButton
									>
								</div>
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
	<div class="flex justify-center m-10">
		<P italic>Loading Applications...</P>
	</div>
	{#if total > 0}
		<div class="justify-center w-1/2 m-auto">
			<Progressbar
				progress={Math.round((progress / total) * 100)}
				labelOutside="Getting Application diffs & manifests"
			/>
		</div>
	{/if}
{/if}
