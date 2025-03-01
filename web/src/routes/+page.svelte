<script lang="ts">
	import { A, Card, Label, Input, GradientButton, Toast } from 'flowbite-svelte';
	import {
		CodeBranchOutline,
		ArrowRightOutline,
		LabelSolid,
		ExclamationCircleSolid
	} from 'flowbite-svelte-icons';

	// User inputs
	let labels: string = $state('');
	let targetRef: string = $state('');

	// Error flags
	let noRefSpecified: boolean = $state(false);
	let invalidLabels: boolean = $state(false);

	// Other strings
	const LABEL_CHECK: RegExp = /^[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*$/;

	function redirectToApplications(labels: string): void {
		if (labels.length != 0 && !LABEL_CHECK.test(labels)) {
			invalidLabels = true;
			return;
		}

		const BASE_URL = '/applications';

		let url: string = BASE_URL;

		if (labels.length > 0) {
			url += `?labels=${labels}`;
		}

		window.location.href = url;
	}

	function redirectToDiff(targetRef: string, labels: string): void {
		if (targetRef.length === 0) {
			noRefSpecified = true;
			return;
		}

		if (labels.length != 0 && !LABEL_CHECK.test(labels)) {
			invalidLabels = true;
			return;
		}

		const BASE_URL = '/diffs';

		let url: string = BASE_URL;

		if (targetRef.length > 0) {
			url += `?targetRef=${targetRef}`;
		}

		if (labels.length > 0) {
			url += `&labels=${labels}`;
		}

		window.location.href = url;
	}
</script>

{#if noRefSpecified}
	<Toast color="red" position="top-right" on:close={() => (noRefSpecified = false)}>
		<svelte:fragment slot="icon">
			<ExclamationCircleSolid class="w-5 h-5" />
			<span class="sr-only">Warning icon</span>
		</svelte:fragment>
		You must provide a target git ref to generate a diff!
	</Toast>
{/if}

{#if invalidLabels}
	<Toast color="red" position="top-right" on:close={() => (invalidLabels = false)}>
		<svelte:fragment slot="icon">
			<ExclamationCircleSolid class="w-5 h-5" />
			<span class="sr-only">Warning icon</span>
		</svelte:fragment>
		Invalid label! Labels must be in format "foo:bar" and separated with commas.
	</Toast>
{/if}

<br />
<Card class="m-auto justify-center">
	<h5 class="mb-2 text-2xl font-bold tracking-tight text-gray-900 dark:text-white">Applications</h5>

	<Label class="space-y-2">
		<span>Labels</span>
		<Input type="text" placeholder="Labels in format 'key:value'" bind:value={labels} size="lg">
			<LabelSolid slot="left" class="w-6 h-6" />
		</Input>
	</Label>

	<br />
	<GradientButton
		color="pinkToOrange"
		class="w-fit"
		on:click={() => redirectToApplications(labels)}
	>
		See applications<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
	</GradientButton>
</Card>

<br />

<Card class="justify-center m-auto">
	<h5 class="mb-2 text-2xl font-bold tracking-tight text-gray-900 dark:text-white">Diffs</h5>

	<Label class="space-y-2">
		<span>Labels</span>
		<Input type="text" placeholder="Labels in format 'key:value'" bind:value={labels} size="lg">
			<LabelSolid slot="left" class="w-6 h-6" />
		</Input>
	</Label>

	<Label class="space-y-2">
		<span>Target Ref</span>
		<Input type="text" placeholder="Git branch" bind:value={targetRef} size="lg">
			<CodeBranchOutline slot="left" class="w-6 h-6" />
		</Input>
	</Label>
	<br />
	<GradientButton
		color="pinkToOrange"
		class="w-fit"
		on:click={() => redirectToDiff(targetRef, labels)}
	>
		See diffs<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
	</GradientButton>
</Card>
<br />
