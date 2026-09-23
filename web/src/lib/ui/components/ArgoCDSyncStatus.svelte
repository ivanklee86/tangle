<script lang="ts">
	import { Badge } from 'flowbite-svelte';
	import { statusAppearance } from '$lib/ui/status';

	interface Props {
		syncStatus: string;
	}

	let { syncStatus }: Props = $props();

	let appearance = $derived(statusAppearance(syncStatus));
</script>

<Badge color={appearance.color} rounded class="gap-1.5 px-2.5 py-1">
	<appearance.icon class="h-3.5 w-3.5" />
	{syncStatus}
</Badge>
