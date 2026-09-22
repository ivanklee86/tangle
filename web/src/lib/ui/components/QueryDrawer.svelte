<script lang="ts">
	import { Badge, Button, Drawer, Heading } from 'flowbite-svelte';
	import { ArrowRightOutline, CloseOutline } from 'flowbite-svelte-icons';
	import { isValidLabelQuery } from '$lib/ui/validation';
	import { type Query } from '$lib/ui/query';
	import QueryEditor from './QueryEditor.svelte';
	import QueryPreview from './QueryPreview.svelte';
	import { untrack } from 'svelte';

	interface Props {
		open?: boolean;
		title: string;
		/** One line saying what applying will cost, e.g. re-rendering manifests. */
		description: string;
		/** The query the page is currently showing; the drawer edits a copy. */
		query: Query;
		targetRefMode?: 'hidden' | 'optional' | 'required';
		applyLabel?: string;
		/**
		 * The URL a given draft would open. Supplied by the page rather than
		 * inferred from targetRefMode, so the preview says where *this* page
		 * would go instead of guessing from an unrelated prop.
		 */
		hrefFor: (query: Query) => string;
		/**
		 * Whether to preview the tangle-cli equivalent alongside the link.
		 * Only the diffs flow has one — see QueryPreview.
		 */
		showCli?: boolean;
		onApply: (query: Query) => void;
	}

	let {
		open = $bindable(false),
		title,
		description,
		query,
		targetRefMode = 'optional',
		applyLabel = 'Apply',
		hrefFor,
		showCli = false,
		onApply
	}: Props = $props();

	// A working copy, so closing without applying leaves the page's query
	// alone. Re-seeded whenever the drawer opens, or a discarded edit would
	// reappear the next time it's opened.
	// untrack: these are one-time seeds from a prop, not a live binding — the
	// $effect below re-seeds them on each open. Without it Svelte warns
	// (state_referenced_locally) that only the initial value is captured,
	// which is exactly what's wanted here.
	let labels: string = $state(untrack(() => query.labels));
	let excludeLabels: string = $state(untrack(() => query.excludeLabels));
	let targetRef: string = $state(untrack(() => query.targetRef));

	let wasOpen: boolean = $state(false);
	$effect(() => {
		if (open && !wasOpen) {
			labels = query.labels;
			excludeLabels = query.excludeLabels;
			targetRef = query.targetRef;
		}
		wasOpen = open;
	});

	let draft = $derived({ labels, excludeLabels, targetRef });

	let changeCount = $derived(
		[
			labels !== query.labels,
			excludeLabels !== query.excludeLabels,
			targetRef !== query.targetRef
		].filter(Boolean).length
	);

	let valid = $derived(
		isValidLabelQuery(labels) &&
			isValidLabelQuery(excludeLabels) &&
			(targetRefMode !== 'required' || targetRef.trim().length > 0)
	);

	function discard(): void {
		open = false;
	}

	function apply(event: SubmitEvent): void {
		event.preventDefault();
		if (!valid) return;
		open = false;
		onApply(draft);
	}
</script>

<Drawer bind:open placement="right" class="flex w-full max-w-xl flex-col p-0">
	<div
		class="flex shrink-0 items-start justify-between gap-4 border-b border-gray-200 px-6 py-4 dark:border-gray-700"
	>
		<div>
			<Heading tag="h2" class="text-lg font-bold">{title}</Heading>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{description}</p>
		</div>
		<Button
			type="button"
			color="alternative"
			class="shrink-0 border-0 p-2"
			aria-label="Close"
			onclick={discard}
		>
			<CloseOutline class="h-4 w-4" />
		</Button>
	</div>

	<form class="flex min-h-0 grow flex-col" onsubmit={apply}>
		<div class="grow space-y-5 overflow-y-auto px-6 py-5">
			<QueryEditor bind:labels bind:excludeLabels bind:targetRef {targetRefMode} />

			<QueryPreview href={hrefFor(draft)} query={draft} cli={showCli} />
		</div>

		<div
			class="flex shrink-0 items-center gap-3 border-t border-gray-200 px-6 py-4 dark:border-gray-700"
		>
			{#if changeCount > 0}
				<Badge color="yellow">
					{changeCount}
					{changeCount === 1 ? 'change' : 'changes'}
				</Badge>
			{/if}
			<div class="grow"></div>
			<Button type="button" color="alternative" onclick={discard}>Discard</Button>
			<Button type="submit" color="primary" disabled={!valid}>
				{applyLabel}<ArrowRightOutline class="ms-2 h-4 w-4" />
			</Button>
		</div>
	</form>
</Drawer>
