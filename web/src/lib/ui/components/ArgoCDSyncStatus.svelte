<script lang="ts">
	import { statusAppearance } from '$lib/ui/status';

	interface Props {
		syncStatus: string;
	}

	let { syncStatus }: Props = $props();

	let appearance = $derived(statusAppearance(syncStatus));
</script>

<div class="flex">
	<appearance.icon class="w-5 h-5 me-2 {appearance.class}" />
	{syncStatus}
</div>
