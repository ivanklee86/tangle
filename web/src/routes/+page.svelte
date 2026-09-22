<script lang="ts">
	import { Button, Card, Heading } from 'flowbite-svelte';
	import { ArrowRightOutline } from 'flowbite-svelte-icons';
	import { isValidLabelQuery } from '$lib/ui/validation';
	import { applicationsHref, diffsHref } from '$lib/ui/query';
	import { CopyableText, QueryEditor } from '$lib/ui/components';

	let labels: string = $state('');
	let excludeLabels: string = $state('');
	let targetRef: string = $state('');

	let query = $derived({ labels, excludeLabels, targetRef });

	let valid = $derived(isValidLabelQuery(labels) && isValidLabelQuery(excludeLabels));
	// Diffs re-render every matching application against the ref, so there is
	// nothing to run without one — the button says so rather than failing later.
	let canDiff = $derived(valid && targetRef.trim().length > 0);

	function go(href: string): void {
		window.location.href = href;
	}
</script>

<svelte:head>
	<title>Home | Tangle</title>
</svelte:head>

<div class="mx-auto mt-10 w-full max-w-3xl">
	<Heading tag="h1" class="text-2xl font-bold">Pick applications by label</Heading>
	<p class="mt-1.5 text-gray-500 dark:text-gray-400">
		Tangle queries every configured Argo CD instance with the same label selector. Build the query
		once, then list the applications or diff them against a git ref.
	</p>

	<Card class="mt-6 w-full max-w-none p-6">
		<!--
			One editor with two submits, rather than two cards that each carried
			their own copy of the label inputs: the query is the same thing in
			both cases, and duplicating it meant retyping to switch intent.
		-->
		<form
			class="space-y-5"
			onsubmit={(event) => {
				event.preventDefault();
				if (valid) go(applicationsHref(query));
			}}
		>
			<QueryEditor bind:labels bind:excludeLabels bind:targetRef targetRefMode="optional" />

			<div class="border-t border-gray-200 pt-4 dark:border-gray-700">
				<CopyableText caption="Link" text={applicationsHref(query)} copyLabel="Copy link" />

				<div class="mt-4 flex flex-wrap items-center justify-end gap-3">
					{#if !canDiff}
						<span id="diff-why" class="text-sm text-gray-500 dark:text-gray-400">
							Enter a target ref to enable diffs
						</span>
					{/if}
					<Button
						type="button"
						color="alternative"
						disabled={!canDiff}
						aria-describedby={canDiff ? undefined : 'diff-why'}
						onclick={() => go(diffsHref(query))}
					>
						See diffs
					</Button>
					<Button type="submit" color="primary" disabled={!valid}>
						See applications<ArrowRightOutline class="ms-2 h-5 w-5" />
					</Button>
				</div>
			</div>
		</form>
	</Card>
</div>
