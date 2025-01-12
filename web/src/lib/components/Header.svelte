<script>
    import { page } from '$app/stores';
    import { Navbar, NavBrand, NavUl, NavLi, DarkMode } from 'flowbite-svelte';
    $: activeUrl = $page.url.pathname;

  </script>

  <Navbar rounded color="form" fluid>
    <NavBrand href="/">
      <span class="self-center whitespace-nowrap text-xl font-semibold dark:text-white">Tangle</span>
      <NavUl>
        <NavLi href="/">Home</NavLi>
        <DarkMode class="justify-end" />
      </NavUl>
    </NavBrand>
  </Navbar>
