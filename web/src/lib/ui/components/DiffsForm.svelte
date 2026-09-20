<script lang="ts">
	import { Card, Label, Input, Button, Heading, Tooltip } from 'flowbite-svelte';
	import { ArrowRightOutline, CodeBranchOutline } from 'flowbite-svelte-icons';
	import { isValidLabelFormat } from '$lib/ui/validation';
	import { untrack } from 'svelte';
	import LabelsInput from './LabelsInput.svelte';

	interface Props {
		initialLabels?: string;
		initialExcludeLabels?: string;
		initialTargetRef?: string;
		onSubmit: (labels: string, excludeLabels: string, targetRef: string) => void;
	}

	let {
		initialLabels = '',
		initialExcludeLabels = '',
		initialTargetRef = '',
		onSubmit
	}: Props = $props();

	let labels: string = $state(untrack(() => initialLabels));
	let excludeLabels: string = $state(untrack(() => initialExcludeLabels));
	let targetRef: string = $state(untrack(() => initialTargetRef));

	// The target ref is required — the submit button stays disabled until
	// it's filled in, with a tooltip on hover explaining why (see below).
	let normalizedTargetRef = $derived(targetRef.trim());
	let canSubmit = $derived(
		normalizedTargetRef.length > 0 &&
			isValidLabelFormat(labels) &&
			isValidLabelFormat(excludeLabels)
	);

	function handleSubmit(event: SubmitEvent): void {
		event.preventDefault();
		if (canSubmit) onSubmit(labels, excludeLabels, normalizedTargetRef);
	}
</script>

<Card class="w-full max-w-none justify-center p-6">
	<Heading tag="h2" class="mb-2 text-2xl">Diffs</Heading>

	<form class="space-y-4" onsubmit={handleSubmit}>
		<LabelsInput label="Labels" bind:value={labels} />
		<LabelsInput label="Exclude Labels" bind:value={excludeLabels} />

		<Label class="space-y-2">
			<span>Target Ref</span>
			<Input type="text" placeholder="Git branch" bind:value={targetRef} size="lg" class="ps-11">
				{#snippet left()}
					<CodeBranchOutline class="h-6 w-6" />
				{/snippet}
			</Input>
		</Label>

		<Button
			type="submit"
			color="primary"
			class="w-fit leading-none bg-gradient-to-br from-primary-400 to-primary-700 hover:from-primary-500 hover:to-primary-800 dark:from-primary-500 dark:to-primary-900"
			disabled={!canSubmit}
		>
			See diffs<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
		</Button>
		{#if normalizedTargetRef.length === 0}
			<Tooltip>Enter a target ref (git branch) to see diffs.</Tooltip>
		{/if}
	</form>
</Card>
