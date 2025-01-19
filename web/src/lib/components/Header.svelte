<script>
	import { Navbar, NavBrand, DarkMode } from 'flowbite-svelte';
</script>

<Navbar color="form" fluid>
	<NavBrand href="/">
		<span class="self-center whitespace-nowrap text-xl font-semibold dark:text-white">Tangle</span>
	</NavBrand>
	<DarkMode class="justify-end" />
</Navbar>
