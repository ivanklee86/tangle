<script lang="ts">
	import { Helper } from 'flowbite-svelte';
	import { cliCommand, type Query } from '$lib/ui/query';
	import { absoluteUrl } from '$lib/ui/links';
	import CopyableText from './CopyableText.svelte';

	interface Props {
		/** The URL this query opens — the page's own href, so it differs per placement. */
		href: string;
		query: Query;
		/** Whether to show the tangle-cli equivalent alongside the link. */
		cli?: boolean;
		/**
		 * Whether this placement lets you edit the target ref.
		 *
		 * `generate-manifests` is the only subcommand tangle-cli has, and it
		 * needs a ref. Where the ref is editable, an empty one is something to
		 * fill in here; where it isn't — the Applications editor has no ref
		 * field, because listing applications doesn't use one — the flag is
		 * something to add in CI instead. Same missing piece, two different
		 * things to tell someone.
		 */
		targetRefEditable?: boolean;
	}

	let { href, query, cli = false, targetRefEditable = true }: Props = $props();

	let hasTargetRef = $derived(query.targetRef.trim().length > 0);
</script>

<div class="space-y-2">
	<!--
		Absolute, not the bare path: this is a link people copy and send, and
		"/applications?labels=env:test" is not something a colleague can open.
	-->
	<CopyableText caption="Link" text={absoluteUrl(href)} copyLabel="Copy link" />

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
				<Helper class="mt-1">
					{#if targetRefEditable}
						Add a target ref — without one there is nothing to compare against.
					{:else}
						Add <code class="font-mono">--target-ref</code> in CI to pick what these applications are
						compared against.
					{/if}
				</Helper>
			{/if}
		</div>
	{/if}
</div>
