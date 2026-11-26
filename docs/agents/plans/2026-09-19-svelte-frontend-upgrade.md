# Svelte frontend upgrade

Status: implemented · 2026-09-19

Upgrade `web/` (the SvelteKit app `task ts:build` compiles into `build/`, embedded into the `tangle-server` image by `Dockerfile`'s stage 2) to the latest SvelteKit/Svelte, Tailwind CSS, and Flowbite Svelte, with no feature or functionality changes — only version currency and the mechanical API changes those versions require. See [ADR 0002](../../adrs/0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md) for why Tailwind v4 and `flowbite-svelte` 1.x are one coupled workstream rather than two.

Do this as a single PR, landing the two workstreams below in order so each is validated before moving to the next. Re-check every exact version number against npm at implementation time (`npm view <pkg> version`) and pin the resolved version exactly in `package.json` per AGENTS.md — the versions below were current as of 2026-09-19.

## 1. Upgrade SvelteKit and Svelte

**Current → target** (`web/package.json`)

| | Current | Target |
| --- | --- | --- |
| `svelte` | 5.16.0 (resolved 5.26.1) | 5.57.1 |
| `@sveltejs/kit` | 2.0.0 (resolved 2.20.5) | 2.70.3 |
| `@sveltejs/adapter-static` | 3.0.6 (resolved 3.0.8) | 3.0.10 |

This is a same-major bump (Svelte 5.x, SvelteKit 2.x) — no snippet/slot API changes, no adapter changes. The only reason to do it before the Tailwind/Flowbite workstream is that `flowbite-svelte` 1.x's peer dependency requires `svelte: ^5.40.0`, so Svelte must be at 5.40.0+ before installing it.

**Steps**

1. Pin exact versions in `web/package.json`'s `devDependencies`: `svelte`, `@sveltejs/kit`, `@sveltejs/adapter-static`.
2. `cd web && npm install`.
3. `npm run check` (svelte-check) and `npm run lint` — expect no changes needed; this range has no breaking API changes for this codebase's usage (runes, `$app/stores`, `svelte:head`, `adapter-static` config are all unaffected between 2.0 and 2.70).
4. `npm run test:unit && npm run test:e2e` and `npm run build` to confirm the static build still emits `build/` the same shape adapter-static did before (`pages: 'build', assets: 'build', fallback: 'index.html'` in `svelte.config.js` is unchanged).
5. Optional, not required for this bump: `$app/stores`' `page` store (used in `Header.svelte`... actually `+layout.svelte`/`applications/+page.svelte`/`diffs/+page.svelte`/`+page.svelte` via `import { page } from '$app/stores'`) has a newer rune-based replacement in `$app/state` since SvelteKit 2.12. It still works unchanged at 2.70, so only do this if you want the codebase on the newer idiom; `npx sv migrate app-state` automates it. Skip if you want to keep this PR strictly to version bumps.

**Rollback**: revert the three `package.json` version pins and `package-lock.json`; nothing else in the repo changes in this workstream.

## 2. Upgrade Tailwind CSS to v4 and Flowbite Svelte to v1

**Current → target** (`web/package.json`)

| | Current | Target |
| --- | --- | --- |
| `tailwindcss` | `^3` (resolved 3.4.17) | 4.3.3 |
| `flowbite-svelte` | 0.47.4 | 1.33.1 |
| `flowbite-svelte-icons` | 2.1.1 | 3.1.0 |
| `flowbite` | 2.5.2 | 3.1.2 — matches what `flowbite-svelte` 1.33.1 itself pins as its own `flowbite` devDependency (`^3.1.2`); deliberately not npm's absolute latest major (4.0.2), which `flowbite-svelte` hasn't been built/tested against |
| `autoprefixer` | 10.4.20 | removed — Tailwind v4's Lightning CSS engine handles vendor prefixing |
| `@tailwindcss/vite` | — | new, `4.x` latest, matching the `tailwindcss` version |
| `postcss.config.js` | present | deleted — replaced by the `@tailwindcss/vite` Vite plugin |
| `tailwind.config.ts` | present | deleted — config moves into `src/app.css` (`@theme`, `@plugin`, `@source`) |

`flowbite-svelte` 1.x's peer dependencies are `svelte: ^5.40.0` and `tailwindcss: ^4.1.4` (verify exact floor at implementation time) — this is why workstream 1 must land first and why Tailwind and Flowbite Svelte move together here; `flowbite-svelte` 0.x is not tested against Tailwind v4 and `flowbite-svelte` 1.x does not run on Tailwind v3.

### 2a. Config migration

1. Delete `web/postcss.config.js` and `web/tailwind.config.ts`.
2. In `web/vite.config.ts`, add the `@tailwindcss/vite` plugin ahead of `sveltekit()`:

   ```ts
   import { defineConfig } from 'vitest/config';
   import { sveltekit } from '@sveltejs/kit/vite';
   import tailwindcss from '@tailwindcss/vite';

   export default defineConfig({
   	plugins: [tailwindcss(), sveltekit()],
   	test: {
   		include: ['src/**/*.{test,spec}.{js,ts}']
   	}
   });
   ```

3. Rewrite `web/src/app.css` (replacing the three `@tailwind` directives) with the CSS-first equivalent of the deleted `tailwind.config.ts` — the primary color palette, the `flowbite/plugin` registration, and (since `app.html` toggles `class="dark"` on `<html>` via the `DarkMode` component, and Tailwind v4 defaults `dark:` to a media query) an explicit class-based dark variant:

   ```css
   @import 'tailwindcss';

   @plugin 'flowbite/plugin';

   @custom-variant dark (&:where(.dark, .dark *));

   @source '../node_modules/flowbite-svelte';
   @source '../node_modules/flowbite-svelte-icons';
   @source '../node_modules/svhighlight';

   @theme {
   	--color-primary-50: #fff5f2;
   	--color-primary-100: #fff1ee;
   	--color-primary-200: #ffe4de;
   	--color-primary-300: #ffd5cc;
   	--color-primary-400: #ffbcad;
   	--color-primary-500: #fe795d;
   	--color-primary-600: #ef562f;
   	--color-primary-700: #eb4f27;
   	--color-primary-800: #cc4522;
   	--color-primary-900: #a5371b;
   }
   ```

   The `@source` lines replace `tailwind.config.ts`'s `content: [...]` array — Tailwind v4 auto-scans the project but not `node_modules`, so any package shipping class names used at runtime (here: `flowbite-svelte`, `flowbite-svelte-icons`, `svhighlight`'s `CodeBlock`) needs an explicit `@source`.
