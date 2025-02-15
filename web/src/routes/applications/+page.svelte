<script lang="ts">
	import { Select, Button } from 'flowbite-svelte';
	import { RefreshOutline } from 'flowbite-svelte-icons';
	import { page } from '$app/stores';
	import { ApplicationGrid } from '$lib/components';

	let selectedRefreshPeriod: number = $state(10);

	const refreshParam = $page.url.searchParams.get('refresh');
	let refreshEnabled: boolean = $state(refreshParam === 'true');

	let refreshPeriods = [
		{ value: 1, name: '1s' },
		{ value: 5, name: '5s' },
		{ value: 10, name: '10s' },
		{ value: 15, name: '15s' }
	];
</script>

<br />

<div class="flex justify-end gap-2">
	<Select class="w-fit" items={refreshPeriods} bind:value={selectedRefreshPeriod} />
	<Button
		class="mr-2"
		color={refreshEnabled ? 'primary' : 'alternative'}
		on:click={() => (refreshEnabled = !refreshEnabled)}><RefreshOutline /></Button
	>
</div>

<ApplicationGrid refresh={refreshEnabled} refreshPeriod={selectedRefreshPeriod} />
