<script lang="ts">
	import { Alert } from 'flowbite-svelte';

	interface Props {
		message: string | undefined;
	}

	let { message }: Props = $props();
</script>

<Alert color="red" border>
	<span class="font-medium">System error!</span>
	<br />
	{message}
</Alert>
