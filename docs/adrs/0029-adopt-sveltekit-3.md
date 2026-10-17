---
status: "accepted"
date: 2026-10-17
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Adopt SvelteKit 3, keeping `$lib` through a deprecated alias until a separate `#lib` migration

## Context and Problem Statement

[PR #253](https://github.com/ivanklee86/tangle/pull/253), the grouped `ts` major Renovate PR, bumps `@sveltejs/kit` 2.70.3 → 3.0.1 and `@sveltejs/adapter-static` 3.0.10 → 4.0.0. It fails `ts`, `image` and both `e2e` jobs with `config_file_unsupported` (`svelte.config.js` is no longer read). Fixing that exposes more SvelteKit 3 breaking changes, each reproduced on the PR branch:

1. Config is a flat object passed to `sveltekit({...})` in `vite.config.ts`. The `kit:` nesting is gone.
2. `$lib` was removed in favour of `#lib` (Node subpath imports declared in `package.json` `imports`). Every `$lib` import fails the build with `module_removed_lib` unless an `alias` restores it.
3. The generated tsconfig moved from `.svelte-kit/tsconfig.json` to `$app/tsconfig`, and it no longer sets `include`/`exclude`. Without them svelte-check typechecks the minified `build/` output: 4,411 of 4,431 errors.
4. Environment variables must be declared in `src/env.ts` with `defineEnvVars`. Undeclared `PUBLIC_BASE_URL` is no longer exported. `$env/static/public` is deprecated for `$app/env/public`.
5. Kit 3's dev-mode env module reads `globalThis.__sveltekit_dev`, which Kit injects into the HTML it serves. Vitest browser mode never sets it, so any browser-mode test that imports a module reading `$app/env/public` fails to load.

Separately, the same week's minor/patch `ts` PR ([#257](https://github.com/ivanklee86/tangle/pull/257)) bumps Playwright 1.63.0 → 1.64.0 and fails because the CI browser-cache key was a literal `1.63.0` that Renovate doesn't manage. It's fixed in the same change but isn't part of this decision. The key now comes from the installed `playwright` package (see `docs/agents/ci.md`).

## Decision Drivers

- Stay current on SvelteKit majors. Renovate keeps re-raising the PR, and later 3.x fixes won't backport.
- Keep the upgrade PR reviewable: only the changes Kit 3 forces, with no unrelated churn across the codebase.
- No behavior change for the shipped SPA: the same `build/` shape (`index.html` fallback plus `_app/`) that `Dockerfile` stage 2 embeds, and the same same-origin `/api` calls when `PUBLIC_BASE_URL` is empty.

## Considered Options

- Upgrade to SvelteKit 3, keeping `$lib` through `alias: { $lib: 'src/lib' }`.
- Upgrade to SvelteKit 3 and migrate every import to `#lib` in the same PR.
- Hold SvelteKit at 2.x with a Renovate `allowedVersions` rule.

## Decision Outcome

Chosen option: "Upgrade to SvelteKit 3, keeping `$lib` through `alias`". It's the smallest change that gets onto Kit 3: one config line instead of rewriting the 45 files that import `$lib`. The `#lib` migration also isn't mechanical yet. With `"imports": { "#lib/*": "./src/lib/*" }`, Vite builds but svelte-check can't resolve the extensionless targets, and extension-bearing targets break the Vite build. That needs its own investigation and PR.

Concretely: `svelte.config.js` is deleted and its options move into `sveltekit({...})`. `tsconfig.json` extends `$app/tsconfig` and declares its own `include`/`exclude`. `src/env.ts` declares `PUBLIC_BASE_URL` as public and static. Imports move to `$app/env/public`. A browser-project Vitest setup file mocks `$app/env/public` with `.env.test`'s value (`''`).

### Consequences

- Good, because the frontend is on the current SvelteKit major, and build, check, lint, unit (253 tests) and mocked e2e (42 tests) all pass unchanged.
- Good, because environment variables are now declared in one typed place (`src/env.ts`).
- Bad, because `config.alias` is deprecated: every build prints `config_option_deprecated_alias`, and a future Kit release will remove it. Removal trigger: [#258](https://github.com/ivanklee86/tangle/issues/258).
- Bad, because the browser-test mock of `$app/env/public` works around a Kit/Vitest integration gap. If Kit starts setting `__sveltekit_dev` under Vitest, delete the mock rather than keeping it.

### Confirmation

CI's `ts` job (`ts:test:unit`, `ts:test:e2e`, `ts:lint`, `ts:build`), `npm run check` reporting 0 errors, and the `image`/`e2e` jobs building the production image.

## Pros and Cons of the Options

### Keep `$lib` through `alias`

- Good, because it's a one-line config change with zero import churn.
- Bad, because it depends on a deprecated option.

### Migrate to `#lib` now

- Good, because it matches Kit 3's direction and leaves nothing deprecated.
- Bad, because the TypeScript/Vite resolution of extensionless subpath-import targets doesn't work out of the box, and it touches 45 files on top of an already large major bump.

### Hold SvelteKit at 2.x

- Good, because nothing changes.
- Bad, because the gap only grows, Renovate keeps raising the PR, and 2.x stops getting fixes.

## More Information

Reproduction and investigation: [docs/agents/plans/fix-ts-renovate-prs-257-253.md](../agents/plans/fix-ts-renovate-prs-257-253.md).
