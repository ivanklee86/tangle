# Structured key/value Labels input

Status: implemented · 2026-09-20

See [ADR 0022](../adrs/0022-structured-key-value-labels-input.md) for the decision and rationale. This document
records what was actually implemented. Implements
[issue #137](https://github.com/ivanklee86/tangle/issues/137): replace the single free-text Labels/Exclude
Labels `<Input>` in `ApplicationsForm.svelte` and `DiffsForm.svelte` with a shared `LabelsInput.svelte`
component — a `key` box, a `value` box, an add button, and the committed pairs shown as removable chips.
Frontend-only: the wire format (`key:value,key2:value2` in the URL query string, parsed by
`internal/tangle/handlers.go`) is unchanged, so no Go changes are in scope.

## 1. `lib/ui/labels.ts` — parse/serialize the wire format

**Problem**: no code today turns the `key:value,key2:value2` string into a structure or back — the string is
opaque end to end on the frontend. `LabelsInput` needs both directions: parse an initial `value` string into
rows to render, and serialize its rows back into that same string for the existing `bind:value` contract.

**New file** `web/src/lib/ui/labels.ts`:

```ts
interface LabelPair {
	key: string;
	value: string;
}

function parseLabels(value: string): LabelPair[] {
	if (value.length === 0) return [];
	return value
		.split(',')
		.map((pair) => pair.split(':'))
		.filter((parts): parts is [string, string] => parts.length === 2)
		.map(([key, value]) => ({ key, value }));
}

function serializeLabels(pairs: LabelPair[]): string {
	return pairs.map(({ key, value }) => `${key}:${value}`).join(',');
}

export { parseLabels, serializeLabels, type LabelPair };
```

`parseLabels` mirrors `handlers.go`'s leniency (`strings.Split` then keep only pairs that split into exactly
two parts) rather than throwing — a hand-edited or old bookmarked URL with a malformed segment should drop
that one segment, not break the page. `isValidLabelFormat` (`web/src/lib/ui/validation.ts`) is unchanged and
untouched by this workstream; `LabelsInput` reuses it in workstream 2 rather than duplicating the format rule.

**Test** `web/src/lib/ui/labels.spec.ts`:

- `parseLabels('')` returns `[]`.
- `parseLabels('env:prod')` returns `[{key: 'env', value: 'prod'}]`.
- `parseLabels('env:prod,tier:frontend')` returns both pairs in order.
- `parseLabels('env:prod,not-a-pair,tier:frontend')` drops the malformed middle segment and keeps the other
  two — matching `handlers.go`'s `if len(rawLabel) == 2` behavior.
- `serializeLabels([])` returns `''`.
- `serializeLabels([{key: 'env', value: 'prod'}, {key: 'tier', value: 'frontend'}])` returns
  `'env:prod,tier:frontend'`.
- `serializeLabels(parseLabels(s)) === s` for a few well-formed `s` (round-trip).

**Rollback**: delete the file; nothing else depends on it yet.

## 2. `lib/ui/components/LabelsInput.svelte` — the shared component

**Problem**: this is the actual UI change from ADR 0022 — two boxes and an add button instead of one free-text
box, with committed pairs shown as removable chips.

**New file** `web/src/lib/ui/components/LabelsInput.svelte`:

```svelte
<script lang="ts">
	import { Label, Input, Button, Badge } from 'flowbite-svelte';
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

	function addPair(): void {
		if (!canAdd) return;
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
	<div class="flex gap-2">
		<Input
			type="text"
			placeholder={keyPlaceholder}
			aria-label="{label} key"
			bind:value={draftKey}
			size="lg"
			onkeydown={handleDraftKeydown}
		/>
		<Input
			type="text"
			placeholder={valuePlaceholder}
			aria-label="{label} value"
			bind:value={draftValue}
			size="lg"
			onkeydown={handleDraftKeydown}
		/>
		<Button
			type="button"
			color="alternative"
			disabled={!canAdd}
			onclick={addPair}
			aria-label="Add {label}"
		>
			<PlusOutline class="h-5 w-5" />
		</Button>
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
```

