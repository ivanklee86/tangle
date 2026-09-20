---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Hold TypeScript majors below 7 until svelte-check supports them, and install `ts` deps in CI with `npm ci`

## Context and Problem Statement

[PR #221](https://github.com/ivanklee86/tangle/pull/221) ("Update ts (major)", the weekly grouped
`ts` major PR from [ADR 0013](0013-renovate-weekly-grouped-updates.md)) is stuck: Renovate's own
`renovate/artifacts` step failed to regenerate `web/package-lock.json`, and the `ts`/`e2e` CI jobs
fail on `npm install`. Reproducing the PR branch in a scratch worktree isolates the cause: a clean
`npm install` fails `ERESOLVE` on `typescript@^7.0.0` alone — `svelte-check@4.7.6` (the newest
release; there is no 5.x) declares `peerDependencies.typescript: "^5.0.0 || ^6.0.0"`, so nothing in
this dependency tree can satisfy both `typescript@7` and `svelte-check@4` at once. Holding
`typescript` back to `^5.0.0` and changing nothing else in the PR — `@eslint/compat` 2,
`@sveltejs/vite-plugin-svelte` 7, `eslint` 10, `flowbite` 4, `globals` 17, `vite` 8 all still bump —
lets the same install succeed (252 packages), and `svelte-check` then passes clean (1638 files, 0
errors). The other five majors in the PR are not the problem; `typescript` alone is.

A second, unrelated defect surfaces once the install succeeds: `web/eslint.config.js` imports
`@eslint/js` directly, but it has never been a declared dependency in `web/package.json` — it
resolved only because `eslint@9` lists `@eslint/js` as one of its own dependencies and npm hoists
it (confirmed via `npm view eslint@9.39.5 dependencies`). `eslint@10` drops that
(`npm view eslint@10.11.0 dependencies` has no `@eslint/js`), so `npm run lint` would fail
`ERR_MODULE_NOT_FOUND` even after the install itself resolves. This has been a latent gap since
`eslint.config.js` was written; the major bump is only what exposes it.

A third problem is procedural rather than a dependency conflict: `tasks/ts.yaml`'s `install` task
runs `npm install`, and `.github/workflows/ci.yaml`'s `ts`/`e2e` jobs both call it. `npm install`
reconciles an out-of-sync lockfile instead of refusing to run, so when Renovate's artifact update
failed and left `web/package-lock.json` unregenerated, CI didn't fail with a clear "lockfile is
stale" error — it failed on a misleading transitive peer conflict
(`@sveltejs/vite-plugin-svelte@5.1.1` from the stale lock vs. the PR's new `^7.0.0` declaration)
that had nothing to do with the real, upstream `typescript`/`svelte-check` incompatibility. `go`'s
equivalent CI installs already avoid this class of problem by having a separate `install-ci` task
(`tasks/go.yaml`); `ts` has no analog.

## Decision Drivers

- The `typescript`/`svelte-check` conflict is a real upstream incompatibility, not something this
  repo's configuration can route around — `svelte-check` has no release supporting TypeScript 7 as
  of this decision, confirmed via `npm view svelte-check@latest peerDependencies`.
- [ADR 0013](0013-renovate-weekly-grouped-updates.md) deliberately keeps all npm updates in one
  weekly `ts` group, reviewed as a batch; keeping that shape (rather than splitting npm majors into
  per-package PRs) was an explicit call on this decision, so the fix has to unblock the *group*
  without restructuring it.
- `--legacy-peer-deps`/`--force` were considered and rejected: npm's own error names the risk
  directly — accepting "an incorrect (and potentially broken) dependency resolution" — and
  `svelte-check`'s peer range is a real compatibility statement (it doesn't yet parse/typecheck
  against the TypeScript 7 API), not a formality that's safe to silence.
- `AGENTS.md`'s pinning rule and the repo's testing philosophy both push toward declaring what's
  actually depended on (`@eslint/js`) rather than relying on an accident of another package's own
  dependency tree.
- `go:install-ci` (`tasks/go.yaml`) already establishes the local pattern of a CI-only install task
  that installs deterministically rather than reconciling; `ts` diverging from that pattern is what
  let a lockfile-regeneration failure present as a misleading dependency error instead of a clear
  one.

## Considered Options

- **Split npm majors into their own per-package PRs** (rejected here) — would have contained the
  `typescript` failure to its own PR instead of blocking five healthy majors alongside it, but
  changes ADR 0013's grouping shape for every future npm major, not just this one incompatible
  package; the narrower fix (hold the one package) was preferred.
- **`--legacy-peer-deps` or `--force`** (rejected) — resolves the install, but ships a dependency
  tree npm itself flags as potentially broken, papering over a real, currently-unsatisfiable
  compatibility gap rather than reflecting it in the manifest.
- **Do nothing and let the PR sit** (rejected) — the weekly `ts` group would keep regenerating the
  same unsatisfiable `typescript@7` proposal (or an even newer one) every run, blocking the other
  five majors indefinitely without ever going green.
