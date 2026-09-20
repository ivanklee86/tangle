# Flowbite design consistency pass

Status: implemented · 2026-09-20

A design/consistency pass over `web/`'s frontend, following up on the pure version bump in
[ADR 0002](../adrs/0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md). See
[ADR 0007](../adrs/0007-flowbite-design-system-consistency.md) for the decision and rationale; this document
records what was actually changed, for anyone auditing the diff or extending the pattern later.

Non-goals: no new features, no backend changes, no component-library swap. Every Flowbite API detail below was
verified against the installed `flowbite-svelte@1.33.1` source in `web/node_modules/flowbite-svelte/dist/`.

## Changes

**Spacing.** Removed `<br />` used as a layout primitive across `routes/+page.svelte`,
`routes/applications/+page.svelte`, `routes/diffs/+page.svelte`, `AppManifests.svelte`, and
`ApplicationsGrid.svelte`, replacing it with `space-y-*` on containers or `mt-*`/`mb-*` on individual elements.

**Shared error alert.** New `lib/ui/components/ErrorAlert.svelte` (`<Alert color="red" border>`, no `class`
override) replaces three duplicated "System error!" blocks in `ApplicationsGrid.svelte`, `AppManifests.svelte`,
and `routes/diffs/+page.svelte`. The prior `class="bg-red-500 text-white"` override was actively harmful, not
just redundant — `tailwind-merge` replaces the theme's light-mode classes but leaves its `dark:` classes in
place, producing a saturated box in light mode and a washed-out one in dark mode.

**Shared status mapping.** New `lib/ui/status.ts` exports `statusAppearance(status)`, returning an icon
component and a dark-mode-aware color class. `ArgoCDHealthStatus.svelte` and `ArgoCDSyncStatus.svelte` are now
thin wrappers over it (`<appearance.icon class="... {appearance.class}" />`), replacing duplicated,
disagreeing fallback behavior (health previously fell back to a red X for any unrecognized status; sync
previously rendered no icon at all) with one deliberate amber "unknown" fallback for both. Covered by
`lib/ui/status.spec.ts`.

**Headings.** All page/card titles use Flowbite's `<Heading>` component; the two hand-rolled
`<h5 class="... text-2xl font-bold ...">` card titles on the home page were replaced with `<Heading tag="h2">`.
Each route also has a real `<h1>` (`sr-only`, per a later round of feedback — see below), which fixed
`e2e/demo.test.ts`'s previously-silently-failing assertion that the home page has a visible `<h1>` (silently
failing because `tasks/ts.yaml`'s `test` target only ran `npm run test:unit`, never `test:e2e`; that target now
runs both, `npm run test:unit -- --run` then `npm run test:e2e`).

**Accent color.** Replaced `<GradientButton color="pinkToOrange">` (three call sites) with `<Button
color="primary">`, which consumes the `--color-primary-*` tokens in `app.css` directly (`bg-primary-700
hover:bg-primary-800 dark:bg-primary-600 ...`). A later round of feedback asked for more visual richness back,
so the primary CTA buttons (in `ApplicationsForm.svelte`/`DiffsForm.svelte`, the two shared form components —
see [the gating plan](gate-applications-and-diffs-searches.md)) additionally carry a
`bg-gradient-to-br from-primary-400 to-primary-700 hover:from-primary-500 hover:to-primary-800` overlay — a
gradient, but built from the theme's own ramp instead of the off-palette `pinkToOrange` preset. These same
buttons also needed `leading-none` added: `Button`'s markup places the button text and the trailing arrow icon
as sibling flex children with `items-center`, and the text's default line-height added enough invisible padding
around the glyphs to visually skew them a couple pixels below the icon's true center.

**Navbar.** `Header.svelte`'s `<Navbar color="form">` did nothing — `Navbar` has no `color` prop at all in
1.x (its props are `fluid`/`class`/`navContainerClass`/`closeOnClickOutside`/`breakpoint`); the value fell
through to `restProps` and rendered as a literal, inert `color="form"` DOM attribute. Removed; the bar is now
styled via `class="border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900"`. Added `NavUl`/
`NavLi`/`NavHamburger` for Home/Applications/Diffs nav links with active-state highlighting.

Getting the active state to actually work took two iterations: `NavLi`'s active match is exact
`href === activeUrl`, but this app's `routes/+layout.ts` sets `trailingSlash = 'always'`, so
`page.url.pathname` is always `/applications/`/`/diffs/` — the initial `href="/applications"` (no trailing
slash) never matched, so two of the three links could never highlight. Fixed by adding the trailing slash to
the `href`s. Separately, the initial active styling (`bg-primary-700` pill) read as "just an orange link," not
clearly "this is where you are" next to the coral CTA buttons; changed to `font-bold text-black
dark:text-white` after visual review — bold, full-contrast text against the muted gray of inactive links.

**Cards.** Flowbite's `Card` (`node_modules/flowbite-svelte/dist/card/theme.js`) ships with **no built-in
padding at all** — its `base` slot has borders/background/radius but nothing else — so every `Card` usage
across the app (`routes/+page.svelte` ×2, `routes/diffs/+page.svelte`'s form, and by extension
`ApplicationsForm.svelte`/`DiffsForm.svelte` after the later componentization) needs an explicit `p-6` in its
`class`, or its contents visibly touch the card's edges.

## Verification

From `/workspaces/tangle/web`: `npm run check`, `npm run lint`, `npm run test:unit -- --run`,
`npm run test:e2e`, plus manual review via `npm run dev` and Playwright screenshots at desktop/mobile widths in
both light and dark mode — screenshots were the only way to actually catch several of these issues (the
dark-mode alert pairing, the padding gap, the button text/icon misalignment, the non-working nav highlight),
since none of them are things a type check or unit test would surface.
