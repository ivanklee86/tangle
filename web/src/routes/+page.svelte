<script lang="ts">
	import { Button, Card, Heading } from 'flowbite-svelte';
	import { ArrowRightOutline } from 'flowbite-svelte-icons';
	import { isValidLabelQuery } from '$lib/ui/validation';
	import { applicationsHref, diffsHref, NO_TARGET_REF_REASON } from '$lib/ui/query';
	import { DisabledReason, QueryEditor, QueryPreview } from '$lib/ui/components';

	let labels: string = $state('');
	let excludeLabels: string = $state('');
	let targetRef: string = $state('');

	let query = $derived({ labels, excludeLabels, targetRef });

	let valid = $derived(isValidLabelQuery(labels) && isValidLabelQuery(excludeLabels));
	// Diffs re-render every matching application against the ref, so there is
	// nothing to run without one — the button says so rather than failing later.
	let hasTargetRef = $derived(targetRef.trim().length > 0);
	let canDiff = $derived(valid && hasTargetRef);

	function go(href: string): void {
		window.location.href = href;
	}
</script>

<svelte:head>
	<title>Home | Tangle</title>
</svelte:head>

<div class="mx-auto mt-10 w-full max-w-3xl">
	<!-- Visually hidden: the form speaks for itself, but the page still needs an h1 to navigate by. -->
	<Heading tag="h1" class="sr-only">Build a label query</Heading>

	<Card class="w-full max-w-none p-6">
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
				<!--
					Home is where both flows start, so it previews both: the link
					the Applications button opens, and the CLI equivalent of the
					diffs button beside it.
				-->
				<QueryPreview href={applicationsHref(query)} {query} cli />

				<div class="mt-4 flex flex-wrap items-center justify-end gap-3">
					<DisabledReason reason={hasTargetRef ? undefined : NO_TARGET_REF_REASON}>
						{#snippet children(describedBy)}
							<Button
								type="button"
								color="alternative"
								disabled={!canDiff}
								aria-describedby={describedBy}
								onclick={() => go(diffsHref(query))}
							>
								See diffs
							</Button>
						{/snippet}
					</DisabledReason>
					<Button type="submit" color="primary" disabled={!valid}>
						See applications<ArrowRightOutline class="ms-2 h-5 w-5" />
					</Button>
				</div>
			</div>
		</form>
	</Card>
</div>
