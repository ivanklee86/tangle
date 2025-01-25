<script>
	import { AccordionItem, Accordion, Spinner } from 'flowbite-svelte';

	import { onMount } from 'svelte';
	import { diffData } from '$lib/data';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';

	const labels = $page.url.searchParams.get('labels');
    const compareRef = $page.url.searchParams.get('compareRef');
	var client = new TangleAPIClient();

    let firstIndex = 0;
	onMount(() => {
		client.getApplicationDiff("test", "test-1", "main", "test_gitops").then((result) => {
			console.log(result);
			diffData.set(result);
		});
	});

	let diff = $state($diffData.response.diff);
</script>

{#if diffData}
<Accordion>
	<AccordionItem>
	  <span slot="header">Diffs</span>
	  <p class="text-gray-500 dark:text-gray-400">
		Hi
		{diff}
	  </p>
	</AccordionItem>
</Accordion>
{:else}
	<Spinner />
{/if}