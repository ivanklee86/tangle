<script lang="ts">
	import { Helper } from 'flowbite-svelte';
	import { cliCommand, type Query } from '$lib/ui/query';
	import CopyableText from './CopyableText.svelte';

	interface Props {
		/** The URL this query opens — the page's own href, so it differs per placement. */
		href: string;
		query: Query;
		/**
		 * Whether to show the tangle-cli equivalent.
		 *
		 * `generate-manifests` is the CLI's version of the *diffs* flow — it
		 * renders manifests and compares them. There is no subcommand that just
		 * lists applications, so offering it beside an Applications query would
		 * hand someone a command that does something else.
		 */
		cli?: boolean;
	}

	let { href, query, cli = false }: Props = $props();

	let hasTargetRef = $derived(query.targetRef.trim().length > 0);
</script>

<div class="space-y-2">
	<CopyableText caption="Link" text={href} copyLabel="Copy link" />

	{#if cli}
		<div>
			<CopyableText
				caption="Same query in CI"
				text={cliCommand(query)}
				copyLabel="Copy tangle-cli command"
			/>
			{#if !hasTargetRef}
				<!--
					Without --target-ref, generate-manifests compares every
					application's live ref against itself: a request per
					application to every Argo CD for a diff that is empty by
					construction. Say so rather than hand over a command that
					quietly does nothing useful.
				-->
				<Helper class="mt-1"
					>Add a target ref — without one there is nothing to compare against.</Helper
				>
			{/if}
		</div>
	{/if}
</div>