**Landed addition**: pressing Enter (or clicking the add button) while exactly one of the two boxes is empty
originally did nothing visible — `canAdd` was `false`, so `addPair()` silently returned. Feedback surfaced this
during review: a user typing a key and hitting Enter got no response at all. Added `attemptedAdd` state, set on
a failed add attempt (not on every keystroke, so an untouched empty row doesn't start out looking like an
error) and cleared on a successful one; `keyMissing`/`valueMissing` derive from it plus each box's own
emptiness. The empty box gets `Input color="red"` plus a `Helper color="red"` ("Key is required" / "Value is
required") directly underneath it, clearing the instant that box is filled in — scoped to the empty-field case
specifically, not the reserved-character case (typing `:`/`,` into a box still just leaves the add button
disabled with no message, matching the original design; a natural follow-up if that turns out to need the same
treatment).

**Landed correction**: the chip text is wrapped in its own `<span>` rather than sitting directly in `Badge`'s
default slot as originally written above. `Badge`'s own `sr-only` "Close" text lives in the same container as
the chip text; without the wrapping `span`, Playwright's `getByText('env:prod')` locator (used throughout
`LabelsInput.svelte.test.ts`) couldn't resolve a single element whose own text was exactly `env:prod`, since
the smallest common container also included the close button's text. Wrapping the pair text isolates it as its
own element with no other descendant text, which `getByText` matches directly — found by running the test
suite, not anticipated at plan time.

Notes on the pieces:

- **External contract is unchanged.** `value` is still a single bindable `key:value,key2:value2` string —
  `ApplicationsForm`/`DiffsForm`'s own `labels`/`excludeLabels` `$state`, `isValidLabelFormat`-based
  `canSubmit`, `onSubmit` signature, and `buildQuery` all stay exactly as they are today (see workstream 3).
  Only the markup and interaction inside the field changes.
- **Per-pair validation, reusing the existing rule.** `canAdd` runs the same `isValidLabelFormat` regex used
  today against the composed `"${draftKey}:${draftValue}"` candidate, rather than introducing a second format
  check. An empty key or value already fails this regex on its own (it requires one-or-more non-`:`/`,`
  characters on each side), so there's no separate empty-string guard needed.
- **`Badge dismissable` for the remove affordance**, not a hand-rolled close button. `Badge`'s own `close`
  handler (`node_modules/flowbite-svelte/dist/forms/tags/../../badge/Badge.svelte`) dispatches a bubbling,
  cancelable native `close` event on its root element before hiding itself; `onclose` on the `Badge` is
  Svelte 5's ordinary DOM-event-listener prop syntax for that event (the same mechanism used for `onclick`
  etc.), not a Flowbite-specific callback prop — this was confirmed by reading `Badge.svelte`'s source rather
  than assumed, since it isn't documented as a named `onclose` prop in `BadgeProps`.
- **Enter now adds a pair, not submits the form** — a deliberate, user-visible behavior change from today
  (where Enter in the single Labels `<Input>` submits the whole form; see `ApplicationsForm.svelte.test.ts`'s
  existing "submits when Enter is pressed" test). With two boxes, pressing Enter after typing only a key
  should commit that pair, not attempt to submit a form the user may not have finished filling in — this
  mirrors the "Enter commits the current entry" idiom flowbite-svelte's own `Tags` component uses. Workstream
  3 updates the affected tests to match.
