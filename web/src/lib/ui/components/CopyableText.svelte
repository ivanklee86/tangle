<script lang="ts">
	import { Button, Tooltip } from 'flowbite-svelte';
	import { CheckOutline, FileCopyOutline } from 'flowbite-svelte-icons';

	interface Props {
		/** Small uppercase caption above the text, e.g. "LINK". */
		caption: string;
		text: string;
		/** What the copy button announces, e.g. "Copy link". */
		copyLabel: string;
	}

	let { caption, text, copyLabel }: Props = $props();

	let copied: boolean = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copy(): Promise<void> {
		try {
			await navigator.clipboard.writeText(text);
			copied = true;
			clearTimeout(timer);
			timer = setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard access can be denied (insecure origin, permissions).
			// The text is on screen and selectable either way, so failing to
			// copy is not worth an error state — just don't claim success.
			copied = false;
		}
	}
</script>

<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-700 dark:bg-gray-900">
	<div class="text-xs font-bold tracking-wider text-gray-500 uppercase dark:text-gray-400">
		{caption}
	</div>
	<div class="mt-1 flex items-center gap-2">
		<code
			class="grow overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs text-gray-700 dark:text-gray-300"
			>{text}</code
		>
		<Button
			type="button"
			size="xs"
			color="alternative"
			class="shrink-0 p-1.5"
			aria-label={copyLabel}
			onclick={copy}
		>
			{#if copied}
				<CheckOutline class="h-3.5 w-3.5" />
			{:else}
				<FileCopyOutline class="h-3.5 w-3.5" />
			{/if}
		</Button>
		<Tooltip>{copied ? 'Copied' : copyLabel}</Tooltip>
	</div>
</div>
