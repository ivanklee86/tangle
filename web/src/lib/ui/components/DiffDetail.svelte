<script lang="ts">
	import {
		Alert,
		Breadcrumb,
		BreadcrumbItem,
		Button,
		Heading,
		TabItem,
		Tabs,
		Tooltip
	} from 'flowbite-svelte';
	import {
		ArrowUpRightFromSquareOutline,
		CheckOutline,
		FileCopyOutline,
		RefreshOutline
	} from 'flowbite-svelte-icons';
	import { CodeBlock } from 'svhighlight';
	import 'highlight.js/styles/an-old-hope.css';
	import { type DiffRow } from '$lib/ui/diffs';
	import ArgoCDHealthStatus from './ArgoCDHealthStatus.svelte';
	import ArgoCDSyncStatus from './ArgoCDSyncStatus.svelte';
	import ErrorAlert from './ErrorAlert.svelte';

	interface Props {
		row: DiffRow;
		targetRef: string;
		onReload: (row: DiffRow) => void;
	}

	let { row, targetRef, onReload }: Props = $props();

	let copied: boolean = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copyDiff(): Promise<void> {
		try {
			await navigator.clipboard.writeText(row.diff?.response.diffs ?? '');
			copied = true;
			clearTimeout(timer);
			timer = setTimeout(() => (copied = false), 2000);
		} catch {
			copied = false;
		}
	}
</script>

<div class="flex h-full min-w-0 flex-col overflow-y-auto p-6">
	<Breadcrumb aria-label="Diff location">
		<BreadcrumbItem home>Diffs</BreadcrumbItem>
		<BreadcrumbItem>{row.instance}</BreadcrumbItem>
		<BreadcrumbItem>{row.name}</BreadcrumbItem>
	</Breadcrumb>

	<div class="mt-3 flex flex-wrap items-center justify-between gap-3">
		<div class="flex flex-wrap items-center gap-3">
			<Heading tag="h2" class="text-xl font-bold">{row.name}</Heading>
			<ArgoCDHealthStatus healthStatus={row.health} />
			<ArgoCDSyncStatus syncStatus={row.syncStatus} />
			<span class="font-mono text-xs text-gray-500 dark:text-gray-400">
				{row.liveRef} → {targetRef}
			</span>
		</div>
		<div class="flex items-center gap-2">
			<Button size="sm" color="alternative" aria-label="Reload diff" onclick={() => onReload(row)}>
				<RefreshOutline class="h-4 w-4" />
			</Button>
			<Tooltip>Reload diff</Tooltip>
			<Button size="sm" color="alternative" href={row.url} target="_blank" rel="external">
				Open in Argo CD<ArrowUpRightFromSquareOutline class="ms-1.5 h-3 w-3" />
			</Button>
		</div>
	</div>

	<div class="mt-5 min-w-0">
		{#if row.outcome === 'pending'}
			<p class="text-sm text-gray-500 italic dark:text-gray-400">Generating this diff…</p>
		{:else if row.diff?.error}
			<ErrorAlert message={row.diff.errorResponse?.error} />
		{:else if row.diff && row.diff.response.manifestGenerationError.length > 0}
			<Alert color="red" border>
				<span class="font-medium">Couldn't generate manifests</span>
				<p class="mt-2 text-sm">{row.diff.response.manifestGenerationError}</p>
			</Alert>
		{:else if row.diff}
			<Tabs tabStyle="underline">
				<TabItem open>
					{#snippet titleSlot()}
						Diff
						{#if row.outcome === 'changed'}
							<span class="ms-1.5 font-mono text-xs text-green-600 dark:text-green-400"
								>+{row.stats.added}</span
							>
							<span class="ms-1 font-mono text-xs text-red-600 dark:text-red-400"
								>−{row.stats.removed}</span
							>
						{/if}
					{/snippet}

					{#if row.outcome === 'unchanged'}
						<p class="text-sm text-gray-500 italic dark:text-gray-400">
							No differences between <span class="font-mono">{row.liveRef}</span> and
							<span class="font-mono">{targetRef}</span>.
						</p>
					{:else}
						<div class="mb-2 flex justify-end">
							<Button size="xs" color="alternative" aria-label="Copy diff" onclick={copyDiff}>
								{#if copied}
									<CheckOutline class="me-1.5 h-3 w-3" />Copied
								{:else}
									<FileCopyOutline class="me-1.5 h-3 w-3" />Copy diff
								{/if}
							</Button>
						</div>
						<CodeBlock language="diff" code={row.diff.response.diffs} showLineNumbers={false} />
					{/if}
				</TabItem>

				<!--
					Both manifests are already in the response, so showing the live
					side costs nothing and answers "what is actually deployed" —
					which is usually the next question after reading a diff.
				-->
				<TabItem>
					{#snippet titleSlot()}Target manifests{/snippet}
					<CodeBlock language="yaml" code={row.diff.response.targetManifests} />
				</TabItem>
				<TabItem>
					{#snippet titleSlot()}Live manifests{/snippet}
					<CodeBlock language="yaml" code={row.diff.response.liveManifests} />
				</TabItem>
			</Tabs>
		{/if}
	</div>
</div>