4. `npm install` the version table above (removing `autoprefixer`, adding `@tailwindcss/vite`).

### 2b. Component rewrites

`flowbite-svelte` 1.x replaces Svelte 4 slots with snippets and `on:event` forwarding with callback props (`onclick`, `onclose`, ...). Every `web/src/**/*.svelte` file that imports from `flowbite-svelte` needs one or both of these mechanical changes — verify each component's exact current prop/snippet names against its source or docs at implementation time (`flowbite-svelte`'s own site, or `node_modules/flowbite-svelte/dist/<component>/<Component>.svelte` after installing), since this list is based on the 1.33.1 source read during planning and prop names can shift between minors:

- **Cross-cutting**: `Alert`'s `color="none"` (used in `ApplicationsGrid.svelte`, `AppManifests.svelte`, and `diffs/+page.svelte` to fully override styling via a `class="bg-red-500 text-white"` instead of picking a themed color) is not a valid `AlertProps.color` in 1.x — its color union no longer includes `"none"`. Change to `color="red"`; the explicit `class` override still wins over the theme's red styling either way, so this is a type-only fix with no visible change.
- **`src/lib/components/Header.svelte`** — `Navbar`, `NavBrand`, `DarkMode` usage is prop-compatible as-is; verify `<Navbar color="form">` is still a valid `color` value in 1.x's navbar theme (its color palette may have changed).
- **`src/lib/components/ApplicationsGrid.svelte`**:
  - `<TabItem title=... open=... disabled=...><span slot="title">...</span></TabItem>` → drop the now-redundant `title` prop and the `slot="title"` span, use the `titleSlot` snippet prop instead:

    ```svelte
    <TabItem open={index === 0} disabled={argoCDApplications.applications.length === 0}>
    	{#snippet titleSlot()}
    		{argoCDApplications.name} ({argoCDApplications.applications.length})
    	{/snippet}
    	...
    {/TabItem}
    ```

  - `<Table hoverable={true} items={argoCDApplications.applications}>` + `<TableBodyRow slot="row" let:item>`: in `flowbite-svelte` 1.x, `Table`'s `items` prop makes it render an auto-generated head/body from the items and **ignore its children entirely** — it can no longer be combined with hand-written `TableHead`/`TableBody`/custom cell rendering the way 0.x allowed. Drop `items` from `<Table>` and replace the `slot="row" let:item` row with a plain `{#each}` inside `<TableBody>` (also `TableBody`'s `tableBodyClass` prop is gone in 1.x — use `class` instead). This also removes the three `{/* @ts-expect-error */}` workarounds in this file — they existed only because 0.x's `let:item` slot prop couldn't be typed; a typed `{#each item of ...}` doesn't need them.
  - **`sort` is gone from `TableHeadCell` entirely in 1.x** — confirmed by reading both the 0.47.4 and 1.33.1 source: in 0.x, `Table`'s `items` prop plus a `sort` comparator on `TableHeadCell` wired into a shared Svelte context (`sorter`) that `TableBody` read to reorder rows and render a ▲/▼ indicator; in 1.x that whole context mechanism (and the `sort`/`defaultDirection`/`direction` props) was removed along with the `items`+custom-children combination, with no replacement. Since this app's column-sort was real, working functionality, it has to be reimplemented locally rather than dropped: track one `{ key, direction }` sort state per tab in a `$state` record, sort a copy of the tab's `applications` array before the `{#each}`, and make each `TableHeadCell` clickable (`onclick`) with an inline ▲/▼ indicator appended to its label. See the actual implementation in `ApplicationsGrid.svelte` for the exact shape.
  - The raw `<a href={item.url} target="_blank">` (an external ArgoCD URL, not an internal SvelteKit route) trips `eslint-plugin-svelte`'s `svelte/no-navigation-without-resolve` rule once `eslint-plugin-svelte` is bumped (2c) — add `rel="external"` per that rule's own documented escape hatch for non-SvelteKit links, rather than wrapping it in `resolve()` (which is for internal routes only).
