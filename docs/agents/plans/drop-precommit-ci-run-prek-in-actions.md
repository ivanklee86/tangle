# Drop pre-commit.ci, run prek in CI

Status: done · 2026-09-20

Remove the hosted [pre-commit.ci](https://pre-commit.ci) integration (badge, `ci:` config key) and
replace it with a new, always-run `pre-commit` job in `.github/workflows/ci.yaml` that runs the
same hooks via [`j178/prek-action`](https://github.com/j178/prek-action) — the tool already used
for this locally since [ADR 0012](../../adrs/0012-pre-commit-ts-and-markdown-checks.md). See
[ADR 0014](../../adrs/0014-drop-precommit-ci-run-prek-in-actions.md) for the decision record and
options considered.

Land the three workstreams below in order; each is independently useful and independently
testable before moving to the next.

## 1. Remove pre-commit.ci

**`README.md`** — drop the first badge from the badge line (leaves `CI` and `Coverage`):

```diff
- [![pre-commit.ci status](https://results.pre-commit.ci/badge/github/ivanklee86/tangle/main.svg)](https://results.pre-commit.ci/latest/github/ivanklee86/tangle/main) [![CI](https://github.com/ivanklee86/tangle/actions/workflows/ci.yaml/badge.svg)](https://github.com/ivanklee86/tangle/actions/workflows/ci.yaml) [![Coverage](https://raw.githubusercontent.com/ivanklee86/octocov-central/main/badges/ivanklee86/tangle/coverage.svg)](https://octocov.dev/ivanklee86/tangle)
+ [![CI](https://github.com/ivanklee86/tangle/actions/workflows/ci.yaml/badge.svg)](https://github.com/ivanklee86/tangle/actions/workflows/ci.yaml) [![Coverage](https://raw.githubusercontent.com/ivanklee86/octocov-central/main/badges/ivanklee86/tangle/coverage.svg)](https://octocov.dev/ivanklee86/tangle)
```

**`.pre-commit-config.yaml`** — drop the trailing `ci:` key (pre-commit.ci-specific, read by
nothing once the app is gone) and rewrite `web-lint`'s comment, which currently justifies itself
in terms of pre-commit.ci's sandbox limitations:

```diff
     - repo: local
       hooks:
-          # `language: system` hooks aren't run by pre-commit.ci (they're
-          # silently skipped there), so this is local, pre-push convenience
-          # only. The actual gate is the `ts` job in
-          # .github/workflows/ci.yaml, which runs `task ts:lint` on every
-          # PR touching web/. Mirroring the flat ESLint config here instead
+          # `language: system` needs web/node_modules installed, which prek
+          # doesn't provision — this stays local, pre-push convenience only.
+          # The `pre-commit` CI job (.github/workflows/ci.yaml) explicitly
+          # skips it (SKIP=web-lint) to avoid duplicating the `ts` job's own
+          # gate (`task ts:lint`), which already runs on every PR touching
+          # web/. Mirroring the flat ESLint config here instead
           # (pre-commit/mirrors-eslint) would mean hand-duplicating every
           # plugin version from web/package.json into additional_dependencies
           # and keeping the two in sync by hand.
           - id: web-lint
             name: web lint (prettier + eslint)
             entry: bash -c 'cd web && npm run lint'
             language: system
             files: ^web/.*\.(ts|js|svelte|css|json)$
             pass_filenames: false
 exclude: |
     (?x)(
         ^integration/kubernetes/example/manifests/1/templates/.*|
         ^integration/kubernetes/example/manifests/3/templates/.*|
         ^charts/tangle/templates/.*
     )
-ci:
-    skip: [web-lint]
```

**Manual, non-file step**: uninstall/disable the pre-commit.ci GitHub App on
`ivanklee86/tangle` (via `https://github.com/apps/pre-commit-ci` → Configure, or the repo's
Settings → Integrations → Installed GitHub Apps). Do this *after* workstream 2 lands and the new
`pre-commit` CI job is confirmed working — otherwise there's a window with no hook enforcement in
CI at all.

**Steps**

1. Make the two file edits above.
2. Confirm `README.md` still renders correctly (badge line still valid Markdown/image syntax).

**Rollback**: revert both file edits; re-add the pre-commit.ci GitHub App if it was already
uninstalled.

## 2. Add the `pre-commit` job to `ci.yaml`

Insert a new job (position doesn't affect scheduling — GitHub Actions runs jobs without `needs` in
parallel — but place it after `docs` and before `e2e` for readability, grouping it with `e2e` as
the two jobs that always run regardless of the path filter):

```yaml
  pre-commit:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
    - name: Install Go
      # go-fmt (dnephin/pre-commit-golang) is `language: script`, so prek
      # doesn't provision a Go toolchain for it — it just runs `gofmt`
      # against whatever's on PATH. Pin explicitly rather than relying on
      # the runner image's ambient Go version.
      uses: actions/setup-go@v5
      with:
        go-version: 1.27
        cache: true   # default; explicit for clarity
    - name: Run pre-commit hooks via prek
      # web-lint stays skipped here, same as it always was under
      # pre-commit.ci's `ci: skip:` — see .pre-commit-config.yaml's comment.
      uses: j178/prek-action@4e14d07f9231acabce116ccfca13b13dd9755ece # v3.0.0
      env:
        SKIP: web-lint
      with:
        prek-version: 0.5.3
```

**Why each piece is shaped this way**

- No `needs: [filter]`/`if:` — most of these hooks (`trailing-whitespace`, `end-of-file-fixer`,
  `check-yaml`, `markdownlint-cli2`, `renovate-config-validator`) are repo-wide with no single
  ecosystem path to filter on, so this job always runs, like `e2e`.
- Only `actions/setup-go@v5` is needed, not `actions/setup-node@v4`/`arduino/setup-task@v2`/
  `task ts:install` — those would only be needed to make `web-lint` runnable, and `web-lint` is
  deliberately skipped (see below), so this job stays lean: checkout + Go + prek, nothing else.
- `SKIP: web-lint` (prek/pre-commit's standard env-var hook-skip mechanism) reproduces exactly
  what `ci: skip: [web-lint]` did under pre-commit.ci — `web-lint` needs `web/node_modules`
  installed (`language: system`, prek doesn't manage that environment), and the `ts` job's
  `task ts:lint` step already gates it for real on every PR touching `web/`. Running it here too
  would just duplicate that check at the cost of a Node/npm-install setup this job doesn't
  otherwise need. **Verify this assumption** (workstream 3, step 2) — `prek`'s `SKIP` handling is
  expected to match classic `pre-commit`'s, but confirm it actually skips the hook rather than
  erroring on missing `node_modules` before relying on it.
- `j178/prek-action` pinned to the exact commit `v3.0.0` resolves to, not the mutable tag — this
  step runs with the workflow's default `pull-requests: write`/`checks: write` permissions, so a
  repointed tag could run unexpected code with those scopes. Same rationale, same pattern, as the
  `report` job's `k1LoW/octocov-action` pin.
- `prek-version: 0.5.3` pinned explicitly (the action's own default is `latest`) — matches the
  version already installed locally via `.devcontainer`'s `postCreateCommand` as of 2026-09-20
  (checked via `gh api repos/j178/prek/releases/latest` at plan-review time), so CI and local runs
  use the identical `prek` build.
- No manual cache step — `j178/prek-action`'s `cache` input defaults to `true` and handles caching
  the installed hook environments internally, unlike this repo's other hand-rolled
  `actions/cache@v6` steps.

**Steps**

1. Add the job to `ci.yaml`.
2. Push a branch with no other changes and confirm the `pre-commit` check appears and passes.
3. Push a branch with a deliberately unformatted `.go` file (skip `task go:fmt`) and confirm the
   `pre-commit` job fails on `go-fmt` — then fix it and confirm it passes.
4. Push a branch with a deliberately malformed `web/` file (e.g. inconsistent quotes ESLint/
   Prettier would flag) and confirm the `pre-commit` job does **not** fail on it (proving
   `SKIP=web-lint` worked) while the `ts` job's `task ts:lint` step does fail on it — confirming
   the "no coverage gap" claim above rather than just assuming it.

**Rollback**: remove the `pre-commit` job from `ci.yaml`; nothing else in this plan depends on it
except workstream 1's decision to drop pre-commit.ci, which should be rolled back together (don't
end up with neither pre-commit.ci nor this job running the hooks in CI).

## 3. Amend ADR 0012, verify `SKIP` behavior locally

**`docs/adrs/0012-pre-commit-ts-and-markdown-checks.md`** — update the "Superseded by" line:

```diff
- Superseded by, if adopted later: none.
+ Superseded by, if adopted later: [ADR 0014](0014-drop-precommit-ci-run-prek-in-actions.md) (the
+ `language: system` vs. `language: node`/pre-commit.ci distinction this ADR's `web-lint` design
+ leans on no longer applies once pre-commit.ci is dropped — the split itself still stands, just
+ for a different reason: avoiding duplication with the `ts` job, not a sandbox limitation).
```

**Steps**

1. Make the ADR edit.
2. Locally, `SKIP=web-lint prek run --all-files` — confirm `web-lint` is reported as skipped, not
   run, and every other hook still runs normally. This is the same command the CI job effectively
   runs; verifying it locally first is faster than iterating via pushed branches.
3. `prek run --all-files` (no `SKIP`) — confirm `web-lint` still runs locally as before (this
   workstream doesn't change local behavior, only CI and pre-commit.ci).

**Rollback**: revert the ADR edit; independent of workstreams 1–2.
