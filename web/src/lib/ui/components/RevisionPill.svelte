<script lang="ts">
	import { CodeBranchOutline, TagOutline } from 'flowbite-svelte-icons';
	import { type SourceKind } from '#lib/backend/data.ts';
	import { shortRevision } from '#lib/ui/repos.ts';

	interface Props {
		revision: string;
		/** Helm and OCI revisions are versions, not git refs. */
		kind?: SourceKind;
	}

	let { revision, kind = 'git' }: Props = $props();

	let isCommit = $derived(kind === 'git' && /^[0-9a-f]{40}$/i.test(revision));
	let label = $derived(isCommit ? 'Commit' : kind === 'git' ? 'Revision' : 'Version');
</script>

<!--
	A pill, so a revision never reads as the end of a path: a branch or tag
	gets a branch glyph, a chart version a tag glyph, and a commit SHA a
	"commit" prefix with the full SHA on hover.
-->
<span
	class="inline-flex max-w-full items-center gap-1 rounded-full border border-gray-300 bg-white px-2 py-0.5 font-mono text-xs text-gray-800 dark:border-gray-600 dark:bg-gray-900 dark:text-gray-200"
	title="{label}: {revision}"
>
	{#if isCommit}
		<span class="font-sans text-gray-500 dark:text-gray-400">commit</span>
	{:else if kind === 'git'}
		<CodeBranchOutline class="h-3 w-3 shrink-0 text-gray-500 dark:text-gray-400" />
	{:else}
		<TagOutline class="h-3 w-3 shrink-0 text-gray-500 dark:text-gray-400" />
	{/if}
	<span class="truncate">{shortRevision(revision)}</span>
</span>
