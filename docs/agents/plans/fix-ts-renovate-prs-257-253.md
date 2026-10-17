# Fix failing ts Renovate PRs (#257, #253)

Status: implemented · 2026-10-17 (one combined PR carrying both PRs' version bumps plus both fixes; the Renovate PRs are superseded)

Two Renovate PRs for the `ts` group are red. They fail for unrelated reasons: #257 (minor/patch) is a CI caching defect that any Playwright bump would hit, and #253 (major) is the real SvelteKit 2 → 3 migration. Both were reproduced locally, and a prototype of each fix passed the full `web/` suite. Land #257's fix first so #253 doesn't pick up the same Playwright failure when Renovate rebases it onto main.

## 1. #257 `Update ts`: Playwright browser cache key is a hardcoded literal

**Symptom.** In the `ts` job, `ts:test:unit` fails with `browserType.launch: Executable doesn't exist at ~/.cache/ms-playwright/chromium_headless_shell-1248/...`. All 181 node tests pass. Every browser-mode test file fails to launch. `e2e (amd64/arm64)` fail the same way in the live frontend suite.

**Root cause.** The PR bumps `playwright`/`@playwright/test` 1.63.0 → 1.64.0, which needs browser revision 1248. But both `Cache Playwright browser` steps in `.github/workflows/ci.yaml` (lines 150 and 320) use the literal key `playwright-${{ runner.os }}-${{ runner.arch }}-1.63.0`. That key gets a cache hit and restores revision 1243, so `Install Playwright browser` is skipped (`cache-hit == 'true'`) and 1.64 finds no browser. Renovate didn't update the literal because no `customManagers` entry matches it. The `.devcontainer/Dockerfile` `PLAYWRIGHT_VERSION` ARG has an entry and was bumped correctly. This literal was introduced in #210 at 1.63.0, and #257 is the first Playwright bump since.

**Local reproduction.** The devcontainer cache has only `chromium_headless_shell-1243`, which matches CI's stale cache. On `renovate/ts`, `npm ci && npx vitest --run` gives the identical error (17/25 files pass, 1 unhandled error). After `node node_modules/playwright/cli.js install chromium chromium-headless-shell` into a scratch `PLAYWRIGHT_BROWSERS_PATH`, it gives 25/25 files and 253/253 tests. Nothing else in the PR is broken.

**Fix.** Derive the key from the installed package, not a literal. This removes the drift class entirely rather than adding another Renovate regex to keep in sync. It still honors the existing comment's intent: the key matches exactly what's installed, and an unrelated `package.json` change doesn't bust the cache.

1. In both the `ts` and `e2e` jobs, add a step before `Cache Playwright browser` (after `task ts:install-ci`, so `node_modules` exists):

   ```yaml
   - name: Resolve Playwright version
     id: playwright-version
     working-directory: web
     run: echo "version=$(node -p "require('./node_modules/playwright/package.json').version")" >> "$GITHUB_OUTPUT"
   ```

2. Change both keys to `playwright-${{ runner.os }}-${{ runner.arch }}-${{ steps.playwright-version.outputs.version }}` and update the comments to say the version is read from the installed package.
3. ~~Land this as its own small PR~~ Superseded: landed together with §2 in one PR that also carries #257's version bumps.
4. Optional hardening: `@vitest/browser-playwright` and `@playwright/test` both resolve `playwright`, so add a CI assertion (or `npm ls playwright`) that all three are deduped to one version. A split here would bring back the wrong-revision problem the existing `node node_modules/playwright/cli.js` comment warns about.

**Rejected alternative.** Adding a Renovate `customManagers` regex for the ci.yaml literal works, but it keeps two sources of truth that can still diverge, for example on a manual bump.

## 2. #253 `Update ts (major)`: SvelteKit 3 / adapter-static 4 migration

**Symptom.** `ts`, `image`, and both `e2e` jobs fail with `SvelteKit error: config_file_unsupported — svelte.config.js is no longer used. Please pass configuration via the sveltekit(...) plugin in your Vite config.` (The `image`/`e2e` jobs fail inside `Dockerfile`'s `npm run build`.)

**Root cause.** SvelteKit 3 has several breaking changes. CI only shows the first one. Fixing it exposes the rest, and all were reproduced locally on `renovate/major-ts`:

| # | Break | Surfaces in | Fix (validated) |
| --- | --- | --- | --- |
| a | `svelte.config.js` is rejected; config is a flat object passed to `sveltekit({...})` (no `kit:` nesting) | build, vitest | Delete `web/svelte.config.js`; pass `preprocess: vitePreprocess()` and `adapter: adapter({...same options...})` to `sveltekit()` in `web/vite.config.ts` |
| b | `$lib` removed in favour of `#lib` (Node subpath imports) | build | See decision below; prototype used `alias: { $lib: 'src/lib' }` |
| c | Generated tsconfig moved: `.svelte-kit/tsconfig.json` → `$app/tsconfig` (written to `node_modules/$app/tsconfig.json`) | build (`Tsconfig not found`), check | `web/tsconfig.json`: `"extends": "$app/tsconfig"` |
| d | The generated tsconfig no longer carries `include`/`exclude`, so svelte-check typechecks `build/_app` (4,411 of 4,431 errors) plus `scripts/` and `eslint.config.js` | check | Add explicit `include` (`.svelte-kit/**/*.d.ts`, `vite.config.ts`, `src/**/*.{js,ts,svelte}`) and `exclude` (`node_modules/**`, `build/**`) to `web/tsconfig.json` |
| e | Env vars must be declared: `PUBLIC_BASE_URL` is no longer exported without a `src/env.ts`; `$env/static/public` is deprecated for `$app/env/public` | build (`MISSING_EXPORT`) | Add `web/src/env.ts` with `defineEnvVars({ PUBLIC_BASE_URL: { public: true, static: true } })`; switch imports in `src/lib/backend/client.ts` and `src/lib/backend/config.ts` |
| f | Dev-mode env module reads `globalThis.__sveltekit_dev`, which Kit injects into served HTML but Vitest browser mode never has | vitest browser project (`applications`/`diffs` page tests fail to import) | Browser-project setup file that runs `vi.mock('$app/env/public', () => ({ PUBLIC_BASE_URL: '' }))` |

**Prototype result** (all of a–f, using the `$lib` alias for b, on top of `renovate/major-ts`): `npm run build` ✅ (`build/` has the same `index.html` fallback + `_app/` shape), `npm run check` 0 errors ✅, `npm run lint` ✅, `vitest --run` 25/25 files, 253/253 tests ✅, mocked Playwright e2e 42/42 ✅. Not run locally: the `image`/`e2e` Docker builds and the live e2e suite. `Dockerfile` copies all of `web/` and only runs `npm run build`, so it needs no changes.

**Decision needed for (b): keep `$lib` via `alias`, or migrate to `#lib`?**

- *Keep `$lib` (recommended for this PR):* one line in `vite.config.ts`, and none of the 45 files that import `$lib` change. Kit warns `config_option_deprecated_alias` on every build, so it's a known, bounded follow-up.
- *Migrate to `#lib`:* matches Kit 3's direction, but it isn't mechanical. A `package.json` `"imports": { "#lib/*": "./src/lib/*" }` builds with Vite, but svelte-check / TypeScript can't resolve extensionless subpath-import targets (`Cannot find module '#lib/ui/query'`). Extension-bearing targets break the Vite build instead. Getting both to agree needs more investigation. Use `npx sv migrate sveltekit-3` if it exists by then, otherwise work out the right `imports` shape. Do this as a separate follow-up PR.

**Steps**

1. After §1 lands, let Renovate rebase `renovate/major-ts` (it's currently behind `main`), or rebase it manually.
2. ~~Apply a–f on top of the Renovate branch~~ Superseded: applied in the combined PR, which carries #253's version bumps itself.
3. Run `task ts:install-ci`, `task ts:test:unit`, `task ts:test:e2e`, `task ts:lint`, `task ts:build`, `npm run check`, then `task docker:build` to cover the `image`/`e2e` jobs locally.
4. Record an ADR (MADR 4.0.0, next number in `docs/adrs`) for adopting SvelteKit 3: the config moving into `vite.config.ts`, explicit env declaration, and the `$lib`-alias stopgap with its removal trigger.
5. Update `docs/agents/internals.md` wherever it describes `web/` config (`svelte.config.js`, `$env/static/public`), and `docs/agents/plans/svelte-frontend-upgrade.md`'s pointer to `svelte.config.js`.

**Tests to add or adjust** (behavior from a caller's perspective)

- Build-output contract: `build/index.html` exists (SPA fallback) and `build/_app/` exists. This is the promise `Dockerfile` stage 2 relies on, and an adapter-static major could silently change it. Mocked e2e against `vite preview` already covers it implicitly; consider a cheap explicit assertion in `ts:build`.
- `PUBLIC_BASE_URL` handling: `fetchConfig`/client tests should keep asserting that an empty base URL produces same-origin `/api/...` requests. The new `vi.mock` must not hide a regression there, so keep the mock's value identical to `.env.test` (`''`).
- Flag: the browser-project `vi.mock` papers over a Kit 3 / Vitest-browser integration gap. If upstream fixes it (Kit setting `__sveltekit_dev` under Vitest), delete the mock instead of keeping it.

**Rollback**: revert the branch's migration commits. Everything is confined to `web/` plus docs.
