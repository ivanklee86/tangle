<script lang="ts">
	import { Card, Label, Input, Button, Heading } from 'flowbite-svelte';
	import { ArrowRightOutline, LabelSolid } from 'flowbite-svelte-icons';
	import { isValidLabelFormat } from '$lib/ui/validation';

	interface Props {
		onSubmit: (labels: string, excludeLabels: string) => void;
	}

	let { onSubmit }: Props = $props();

	let labels: string = $state('');
	let excludeLabels: string = $state('');

	let canSubmit = $derived(isValidLabelFormat(labels) && isValidLabelFormat(excludeLabels));

	function handleSubmit(event: SubmitEvent): void {
		event.preventDefault();
		if (canSubmit) onSubmit(labels, excludeLabels);
	}
</script>

<Card class="w-full max-w-none justify-center p-6">
	<Heading tag="h2" class="mb-2 text-2xl">Applications</Heading>

	<form class="space-y-4" onsubmit={handleSubmit}>
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

		<Button
			type="submit"
			color="primary"
			class="w-fit leading-none bg-gradient-to-br from-primary-400 to-primary-700 hover:from-primary-500 hover:to-primary-800 dark:from-primary-500 dark:to-primary-900"
			disabled={!canSubmit}
		>
			See applications<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
		</Button>
	</form>
</Card>
