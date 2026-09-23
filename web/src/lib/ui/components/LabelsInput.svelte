<script lang="ts">
	import { Badge, Button, Helper, Input, Tooltip } from 'flowbite-svelte';
	import { CloseOutline } from 'flowbite-svelte-icons';
	import { isValidLabelFormat } from '$lib/ui/validation';
	import { parseLabelsStrict, serializeLabels, type LabelPair } from '$lib/ui/labels';
	import { untrack } from 'svelte';

	interface Props {
		value?: string;
		/** The section heading, e.g. "Include applications with all of these labels". */
		label: string;
		/** Short name used in aria-labels and messages, e.g. "label" or "exclusion". */
		noun?: string;
		/** Chips render in the accent colour for includes, neutral for exclusions. */
		tone?: 'accent' | 'neutral';
		/**
		 * Whether to show the resting how-to line. The rules are the same for
		 * every one of these, so repeating them under each input is noise —
		 * problems still report themselves either way.
		 */
		instructions?: boolean;
	}

	let {
		value = $bindable(''),
		label,
		noun = 'label',
		tone = 'accent',
		instructions = true
	}: Props = $props();

	const uid = $props.id();
	const hintId = `${uid}-hint`;

	const parsed = untrack(() => parseLabelsStrict(value));

	let pairs: LabelPair[] = $state(parsed.pairs);
	let draftKey: string = $state('');
	let draftValue: string = $state('');

	// A value arriving with segments we can't use — a hand-edited URL, or a
	// link from an older Tangle — only renders the usable chips. Say so once,
	// rather than silently showing a narrower query than the link described;
	// the server would reject the same string with a 400 (ADR 0026).
	let droppedOnLoad: string[] = $state([
		...parsed.invalid.map((segment) => `"${segment}" isn't key:value`),
		...parsed.duplicateKeys.map((key) => `"${key}" appeared more than once`)
	]);

	// Reserialize immediately so the bound value matches the chips actually
	// rendered, rather than leaving a malformed string bound that a caller
	// would keep validating and could get stuck on.
	value = untrack(() => serializeLabels(pairs));

	let keys = $derived(new Set(pairs.map((pair) => pair.key)));
	let isDuplicate = $derived(draftKey.length > 0 && keys.has(draftKey));
	let wellFormed = $derived(isValidLabelFormat(`${draftKey}:${draftValue}`));
	let canAdd = $derived(wellFormed && !isDuplicate);

	let count = $derived(
		pairs.length === 0 ? 'none' : pairs.length === 1 ? `1 ${noun}` : `${pairs.length} ${noun}s`
	);

	// Only shown after a failed add attempt, so an untouched row doesn't start
	// out looking like an error.
	let attemptedAdd: boolean = $state(false);

	let hint = $derived.by(() => {
		if (isDuplicate) return `"${draftKey}" is already used. Each key can be used once.`;
		if (canAdd) return null;
		const keyEmpty = draftKey.length === 0;
		const valueEmpty = draftValue.length === 0;
		if (keyEmpty && valueEmpty) return `Enter a key and value to add this ${noun}.`;
		if (keyEmpty) return `Enter a key to add this ${noun}.`;
		if (valueEmpty) return `Enter a value to add this ${noun}.`;
		return "Colons and commas separate pairs, so a key or value can't contain them.";
	});

	// A duplicate key shows immediately — the user can see the chip they're
	// colliding with — while an empty box waits for an actual add attempt, so
	// an untouched row doesn't greet anyone with an error.
	let showProblem = $derived(hint !== null && (isDuplicate || attemptedAdd));

	function addPair(): void {
		if (!canAdd) {
			attemptedAdd = true;
			return;
		}
		attemptedAdd = false;
		droppedOnLoad = [];
		pairs = [...pairs, { key: draftKey, value: draftValue }];
		draftKey = '';
		draftValue = '';
		value = serializeLabels(pairs);
	}

	function removePair(index: number): void {
		droppedOnLoad = [];
		pairs = pairs.filter((_, i) => i !== index);
		value = serializeLabels(pairs);
	}

	function handleDraftKeydown(event: KeyboardEvent): void {
		if (event.key !== 'Enter') return;
		// This input lives inside a form whose submit runs the query; Enter
		// here means "add this pair", never "run it".
		event.preventDefault();
		addPair();
	}
</script>

<fieldset class="space-y-2.5 border-0 p-0">
	<legend class="sr-only">{label}</legend>
	<div class="flex items-baseline justify-between gap-4">
		<span class="text-sm font-semibold text-gray-900 dark:text-white">
			{label}
		</span>
		<span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{count}</span>
	</div>

	{#if pairs.length > 0}
		<ul class="flex list-none flex-wrap gap-2 p-0">
			{#each pairs as pair, index (pair.key)}
				<li>
					<Badge
						large
						color={tone === 'accent' ? 'primary' : 'gray'}
						class="gap-1 pe-1 font-mono font-semibold"
					>
						{pair.key}:{pair.value}
						<button
							type="button"
							aria-label="Remove {noun} {pair.key}:{pair.value}"
							class="ms-0.5 inline-flex h-5 w-5 items-center justify-center rounded-sm hover:bg-black/10 dark:hover:bg-white/10"
							onclick={() => removePair(index)}
						>
							<CloseOutline class="h-3 w-3" />
						</button>
					</Badge>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="flex items-start gap-2">
		<Input
			type="text"
			placeholder="Key"
			aria-label="{label} key"
			aria-invalid={isDuplicate || (attemptedAdd && draftKey.length === 0)}
			aria-describedby={hintId}
			bind:value={draftKey}
			color={isDuplicate || (attemptedAdd && draftKey.length === 0) ? 'red' : 'default'}
			class="font-mono"
			onkeydown={handleDraftKeydown}
		/>
		<span aria-hidden="true" class="pt-2 font-mono text-gray-500 dark:text-gray-400">:</span>
		<Input
			type="text"
			placeholder="Value"
			aria-label="{label} value"
			aria-invalid={attemptedAdd && draftValue.length === 0}
			aria-describedby={hintId}
			bind:value={draftValue}
			color={attemptedAdd && draftValue.length === 0 ? 'red' : 'default'}
			class="font-mono"
			onkeydown={handleDraftKeydown}
		/>
		<Button
			type="button"
			color="alternative"
			disabled={!canAdd}
			onclick={addPair}
			aria-label="Add {noun}">Add</Button
		>
		{#if hint}
			<Tooltip>{hint}</Tooltip>
		{/if}
	</div>

	<!--
		One helper slot, but it never goes quiet when something is wrong: a
		failed add says which box is empty, a repeated key says so by name, and
		a link that arrived with unusable segments reports them. The bare
		instruction is only the resting state.
	-->
	<Helper id={hintId} color={showProblem ? 'red' : 'gray'} aria-live="polite">
		{#if showProblem}
			{hint}
		{:else if droppedOnLoad.length > 0}
			Dropped from the link: {droppedOnLoad.join(', ')}.
		{:else if instructions}
			Press Enter to add. Colons and commas are reserved.
		{/if}
	</Helper>
</fieldset>
