<script lang="ts">
	import { Badge, Button, Helper, Input, Tooltip } from 'flowbite-svelte';
	import { CloseOutline } from 'flowbite-svelte-icons';
	import { shortRepo } from '$lib/ui/repos';

	interface Props {
		/** The repos, in the order they were added. */
		value?: string[];
	}

	let { value = $bindable([]) }: Props = $props();

	const uid = $props.id();
	const inputId = `${uid}-repo`;
	const hintId = `${uid}-hint`;

	let draft: string = $state('');
	let attemptedAdd: boolean = $state(false);

	let trimmed = $derived(draft.trim());
	// Two spellings of one repo would match the same sources, so they count as
	// the same entry here, as they do on the server.
	let isDuplicate = $derived(
		trimmed.length > 0 && value.some((repo) => shortRepo(repo) === shortRepo(trimmed))
	);
	let canAdd = $derived(trimmed.length > 0 && !isDuplicate);

	let hint = $derived.by(() => {
		if (isDuplicate) return `${shortRepo(trimmed)} is already a target repo.`;
		if (attemptedAdd && trimmed.length === 0) return 'Enter a repo URL to add it.';
		return null;
	});

	function add(): void {
		if (!canAdd) {
			attemptedAdd = true;
			return;
		}
		attemptedAdd = false;
		value = [...value, trimmed];
		draft = '';
	}

	function remove(index: number): void {
		value = value.filter((_, i) => i !== index);
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key !== 'Enter') return;
		// Inside the query form, Enter here means "add this repo", never "run".
		event.preventDefault();
		add();
	}
</script>

<div class="space-y-2">
	<div class="flex items-baseline justify-between gap-4">
		<label for={inputId} class="text-sm font-medium text-gray-900 dark:text-white">
			Target repos
		</label>
		<span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">Optional</span>
	</div>

	{#if value.length > 0}
		<ul class="flex list-none flex-wrap gap-2 p-0" aria-label="Target repos">
			{#each value as repo, index (repo)}
				<li>
					<Badge large color="primary" class="gap-1 pe-1 font-mono font-semibold" title={repo}>
						{shortRepo(repo)}
						<button
							type="button"
							aria-label="Remove target repo {shortRepo(repo)}"
							class="ms-0.5 inline-flex h-5 w-5 items-center justify-center rounded-sm hover:bg-black/10 dark:hover:bg-white/10"
							onclick={() => remove(index)}
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
			id={inputId}
			type="text"
			placeholder="https://github.com/org/repo"
			aria-invalid={hint !== null}
			aria-describedby={hintId}
			bind:value={draft}
			color={hint !== null ? 'red' : 'default'}
			class="font-mono"
			onkeydown={handleKeydown}
		/>
		<Button
			type="button"
			color="alternative"
			disabled={!canAdd}
			onclick={add}
			aria-label="Add target repo">Add</Button
		>
		{#if hint}
			<Tooltip>{hint}</Tooltip>
		{/if}
	</div>

	<Helper id={hintId} color={hint !== null ? 'red' : 'gray'} aria-live="polite">
		{#if hint}
			{hint}
		{:else}
			Sources in these repos get the target ref and every other source stays at its live revision.
			Applications with no source in them are skipped as not affected.
		{/if}
	</Helper>
</div>
