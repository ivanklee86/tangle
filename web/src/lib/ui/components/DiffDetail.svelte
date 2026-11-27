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
	import { ArrowUpRightFromSquareOutline, RefreshOutline } from 'flowbite-svelte-icons';
	import { CodeBlock } from 'svhighlight';
	import 'highlight.js/styles/an-old-hope.css';
	import { type DiffRow } from '#lib/ui/diffs.ts';
	import { normalizeRepo, shortRepo, type RepoUsage } from '#lib/ui/repos.ts';
	import ArgoCDHealthStatus from './ArgoCDHealthStatus.svelte';
	import ArgoCDSyncStatus from './ArgoCDSyncStatus.svelte';
	import ErrorAlert from './ErrorAlert.svelte';
	import SourcesPanel from './SourcesPanel.svelte';

	interface Props {
		row: DiffRow;
		targetRef: string;
		/** The query's target repos, to say why an application isn't affected. */
		targetRepos?: string[];
		onReload: (row: DiffRow) => void;
		/** Adds repos to the query's target repos, which re-runs every diff (D8). */
		onAddTargetRepos?: (repos: string[]) => void;
		/** Which repos the listed applications use, to say how far a candidate reaches. */
		usage?: RepoUsage[];
	}

	let {
		row,
		targetRef,
		targetRepos = [],
		onReload,
		onAddTargetRepos,
		usage = []
	}: Props = $props();

	/**
	 * "used by 6 other apps": adding a candidate re-runs every application,
	 * and those that use it may change from "not affected" to a diff. The
	 * count leaves this application out, when the list says it uses the repo.
	 */
	let ownRepos = $derived(
		new Set(row.sources.filter((s) => s.kind === 'git').map((s) => normalizeRepo(s.repoURL)))
	);

	function otherApps(candidate: string): string {
		const repo = normalizeRepo(candidate);
		const apps = usage.find((entry) => entry.repo === repo)?.apps ?? 0;
		const others = apps - (ownRepos.has(repo) ? 1 : 0);
		if (others <= 0) return '';
		return `used by ${others} other ${others === 1 ? 'app' : 'apps'}`;
	}

	// Explicit colours: Flowbite's alerts render as a light block in dark mode,
	// with text that is hard to read in either theme.
	const WARNING_ALERT =
		'border-yellow-400 bg-yellow-50 text-yellow-900 dark:border-yellow-600 dark:bg-gray-800 dark:text-yellow-200';
	const ERROR_ALERT =
		'border-red-400 bg-red-50 text-red-900 dark:border-red-700 dark:bg-gray-800 dark:text-red-300';

	let response = $derived(row.diff?.response);
	let sources = $derived(response?.sources ?? []);
	let candidates = $derived(response?.candidateRepos ?? []);
	let movedSources = $derived(sources.filter((source) => source.moved));

	// A one-entry spec.sources reports no liveRef, so its live side is its
	// source's revision.
	let liveLabel = $derived(
		row.liveRef || (row.sources.length === 1 ? row.sources[0].targetRevision : '')
	);

	// The first candidate is picked to begin with, so the action has something
	// to add even if the person doesn't touch the list.
	// Writable: the checkboxes change it, and new candidates reset it.
	let picked: string[] = $derived(candidates.slice(0, 1));

	let addLabel = $derived(
		picked.length === 1
			? `Add ${shortRepo(picked[0])} to target repos`
			: `Add ${picked.length} repos to target repos`
	);
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
				{#if row.sources.length > 1}
					{row.sources.length} sources → {targetRef}
				{:else}
					{liveLabel} → {targetRef}
				{/if}
			</span>
		</div>
		<div class="flex items-center gap-2">
			<Button size="sm" color="alternative" aria-label="Reload diff" onclick={() => onReload(row)}>
				<RefreshOutline class="h-4 w-4" />
			</Button>
			<Tooltip>Reload diff</Tooltip>
			<Button size="sm" color="alternative" href={row.url} target="_blank" rel="external">
				Open in ArgoCD<ArrowUpRightFromSquareOutline class="ms-1.5 h-3 w-3" />
			</Button>
		</div>
	</div>

	<div class="mt-5 min-w-0">
		{#if row.outcome === 'pending'}
			<p class="text-sm text-gray-500 italic dark:text-gray-400">Generating this diff…</p>
		{:else if row.diff?.error}
			<ErrorAlert message={row.diff.errorResponse?.error} />
		{:else if row.outcome === 'needsTargetRepo'}
			<!--
				Not an error in the application: the query didn't say which repo
				the ref belongs to. The fix is one action, and it changes the whole
				query, because a target repo applies to every application (D8).
			-->
			<Alert color="yellow" border class={WARNING_ALERT}>
				<span class="font-medium">
					Which repo is <span class="font-mono">{targetRef}</span> in?
				</span>
				<p class="mt-2 text-sm">
					{#if candidates.length === 1}
						Its git sources are all in <span class="font-mono" title={candidates[0]}>
							{shortRepo(candidates[0])}
						</span>, but the query has no target repo, so nothing was rendered. Add it below, or
						turn on Infer target repo. CI fails on this until one of them is set.
					{:else if candidates.length > 1}
						This application has sources in {candidates.length} repos, and the query doesn't say which
						one the target ref belongs to, so nothing was rendered. CI fails on this until a target repo
						is given.
					{:else}
						This application has several sources and none of them is a git repo, so a git ref can't
						apply to it.
					{/if}
				</p>

				{#if candidates.length > 0 && onAddTargetRepos}
					<fieldset class="mt-4 space-y-2 border-0 p-0">
						<legend class="mb-2 text-sm font-semibold">Candidate repos</legend>
						{#each candidates as candidate (candidate)}
							<label class="flex items-center gap-2 text-sm">
								<input
									type="checkbox"
									class="text-primary-600 h-4 w-4 rounded"
									value={candidate}
									bind:group={picked}
								/>
								<span class="font-mono" title={candidate}>{shortRepo(candidate)}</span>
								{#if otherApps(candidate)}
									<span class="text-xs opacity-80">· {otherApps(candidate)}</span>
								{/if}
							</label>
						{/each}
					</fieldset>
					<div class="mt-4 flex flex-wrap items-center gap-3">
						<Button
							size="sm"
							color="primary"
							disabled={picked.length === 0}
							onclick={() => onAddTargetRepos(picked)}
						>
							{addLabel}
						</Button>
						<span class="text-xs">Re-runs every application, so others may change status.</span>
					</div>
				{/if}
			</Alert>
			{#if sources.length > 0}
				<div class="mt-4"><SourcesPanel {sources} rendered={false} /></div>
			{/if}
		{:else if row.outcome === 'notAffected'}
			<div
				class="rounded-lg border border-dashed border-gray-300 bg-gray-50 p-4 dark:border-gray-600 dark:bg-gray-800"
			>
				<p class="font-medium text-gray-700 dark:text-gray-300">Not affected</p>
				<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
					No sources in
					{#each targetRepos as repo, index (repo)}
						{index > 0 ? (index === targetRepos.length - 1 ? ' or ' : ', ') : ''}<span
							class="font-mono"
							title={repo}
						>
							{shortRepo(repo)}
						</span>
					{/each}, so nothing was rendered.
				</p>
				{#if sources.length > 0}
					<ul class="mt-3 list-none space-y-1 p-0 text-sm text-gray-600 dark:text-gray-400">
						{#each sources as source (source.position)}
							<li>
								Uses <span class="font-mono" title={source.repoURL}>
									{shortRepo(source.repoURL)}
								</span>
								at <span class="font-mono">{source.liveRevision}</span>.
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		{:else if row.diff && row.diff.response.manifestGenerationError.length > 0}
			<Alert color="red" border class={ERROR_ALERT}>
				<span class="font-medium">Couldn't generate manifests</span>
				<p class="mt-2 text-sm">{row.diff.response.manifestGenerationError}</p>
				{#if sources.length > 1 && movedSources.length > 0}
					<!--
						#293: ArgoCD's error rarely says which source failed. The
						usual cause is the target ref missing from a repo it was sent
						to, so name those.
					-->
					<p class="mt-2 text-sm">
						<span class="font-mono">{targetRef}</span> was sent to
						{movedSources.length === 1 ? 'source' : 'sources'}
						{#each movedSources as source, index (source.position)}{index > 0
								? index === movedSources.length - 1
									? ' and '
									: ', '
								: ''}{source.position} (<span class="font-mono" title={source.repoURL}>
								{shortRepo(source.repoURL)}
							</span>){/each}. Check that it exists there.
					</p>
				{/if}
			</Alert>
			{#if sources.length > 1}
				<div class="mt-4"><SourcesPanel {sources} /></div>
			{/if}
		{:else if row.diff}
			{#if sources.length > 1}
				<div class="mb-4"><SourcesPanel {sources} /></div>
			{/if}
			<Tabs tabStyle="underline">
				<TabItem open>
					{#snippet titleSlot()}
						Diff
						{#if row.outcome === 'changed'}
							<span class="ms-1.5 font-mono text-xs text-green-600 dark:text-green-400">
								+{row.stats.added}
							</span>
							<span class="ms-1 font-mono text-xs text-red-600 dark:text-red-400">
								−{row.stats.removed}
							</span>
						{/if}
					{/snippet}

					{#if row.outcome === 'unchanged'}
						<p class="text-sm text-gray-500 italic dark:text-gray-400">
							{#if row.sources.length > 1}
								No differences between the live revisions and
								<span class="font-mono">{targetRef}</span>.
							{:else}
								No differences between <span class="font-mono">{liveLabel}</span> and
								<span class="font-mono">{targetRef}</span>.
							{/if}
						</p>
					{:else}
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
