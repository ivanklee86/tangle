<script lang="ts">
	import { Card, Button, Heading } from 'flowbite-svelte';
	import { ArrowRightOutline } from 'flowbite-svelte-icons';
	import { isValidLabelFormat } from '$lib/ui/validation';
	import LabelsInput from './LabelsInput.svelte';

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
		<LabelsInput label="Labels" bind:value={labels} />
		<LabelsInput label="Exclude Labels" noun="exclusion" bind:value={excludeLabels} />

		<Button type="submit" color="primary" class="w-fit leading-none" disabled={!canSubmit}>
			See applications<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
		</Button>
	</form>
</Card>
