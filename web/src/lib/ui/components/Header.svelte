<script lang="ts">
	import { Navbar, NavBrand, NavUl, NavLi, NavHamburger, DarkMode } from 'flowbite-svelte';
	import { page } from '$app/state';
</script>

<Navbar fluid class="border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900">
	<NavBrand href="/">
		<span class="self-center whitespace-nowrap text-xl font-semibold dark:text-white">Tangle</span>
	</NavBrand>
	<div class="flex items-center gap-2 md:order-2">
		<DarkMode />
		<NavHamburger />
	</div>
	<NavUl
		activeUrl={page.url.pathname}
		activeClass="font-bold text-black dark:text-white md:font-bold md:text-black md:dark:text-white"
	>
		<NavLi href="/">Home</NavLi>
		<NavLi href="/applications/">Applications</NavLi>
		<NavLi href="/diffs/">Diffs</NavLi>
	</NavUl>
</Navbar>
