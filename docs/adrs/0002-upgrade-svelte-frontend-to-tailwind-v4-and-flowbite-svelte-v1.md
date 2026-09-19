---
status: "accepted"
date: 2026-09-19
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Upgrade the Svelte frontend to Tailwind CSS v4 and Flowbite Svelte v1

## Context and Problem Statement

`web/` (the SvelteKit app served by `tangle-server`) is pinned to Tailwind CSS `^3` and `flowbite-svelte` `^0.47.4`, both several major versions behind current (Tailwind CSS 4.x, `flowbite-svelte` 1.x). `flowbite-svelte` 0.x is Svelte 4's slot-based component API running on top of Svelte 5 only via its legacy-slot compatibility shim, even though the rest of the app (`$props()`, `$state()`, `$effect()`) is already written against Svelte 5 runes. `flowbite-svelte` 1.x is a from-scratch, Svelte-5-native rewrite (snippets instead of slots, callback props instead of `on:` event forwarding) and only supports Tailwind CSS v4 — the two upgrades are coupled upstream and cannot be taken independently. Every `web/` component (`Header`, `ApplicationsGrid`, `AppManifests`, the three routes) uses `flowbite-svelte` and/or `flowbite-svelte-icons` directly, so this touches the whole frontend.

## Decision Drivers

- AGENTS.md requires pinned versions; the current `^3`/`^0.47.4` ranges on an unmaintained major make it easy to drift further from upstream without noticing.
- `flowbite-svelte` 0.x is not receiving new features (all current development targets 1.x per its changelog), so staying on 0.x is a slow-motion dead end, not a stable resting point.
- The app already writes Svelte 5 runes everywhere except inside `flowbite-svelte` component usage (slots, `on:click`), so the codebase is already halfway migrated to the idiom `flowbite-svelte` 1.x expects.
- The user asked specifically to land on latest SvelteKit, Tailwind, and Flowbite with no feature/functionality changes — this is a pure dependency-currency decision, not a redesign.

## Considered Options

- Upgrade in place: Tailwind CSS v4 + `flowbite-svelte` 1.x + `flowbite-svelte-icons` 3.x, rewriting each component's slot/event usage to the new snippet/callback-prop API, no visual or behavioral changes.
- Stay pinned to the current `^3`/`^0.47.4` stack and defer.
- Migrate off Flowbite entirely to a different Svelte-5-native component library (e.g. shadcn-svelte, Skeleton, bits-ui/melt-ui) while adopting Tailwind v4.

## Decision Outcome

Chosen option: "Upgrade in place", because it is the only option that satisfies the request (latest SvelteKit/Tailwind/Flowbite, no functionality change) without also taking on a component-library migration, which would be a much larger, riskier change than a version bump and was explicitly out of scope.

### Consequences

- Good, because it removes the growing gap between the app's Svelte 5 runes usage and `flowbite-svelte`'s Svelte 4 slot-based API — after this, the whole frontend is idiomatically Svelte 5.
- Good, because it unblocks picking up upstream `flowbite-svelte`/Tailwind bugfixes and features going forward instead of being stuck on an unmaintained major.
- Bad, because every component file that touches `flowbite-svelte` needs a mechanical but non-trivial rewrite (slots → snippets, `on:click` → `onclick`, `tailwind.config.ts` → CSS-first `@theme`/`@plugin`/`@source` in `app.css`), all of which must be re-verified visually since there is no automated visual regression suite.
- Bad, because Tailwind v4's default dark-mode variant is media-query-based, not class-based, so `app.html`'s `class="dark"` toggle (driven by `flowbite-svelte`'s `DarkMode` component) requires an explicit `@custom-variant dark (&:where(.dark, .dark *));` to keep working — an easy detail to miss and silently lose dark mode over.

## Pros and Cons of the Options

### Upgrade in place

- Good, because it's the smallest change that satisfies "latest Sveltekit/Tailwind/Flowbite" — no new library to learn, existing component choices (`Table`, `Tabs`, `GradientButton`, etc.) stay the same.
- Good, because `flowbite-svelte` 1.x kept most component prop names stable (color/size/variant props), so most of the diff is slot→snippet/event-prop mechanics rather than redesigning markup.
- Bad, because Tailwind v4 and `flowbite-svelte` 1.x must land together (flowbite-svelte 1.x's peer dependency is `tailwindcss: ^4.1.4`), so this can't be split into smaller independent PRs the way the SvelteKit/Svelte bump can.

### Stay pinned and defer

- Good, because zero work and zero regression risk right now.
- Bad, because it directly contradicts what was asked, and every month of deferral makes the eventual jump (0.x → 1.x, v3 → v4) larger, not smaller, since 0.x gets no further backports.

### Migrate off Flowbite

- Good, because it would be an opportunity to pick a component library with a smaller/more active maintenance surface if `flowbite-svelte`'s pace becomes a problem again.
- Bad, because it is a full component-library migration (different theming model, different component set, different a11y/behavior guarantees), which is explicitly a functionality-adjacent risk the user asked to avoid this round.
- Bad, because it throws away the existing Tailwind/Flowbite-specific styling (`primary` gradient palette, `pinkToOrange` gradient buttons) that would need to be redesigned, not just ported.

## More Information

- [Flowbite Svelte introduction / setup docs](https://flowbite-svelte.com/docs/pages/introduction)
- [Tailwind CSS v4 upgrade guide](https://tailwindcss.com/docs/upgrade-guide)
- [`flowbite-svelte` CHANGELOG](https://github.com/themesberg/flowbite-svelte/blob/main/CHANGELOG.md)
- Implementation plan: [Svelte frontend upgrade](../agents/plans/svelte-frontend-upgrade.md)
- Superseded by, if adopted later: none.
