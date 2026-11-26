# Automate the `tangle-deployments` chart bump

Status: done · 2026-09-20

Implements [ADR 0021](../adrs/0021-automate-tangle-deployments-chart-version-bump.md): publishing
a release on `ivanklee86/tangle` should automatically bump `charts/tangle/Chart.yaml`'s
`appVersion` (and patch-bump its own chart `version`) in `ivanklee86/tangle-deployments`, land that
as a PR, and cut a real chart release once it merges — with no manual edit or manually-cut release
in either repository. Verified end to end on a real release (`v0.2.0` → chart `tangle-0.0.13`,
`gh-pages` index updated) with no manual intervention required.

This plan spans two repositories. Each workstream says which one it touches. Workstreams 1–3 were
landed roughly as originally written; workstream 2's workflow and workstream 5's PAT section below
reflect the *final*, working shape, not the first draft — three real issues only surfaced once the
chain was actually exercised with a live dispatch, each documented inline where it changed the
design. See ADR 0021's "Found during a live test" section for the narrative version.

## 1. Add PR-time CI to `tangle-deployments`

**Repo: `tangle-deployments`.** It has no CI today — only `.pre-commit-config.yaml` (whitespace/
EOF/YAML checks) and the release workflow. `Taskfile.yaml` already defines `template:default`
(`helm template` + `kubeconform`) and `tests` (the same, matrixed over `examples/*.yaml`), but
nothing runs them on a pull request. Workstream 2's auto-merged bump PR needs a real check to gate
on; add it first so it exists before anything depends on it.

**New file: `.github/workflows/ci.yaml`**

```yaml
name: CI

on:
  pull_request:

permissions:
  contents: read

jobs:
  chart:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7

      - name: Set up Helm
        uses: Azure/setup-helm@v5.0.1
        with:
          version: "v4.3.0"

      - name: Install kubeconform
        run: |
          curl -sSL https://github.com/yannh/kubeconform/releases/download/v0.8.0/kubeconform-linux-amd64.tar.gz \
            | tar -xz -C /usr/local/bin kubeconform

      - name: Install Task
        uses: arduino/setup-task@v3.0.0
        with:
          version: "3.53.1"
          repo-token: ${{ secrets.GITHUB_TOKEN }}

      - run: task template:default
      - run: task tests
```

