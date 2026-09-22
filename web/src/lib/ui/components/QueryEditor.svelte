<script lang="ts">
	import { Helper, Input, Label } from 'flowbite-svelte';
	import { CodeBranchOutline } from 'flowbite-svelte-icons';
	import LabelsInput from './LabelsInput.svelte';

	interface Props {
		labels?: string;
		excludeLabels?: string;
		targetRef?: string;
		/** Diffs can't run without a ref; Applications ignores it entirely. */
		targetRefMode?: 'hidden' | 'optional' | 'required';
	}

	let {
		labels = $bindable(''),
		excludeLabels = $bindable(''),
		targetRef = $bindable(''),
		targetRefMode = 'optional'
	}: Props = $props();

	const uid = $props.id();
	const targetHelpId = `${uid}-target-help`;
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
		optional
		instructions={false}
		bind:value={excludeLabels}
	/>

	{#if targetRefMode !== 'hidden'}
		<div
			class="space-y-2 rounded-lg border border-gray-200 p-4 dark:border-gray-700 dark:bg-gray-900"
		>
			<Label for="{uid}-target" class="flex items-baseline justify-between gap-4">
				<span>
					Target ref
					{#if targetRefMode === 'required'}<span class="text-red-500" aria-hidden="true">*</span
						>{/if}
				</span>
				{#if targetRefMode === 'optional'}
					<span class="text-xs font-normal text-gray-500 dark:text-gray-400"
						>only needed for diffs</span
					>
				{/if}
			</Label>
			<Input
				id="{uid}-target"
				type="text"
				placeholder="branch, tag or commit"
				required={targetRefMode === 'required'}
				aria-describedby={targetHelpId}
				bind:value={targetRef}
				class="font-mono"
			>
				{#snippet left()}
					<CodeBranchOutline class="h-4 w-4 text-gray-500 dark:text-gray-400" />
				{/snippet}
			</Input>
			<Helper id={targetHelpId}>
				Manifests are rendered from this ref and compared with each application's live ref.
			</Helper>
		</div>
	{/if}
</div>
