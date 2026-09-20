# Unblock ts major updates

Status: proposed · 2026-09-20

Unblock the weekly grouped `ts` major Renovate PR ([#221](https://github.com/ivanklee86/tangle/pull/221)),
stuck on an unsatisfiable `typescript@7` vs. `svelte-check@4` peer conflict, plus fix a latent
undeclared dependency (`@eslint/js`) that eslint 10 exposes, and make CI fail legibly the next time
a Renovate artifact update doesn't land. See
[ADR 0019](../../adrs/0019-hold-typescript-majors-until-svelte-check-supports-them.md) for the
decision record and the reproduction that isolated these as the three real defects.

## 1. `renovate.json` — hold `typescript` below 7

Added alongside the existing `ts` group rule:

```json
{
  "matchDatasources": ["npm"],
  "groupName": "ts"
},
{
  "description": "svelte-check 4.x peers typescript ^5 || ^6; drop this rule when it supports 7",
  "matchDatasources": ["npm"],
  "matchPackageNames": ["typescript"],
  "allowedVersions": "<7"
},
```

`allowedVersions` (not `enabled: false`) so 5.x/6.x updates still flow; only the unsatisfiable major
jump is suppressed. Validated with `prek run renovate-config-validator --all-files`.

## 2. `web/package.json` — declare `@eslint/js`

```json
"devDependencies": {
  "@eslint/compat": "^1.2.3",
  "@eslint/js": "9.39.5",
  ...
```

Pinned to the exact version already resolved in `web/package-lock.json` — confirmed via
`npm install`, which touched only that one line of the lockfile (the `""` root package's
`devDependencies`), leaving every resolved package entry unchanged. This makes the direct import in
`web/eslint.config.js:2` an honest dependency instead of relying on it being hoisted from `eslint`'s
own (major-version-dependent) dependency list.

## 3. `tasks/ts.yaml` + `.github/workflows/ci.yaml` — deterministic CI installs

**`tasks/ts.yaml`**, new task next to `install`:

```yaml
  install-ci:
    desc: Install dependencies from the lockfile, exactly (CI).
    dir: web
    cmds:
      - npm ci
```

**`.github/workflows/ci.yaml`**: both frontend-install steps (`ts` job's "Install packages",
`e2e` job's "Install frontend packages") switched from `task ts:install` to `task ts:install-ci`.
`task ts:install` (`npm install`) is unchanged for local development.

This mirrors `go:install-ci` (`tasks/go.yaml:21`)'s existing pattern of a separate, deterministic
CI-only install task. `npm ci` refuses to run against a `package.json`/`package-lock.json` that are
out of sync, so the next time a Renovate artifact update fails to regenerate the lockfile, CI fails
immediately at the install step with a clear message — not several steps later with a misleading
transitive peer-dependency error, which is what happened on #221 (`ts` job resolved against the
PR's stale lockfile and reported an `@sveltejs/vite-plugin-svelte@5.1.1` conflict unrelated to the
real, upstream `typescript`/`svelte-check` incompatibility).

## 4. Docs

- [ADR 0019](../../adrs/0019-hold-typescript-majors-until-svelte-check-supports-them.md).
- `docs/agents/ci.md`: note the `ts`/`e2e` jobs install via `task ts:install-ci` (`npm ci`), and why.

## Verification performed

On `main`, after all changes:

- `cd web && rm -rf node_modules && npm ci` — succeeds; proves the committed lockfile is
  self-consistent with the new `@eslint/js` entry.
- `task ts:lint` (`svelte-check` + `eslint`/`prettier`) — passes, 1623 files, 0 errors; no behavior
  change under eslint 9.
- `task ts:test:unit` — 93/93 tests pass.
- `task ts:build` — succeeds.
- `prek run renovate-config-validator --all-files` — passes.

Dry-run of the *next* grouped `ts` major PR, in a disposable worktree of #221's branch with
`typescript` held at `^5.0.0` (everything else in #221 unchanged) plus `@eslint/js@10.0.1` added —
i.e., exactly what Renovate should propose once this lands:

- `npm install` — resolves cleanly (253 packages, no ERESOLVE).
- `npm run check` (`svelte-check`) — 1638 files, 0 errors.
- `npm run lint` — passes under eslint 10 (proves `@eslint/js` resolves once eslint no longer
  bundles it).
- `npx vitest --run` — 93/93 tests pass.
- `npm run test:e2e` — 16/16 mocked Playwright tests pass.
- `npm run build` — succeeds.

This is full parity with the CI `ts` job's own steps, run against the exact future dependency set,
not just the isolated install.

## Follow-up (operational, not part of this change)

1. Close PR #221 rather than rebase — the config change removes `typescript` from the proposed
   batch entirely, so the next scheduled Friday run should open a *different* PR, not a corrected
   version of this one.
2. If Renovate's close-means-ignore memory suppresses the next proposal, use the Dependency
   Dashboard issue's checkbox to force recreation.
3. Revisit [ADR 0019](../../adrs/0019-hold-typescript-majors-until-svelte-check-supports-them.md)'s
   `allowedVersions` rule once `svelte-check` ships TypeScript 7 support.