`Task`/`helm`/`kubeconform` versions are pinned per `tangle`'s own `AGENTS.md` convention (already
mirrored into this file); `v4.3.0`/`v0.8.0`/`3.53.1` were each repo's latest release as of
2026-09-20, checked via `gh api repos/<owner>/<repo>/releases/latest`. `tangle-deployments`
doesn't yet have `tangle`'s local `./.github/actions/setup-task` composite action or
`renovate.json` custom-manager wiring for these pins — out of scope here; either port that pattern
over in a follow-up or accept plain Renovate tracking of `Azure/setup-helm`'s own `version:` input
(Renovate's `github-actions` manager already tracks the action's `@v5.0.1` pin, but not the nested
Helm version string) and the `kubeconform` URL (needs a `customManagers` regex entry, same shape as
`tangle`'s `K3D_VERSION` one, if this repo wants it tracked rather than manually bumped).

**Steps**

1. Add the file above.
2. Open a throwaway PR (e.g. a no-op whitespace change) to confirm the `chart` check runs and
   passes.
3. Add branch protection on `main` requiring the `chart` check (see workstream 6) — **not
   optional**: without it, `gh pr merge --auto` in workstream 2 has no required check to wait for
   and merges almost immediately regardless of CI outcome, as found live (a test PR merged in ~2
   seconds, before its `chart` job had even started).

**Rollback**: delete the file; `tangle-deployments` goes back to having no PR-time CI.

## 2. Add the chart-bump receiver workflow

**Repo: `tangle-deployments`.** Reacts to the `repository_dispatch` fired by workstream 4, bumps
`Chart.yaml`, and opens (or updates) a PR.

**New file: `.github/workflows/bump-chart-version.yaml`**

> Since revised in place — the workflow also bumps `values.yaml`'s `image.tag`, and the chart
> `version` bump is minor rather than patch. See ADR 0021's "Revised: `image.tag` and minor bumps";
> the snippet below is the plan as originally written.

```yaml
name: Bump chart version

on:
  repository_dispatch:
    types: [tangle-release]

permissions:
  contents: write
  pull-requests: write

concurrency:
  group: bump-chart-version
  cancel-in-progress: false

jobs:
  bump:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7

      - name: Install yq
        run: |
          curl -sSL -o /usr/local/bin/yq \
            https://github.com/mikefarah/yq/releases/download/v4.53.6/yq_linux_amd64
          chmod +x /usr/local/bin/yq

      - name: Bump Chart.yaml
        id: bump
        run: |
          RAW_VERSION="${{ github.event.client_payload.version }}"
          APP_VERSION="${RAW_VERSION#v}"
          CHART_FILE="charts/tangle/Chart.yaml"

          CURRENT_CHART_VERSION="$(yq '.version' "$CHART_FILE")"
          NEW_CHART_VERSION="$(echo "$CURRENT_CHART_VERSION" | awk -F. -v OFS=. '{$NF+=1; print}')"

          yq -i ".appVersion = \"${APP_VERSION}\"" "$CHART_FILE"
          yq -i ".version = \"${NEW_CHART_VERSION}\"" "$CHART_FILE"

          echo "app_version=${APP_VERSION}" >> "$GITHUB_OUTPUT"
          echo "chart_version=${NEW_CHART_VERSION}" >> "$GITHUB_OUTPUT"

      - name: Open pull request
        id: pr
        uses: peter-evans/create-pull-request@v8.1.1
        with:
          # A PAT, not GITHUB_TOKEN: a PR authored by github-actions[bot] has
          # author_association "CONTRIBUTOR" on this public repo, which GitHub gates behind
          # manual approval before any pull_request-triggered workflow (our "chart" CI
          # check) will run at all. A PAT makes the PR's author the token owner (an
          # OWNER/COLLABORATOR), which isn't gated. (Found live — see below.)
          token: ${{ secrets.CHART_BUMP_PAT }}
          commit-message: "chore: bump chart to appVersion ${{ steps.bump.outputs.app_version }}"
          title: "chore: bump chart to appVersion ${{ steps.bump.outputs.app_version }}"
          body: |
            Automated bump triggered by
            [`tangle` ${{ github.event.client_payload.version }}](https://github.com/ivanklee86/tangle/releases/tag/${{ github.event.client_payload.version }}).

            - `appVersion`: `${{ steps.bump.outputs.app_version }}`
            - `version`: `${{ steps.bump.outputs.chart_version }}`
          branch: chore/bump-chart-appversion
          delete-branch: true
          labels: automated

      - name: Enable auto-merge
        # The same PAT, not GITHUB_TOKEN: GitHub also suppresses the pull_request:closed /
        # push events that would otherwise fire once this merge completes when the merge
        # itself is GITHUB_TOKEN-authenticated (same anti-recursion rule as above). A
        # PAT-authenticated merge behaves like an ordinary user merge and triggers
        # release.yaml's plain `push: main` trigger with no special-casing needed.
        # (Also found live — the first fix attempt tried an explicit workflow_dispatch
        # instead of this and didn't work, for the same underlying reason.)
        if: steps.pr.outputs.pull-request-operation == 'created'
        run: gh pr merge --auto --squash "${{ steps.pr.outputs.pull-request-number }}"
        env:
          GH_TOKEN: ${{ secrets.CHART_BUMP_PAT }}
```

Notes on the pieces:

- A fixed `branch: chore/bump-chart-appversion` (rather than `create-pull-request`'s default of a
  hash-suffixed name) means a second `tangle` release landing before the first bump PR merges
  updates the same PR in place instead of opening a parallel one — `create-pull-request` diffs
  against the existing branch and pushes a new commit only if the working tree actually changed.
- `concurrency.group` prevents two dispatches from racing the same checkout/bump/push sequence;
  `cancel-in-progress: false` so the second run waits and finds the first commit already there
  instead of clobbering it mid-flight.
- The chart-version bump is a plain "increment the last dot-separated segment" — deliberately not
  semver-aware beyond that, matching ADR 0021's decision to only ever patch-bump automatically and
  leave minor/major chart-version calls to the maintainer. If `version` is ever a pre-release
  string (e.g. `0.1.0-rc.1`) this `awk` needs revisiting; today's `Chart.yaml` is a plain
  three-segment version, so this is a known, accepted limitation rather than a hidden one.
- `gh pr merge --auto` (not an immediate merge) queues the merge behind whatever required checks
  exist — workstream 1's `chart` job — so it still respects CI even though no human clicks merge.
  This only actually gates anything once `main` has branch protection requiring `chart` *and* the
  repo's "Allow auto-merge" setting is on — both off by default, both needed (see workstream 1's
  steps and workstream 6).
