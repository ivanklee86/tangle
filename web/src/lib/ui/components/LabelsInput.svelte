<script lang="ts">
	import { Label, Input, Button, Badge, Helper, Tooltip } from 'flowbite-svelte';
	import { PlusOutline } from 'flowbite-svelte-icons';
	import { isValidLabelFormat } from '$lib/ui/validation';
	import { parseLabels, serializeLabels, type LabelPair } from '$lib/ui/labels';
	import { untrack } from 'svelte';

	interface Props {
		value?: string;
		label: string;
		keyPlaceholder?: string;
		valuePlaceholder?: string;
	}

	let {
		value = $bindable(''),
		label,
		keyPlaceholder = 'Key',
		valuePlaceholder = 'Value'
	}: Props = $props();

	let pairs: LabelPair[] = $state(untrack(() => parseLabels(value)));
	let draftKey: string = $state('');
	let draftValue: string = $state('');

	let canAdd = $derived(isValidLabelFormat(`${draftKey}:${draftValue}`));

	// Only shown after a failed add attempt (Enter or the add button) — not
	// on every keystroke, so an empty row doesn't start out looking like an
	// error before the user has done anything.
	let attemptedAdd: boolean = $state(false);
	let keyMissing = $derived(attemptedAdd && draftKey.length === 0);
	let valueMissing = $derived(attemptedAdd && draftValue.length === 0);

	// Nudges hovering over the add button while it's disabled, regardless of
	// whether an add was attempted yet — unlike keyMissing/valueMissing above,
	// this covers the pristine, nothing-typed-in state too.
	let addHint = $derived.by(() => {
		if (canAdd) return null;
		const keyEmpty = draftKey.length === 0;
		const valueEmpty = draftValue.length === 0;
		if (keyEmpty && valueEmpty) return 'Enter a key and value to add this label.';
		if (keyEmpty) return 'Enter a key to add this label.';
		if (valueEmpty) return 'Enter a value to add this label.';
		return "Key and value can't contain ':' or ','.";
	});

	function addPair(): void {
		if (!canAdd) {
			attemptedAdd = true;
			return;
		}
		attemptedAdd = false;
		pairs = [...pairs, { key: draftKey, value: draftValue }];
		draftKey = '';
		draftValue = '';
		value = serializeLabels(pairs);
	}

	function removePair(index: number): void {
		pairs = pairs.filter((_, i) => i !== index);
		value = serializeLabels(pairs);
	}

	function handleDraftKeydown(event: KeyboardEvent): void {
		if (event.key !== 'Enter') return;
		event.preventDefault();
		addPair();
	}
</script>

<div class="space-y-2">
	<Label>{label}</Label>
	<div class="flex items-start gap-2">
		<div class="flex-1 space-y-1">
			<Input
				type="text"
				placeholder={keyPlaceholder}
				aria-label="{label} key"
				bind:value={draftKey}
				color={keyMissing ? 'red' : 'default'}
				size="lg"
				onkeydown={handleDraftKeydown}
			/>
			{#if keyMissing}
				<Helper color="red">Key is required</Helper>
			{/if}
		</div>
		<div class="flex-1 space-y-1">
			<Input
				type="text"
				placeholder={valuePlaceholder}
				aria-label="{label} value"
				bind:value={draftValue}
				color={valueMissing ? 'red' : 'default'}
				size="lg"
				onkeydown={handleDraftKeydown}
			/>
			{#if valueMissing}
				<Helper color="red">Value is required</Helper>
			{/if}
		</div>
		<Button
			type="button"
			color="alternative"
			disabled={!canAdd}
			onclick={addPair}
			aria-label="Add {label}"
		>
			<PlusOutline class="h-5 w-5" />
		</Button>
		{#if addHint}
			<Tooltip>{addHint}</Tooltip>
		{/if}
	</div>

	{#if pairs.length > 0}
		<div class="flex flex-wrap gap-2">
			{#each pairs as pair, index (index)}
				<Badge dismissable onclose={() => removePair(index)}>
					<span>{pair.key}:{pair.value}</span>
				</Badge>
			{/each}
		</div>
	{/if}
</div>
