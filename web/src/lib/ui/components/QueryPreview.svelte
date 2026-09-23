<script lang="ts">
	import { cliCommand, type Query } from '$lib/ui/query';
	import { absoluteUrl } from '$lib/ui/links';
	import CopyableText from './CopyableText.svelte';

	interface Props {
		/** The URL this query opens — the page's own href, so it differs per placement. */
		href: string;
		query: Query;
		/** Whether to show the tangle-cli equivalent alongside the link. */
		cli?: boolean;
	}

	let { href, query, cli = false }: Props = $props();
</script>

<div class="space-y-2">
	<!--
		Absolute, not the bare path: this is a link people copy and send, and
		"/applications?labels=env:test" is not something a colleague can open.
	-->
	<CopyableText caption="Link" text={absoluteUrl(href)} copyLabel="Copy link" />

	{#if cli}
		<CopyableText
			caption="Same query in CI"
			text={cliCommand(query)}
			copyLabel="Copy tangle-cli command"
		/>
	{/if}
</div>
