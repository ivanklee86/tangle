<script lang="ts">
	import {
		CheckCircleSolid,
		CloseCircleSolid,
		ExclamationCircleSolid
	} from 'flowbite-svelte-icons';

	interface Props {
		syncStatus: string;
	}

	let { syncStatus }: Props = $props();
</script>

<div class="flex">
	{#if syncStatus == 'Synced'}
		<CheckCircleSolid class="w-5 h-5 me-2 text-green-500 dark:text-green-400" />
	{:else if syncStatus == 'OutOfSync'}
		<CloseCircleSolid class="w-5 h-5 me-2 text-red-500 dark:text-red-400" />
	{:else if syncStatus == 'Unknown'}
		<ExclamationCircleSolid class="w-5 h-5 me-2 text-amber-500 dark:text-amber-400" />
	{/if}
	{syncStatus}
</div>
