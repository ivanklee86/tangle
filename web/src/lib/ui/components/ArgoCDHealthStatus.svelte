<script lang="ts">
	import { statusAppearance } from '$lib/ui/status';

	interface Props {
		healthStatus: string;
	}

	let { healthStatus }: Props = $props();

	let appearance = $derived(statusAppearance(healthStatus));
</script>

<div class="flex">
	<appearance.icon class="w-5 h-5 me-2 {appearance.class}" />
	{healthStatus}
</div>
