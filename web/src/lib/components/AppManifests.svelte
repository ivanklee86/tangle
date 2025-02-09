<script lang="ts">
	import { AccordionItem, Accordion, Alert, Card, Spinner } from 'flowbite-svelte';
	import { InfoCircleSolid, FileLinesSolid } from 'flowbite-svelte-icons';
	import { CodeBlock } from 'svhighlight';
	import 'highlight.js/styles/an-old-hope.css';

	import { onMount } from 'svelte';
	import { writable } from 'svelte/store';
	import { type ApplicationDiff } from '$lib/data';
	import TangleAPIClient from '$lib/client';

	interface Props {
		argocd: string;
		appName: string;
		liveRef: string;
		targetRef: string;
	}

	let { argocd, appName, liveRef, targetRef }: Props = $props();

	const diffData = writable<ApplicationDiff>({
		response: { liveManifests: '', targetManifests: '', diffs: '', manifestGenerationError: '' },
		errorResponse: { error: '' },
		error: false,
		loaded: false
	});

	var client = new TangleAPIClient();

	onMount(() => {
		client.getApplicationDiff(argocd, appName, liveRef, targetRef).then((result) => {
			diffData.set(result);
		});
	});
</script>

{#if $diffData.loaded}
	{#if $diffData.response.manifestGenerationError.length > 0}
		<Card class="m-auto dark:bg-red-400" size="xl">
			<Alert>
				<div class="flex items-center gap-3">
					<InfoCircleSolid class="w-5 h-5" />
					<span class="text-lg font-medium">Error generating manifests!</span>
				</div>
				<p class="mt-2 mb-4 text-sm">{$diffData.response.manifestGenerationError}</p>
			</Alert>
		</Card>
	{:else if $diffData.loaded}
		<Accordion>
			<AccordionItem open={$diffData.response.diffs.length > 0}>
				<span slot="header" class="text-base flex gap-2">
					<FileLinesSolid class="w-5 h-5" />
					<span>Diffs</span>
				</span>
				<CodeBlock language="diff" code={$diffData.response.diffs} showLineNumbers={false} />
			</AccordionItem>
			<AccordionItem>
				<span slot="header">Manifests</span>
				<CodeBlock language="yaml" code={$diffData.response.targetManifests} />
			</AccordionItem>
		</Accordion>
	{/if}
{:else}
	<div class="text-center"><Spinner /></div>
{/if}
