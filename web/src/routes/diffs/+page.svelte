<script lang="ts">
	import {
		A,
		Button,
		Heading,
		List,
		Li,
		P,
		Tabs,
		TabItem,
		Progressbar,
		Tooltip
	} from 'flowbite-svelte';

	import { ExclamationCircleSolid, RefreshOutline, BellRingSolid } from 'flowbite-svelte-icons';

	import { writable } from 'svelte/store';
	import { type ApplicationsDiffsData, type ApplicationResponseStore } from '$lib/backend/data';
	import { filterOutZeroResults } from '$lib/ui/utils';
	import { fetchDiffs } from '$lib/backend/diffs';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { buildQuery } from '$lib/backend/url';
	import TangleAPIClient from '$lib/backend/client';
	import {
		AppManifests,
		ArgoCDHealthStatus,
		ArgoCDSyncStatus,
		DiffsForm,
		ErrorAlert
	} from '$lib/ui/components';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let targetRef = $derived(page.url.searchParams.get('targetRef')?.trim() || null);

	function goToDiffs(labels: string, excludeLabels: string, targetRef: string): void {
		const query = buildQuery({ targetRef, labels, excludeLabels });
		goto(resolve(`/diffs${query}` as '/diffs'));
	}

	// Separate from the client `+page.ts`'s load() uses — this one drives the
	// diff fan-out and reloadDiff, which aren't part of the load() lifecycle.
	const client = new TangleAPIClient();

	let applications: ApplicationResponseStore | undefined = $state();
	let progress: number = $state(0);
	let total: number = $state(0);
	let diffsLoaded: boolean = $state(false);

	const diffData = writable<ApplicationsDiffsData>({});
	let alertStatuses: string[] = ['OutOfSync', 'Unknown'];

	// The diff fan-out has to be triggered from an effect-like context, not
	// from inside a template expression — calling it directly as an
	// {#await EXPR} expression mutates $state (via the onTotal/onProgress
	// callbacks) synchronously while Svelte is evaluating that expression,
	// which Svelte 5 forbids (`state_unsafe_mutation`). $effect (rather than
	// onMount) is required here specifically because SvelteKit can reuse this
	// component across client-side navigations (e.g. a labels/excludeLabels
	// or targetRef-only change) without remounting it — onMount would only
	// ever see the first `data.applications` and the first `targetRef`. The
	// `cancelled` guard in the cleanup function stops a still-in-flight fetch
	// from a superseded navigation overwriting state from a newer one.
	$effect(() => {
		const currentApplications = data.applications;
		const currentTargetRef = targetRef;
		let cancelled = false;

		applications = undefined;
		diffsLoaded = false;
		progress = 0;
		total = 0;
		diffData.set({});

		currentApplications.then(async (result) => {
			if (cancelled) {
				return;
			}

			applications = result;

			if (result.error) {
				return;
			}

			if (!currentTargetRef) {
				// No target ref means there's nothing to diff against — bail out
				// before fetchDiffs, which otherwise fans out one diff-generation
				// request per application to every ArgoCD instance (falling back
				// to comparing each app's live ref against itself).
				return;
			}

			const diffDataMap = await fetchDiffs(client, result.response.results, currentTargetRef, {
				onTotal: (n) => {
					if (!cancelled) total = n;
				},
				onProgress: () => {
					if (!cancelled) progress += 1;
				}
			});

			if (cancelled) {
				return;
			}

			diffData.set(diffDataMap);
			diffsLoaded = true;
		});

		return () => {
			cancelled = true;
		};
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

<Heading tag="h1" class="sr-only">Diffs</Heading>

<div class="mt-4">
	{#if !applications}
		<div class="flex justify-center m-10">
			<P italic>Loading Applications...</P>
		</div>
	{:else if applications.error}
		<ErrorAlert message={applications.errorResponse?.error} />
	{:else if !targetRef}
		<DiffsForm
			initialLabels={page.url.searchParams.get('labels') ?? ''}
			initialExcludeLabels={page.url.searchParams.get('excludeLabels') ?? ''}
			onSubmit={goToDiffs}
		/>
	{:else if !diffsLoaded}
		<div class="flex justify-center m-10">
			<P italic>Loading diffs...</P>
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
				<TabItem open={index === 0}>
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
								<Heading tag="h3" class="mb-2">Status</Heading>
								<List tag="ul" class="list-none space-y-1 text-gray-500 dark:text-gray-400">
									<Li icon>
										<ArgoCDHealthStatus healthStatus={application.health} />
									</Li>
									<Li icon>
										<ArgoCDSyncStatus syncStatus={application.syncStatus} />
									</Li>
								</List>
								<div class="flex items-center justify-between mt-3">
									<P>(<A href={application.url} target="_blank" class="text-xs">More Info</A>)</P>
									<Button
										outline
										color="primary"
										aria-label="Reload diff"
										onclick={() =>
											reloadDiff(
												argoCDApplications.name,
												application.name,
												application.liveRef,
												targetRef ? targetRef : application.liveRef
											)}><RefreshOutline /></Button
									>
									<Tooltip>Reload diff</Tooltip>
								</div>
								<div class="mt-4">
									<AppManifests diffData={$diffData[argoCDApplications.name]?.[application.name]} />
								</div>
							</TabItem>
						{/each}
					</Tabs>
				</TabItem>
			{/each}
		</Tabs>
	{/if}
</div>
