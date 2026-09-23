<script lang="ts">
	import { Alert } from 'flowbite-svelte';

	interface Props {
		message: string | undefined;
	}

	let { message }: Props = $props();
</script>

<Alert color="red" border>
	<span class="font-medium">System error!</span>
	<!--
		The message is its own element rather than a bare text node beside the
		heading, so assistive tech and tests can address it on its own instead
		of only reading the alert's whole text content.
	-->
	<p class="mt-1 text-sm">{message}</p>
</Alert>
