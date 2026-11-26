# Cache the Task binary in CI

Status: proposed · 2026-09-20

Stop CI failing with `API rate limit exceeded` at the `Install Task` step, and stop re-downloading
the Task CLI in every job, by moving all six `arduino/setup-task` call sites behind one local
composite action that pins the version and caches the binary. See
[ADR 0018](../../adrs/0018-pin-and-cache-the-task-cli-in-ci.md) for the decision record.

## Background: which request is actually failing

`arduino/setup-task`'s default `version` is the range `3.x`. Its
[`computeVersion()`](https://github.com/arduino/setup-task/blob/main/src/installer.ts) resolves a
range by listing every tag in `go-task/task` via `api.github.com`, anonymously unless `repo-token`
is set — 60 requests/hour shared across every job on the runner's public IP. That is the request
in the failure, not the binary download:

```text
##[group]Run arduino/setup-task@v3.0.0
with:
  version: 3.x
##[error]API rate limit exceeded for 4.236.173.160. (But here's the good news: Authenticated
requests get a higher rate limit. Check out the documentation for more details.)
```

An exact semver pin skips that lookup entirely — `computeVersion()` returns `v${version}` as soon
as `semver.valid(version)` passes, before `fetchVersions()` is reached. So the pin is what fixes
the failure; the cache is what makes the install cheap.

## 1. Add the composite action

**`.github/actions/setup-task/action.yml`** (new) — one place owning the version, the cache key,
and the reasoning:

```yaml
name: 'Setup Task'
description: 'Install a pinned Task CLI, restored from the Actions cache instead of re-downloaded on every job.'

inputs:
  version:
    description: 'Exact Task version to install, without the leading `v`.'
    required: false
    default: '3.53.1' # github-releases/go-task/task
  repo-token:
    description: 'Token used to authenticate the go-task/task tag lookup.'
    required: false
    default: ${{ github.token }}

runs:
  using: 'composite'
  steps:
    - name: Restore cached Task binary
      uses: actions/cache@v6
      with:
        path: ${{ runner.tool_cache }}/task
        key: task-${{ runner.os }}-${{ runner.arch }}-${{ inputs.version }}
    - name: Install Task
      uses: arduino/setup-task@v3.0.0
      with:
        version: ${{ inputs.version }}
        repo-token: ${{ inputs.repo-token }}
```

**Why `${{ runner.tool_cache }}/task` and not a directory of our own**: `arduino/setup-task`
installs through `@actions/tool-cache`, which writes `<tool cache>/task/<version>/<arch>/task`
plus a *sibling* `<arch>.complete` marker file that its own `tc.find()` checks for. Caching the
whole `task` directory restores the binary and the marker together, so the unmodified upstream
action finds it and does nothing but add it to PATH — no need to reimplement its PATH handling
here. Cache the versioned subdirectory alone and the marker is left behind, making every restore
look like a miss.

**Why `repo-token` when the version is pinned**: it is unused while the pin holds (a valid semver
never reaches the tag listing). It is there for the day the pin is loosened back to a range, so
that lookup is an authenticated request against this repo's own 5,000/hour budget rather than the
shared anonymous one that failed here.

## 2. Point every workflow at it

Six call sites, all currently `uses: arduino/setup-task@v3.0.0`, all in jobs that already check
out the repo first (a local action cannot resolve before checkout):

| File | Job | Line |
| --- | --- | --- |
| `.github/workflows/ci.yaml` | `go` | 47 |
| `.github/workflows/ci.yaml` | `ts` | 100 |
| `.github/workflows/ci.yaml` | `docs` | 152 |
| `.github/workflows/ci.yaml` | `e2e` | 206 |
| `.github/workflows/release-docs.yaml` | `publish` | 21 |
| `.github/workflows/release.yaml` | `goreleaser` | 55 |

```diff
     - name: Install Task
-      uses: arduino/setup-task@v3.0.0
+      uses: ./.github/actions/setup-task
```

The `pre-commit` job is untouched — it runs no `task` command and never installed Task.

## 3. Keep the pin fresh in `renovate.json`

A pinned version that nothing tracks is a stale version. Two additions:

```diff
     {
       "matchDatasources": ["github-releases"],
       "groupName": "k8s"
     },
+    {
+      "matchDatasources": ["github-releases"],
+      "matchPackageNames": ["go-task/task"],
+      "groupName": "ci"
+    },
```

```diff
+    {
+      "customType": "regex",
+      "managerFilePatterns": ["/^\\.github/actions/setup-task/action\\.yml$/"],
+      "matchStrings": ["default: '(?<currentValue>.*?)' # (?<datasource>.*?)/(?<depName>.*?)\\s"],
+      "extractVersionTemplate": "^v?(?<version>.+)$"
+    },
```

The custom manager follows the same `# <datasource>/<depName>` trailing-comment convention as the
existing `K3D_VERSION`/`ARGOCD_VERSION` manager. `extractVersionTemplate` strips the `v` from
upstream's `v3.53.1` tags so the replacement matches how the pin is written (`3.53.1`, the form
`arduino/setup-task` wants). The package rule is needed because `renovate.json`'s bare
`github-releases` → "k8s" rule would otherwise file Task bumps in the Kubernetes group; a later
rule wins on `groupName`, so this one has to sit after it.

Renovate's `github-actions` manager already covers `.github/actions/**/action.yml`, so the
`arduino/setup-task@v3.0.0` and `actions/cache@v6` references inside the composite action keep
getting updated in the "ci" group with no extra configuration.

## 4. Update `docs/agents/ci.md`

- In "Things worth revisiting", extend the resolved caching bullet to include the Task CLI, noting
  it was the one dependency workstream 12 missed and that it is now pinned as well as cached.
- Note in the `e2e` walkthrough's install step that Task comes from the local composite action
  rather than `arduino/setup-task` directly.

## 5. Validate

1. `python3 -c "import yaml; yaml.safe_load(open(f))"` over the three workflows and the new action
   — the composite action's `runs.using: composite` and nested `uses:` steps have to parse before
   anything reaches a runner.
2. Push the branch and confirm every `Install Task` step succeeds. The first run is the
   cache-miss path end to end: `arduino/setup-task` should log
   `Successfully setup Task version v3.53.1` with no API call, and each job's post step should
   report saving `task-Linux-X64-3.53.1`.
3. Confirm concurrent jobs racing to save the same key log the benign
   `Unable to reserve cache ... already exists` warning rather than failing — four `ci.yaml` jobs
   miss simultaneously on that first run, and only one of them wins the save.
4. On the second run, confirm `Cache restored from key: task-Linux-X64-3.53.1` and that the
   install step downloads nothing.
5. Remember the cache is branch-scoped: PR branches get hits from `main`'s entry, so the full
   benefit only shows up once this has run on `main`.

## Rollback

Revert the six `uses:` lines to `arduino/setup-task@v3.0.0`, delete
`.github/actions/setup-task/`, and drop the two `renovate.json` additions. Nothing else in CI
depends on the composite action, and no workflow behavior changes beyond how Task gets onto PATH —
but note the rollback restores the rate-limit failure this plan exists to fix, so the narrower
rollback is to keep the action and change only its pinned `version` default.
