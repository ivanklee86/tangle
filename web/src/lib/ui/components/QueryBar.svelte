<script lang="ts">
	import { Badge, Button, Heading, Tooltip } from 'flowbite-svelte';
	import { CheckOutline, CodeBranchOutline, EditOutline, LinkOutline } from 'flowbite-svelte-icons';
	import { parseLabels } from '$lib/ui/labels';
	import { absoluteUrl } from '$lib/ui/links';
	import { isEmptyQuery, type Query } from '$lib/ui/query';
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		/** One line under the heading saying what is on screen. */
		summary?: Snippet;
		query: Query;
		/** Diffs compares against a ref, so its chip row ends "against <ref>". */
		showTargetRef?: boolean;
		onEdit: () => void;
		/** The page's own primary action, right of Copy link. */
		actions?: Snippet;
	}

	let { title, summary, query, showTargetRef = false, onEdit, actions }: Props = $props();

	let includes = $derived(parseLabels(query.labels));
	let excludes = $derived(parseLabels(query.excludeLabels));
	let empty = $derived(isEmptyQuery(query));

	let copied: boolean = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copyLink(): Promise<void> {
		try {
			// Built on the configured domain like QueryPreview's link (ADR
			// 0027), so a port-forwarded address bar doesn't leak into it.
			await navigator.clipboard.writeText(
				absoluteUrl(`${window.location.pathname}${window.location.search}`)
			);
			copied = true;
			clearTimeout(timer);
			timer = setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard access can be denied. Don't claim a success that
			// didn't happen.
			copied = false;
		}
	}
</script>

<div
	class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200 bg-gray-50 px-6 py-4 dark:border-gray-700 dark:bg-gray-800"
>
	<div class="min-w-0">
		<Heading tag="h1" class="text-xl font-bold">{title}</Heading>
		<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
			{#if summary}{@render summary()}{/if}
		</p>
	</div>

	<div class="flex flex-wrap items-center gap-2">
		<span class="text-xs font-bold tracking-wider text-gray-500 uppercase dark:text-gray-400">
			Query
		</span>

		{#if empty}
			<!--
				Same words whether nobody has asked yet or somebody submitted a
				query with no filters: before a first query the page opens the
				editor anyway (ADR 0008), so the bar needn't say it twice.
			-->
			<span class="text-sm text-gray-500 italic dark:text-gray-400">No filters</span>
		{:else}
			{#each includes as pair (pair.key)}
				<Badge color="primary" class="font-mono font-semibold">{pair.key}:{pair.value}</Badge>
			{/each}
			{#if excludes.length > 0}
				<span class="text-sm text-gray-500 dark:text-gray-400">excluding</span>
				{#each excludes as pair (pair.key)}
					<Badge color="gray" class="font-mono font-semibold">{pair.key}:{pair.value}</Badge>
				{/each}
			{/if}
		{/if}

		{#if showTargetRef && query.targetRef.length > 0}
			<span class="text-sm text-gray-500 dark:text-gray-400">against</span>
			<Badge color="primary" class="gap-1 font-mono font-semibold">
				<CodeBranchOutline class="h-3 w-3" />
				{query.targetRef}
			</Badge>
		{/if}

		<Button type="button" size="sm" color="alternative" onclick={onEdit}>
			<EditOutline class="me-1.5 h-4 w-4" />Edit query
		</Button>

		<Button
			type="button"
			size="sm"
			color="alternative"
			class="p-2"
			aria-label="Copy link to this view"
			onclick={copyLink}
		>
			{#if copied}
				<CheckOutline class="h-4 w-4" />
			{:else}
				<LinkOutline class="h-4 w-4" />
			{/if}
		</Button>
		<Tooltip>{copied ? 'Copied' : 'Copy link to this view'}</Tooltip>

		{#if actions}{@render actions()}{/if}
	</div>
</div>