- **`src/lib/components/AppManifests.svelte`** — `<AccordionItem><span slot="header">Manifests</span>...</AccordionItem>` → `header` snippet prop:

  ```svelte
  <AccordionItem>
  	{#snippet header()}
  		Manifests
  	{/snippet}
  	<CodeBlock language="yaml" code={diffData.response.targetManifests} />
  </AccordionItem>
  ```

  Verify `<Card size="xl">`'s `size="xl"` is still a valid value in 1.x's card theme.
- **`src/routes/+page.svelte`**:
  - `<Toast ... on:close={() => (...)}>` + `<svelte:fragment slot="icon">...</svelte:fragment>` → `onclose` callback prop + `icon` snippet prop:

    ```svelte
    <Toast color="red" position="top-right" onclose={() => (noRefSpecified = false)}>
    	{#snippet icon()}
    		<ExclamationCircleSolid class="h-5 w-5" />
    		<span class="sr-only">Warning icon</span>
    	{/snippet}
    	You must provide a target git ref to generate a diff!
    </Toast>
    ```

  - `<Input ...><LabelSolid slot="left" class="w-6 h-6" /></Input>` → `left` snippet prop:

    ```svelte
    <Input type="text" placeholder="..." bind:value={labels} size="lg">
    	{#snippet left()}
    		<LabelSolid class="w-6 h-6" />
    	{/snippet}
    </Input>
    ```

    (every `slot="left"` `Input` usage in this file — 5 occurrences: labels ×2, excludeLabels ×2, targetRef ×1; the `CodeBranchOutline slot="left"` on the targetRef `Input` follows the same pattern)

    **Also add `class="ps-11"` to each of these `Input`s.** Confirmed visually (screenshot) that without it, the icon overlaps the placeholder/typed text: 0.x's `Input` automatically added left padding (`ps-9`/`ps-10`/`ps-11` for sm/md/lg) whenever a `left` slot was present; 1.x's `input` theme (`node_modules/flowbite-svelte/dist/forms/input-field/theme.ts`) does not — its padding is a fixed `px-3 py-3` for `lg` regardless of whether `left`/`right` are used, so the caller has to reserve the space itself. `size="lg"` here maps to 0.x's `ps-11`.
  - `<GradientButton ... on:click={...}>` → `onclick={...}` (both buttons in this file).
- **`src/routes/applications/+page.svelte`** — `<Button ... on:click={...}>` → `onclick={...}`; `Select` usage (`items`, `bind:value`) is unchanged.
- **`src/routes/diffs/+page.svelte`**:
  - Outer `<TabItem title=... open=...>` → drop `title`, add `titleSlot` snippet (same pattern as `ApplicationsGrid.svelte`).
  - Inner `<TabItem title=... open=...><div slot="title" class="flex items-center">...</div></TabItem>` → `titleSlot` snippet wrapping the existing `<div>`.
  - `<A href={...} target="_blank" aClass="xs">More Info</A>` — `aClass` does not exist on 1.x's `A` component (`AnchorProps` has `color`/`class`/`href`/`asButton`/`onclick`, no `aClass`); replace with `class="text-xs"` (verify against the anchor theme's size-equivalent utility at implementation time).
  - `<List tag="ul" class="..." list="none">` — verify `list="none"` is still a supported prop on 1.x's `List` (its prop list in the 1.33.1 source read during planning was `tag`, `isContenteditable`, `position`, `ctxClass`, `class`, with no `list` prop); if it's gone, replace with `class="... list-none"`.
  - `<GradientButton ... on:click={...}>` → `onclick={...}`.