- **No icon in the input** (the old field had a `LabelSolid` icon via `Input`'s `left` snippet). With three
  elements now in the row (key box, value box, add button) an icon on the first box only reads as unbalanced;
  dropped rather than carried over unchanged. Purely a visual call, easy to revisit.
- **Duplicate keys are allowed** — `pairs` is a plain array, not deduplicated by key. The wire format
  (`map[string]string` server-side) would let a later duplicate silently win, same as if a user hand-typed
  `"env:prod,env:staging"` into the old free-text field today; not a new failure mode this introduces, so not
  specially guarded against here.

**Test** `web/src/lib/ui/components/LabelsInput.svelte.test.ts`, following this repo's existing
`vitest-browser-svelte` + `userEvent` pattern of querying by role/placeholder and asserting on observable
behavior (per AGENTS.md's testing philosophy):

- Typing a key and value and clicking the add button renders a chip showing `key:value` and clears both boxes.
- Typing a key and value and pressing Enter in either box adds the pair the same way, without needing a click.
- The add button is disabled while the key or value box is empty, and while either contains `:` or `,`.
- Clicking a chip's close button removes that pair and its chip.
- An initial `value` prop (e.g. `'env:prod,tier:frontend'`) renders two pre-existing chips on mount.
- Adding then removing a pair leaves the bound `value` matching `serializeLabels` of whatever pairs remain —
  exercised either directly (if this codebase's Svelte/testing versions support asserting a `$bindable` prop
  from outside, which no existing component test here currently does — check `vitest-browser-svelte` 3.1.0's
  support before committing to this) or indirectly through workstream 3's `ApplicationsForm`/`DiffsForm` tests,
  which already assert on the exact string `onSubmit` receives.

**Landed addition (tooltip on the add button)**: separately from the empty-field `Helper` text above (which
only appears after a failed add attempt), the add button also gets a `Tooltip` on hover, active any time it's
disabled — including the pristine, nothing-typed-in state, which the `Helper` text deliberately doesn't cover.
An `addHint` derivation picks the message: both boxes empty → "Enter a key and value to add this label.";
only one empty → "Enter a key to add this label." / "Enter a value to add this label."; reserved character in
either box → "Key and value can't contain ':' or ','." First draft of the single-field messages suggested
clearing the *other* box ("Enter a key, or clear the value, ...") as an alternative — dropped after feedback
that it's a false choice: there's nothing to "clear" toward, the user either fills in the missing half or
simply doesn't add this pair yet and keeps going. `DiffsForm.svelte`'s "See diffs" button gets the same
treatment: a `Tooltip` reading "Enter a target ref (git branch) to see diffs." whenever
`normalizedTargetRef.length === 0`, replacing the older "design, not text" comment that relied solely on the
disabled state with no explanation.

**Rollback**: delete the file; nothing imports it until workstream 3.

## 3. Wire `LabelsInput` into `ApplicationsForm.svelte` and `DiffsForm.svelte`

**Problem**: swap the duplicated inline `<Label><Input placeholder="Labels in format 'key:value'">` markup in
both forms for the new shared component.

**`ApplicationsForm.svelte` changes**:

```diff
-	import { Card, Label, Input, Button, Heading } from 'flowbite-svelte';
-	import { ArrowRightOutline, LabelSolid } from 'flowbite-svelte-icons';
+	import { Card, Button, Heading } from 'flowbite-svelte';
+	import { ArrowRightOutline } from 'flowbite-svelte-icons';
+	import LabelsInput from './LabelsInput.svelte';
 	import { isValidLabelFormat } from '$lib/ui/validation';
```

```diff
-		<Label class="space-y-2">
-			<span>Labels</span>
-			<Input
-				type="text"
-				placeholder="Labels in format 'key:value'"
-				bind:value={labels}
-				size="lg"
-				class="ps-11"
-			>
-				{#snippet left()}
-					<LabelSolid class="h-6 w-6" />
-				{/snippet}
-			</Input>
-		</Label>
-
-		<Label class="space-y-2">
-			<span>Exclude Labels</span>
-			<Input
-				type="text"
-				placeholder="Labels to exclude in format 'key:value'"
-				bind:value={excludeLabels}
-				size="lg"
-				class="ps-11"
-			>
-				{#snippet left()}
-					<LabelSolid class="h-6 w-6" />
-				{/snippet}
-			</Input>
-		</Label>
+		<LabelsInput label="Labels" bind:value={labels} />
+		<LabelsInput label="Exclude Labels" bind:value={excludeLabels} />
```

`labels`/`excludeLabels`'s own `$state('')`, `canSubmit = $derived(isValidLabelFormat(labels) &&
isValidLabelFormat(excludeLabels))`, and `handleSubmit`/`onSubmit` are untouched — `isValidLabelFormat` is now
always true in practice (every pair `LabelsInput` can produce already passed the same check on the way in),
kept anyway as a zero-cost defensive check rather than removed, since nothing about this plan changes that
invariant being worth guarding.

**`DiffsForm.svelte` changes**: the same swap for its Labels/Exclude Labels blocks (its `Target Ref` field is
untouched). `initialLabels`/`initialExcludeLabels` props keep seeding `labels`/`excludeLabels`'s `$state` via
`untrack(() => ...)` exactly as today — `LabelsInput` itself calls `parseLabels(value)` once on its own mount
to turn that seeded string into its initial chips, so no change is needed to how `DiffsForm` receives or
stores its props.

**`web/src/lib/ui/components/index.ts`**: add `export { default as LabelsInput } from './LabelsInput.svelte';`
(alongside the existing `ApplicationsForm`/`DiffsForm` exports) in case another route wants it directly later,
even though both current call sites import it by relative path as a sibling file.

**Test updates**:

- `ApplicationsForm.svelte.test.ts` / `DiffsForm.svelte.test.ts`: replace `getByPlaceholder("Labels in format
  'key:value'")` interactions with the new two-box flow — `getByRole('textbox', { name: 'Labels key' })` +
  `getByRole('textbox', { name: 'Labels value' })` + `getByRole('button', { name: 'Add Labels' })` (or an Enter
  press) to commit a pair, then submit. The "submits when Enter is pressed in a field, without a button click"
  test's premise no longer holds (Enter now adds a pair, per workstream 2) — replace it with a test that Enter
  in the key/value boxes adds a chip rather than submitting, and keep a separate "submits when the submit
  button is clicked after adding a pair" case for the actual submit path. The "malformed labels disable the
  button" test becomes "the add button stays disabled for a malformed key/value pair" (asserted on
  `LabelsInput`'s own add button) plus "the submit button stays disabled with no pairs added" if that's not
  already implied by an unrelated existing case — since `labels`/`excludeLabels` default to `''` and
  `isValidLabelFormat('')` is `true`, no-pairs-added should still allow submitting an empty filter, matching
  today's behavior; confirm this explicitly with a test rather than assuming it survives the refactor
  unexamined.

**Rollback**: revert both forms' template/import hunks back to the inline `<Label><Input>` markup; delete
`LabelsInput.svelte`'s export from `index.ts`. `labels.ts` and `LabelsInput.svelte` can be left in place
unused, or deleted alongside (workstreams 1-2's rollback).

## Sequencing notes

- Workstream 1 (`labels.ts`) has no dependents until workstream 2; land it first.
- Workstream 2 (`LabelsInput.svelte`) depends on workstream 1's `parseLabels`/`serializeLabels` and on
  `validation.ts`'s existing `isValidLabelFormat` (untouched).
- Workstream 3 depends on workstream 2 existing; it's the only workstream that changes either form's own
  files or their tests.
- After each workstream: `npm run check` (svelte-check + TS) and `npm run test:unit -- --run` should stay
  green; run `npm run lint` once at the end.
- This plan doesn't touch `+page.svelte` routes, `buildQuery`, `client.ts`, or `handlers.go` — the wire format
  and URL-building are unchanged by construction (workstream 2's contract note), so no backend or route-level
  test should need updating; if one does, that's a signal the `bind:value` contract wasn't actually preserved
  and workstream 2's implementation needs a second look before landing.

## Verification

`npm run check` (0 errors/warnings), `npm run test:unit -- --run` (116 tests, all green), and `npm run lint`
(prettier + eslint clean) all pass. Manually verified via `npm run dev` and a headless-Chromium (Playwright)
drive of the home page (`/`, which needs no backend — both forms render standalone): the add button starts
disabled, filling the key and value boxes for "Labels" enables it, clicking it renders a coral `env:prod` chip
and clears both boxes, and clicking the chip's close button removes it and collapses the layout back; filling
in only the key and pressing Enter turns the value box red with a "Value is required" message underneath it,
which clears the instant a value is typed — no console errors during any of it. `/applications` and `/diffs`
(which do need a live backend to load data) were
exercised through their existing component/page test suites rather than manually, since `ApplicationsForm`/
`DiffsForm`'s own contract with `LabelsInput` is identical to what the home page's forms use.
