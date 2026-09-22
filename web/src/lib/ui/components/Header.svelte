<script lang="ts">
	import { Navbar, NavBrand, NavUl, NavLi, NavHamburger, DarkMode } from 'flowbite-svelte';
	import { page } from '$app/state';
</script>

<Navbar fluid class="border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900">
	<NavBrand href="/">
		<!-- The one place coral survives: a wordmark never competes with a status
			 badge, and primary is indigo everywhere else (see app.css). -->
		<span class="self-center whitespace-nowrap text-xl font-bold tracking-tight text-[#fe795d]"
			>Tangle</span
		>
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
