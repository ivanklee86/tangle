# Svelte component unit tests

Status: proposed · 2026-09-19

Close the remaining `$lib` unit-test gaps and add real component-level tests for `web/src/lib/ui/components/*.svelte` via `vitest-browser-svelte`. See [ADR 0005](../../adrs/0005-svelte-component-unit-tests.md) for why `vitest-browser-svelte` over jsdom, why route-level `+page.svelte` files are out of scope, and why the devcontainer/font work here doubles as a prerequisite for [ADR 0003](../../adrs/0003-svelte-e2e-testing-strategy.md)'s still-unimplemented e2e suite.

Land the six workstreams below roughly in order — 1 is independent and can land any time; 2–4 are prerequisites for 5; 6 lands last.

## 1. Close the remaining `$lib` gaps

**`lib/backend/client.ts`** has no direct test — `getApplications`/`getApplicationDiff` are thin wrappers around `fetchEnvelope`/`buildQuery` (both already tested), but the URL/method/body assembly and `requestDetails` attachment that `client.ts` itself owns aren't covered.

**New file** `web/src/lib/backend/client.spec.ts`: mock global `fetch` (same pattern as `http.spec.ts`) and assert — `getApplications(null, null)` hits `PATH_APPLICATIONS` with no query string; `getApplications('a:b', 'c:d')` includes both params via `buildQuery`; `getApplicationDiff(...)` POSTs to `` `/api/argocd/${argoCD}/applications/${applicationName}/diffs` `` with `Content-Type: application/json` and a JSON body of exactly `{liveRef, targetRef}`; the returned `ApplicationDiff` has `requestDetails: {argoCD, applicationName}` attached regardless of success/error.

**New file** `web/src/lib/backend/data.spec.ts`: `emptyApplicationResponseStore()` returns `{response: {results: []}, errorResponse: {error: ''}, error: false, loaded: false}`, and returns a fresh object each call (mutating one result doesn't affect the next call's result).

**Rollback**: revert the two new spec files; nothing else depends on them.

## 2. Devcontainer: Playwright browser + font packages