### 2c. Supporting tooling

Bump these devDependencies alongside `flowbite-svelte`/Tailwind so linting and formatting understand the new syntax (snippets, Tailwind v4 class names) instead of flagging it:

| | Current | Target |
| --- | --- | --- |
| `eslint-plugin-svelte` | 3.0.2 | 3.23.0 |
| `prettier-plugin-svelte` | 3.3.3 | 4.1.1 (peer: `prettier ^3.0.0`, `svelte ^5.0.0` — both already satisfied) |
| `prettier-plugin-tailwindcss` | 0.6.11 | 0.8.1 (needed for Tailwind v4 class sorting) |

**Steps**

1. Pin the version table above in `web/package.json`, `npm install`.
2. Rewrite every file listed in 2b.
3. `npm run format && npm run lint && npm run check` until clean.
4. `npm run test:unit && npm run test:e2e`.
5. `npm run dev` and manually exercise all three routes (`/`, `/applications`, `/diffs`) in both light and dark mode (toggle via the `DarkMode` button in the header): confirm Tailwind styling renders (a wall of unstyled/default-browser-styled elements means a missing `@source` glob), confirm tabs/table sorting/toast dismissal/accordion expand/gradient buttons/dark-mode toggle all still work exactly as before. If doing this via Playwright (`npx playwright screenshot` or a driven `chromium.launch()`) in a minimal container, note two sandbox-only gotchas that are not app bugs: (a) `npx playwright install --with-deps` can fail on missing font packages (`ttf-ubuntu-font-family`/`ttf-unifont`) — the browser binary still installs fine, `sudo apt-get install -y fonts-liberation fonts-dejavu-core` separately is enough to get real text rendering (without any fonts, all text renders as zero-height/invisible while icons and colors still show, which looks like a CSS bug but isn't); (b) `npx playwright install --with-deps chromium`'s dependency validation can fail cleanly while the earlier browser download still succeeded — `sudo dpkg --configure -a` (to finish whatever apt left interrupted) followed by a plain `npx playwright install chromium` (no `--with-deps`) plus the runtime `.so` packages the error message lists is usually enough.
6. `task ts:build` (equivalent to `npm run build` + copy into `../build`), then a full `docker build .` from the repo root to confirm the `Dockerfile` stage-2 `npm install && npm run build` still succeeds in a clean `node:22-alpine` environment (no local `node_modules` leaking in stale peer resolutions).
7. Confirm CI (`.github/workflows/ci.yaml`'s `ts` job: `task ts:install`, `task ts:lint`, `task ts:build`) is green.

**Rollback**: revert all `web/` changes from this workstream (`package.json`, `package-lock.json`, `vite.config.ts`, `app.css`, the deleted `tailwind.config.ts`/`postcss.config.js`, and every rewritten `.svelte` file) — workstream 1's SvelteKit/Svelte bump can stay in place independently since it has no dependency on this workstream.

## Sequencing notes

- Do workstream 1 (SvelteKit/Svelte) first — it's independent, has no breaking API surface for this codebase, and satisfies `flowbite-svelte` 1.x's `svelte >= 5.40.0` peer floor before workstream 2 needs it.
- Workstream 2 (Tailwind v4 + Flowbite Svelte 1.x + Flowbite Svelte Icons 3.x) must land as one atomic change — the config migration (2a) alone will not render correctly until the component rewrites (2b) are also done, since `flowbite-svelte` 0.x components are not styled for a Tailwind v4 + `flowbite-svelte` 1.x theme, and `flowbite-svelte` 1.x components will not resolve their classes without 2a's `@source`/`@theme`/`@plugin` setup. Don't try to commit 2a and 2b as separate deployable states.
- Commit workstream 1 separately from workstream 2 within the branch (even though they ship as one PR) so a regression is easy to bisect, and re-run the full verification pass (`npm run check && npm run lint && npm run test:unit && npm run test:e2e` plus a manual dev-server pass) after workstream 2 to confirm nothing from workstream 1 regressed.
