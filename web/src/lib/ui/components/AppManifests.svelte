<script lang="ts">
	import { AccordionItem, Accordion, Alert, Heading, P } from 'flowbite-svelte';
	import { InfoCircleSolid, FileLinesSolid } from 'flowbite-svelte-icons';
	import { CodeBlock } from 'svhighlight';
	import 'highlight.js/styles/an-old-hope.css';
	import { type ApplicationDiff } from '$lib/backend/data';
	import { ErrorAlert } from '$lib/ui/components';

	interface Props {
		diffData: ApplicationDiff;
	}

	let { diffData }: Props = $props();
</script>

{#if diffData.error}
	<ErrorAlert message={diffData.errorResponse?.error} />
{:else if diffData.response.manifestGenerationError.length > 0}
	<Alert color="red" border>
		<div class="flex items-center gap-3">
			<InfoCircleSolid class="w-5 h-5" />
			<span class="text-lg font-medium">Error generating manifests!</span>
		</div>
		<p class="mt-2 mb-4 text-sm">{diffData.response.manifestGenerationError}</p>
	</Alert>
{:else if diffData.loaded}
	<Heading tag="h4" class="flex items-center gap-2 mb-3"><FileLinesSolid size="lg" />Diffs</Heading>
	{#if diffData.response.diffs.length === 0}
		<P italic>No diffs found.</P>
	{:else}
		<CodeBlock language="diff" code={diffData.response.diffs} showLineNumbers={false} />
	{/if}
	<Accordion class="mt-4">
		<AccordionItem>
			{#snippet header()}
				Manifests
			{/snippet}
			<CodeBlock language="yaml" code={diffData.response.targetManifests} />
		</AccordionItem>
	</Accordion>
{/if}
