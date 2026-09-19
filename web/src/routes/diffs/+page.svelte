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

	import { ExclamationCircleSolid, RefreshOutline, BellRingSolid } from 'flowbite-svelte-icons';

	import { onMount } from 'svelte';
	import { writable } from 'svelte/store';
	import { type ApplicationsDiffsData, type ApplicationResponseStore } from '$lib/backend/data';
	import { filterOutZeroResults } from '$lib/ui/utils';
	import { fetchDiffs } from '$lib/backend/diffs';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/backend/client';
	import { AppManifests, ArgoCDHealthStatus, ArgoCDSyncStatus } from '$lib/ui/components';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const targetRef = $page.url.searchParams.get('targetRef');

	// Separate from the client `+page.ts`'s load() uses — this one drives the
	// diff fan-out and reloadDiff, which aren't part of the load() lifecycle.
	const client = new TangleAPIClient();

	let applications: ApplicationResponseStore | undefined = $state();
	let progress: number = $state(0);
	let total: number = $state(0);
	let diffsLoaded: boolean = $state(false);

	const diffData = writable<ApplicationsDiffsData>({});
	let alertStatuses: string[] = ['OutOfSync', 'Unknown'];

	// The diff fan-out has to be triggered from an effect-like context (onMount),
	// not from inside a template expression — calling it directly as an
	// {#await EXPR} expression mutates $state (via the onTotal/onProgress
	// callbacks) synchronously while Svelte is evaluating that expression,
	// which Svelte 5 forbids (`state_unsafe_mutation`).
	onMount(() => {
		data.applications.then(async (result) => {
			applications = result;

			if (result.error) {
				return;
			}

			const diffDataMap = await fetchDiffs(client, result.response.results, targetRef, {
				onTotal: (n) => (total = n),
				onProgress: () => (progress += 1)
			});

			diffData.set(diffDataMap);
			diffsLoaded = true;
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

<svelte:head>
	<title>Tangle - Diffs</title>
</svelte:head>

{#if !applications}
	<div class="flex justify-center m-10">
		<P italic>Loading Applications...</P>
	</div>
{:else if applications.error}
	<Alert color="red" class="bg-red-500 text-white">
		<span class="font-medium">System error!</span>
		<br />
		{applications.errorResponse?.error}
	</Alert>
{:else if !diffsLoaded}
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
{:else}
	<Tabs tabStyle="underline" class="ml-5 mr-5">
		{#each filterOutZeroResults(applications.response.results) as argoCDApplications, index (argoCDApplications.name)}
			<TabItem open={index === 0} disabled={argoCDApplications.applications.length === 0}>
				{#snippet titleSlot()}
					{argoCDApplications.name}
				{/snippet}
				<Tabs>
					{#each argoCDApplications.applications as application, appIndex (application.name)}
						<TabItem open={appIndex === 0}>
							{#snippet titleSlot()}
								<div class="flex items-center">
									{#if alertStatuses.includes(application.syncStatus) || application.health !== 'Healthy' || $diffData[argoCDApplications.name]?.[application.name].error || $diffData[argoCDApplications.name]?.[application.name].response.manifestGenerationError.length > 0}<ExclamationCircleSolid
											class="w-5 h-5 me-2 text-rose-500 dark:text-rose-400"
										/>
									{:else if $diffData[argoCDApplications.name]?.[application.name].response.diffs.length > 0}
										<BellRingSolid class="w-5 h-5 me-2 text-amber-500 dark:text-amber-400" />
									{/if}
									{application.name}
								</div>
							{/snippet}
							<Heading tag="h3">Status</Heading>
							<List tag="ul" class="list-none space-y-1 text-gray-500 dark:text-gray-400">
								<Li icon>
									<ArgoCDHealthStatus healthStatus={application.health} />
								</Li>
								<Li icon>
									<ArgoCDSyncStatus syncStatus={application.syncStatus} />
								</Li>
							</List>
							<br />
							<div class="align-bottom">
								<P>(<A href={application.url} target="_blank" class="text-xs">More Info</A>)</P>
								<GradientButton
									class="absolute right-5"
									outline
									color="pinkToOrange"
									onclick={() =>
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
