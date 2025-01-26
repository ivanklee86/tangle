<script lang="ts">
	import { A, Card, Label, Input, GradientButton } from 'flowbite-svelte';
	import { CodeBranchOutline, ArrowRightOutline, LabelSolid } from 'flowbite-svelte-icons';

	let labels: string = $state('');
	let targetRef: string = $state('');

	function generateApplicationsUrl(labels: string): string {
		const BASE_URL = '/applications';

		let url: string = BASE_URL;

		if (labels.length > 0) {
			url += `?labels=${labels}`;
		}

		return url;
	}

	function generateDiffUrl(targetRef: string, labels: string): string {
		const BASE_URL = '/diffs';

		let url: string = BASE_URL;

		if (targetRef.length > 0) {
			url += `?targetRef=${targetRef}`;
		}

		if (labels.length > 0) {
			url += `&labels=${labels}`;
		}

		return url;
	}
</script>

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
	<A href={generateApplicationsUrl(labels)}>
		<GradientButton color="pinkToOrange" class="w-fit">
			See applications<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
		</GradientButton>
	</A>
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
	<A href={generateDiffUrl(targetRef, labels)}>
		<GradientButton color="pinkToOrange" class="w-fit">
			See diffs<ArrowRightOutline class="w-6 h-6 ms-2 text-white" />
		</GradientButton>
	</A>
</Card>
<br />
