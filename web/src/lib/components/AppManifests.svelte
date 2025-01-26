<script lang="ts">
	import { AccordionItem, Accordion, Spinner } from 'flowbite-svelte';
	import { CodeBlock } from 'svhighlight';
	import 'highlight.js/styles/an-old-hope.css';

	import { onMount } from 'svelte';
	import { diffData } from '$lib/data';
	import { page } from '$app/stores';
	import TangleAPIClient from '$lib/client';


	interface Props {
		argocd: string;
		appName: string;
		currentRef: string;
		compareRef: string;
	}

	let { argocd, appName, currentRef, compareRef }: Props = $props();

	var client = new TangleAPIClient();

	onMount(() => {
		client.getApplicationDiff(argocd, appName, currentRef, compareRef).then((result) => {
			console.log(result);
			diffData.set(result);
		});
	});
</script>

{#if diffData}
<Accordion>
	<AccordionItem>
	  <span slot="header">Diffs</span>
	   <CodeBlock
			language="diff"
			code={$diffData.response.diffs}
		/>
	</AccordionItem>
</Accordion>
<Accordion>
	<AccordionItem>
	  <span slot="header">Manifests</span>
		<CodeBlock
			language="yaml"
			code={$diffData.response.targetManifests}
		/>
	</AccordionItem>
</Accordion>
{:else}
	<Spinner />
{/if}
