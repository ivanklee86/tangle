<script lang="ts">
	import { Card, Label, Input, Button, Heading } from 'flowbite-svelte';
	import { ArrowRightOutline, CodeBranchOutline, LabelSolid } from 'flowbite-svelte-icons';
	import { isValidLabelFormat } from '$lib/ui/validation';
	import { untrack } from 'svelte';

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

	// The target ref is required — rather than explain that in a toast or a
	// paragraph, the submit button simply stays disabled until it's filled in.
	let canSubmit = $derived(
		targetRef.length > 0 && isValidLabelFormat(labels) && isValidLabelFormat(excludeLabels)
	);
</script>

<Card class="w-full max-w-none justify-center p-6">
	<Heading tag="h2" class="mb-2 text-2xl">Diffs</Heading>

	<div class="space-y-4">
		<Label class="space-y-2">
			<span>Labels</span>
			<Input
				type="text"
				placeholder="Labels in format 'key:value'"
				bind:value={labels}
				size="lg"
				class="ps-11"
			>
				{#snippet left()}
					<LabelSolid class="h-6 w-6" />
				{/snippet}
			</Input>
		</Label>

		<Label class="space-y-2">
			<span>Exclude Labels</span>
			<Input
				type="text"
				placeholder="Labels to exclude in format 'key:value'"
				bind:value={excludeLabels}
				size="lg"
				class="ps-11"
			>
				{#snippet left()}
					<LabelSolid class="h-6 w-6" />
				{/snippet}
			</Input>
		</Label>

		<Label class="space-y-2">
			<span>Target Ref</span>
			<Input type="text" placeholder="Git branch" bind:value={targetRef} size="lg" class="ps-11">
				{#snippet left()}
					<CodeBranchOutline class="h-6 w-6" />
				{/snippet}
			</Input>
		</Label>

		<Button
			color="primary"
			class="w-fit leading-none bg-gradient-to-br from-primary-400 to-primary-700 hover:from-primary-500 hover:to-primary-800 dark:from-primary-500 dark:to-primary-900"
			disabled={!canSubmit}
			onclick={() => onSubmit(labels, excludeLabels, targetRef)}
		>
			See diffs<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
		</Button>
	</div>
</Card>