- **`allowedVersions: "<7"` on `typescript` alone, plus declare `@eslint/js` and switch CI to
  `npm ci`** (chosen) — unblocks exactly the one incompatible package, fixes the latent dependency
  gap eslint 10 exposes, and makes the next lockfile-desync failure legible instead of misleading.

## Decision Outcome

1. **`renovate.json`**: add one `packageRule` scoped to `matchPackageNames: ["typescript"]` with
   `allowedVersions: "<7"`, following the targeted-hold precedent of
   [ADR 0015](0015-restrict-argo-cd-go-module-updates-to-major-only.md)'s `argo-cd` rule.
   `allowedVersions` rather than `enabled: false` so 5.x/6.x updates keep flowing normally and only
   the unsatisfiable major jump is suppressed.
2. **`web/package.json`**: add `"@eslint/js": "9.39.5"` to `devDependencies` — the exact version
   already resolved in `web/package-lock.json` today, so a no-op for the current tree that makes
   the `eslint.config.js` import honest and lets it ride the `ts` group to 10.x alongside `eslint`
   itself in the regenerated PR.
3. **`tasks/ts.yaml`**: add an `install-ci` task (`npm ci`, mirroring `go:install-ci`'s naming).
   **`.github/workflows/ci.yaml`**: switch the `ts` and `e2e` jobs' frontend-install steps to
   `task ts:install-ci`. `task ts:install` (`npm install`) is unchanged for local use.
4. Close PR #221 rather than rebase it — the config change removes `typescript` from the batch
   entirely, so the next scheduled run proposes a different upgrade set, not a corrected version of
   the same one.

Verified end-to-end in a scratch worktree with every #221 bump *except* `typescript`, plus
`@eslint/js@10.0.1`: clean `npm install`, `svelte-check` (1638 files, 0 errors), `eslint`/`prettier`
lint, all 93 vitest unit tests, all 16 mocked Playwright e2e tests, and the production build all
pass. See [the implementation plan](../agents/plans/unblock-ts-major-updates.md) for the exact diff.

### Consequences

- Good, because the weekly `ts` group PR is unblocked without changing ADR 0013's grouping shape —
  the other five healthy majors (`eslint`, `vite`, `vite-plugin-svelte`, `flowbite`, `globals`) can
  land on their normal schedule instead of waiting on an unrelated upstream gap.
- Good, because `@eslint/js` becomes a declared, Renovate-tracked dependency instead of an
  undeclared one riding along inside `eslint`'s own dependency tree — the kind of pin AGENTS.md's
  rule exists to catch before it silently breaks on the next major.
- Good, because `npm ci` in CI turns "Renovate failed to regenerate the lockfile" into an immediate,
  legible failure at the install step, instead of a misleading downstream peer-conflict error that
  points at the wrong package.
- Neutral, because this is a point-in-time hold: it has to be revisited once `svelte-check` ships
  TypeScript 7 support, tracked only by the rule's own `description` field — there's no dashboard or
  reminder that surfaces it automatically.
- Bad, because TypeScript 5.x/6.x security or bugfix releases still update normally, but the repo
  will not learn about TypeScript 7 becoming compatible until someone notices upstream or happens
  to revisit this rule.

## Pros and Cons of the Options

### Split npm majors into their own per-package PRs

- Good, because one incompatible package would only ever block its own PR, never its batch-mates.
- Bad, because it reopens ADR 0013's grouping decision for every future npm major, not just this
  one incompatible package — a broader change than this specific, resolvable conflict calls for.

### `--legacy-peer-deps` / `--force`

- Good, because it's a one-line install-time flag with no manifest changes.
- Bad, because it accepts a tree npm itself calls "potentially broken" rather than reflecting a real
  compatibility gap; `svelte-check` genuinely doesn't parse/typecheck against TypeScript 7 yet.

### Do nothing

- Good, because it requires no change and waits for upstream to catch up on its own.
- Bad, because the weekly `ts` group keeps regenerating the same unsatisfiable proposal indefinitely,
  blocking every other npm major bundled alongside it.

### Hold `typescript` below 7, declare `@eslint/js`, switch to `npm ci` (chosen)

See Decision Outcome.

## More Information

- Blocked PR: [#221](https://github.com/ivanklee86/tangle/pull/221)
- Implementation plan: [Unblock ts major updates](../agents/plans/unblock-ts-major-updates.md)
- [ADR 0013](0013-renovate-weekly-grouped-updates.md) (establishes the `ts` group and the
  major-updates-stay-manual rule this decision works within, not against)
- [ADR 0015](0015-restrict-argo-cd-go-module-updates-to-major-only.md) (the targeted-hold
  `packageRule` precedent this decision follows for `typescript`)
- Current-state reference: [docs/agents/ci.md](../agents/ci.md)
- Superseded by, if adopted later: none — expected to be reverted (the `allowedVersions` rule
  removed) once `svelte-check` supports TypeScript 7, not superseded by a different approach.
