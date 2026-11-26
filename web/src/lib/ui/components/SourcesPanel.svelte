<script lang="ts">
	import { Badge } from 'flowbite-svelte';
	import { ChevronDownOutline } from 'flowbite-svelte-icons';
	import { type DiffSource } from '$lib/backend/data';
	import { shortRepo } from '$lib/ui/repos';

	interface Props {
		sources: DiffSource[];
		/** False when nothing was rendered, so there is no target side to show. */
		rendered?: boolean;
	}

	let { sources, rendered = true }: Props = $props();

	let movedCount = $derived(sources.filter((source) => source.moved).length);

	const kindLabels: Record<string, string> = { git: 'Git', helm: 'Helm', oci: 'OCI' };
</script>

<!--
	A native <details>: collapsible with no script, and keyboard and screen
	reader behaviour come with it. Open by default, because which sources moved
	is the first thing to check when a multi-source diff surprises you.
-->
<details open class="group rounded-lg border border-gray-200 dark:border-gray-700">
	<summary
		class="flex cursor-pointer list-none items-center gap-2 px-4 py-2.5 text-sm font-semibold text-gray-900 dark:text-white [&::-webkit-details-marker]:hidden"
	>
		<ChevronDownOutline class="h-4 w-4 -rotate-90 transition-transform group-open:rotate-0" />
		Sources
		<span class="font-normal text-gray-500 dark:text-gray-400">
			{#if rendered}
				{movedCount} of {sources.length} moved
			{:else}
				{sources.length}
			{/if}
		</span>
	</summary>

	<div class="overflow-x-auto border-t border-gray-200 dark:border-gray-700">
		<table class="w-full text-left text-sm">
			<thead class="text-xs text-gray-500 uppercase dark:text-gray-400">
				<tr>
					<th scope="col" class="w-10 px-4 py-2">#</th>
					<th scope="col" class="px-4 py-2">Source</th>
					<th scope="col" class="px-4 py-2">Live</th>
					<th scope="col" class="px-4 py-2">Target</th>
				</tr>
			</thead>
			<tbody>
				{#each sources as source (source.position)}
					<tr
						class="border-t border-gray-200 align-top dark:border-gray-700 {source.moved
							? 'bg-primary-50 dark:bg-primary-950/40'
							: ''}"
					>
						<td class="px-4 py-2 font-mono text-gray-500 dark:text-gray-400">{source.position}</td>
						<td class="px-4 py-2">
							<div class="flex flex-wrap items-center gap-1.5">
								<Badge color="gray">{kindLabels[source.kind] ?? source.kind}</Badge>
								<span class="font-mono text-gray-900 dark:text-white" title={source.repoURL}
									>{shortRepo(source.repoURL)}</span
								>
								{#if source.ref}
									<Badge color="gray" class="font-mono">ref: {source.ref}</Badge>
								{/if}
							</div>
							{#if source.chart}
								<div class="mt-1 text-gray-500 dark:text-gray-400">
									Chart <span class="font-mono text-gray-700 dark:text-gray-300"
										>{source.chart}</span
									>
								</div>
							{:else if source.path}
								<div class="mt-1 text-gray-500 dark:text-gray-400">
									Path <span class="font-mono text-gray-700 dark:text-gray-300">{source.path}</span>
								</div>
							{/if}
						</td>
						<td class="px-4 py-2 font-mono">{source.liveRevision}</td>
						<td class="px-4 py-2">
							{#if !rendered}
								<span class="text-gray-500 dark:text-gray-400">Not rendered</span>
							{:else if source.moved}
								<span class="font-mono font-semibold text-gray-900 dark:text-white"
									>{source.targetRevision}</span
								>
								<!-- D9: where it moved from, whatever kind of revision that was. -->
								<div class="mt-0.5 text-xs text-gray-600 dark:text-gray-400">
									moved from <span class="font-mono">{source.liveRevision}</span>
								</div>
							{:else}
								<span class="font-mono text-gray-500 dark:text-gray-400"
									>{source.targetRevision}</span
								>
								<span class="text-xs text-gray-500 dark:text-gray-400">· unchanged</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</details>
