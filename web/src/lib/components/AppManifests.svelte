<script lang="ts">
	import { AccordionItem, Accordion, Alert, Card, Heading, P } from 'flowbite-svelte';
	import { InfoCircleSolid, FileLinesSolid } from 'flowbite-svelte-icons';
	import { CodeBlock } from 'svhighlight';
	import 'highlight.js/styles/an-old-hope.css';
	import { type ApplicationDiff } from '$lib/data';

	interface Props {
		diffData: ApplicationDiff;
	}

	let { diffData }: Props = $props();
</script>

{#if diffData.error}
	<Alert color="none" class="bg-red-500 text-white">
		<span class="font-medium">System error!</span>
		<br />
		{diffData.errorResponse?.error}
	</Alert>
{:else if diffData.response.manifestGenerationError.length > 0}
	<Card class="m-auto dark:bg-red-400" size="xl">
		<Alert>
			<div class="flex items-center gap-3">
				<InfoCircleSolid class="w-5 h-5" />
				<span class="text-lg font-medium">Error generating manifests!</span>
			</div>
			<p class="mt-2 mb-4 text-sm">{diffData.response.manifestGenerationError}</p>
		</Alert>
	</Card>
{:else if diffData.loaded}
	<Heading tag="h4" class="flex items-center"><FileLinesSolid size="lg" />Diffs</Heading>
	{#if diffData.response.diffs.length === 0}
		<P italic>No diffs found.</P>
	{:else}
		<CodeBlock language="diff" code={diffData.response.diffs} showLineNumbers={false} />
	{/if}
	<br />
	<Accordion>
		<AccordionItem>
			<span slot="header">Manifests</span>
			<CodeBlock language="yaml" code={diffData.response.targetManifests} />
		</AccordionItem>
	</Accordion>
{/if}
