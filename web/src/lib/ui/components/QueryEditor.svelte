<script lang="ts">
	import { Helper, Input, Label, Toggle } from 'flowbite-svelte';
	import { CodeBranchOutline } from 'flowbite-svelte-icons';
	import LabelsInput from './LabelsInput.svelte';
	import TargetReposInput from './TargetReposInput.svelte';

	interface Props {
		labels?: string;
		excludeLabels?: string;
		targetRef?: string;
		/** Repos the target ref belongs to (ADR 0032). */
		targetRepos?: string[];
		/** Let a multi-source app whose git sources are all in one repo use it. */
		inferTargetRepo?: boolean;
		/** Diffs can't run without a ref; Applications ignores it entirely. */
		targetRefMode?: 'hidden' | 'optional' | 'required';
	}

	let {
		labels = $bindable(''),
		excludeLabels = $bindable(''),
		targetRef = $bindable(''),
		targetRepos = $bindable([]),
		inferTargetRepo = $bindable(false),
		targetRefMode = 'optional'
	}: Props = $props();

	const uid = $props.id();
	const targetHelpId = `${uid}-target-help`;
	const inferHelpId = `${uid}-infer-help`;
</script>

<div class="space-y-5">
	<!--
		"all of these" and "any of these" aren't stylistic: the server ANDs the
		includes into one Kubernetes selector and ORs the exclusions out of it,
		so anything vaguer would describe a query Tangle doesn't run.
	-->
	<LabelsInput
		label="Include applications with all of these labels"
		noun="label"
		tone="accent"
		bind:value={labels}
	/>

	<LabelsInput
		label="Exclude applications with any of these labels"
		noun="exclusion"
		tone="neutral"
		instructions={false}
		bind:value={excludeLabels}
	/>

	{#if targetRefMode !== 'hidden'}
		<div
			class="space-y-2 rounded-lg border border-gray-200 p-4 dark:border-gray-700 dark:bg-gray-900"
		>
			<Label for="{uid}-target">
				Target ref
				{#if targetRefMode === 'required'}<span class="text-red-500" aria-hidden="true">*</span
					>{/if}
			</Label>
			<!--
				ps-10 clears the leading icon. Flowbite positions a `left` snippet
				over the field but doesn't pad the input for it, so without this the
				first character sits on top of the branch glyph.
			-->
			<Input
				id="{uid}-target"
				type="text"
				placeholder="Branch, tag or commit"
				required={targetRefMode === 'required'}
				aria-describedby={targetHelpId}
				bind:value={targetRef}
				class="ps-10 font-mono"
			>
				{#snippet left()}
					<CodeBranchOutline class="h-4 w-4 text-gray-500 dark:text-gray-400" />
				{/snippet}
			</Input>
			<Helper id={targetHelpId}>
				Manifests are rendered from this ref and compared with live manifests.
			</Helper>

			<!--
				Only meaningful with a ref, so they live in its box. Both are
				optional: without them, single-source applications diff as they
				always have, and multi-source ones say they need a target repo.
			-->
			<div class="space-y-4 border-t border-gray-200 pt-4 dark:border-gray-700">
				<TargetReposInput bind:value={targetRepos} />
				<div>
					<Toggle bind:checked={inferTargetRepo} aria-describedby={inferHelpId}>
						Infer target repo
					</Toggle>
					<Helper id={inferHelpId} class="mt-1">
						{#if targetRepos.length > 0 && inferTargetRepo}
							<!-- The server only infers without target repos, so say so rather than imply it adds to them. -->
							Ignored while target repos are set: they decide which sources get the target ref.
						{:else}
							With no target repo, an application whose git sources are all in one repo uses that
							repo.
						{/if}
					</Helper>
				</div>
			</div>
		</div>
	{/if}
</div>
