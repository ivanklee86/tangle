<script lang="ts">
	import { Tooltip } from 'flowbite-svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		/** Why the wrapped control is disabled, or undefined when it isn't. */
		reason?: string;
		/** The control, handed the id to use as its aria-describedby. */
		children: Snippet<[string | undefined]>;
	}

	let { reason, children }: Props = $props();

	const uid = $props.id();
	const wrapperId = `${uid}-wrapper`;
	const reasonId = `${uid}-reason`;
</script>

<!--
	A disabled button fires no mouse events, so the wrapper is what gets
	hovered. The tooltip's text only exists in the DOM while it's showing, so a
	visually hidden copy is what aria-describedby points at.
-->
<span id={wrapperId} class="inline-flex">
	{@render children(reason ? reasonId : undefined)}
</span>
{#if reason}
	<span id={reasonId} class="sr-only">{reason}</span>
	<Tooltip triggeredBy="#{wrapperId}">{reason}</Tooltip>
{/if}
