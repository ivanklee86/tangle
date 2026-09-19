<script lang="ts">
	import { Select, Button, Spinner } from 'flowbite-svelte';
	import { RefreshOutline } from 'flowbite-svelte-icons';
	import { page } from '$app/stores';
	import { invalidateAll } from '$app/navigation';
	import { onMount } from 'svelte';
	import { ApplicationGrid } from '$lib/ui/components';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let selectedRefreshPeriod: number = $state(10);

	const refreshParam = $page.url.searchParams.get('refresh');
	let refreshEnabled: boolean = $state(refreshParam === 'true');

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
	<title>Tangle - Applications</title>
</svelte:head>

<br />

<div class="flex justify-end gap-2">
	<Select class="w-fit" items={refreshPeriods} bind:value={selectedRefreshPeriod} />
	<Button
		class="mr-2"
		color={refreshEnabled ? 'primary' : 'alternative'}
		onclick={() => (refreshEnabled = !refreshEnabled)}><RefreshOutline /></Button
	>
</div>

{#await data.applications}
	<br />
	<div class="text-center"><Spinner /></div>
{:then applications}
	<ApplicationGrid {applications} />
{/await}
