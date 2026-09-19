# Svelte frontend upgrade

Status: proposed · 2026-09-19

Upgrade `web/` (the SvelteKit app `task ts:build` compiles into `build/`, embedded into the `tangle-server` image by `Dockerfile`'s stage 2) to the latest SvelteKit/Svelte, Tailwind CSS, and Flowbite Svelte, with no feature or functionality changes — only version currency and the mechanical API changes those versions require. See [ADR 0002](../../adrs/0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md) for why Tailwind v4 and `flowbite-svelte` 1.x are one coupled workstream rather than two.

Do this as a single PR, landing the two workstreams below in order so each is validated before moving to the next. Re-check every exact version number against npm at implementation time (`npm view <pkg> version`) and pin the resolved version exactly in `package.json` per AGENTS.md — the versions below were current as of 2026-09-19.

## 1. Upgrade SvelteKit and Svelte

**Current → target** (`web/package.json`)

| | Current | Target |
|---|---|---|
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
|---|---|---|
| `tailwindcss` | `^3` (resolved 3.4.17) | `4.x` latest (e.g. 4.3.3) |
| `flowbite-svelte` | 0.47.4 | `1.x` latest (e.g. 1.33.1) |
| `flowbite-svelte-icons` | 2.1.1 | `3.x` latest (e.g. 3.1.0) |
| `flowbite` | 2.5.2 | Match whatever `flowbite-svelte` 1.x's own repo pins as its `flowbite` devDependency at implementation time (its `@plugin "flowbite/plugin"` CSS import needs a real `flowbite` package on disk; don't blindly take npm's absolute latest major without checking `flowbite-svelte` has been built/tested against it) |
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

   @source '../node_modules/flowbite-svelte/**/*.{html,js,svelte,ts}';
   @source '../node_modules/flowbite-svelte-icons/**/*.{html,js,svelte,ts}';
   @source '../node_modules/svhighlight/**/*.svelte';

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
  - `<Table hoverable={true} items={argoCDApplications.applications}>` + `<TableBodyRow slot="row" let:item>`: in `flowbite-svelte` 1.x, `Table`'s `items` prop makes it render an auto-generated head/body from the items and **ignore its children entirely** — it can no longer be combined with hand-written `TableHead`/`TableBody`/custom cell rendering the way 0.x allowed. Drop `items` from `<Table>` and replace the `slot="row" let:item` row with a plain `{#each}` inside `<TableBody>`:
    ```svelte
    <Table hoverable={true}>
    	<TableHead>...</TableHead>
    	<TableBody tableBodyClass="divide-y">
    		{#each argoCDApplications.applications as item (item.name)}
    			<TableBodyRow>
    				<TableBodyCell>
    					<a href={item.url} target="_blank" class="link-underline-primary">{item.name}</a>
    				</TableBodyCell>
    				<TableBodyCell><ArgoCDHealthStatus healthStatus={item.health} /></TableBodyCell>
    				<TableBodyCell><ArgoCDSyncStatus syncStatus={item.syncStatus} /></TableBodyCell>
    			</TableBodyRow>
    		{/each}
    	</TableBody>
    </Table>
    ```
    This also removes the three `{/* @ts-expect-error */}` workarounds in this file — they existed only because 0.x's `let:item` slot prop couldn't be typed; a typed `{#each item of ...}` doesn't need them.
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
|---|---|---|
| `eslint-plugin-svelte` | 3.0.2 | latest 3.x |
| `prettier-plugin-svelte` | 3.3.3 | latest 3.x |
| `prettier-plugin-tailwindcss` | 0.6.11 | latest 0.x (needed for Tailwind v4 class sorting) |

**Steps**

1. Pin the version table above in `web/package.json`, `npm install`.
2. Rewrite every file listed in 2b.
3. `npm run format && npm run lint && npm run check` until clean.
4. `npm run test:unit && npm run test:e2e`.
5. `npm run dev` and manually exercise all three routes (`/`, `/applications`, `/diffs`) in both light and dark mode (toggle via the `DarkMode` button in the header): confirm Tailwind styling renders (a wall of unstyled/default-browser-styled elements means a missing `@source` glob), confirm tabs/table sorting/toast dismissal/accordion expand/gradient buttons/dark-mode toggle all still work exactly as before.
6. `task ts:build` (equivalent to `npm run build` + copy into `../build`), then a full `docker build .` from the repo root to confirm the `Dockerfile` stage-2 `npm install && npm run build` still succeeds in a clean `node:22-alpine` environment (no local `node_modules` leaking in stale peer resolutions).
7. Confirm CI (`.github/workflows/ci.yaml`'s `ts` job: `task ts:install`, `task ts:lint`, `task ts:build`) is green.

**Rollback**: revert all `web/` changes from this workstream (`package.json`, `package-lock.json`, `vite.config.ts`, `app.css`, the deleted `tailwind.config.ts`/`postcss.config.js`, and every rewritten `.svelte` file) — workstream 1's SvelteKit/Svelte bump can stay in place independently since it has no dependency on this workstream.

## Sequencing notes

- Do workstream 1 (SvelteKit/Svelte) first — it's independent, has no breaking API surface for this codebase, and satisfies `flowbite-svelte` 1.x's `svelte >= 5.40.0` peer floor before workstream 2 needs it.
- Workstream 2 (Tailwind v4 + Flowbite Svelte 1.x + Flowbite Svelte Icons 3.x) must land as one atomic change — the config migration (2a) alone will not render correctly until the component rewrites (2b) are also done, since `flowbite-svelte` 0.x components are not styled for a Tailwind v4 + `flowbite-svelte` 1.x theme, and `flowbite-svelte` 1.x components will not resolve their classes without 2a's `@source`/`@theme`/`@plugin` setup. Don't try to commit 2a and 2b as separate deployable states.
- Commit workstream 1 separately from workstream 2 within the branch (even though they ship as one PR) so a regression is easy to bisect, and re-run the full verification pass (`npm run check && npm run lint && npm run test:unit && npm run test:e2e` plus a manual dev-server pass) after workstream 2 to confirm nothing from workstream 1 regressed.
