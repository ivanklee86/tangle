<script lang="ts">
	import '../app.css';
	import { Header } from '$lib/components';
	let { children } = $props();
</script>

<svelte:head>
</svelte:head>

<Header />

{@render children()}