Reuses exactly the package list [ADR-0003's plan](svelte-e2e-testing.md) (workstream 2) already researched and verified in this devcontainer — a Chromium binary is already cached here (`~/.cache/ms-playwright`), confirming the binary itself installs fine, but the apt font/runtime packages aren't in the `Dockerfile` yet, and without them Chromium renders real content with genuinely zero-height/invisible text (a real risk for component tests asserting on visible text, not just a screenshot concern).

**Steps**

1. In `.devcontainer/Dockerfile`, add (verbatim from ADR-0003's plan, keeping `PLAYWRIGHT_VERSION` in sync with `web/package.json`'s `playwright`/`@playwright/test` pins from workstream 3 below):

   ```dockerfile
   # Playwright (for web/ e2e tests and vitest-browser-svelte component tests) —
   # keep the version in sync with playwright/@playwright/test in web/package.json.
   ARG PLAYWRIGHT_VERSION=1.63.0
   RUN npx --yes playwright@${PLAYWRIGHT_VERSION} install chromium chromium-headless-shell && \
       apt-get update && apt-get install -y --no-install-recommends \
           libglib2.0-0 libnss3 libnspr4 libdbus-1-3 libatk1.0-0 \
           libatk-bridge2.0-0 libatspi2.0-0 libx11-6 libxcomposite1 \
           libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 \
           libxkbcommon0 libasound2 libcups2 libpango-1.0-0 libcairo2 \
           fonts-liberation fonts-dejavu-core && \
       rm -rf /var/lib/apt/lists/*
   ```

   **Corrected during this plan's implementation** (2026-09-19) from ADR-0003's original list: `chromium-headless-shell` is a separate download from `chromium` — `@vitest/browser-playwright` launches the headless-shell binary specifically, and installing only `chromium` produces a `browserType.launch: Executable doesn't exist at .../chrome-headless-shell` error. Three more apt packages (`libcups2`, `libpango-1.0-0`, `libcairo2`) were also missing from ADR-0003's original researched list — surfaced by Playwright's own `Host system is missing dependencies` warning when actually launching a browser, not by the earlier `install`-only check. ADR-0003's plan doc, if implemented later, should pick up this corrected list rather than its own original one.
2. `task devcontainer` to confirm the image still builds.
3. Inside a container built from the new image, confirm a rendered component actually shows visible text (workstream 5 below is the real test of this, but a quick manual `npx playwright test` sanity smoke — even against an empty suite — confirms the binary + fonts are both present).

**Rollback**: revert the `Dockerfile` hunk; nothing else depends on it (the currently-cached-by-hand browser in this running container is not itself reproducible without this step, so skipping it works today but silently breaks on the next container rebuild).

## 3. Upgrade Vitest to Browser Mode and add the new devDependencies

**Verified compatibility** (`npm view <pkg> peerDependencies` / `npm ls`, 2026-09-19): `vitest-browser-svelte@3.1.0` requires `vitest >=5.0.0`; `vitest@5.0.1` requires `vite ^6.4.0 || ^7 || ^8`; this repo currently resolves `vite` to `6.2.6` under its `^6.0.0` pin. So this is a real upgrade, not just new packages.

**`web/package.json` devDependencies changes** (exact pins, per `AGENTS.md`):

- `vitest`: `^3.0.0` → `5.0.1`
- `vite`: `^6.0.0` → `6.4.3`
- `@vitest/browser`: add, `5.0.1` (must match `vitest`'s exact version)
- `@vitest/browser-playwright`: add, `5.0.1` (must match `vitest`'s exact version — this is Vitest 5's current browser-provider package; confirm at implementation time it hasn't been superseded again, Vitest's browser-provider API has moved once already between major versions)
- `vitest-browser-svelte`: add, `3.1.0`
- `playwright`: add, `1.63.0` (exact match with the already-pinned `@playwright/test`'s `1.63.0` — `@vitest/browser-playwright` drives the browser via the plain `playwright` package, distinct from `@playwright/test`, which ADR-0003's e2e suite uses; both share the one Chromium binary cache from workstream 2)

**Steps**

1. Apply the version bumps and new devDependencies, `npm install`.
2. `npm run test:unit -- --run` — the existing six `$lib` spec files must still pass unmodified before any new test-writing starts, so a break is isolated to "the bump," not tangled with new component-test code. `npm run check` and `npm run lint` too, in case the Vite/Vitest bump shifts any type inference.
3. `npm run build` to confirm `adapter-static`'s prerendered output is unaffected by the Vite bump.

**Rollback**: revert the `package.json`/`package-lock.json` changes; nothing in workstream 1 depends on this (it's plain Node-environment vitest, unaffected by Browser Mode).

## 4. `vite.config.ts`: split into `node` and `browser` test projects

Vitest 5's multi-environment setup uses `test.projects` — one project keeps running the existing plain-`.ts` specs in Node (unchanged behavior, no browser needed), the other runs new `*.svelte.test.ts` files in Browser Mode.

```typescript
// vite.config.ts
import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { playwright } from '@vitest/browser-playwright';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],

	test: {
		projects: [
			{
				extends: true,
				test: {
					name: 'node',
					environment: 'node',
					include: ['src/**/*.spec.{js,ts}'],
					exclude: ['src/**/*.svelte.test.{js,ts}']
				}
			},
			{
				extends: true,
				test: {
					name: 'browser',
					include: ['src/**/*.svelte.test.{js,ts}'],
					setupFiles: ['vitest-browser-svelte'],
					browser: {
						enabled: true,
						headless: true,
						provider: playwright(),
						instances: [{ browser: 'chromium' }]
					}
				}
			}
		]
	}
});
```

`extends: true` inherits the shared `plugins`/root config into each project (the exact key/shape to confirm against `@vitest/browser-playwright@5.0.1`'s docs at implementation time — this config surface changed across Vitest 4→5 per the source review done for ADR-0005, so treat the snippet above as "verified shape as of this plan's writing," not gospel). Naming convention: plain-function specs stay `*.spec.ts` (unchanged, matches workstream 1 and everything from the ADR-0004 refactor); component tests use `*.svelte.test.ts`, colocated next to the component they test (e.g. `lib/ui/components/Header.svelte.test.ts`) — the two suffixes are what the `include`/`exclude` split above keys off, so a misnamed file silently lands in the wrong project.

**Steps**

1. Apply the config split.
2. `npm run test:unit -- --run` once more — should still show all six existing specs passing under the `node` project, zero browser tests yet (workstream 5 adds those), confirming the split itself doesn't break anything before any component test is written.

**Rollback**: revert `vite.config.ts`; workstream 3's dependency additions are harmless if unused.

## 5. Write component tests

In order of complexity — start with pure prop-driven components with no external calls, end with the one "smart" component that fetches its own data.

- **`ArgoCDHealthStatus.svelte.test.ts`**: renders `CheckCircleSolid` + green text for `healthStatus="Healthy"`, `CloseCircleSolid` + red text for any other value (e.g. `"Missing"`), and the status text itself is visible.
- **`ArgoCDSyncStatus.svelte.test.ts`**: same shape for `syncStatus` — `Synced` (green check), `OutOfSync` (red close), `Unknown` (amber exclamation).
- **`Header.svelte.test.ts`**: renders the "Tangle" brand text and a dark-mode toggle control; clicking the toggle flips `document.documentElement`'s `dark` class (this is `flowbite-svelte`'s `DarkMode` component's own behavior — the test is really "did we wire it up," not re-testing `flowbite-svelte` itself).
- **`AppManifests.svelte.test.ts`** (prop-driven, several branches — construct each `ApplicationDiff` fixture inline, same shape as `diffs.spec.ts`'s `makeDiff` helper): `error: true` shows the red system-error alert with `errorResponse.error`'s text; `manifestGenerationError` non-empty shows the error card with that message and nothing else; `loaded: true` with empty `diffs` shows "No diffs found."; `loaded: true` with non-empty `diffs` renders the `CodeBlock` diff content and a collapsed "Manifests" accordion that expands to show `targetManifests`.
- **`ApplicationsGrid.svelte.test.ts`** (the smart component): `vi.mock('$app/stores', ...)` to control `page.url.searchParams` (matches what `routes/diffs/+page.svelte` itself reads), and mock global `fetch` the same way `http.spec.ts` does to stand in for `TangleAPIClient`'s real network call. Cases: loading state shows a spinner before the fetch resolves; a successful response renders one tab per ArgoCD with the `(count)` suffix, `filterOutZeroResults` applied (an ArgoCD with zero applications gets no tab); clicking a column header once sorts ascending (▲ indicator, `aria-sort="ascending"`), clicking again sorts descending; an error response shows the red system-error alert instead of the table.

**Rollback**: revert the five new `*.svelte.test.ts` files; workstreams 1–4 stand alone without them.

## 6. Wire `task ts:test` into CI

**Finding**: `.github/workflows/ci.yaml`'s `ts` job currently runs `task ts:install` → `task ts:lint` → `task ts:build` only — `task ts:test` (→ `npm run test:unit`) is never invoked. This gap predates this plan and exists independently of component tests, but there's no reason to add new test coverage without also running it in CI.

**Steps**

1. Add a Playwright browser install step before the test step, **after** `task ts:install` has already run `npm install` (so `web/node_modules/playwright` exists):

   ```yaml
   - name: Install Playwright browser
     run: node node_modules/playwright/cli.js install --with-deps chromium chromium-headless-shell
     working-directory: web
   - name: Run unit tests
     run: task ts:test
   ```

   **Corrected during implementation** (2026-09-19): the original `npx --yes playwright@1.63.0 install --with-deps chromium` failed CI with `browserType.launch: Executable doesn't exist at .../chromium_headless_shell-1243/...` — it silently downloaded revision **1161** instead of **1243**. Root cause: this project depends on both `playwright` (pinned `1.63.0`) and `@playwright/test` (pinned `^1.45.3`, resolving to `1.51.1`), and both packages provide a same-named `playwright` CLI binary; `npx playwright@1.63.0` resolved to whichever locally-installed bin won the naming collision (`@playwright/test`'s older one) instead of strictly fetching `1.63.0`, so it installed the browser revision that *older* version expects. Invoking `node node_modules/playwright/cli.js` directly bypasses bin resolution entirely and always uses the exact `playwright` version declared in `web/package.json`. This only works once a project checkout with `node_modules` exists — it's not usable for the devcontainer Dockerfile step (workstream 2), which has no checkout yet at that build stage; whether that step's plain `npx playwright@${VERSION} install` is reliably safe in a truly clean environment (no competing bin to collide with) was not independently verified — treat it as a real risk, not a settled fact, and re-check if the devcontainer's cached browser revision ever mismatches what a later `npm install` expects.
   Keep `1.63.0` in sync with `web/package.json`'s `playwright`/`@playwright/test` pins (workstream 3) and the devcontainer's `PLAYWRIGHT_VERSION` (workstream 2).
2. Insert this after `Install packages` and before `Lint code` (or after — order doesn't matter functionally, but running tests before lint gives a slightly faster failure signal for the more common failure mode).
3. Push a branch and confirm the `ts` job's new step runs and passes.

**Rollback**: revert the CI workflow hunk; `task ts:test` still exists and can be run manually, it just won't gate PRs.

## Sequencing notes

- Do 1 any time — it has no dependency on 2–6 and is useful on its own.
- Do 2 → 3 → 4 in order before 5: the devcontainer's fonts (2) and the dependency/config upgrade (3, 4) are both needed for a component test to even render correctly, let alone pass.
- After 3, before touching `vite.config.ts` in 4: confirm the existing six specs still pass under the bumped `vitest`/`vite` alone, to isolate upgrade breakage from config-split breakage.
- Do 6 last, once 5's tests exist locally-green — no point wiring a CI step for tests that don't exist yet or don't yet pass reliably in a fresh browser instance.
- Commit each workstream separately, matching this repo's existing convention ([svelte-e2e-testing.md](svelte-e2e-testing.md), [svelte-frontend-lib-refactor.md](svelte-frontend-lib-refactor.md)).
- This plan and ADR-0003's e2e plan now share two prerequisites (the devcontainer Playwright/font install, the `playwright`/`@playwright/test` version pin) — whichever plan is implemented second should reuse what the first one already landed rather than redoing the research; workstream 2 above is written to be that shared piece regardless of order.
