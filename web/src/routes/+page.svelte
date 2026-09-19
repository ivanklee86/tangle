<script lang="ts">
	import { Card, Label, Input, GradientButton, Toast } from 'flowbite-svelte';
	import {
		CodeBranchOutline,
		ArrowRightOutline,
		LabelSolid,
		ExclamationCircleSolid
	} from 'flowbite-svelte-icons';
	import { buildQuery } from '$lib/backend/url';
	import { isValidLabelFormat } from '$lib/ui/validation';

	// User inputs
	let labels: string = $state('');
	let excludeLabels: string = $state('');
	let targetRef: string = $state('');

	// Error flags
	let noRefSpecified: boolean = $state(false);
	let invalidLabels: boolean = $state(false);

	function redirectToApplications(labels: string, excludeLabels: string): void {
		if (!isValidLabelFormat(labels)) {
			invalidLabels = true;
			return;
		}

		window.location.href = `/applications${buildQuery({ labels, excludeLabels })}`;
	}

	function redirectToDiff(targetRef: string, labels: string, excludeLabels: string): void {
		if (targetRef.length === 0) {
			noRefSpecified = true;
			return;
		}

		if (!isValidLabelFormat(labels)) {
			invalidLabels = true;
			return;
		}

		window.location.href = `/diffs${buildQuery({ targetRef, labels, excludeLabels })}`;
	}
</script>

<svelte:head>
	<title>Tangle - Home</title>
</svelte:head>

{#if noRefSpecified}
	<Toast color="red" position="top-right" onclose={() => (noRefSpecified = false)}>
		{#snippet icon()}
			<ExclamationCircleSolid class="h-5 w-5" />
			<span class="sr-only">Warning icon</span>
		{/snippet}
		You must provide a target git ref to generate a diff!
	</Toast>
{/if}

{#if invalidLabels}
	<Toast color="red" position="top-right" onclose={() => (invalidLabels = false)}>
		{#snippet icon()}
			<ExclamationCircleSolid class="h-5 w-5" />
			<span class="sr-only">Warning icon</span>
		{/snippet}
		Invalid label! Labels must be in format "foo:bar" and separated with commas.
	</Toast>
{/if}

<br />
<Card class="m-auto justify-center">
	<h5 class="mb-2 text-2xl font-bold tracking-tight text-gray-900 dark:text-white">Applications</h5>

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

	<br />

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

	<br />

	<GradientButton
		color="pinkToOrange"
		class="w-fit"
		onclick={() => redirectToApplications(labels, excludeLabels)}
	>
		See applications<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
	</GradientButton>
</Card>

<br />

<Card class="justify-center m-auto">
	<h5 class="mb-2 text-2xl font-bold tracking-tight text-gray-900 dark:text-white">Diffs</h5>

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
	<br />

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
	<br />

	<Label class="space-y-2">
		<span>Target Ref</span>
		<Input type="text" placeholder="Git branch" bind:value={targetRef} size="lg" class="ps-11">
			{#snippet left()}
				<CodeBranchOutline class="h-6 w-6" />
			{/snippet}
		</Input>
	</Label>
	<br />
	<GradientButton
		color="pinkToOrange"
		class="w-fit"
		onclick={() => redirectToDiff(targetRef, labels, excludeLabels)}
	>
		See diffs<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
	</GradientButton>
</Card>
<br />
