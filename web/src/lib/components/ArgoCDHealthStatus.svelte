<script lang="ts">
	import { CheckCircleSolid, CloseCircleSolid } from 'flowbite-svelte-icons';

	interface Props {
		healthStatus: string;
	}

	let { healthStatus }: Props = $props();
</script>

<div class="flex">
	{#if healthStatus == 'Healthy'}
		<CheckCircleSolid class="w-5 h-5 me-2 text-green-500 dark:text-green-400" />
	{:else}
		<CloseCircleSolid class="w-5 h-5 me-2 text-red-500 dark:text-red-400" />
	{/if}
	{healthStatus}
</div>