- **Both steps use `CHART_BUMP_PAT`**, a PAT scoped to `tangle-deployments` itself (see
  workstream 6) — not the workflow's own default `GITHUB_TOKEN`, and not the
  `TANGLE_DEPLOYMENTS_DISPATCH_TOKEN` from workstream 5 (that one lives in `tangle` and is only
  used for the cross-repo dispatch call). The original design assumed `GITHUB_TOKEN` would work for
  this whole job since it's a same-repo commit/PR/merge — it doesn't, for two independent reasons
  documented inline above and in ADR 0021's "Found during a live test."

**Steps**

1. Add the file above.
2. Trigger it manually to test before wiring up the real sender:
   `gh api repos/ivanklee86/tangle-deployments/dispatches -f event_type=tangle-release -f 'client_payload[version]=v0.1.1'`
   (using your own `gh` session is fine for this manual test — only the automated sender in
   workstream 4 needs the scoped dispatch PAT).
3. Confirm a PR opens on `chore/bump-chart-appversion`, authored by a real collaborator (not
   `github-actions[bot]`), with the expected `appVersion`/`version` values; confirm workstream 1's
   `chart` check actually runs (not stuck on `action_required`) and the PR auto-merges only once it
   passes.
4. Send a second manual dispatch with a different version before the first PR merges (or right
   after) to confirm the fixed-branch/concurrency behavior updates rather than duplicates.
5. Confirm `release.yaml` fires immediately after the merge via its ordinary `push: main` trigger
   (no separate workaround needed) and a real chart release / `gh-pages` index update follows.

**Rollback**: delete the file; a dispatch to `tangle-deployments` becomes a no-op (GitHub still
accepts and drops `repository_dispatch` events with no listener, so workstream 4 doesn't need to
be rolled back in lockstep).

## 3. Retarget `release.yaml` to `push: main`

**Repo: `tangle-deployments`.** Today's trigger requires a human to manually cut a GitHub Release
on this repo to invoke `chart-releaser-action` — the workaround this whole plan removes. Switch it
to the action's own documented trigger, so workstream 2's merge is what cuts the release from here
on.

**`.github/workflows/release.yaml`**

```diff
 name: Release Charts

 on:
-  release:
-    types: [published]
+  push:
+    branches: [main]
```

**Steps**

1. Make the diff above.
2. Merge it, then confirm `chart-releaser-action` runs on the merge commit and — since
   `Chart.yaml`'s `version` didn't change in *this* PR — packages nothing (its whole mechanism is
   diffing `version` against what it last saw; a no-version-change push is a correct no-op, not a
   failure).
3. After workstream 2 lands and its first bump PR auto-merges, confirm this workflow fires again
   on that merge commit and this time does publish a new chart release / update the `gh-pages`
   index, closing the loop end to end.

**Rollback**: revert the trigger to `release: types: [published]`; manual release-cutting on
`tangle-deployments` becomes required again.

## 4. Add the dispatch job to `tangle`'s `release.yaml`

**Repo: `tangle`.** The sender half — fires once a `tangle` GitHub Release is published, carrying
the release tag to `tangle-deployments`.

**`.github/workflows/release.yaml`**

```diff
 jobs:
   docker:
     ...
   goreleaser:
     ...
+
+  notify-tangle-deployments:
+    runs-on: ubuntu-latest
+    if: github.event.release.prerelease == false
+    steps:
+      - name: Dispatch chart bump
+        run: |
+          gh api repos/ivanklee86/tangle-deployments/dispatches \
+            -f event_type=tangle-release \
+            -f "client_payload[version]=${{ github.event.release.tag_name }}"
+        env:
+          GH_TOKEN: ${{ secrets.TANGLE_DEPLOYMENTS_DISPATCH_TOKEN }}
```

`if: github.event.release.prerelease == false` keeps draft/pre-release tags from cutting an
automatic chart bump — only a release explicitly published as a full release reaches
`tangle-deployments`. This job has no `needs:` on `docker`/`goreleaser` — it's independent of
whether the Docker image or CLI binary build succeeds, since the chart bump only needs the release
tag name, not either build artifact.

**Steps**

1. Make the diff above (only after workstream 5's token exists and is stored, since this job will
   fail without it).
2. Cut a real (or test) `tangle` release and confirm the dispatch fires — check
   `tangle-deployments`' Actions tab for the resulting `bump-chart-version.yaml` run, or
   `gh api repos/ivanklee86/tangle-deployments/dispatches` response status locally beforehand as a
   dry run of the exact call.
3. Confirm the full chain: `tangle` release → dispatch → bump PR → auto-merge → chart release,
   end to end, on a real (or throwaway pre-1.0) version bump.

**Rollback**: revert the job; releasing `tangle` goes back to not touching
`tangle-deployments` at all.

## 5. Create and store the cross-repo dispatch PAT

**Manual, not scriptable** — do this before workstream 4's diff is merged, since that job depends
on the secret existing. This is the first of two PATs this plan ends up needing (see workstream 6
for the second, found only once the chain was actually tested end to end).

1. On GitHub, create a fine-grained personal access token scoped to **only**
   `ivanklee86/tangle-deployments`, with repository permission **Contents: Read and write** (the
   minimum GitHub documents for creating a `repository_dispatch` event) and no other repository or
   account permissions. Set an explicit expiration (e.g. 1 year), not "No expiration."
2. Add it as a secret named `TANGLE_DEPLOYMENTS_DISPATCH_TOKEN` in `ivanklee86/tangle`'s repo
   settings (Settings → Secrets and variables → Actions) — scoped to that one repo's Actions runs,
   not an organization-wide secret.
3. Put a reminder somewhere durable (calendar, or a note in this doc's "Status" line when it's
   updated) to rotate the token before its expiration date, since an expired token fails
   workstream 4's job silently until someone notices a `tangle` release stopped propagating.

**Rollback**: revoke the token and delete the secret; workstream 4's job starts failing
immediately and visibly (a 401/403 from the `dispatches` API call), which is the correct failure
mode — nothing silently falls back to the old manual process.

## 6. Create the bump-PR PAT, and two repo settings, in `tangle-deployments`

**Manual (the PAT) plus two repo settings** — all three were found necessary only once the chain
was tested with a real dispatch; none of them were part of the original design. Do these before
workstream 2's workflow is exercised for real, since `bump-chart-version.yaml` depends on all
three.

1. **`CHART_BUMP_PAT`**: create a second fine-grained PAT, scoped to **only**
   `ivanklee86/tangle-deployments`, with repository permissions **Contents: Read and write** and
   **Pull requests: Read and write**. Set an explicit expiration, same as workstream 5's token.
   Add it as a secret named `CHART_BUMP_PAT` in `ivanklee86/tangle-deployments`'s own repo settings
   (not `tangle`'s — this one is used entirely within `tangle-deployments`' own workflow). Used for
   both the `create-pull-request` step and the `gh pr merge --auto` step in workstream 2's
   workflow, for the two independent reasons documented there.
2. **Branch protection on `main`**, requiring the `chart` status check
   (`gh api --method PUT repos/ivanklee86/tangle-deployments/branches/main/protection` with
   `required_status_checks: {strict: true, contexts: ["chart"]}`, `enforce_admins: false`) — see
   workstream 1's steps for why this is load-bearing, not optional.
3. **"Allow auto-merge" at the repo level** (`allow_auto_merge`, off by default) —
   `gh api --method PATCH repos/ivanklee86/tangle-deployments -f allow_auto_merge=true`. Without
   it, `gh pr merge --auto` fails outright with `GraphQL: Auto merge is not allowed for this
   repository`, found live on the second test dispatch.

**Rollback**: revoke `CHART_BUMP_PAT` and delete the secret (workstream 2's job starts failing
visibly at the `create-pull-request`/merge step); branch protection and `allow_auto_merge` can each
be reverted independently via the same API calls with the opposite values, though doing so
reopens the "auto-merge doesn't actually wait for CI" gap from workstream 1.

## Deferred: collapse the duplicate version in `values.yaml`

**Resolved 2026-09-21, the other way round — see ADR 0021's "Revised: `image.tag` and minor
bumps".** The check below was run and came back negative: `ghcr.io/ivanklee86/tangle` publishes
v-prefixed tags only, so `appVersion` (`v` stripped) is not a pullable tag and the override cannot
simply be removed. The duplicate stays and `bump-chart-version.yaml` now writes it as well, using
the raw dispatch tag. Leaving the original text below for the reasoning it records.

Not part of this plan (see ADR 0021's "Left out of this decision"). `charts/tangle/values.yaml`'s
`image.tag: "v0.1.0"` duplicates `Chart.yaml`'s `appVersion` — the chart template already falls
back to `.Chart.AppVersion` when `image.tag` is unset, so removing the override would leave a
single source of truth and retire Renovate's separate docker-tag PRs against this file. Before
doing that: confirm `ghcr.io/ivanklee86/tangle` actually publishes a tag matching the *exact*
string this plan puts in `appVersion` (no `v` prefix) — today's `values.yaml` has the `v` prefix
and `Chart.yaml` doesn't, so removing the override without checking which tag format the registry
actually carries could point deployments at a tag that doesn't exist.
