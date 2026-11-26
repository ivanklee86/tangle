# Support multi-source Applications

Status: phase 1 and phase 2 done, decisions recorded in [ADR 0032](../../adrs/0032-choose-multi-source-revisions-by-target-repo.md) and below · 2026-11-26

Make Tangle diff Applications that use `spec.sources` (ArgoCD multi-source) correctly, while every existing caller (CLI, UI, API, single-source apps) keeps working unchanged. This document covers requirements and use cases only. No implementation is planned yet, and the UI pattern was worked out with Claude Design (see [Design](#design-2026-11-26)).

## Current behavior

- **Live ref.** `ListApplicationsByLabels` reads `LiveRevision` only from `spec.source.targetRevision` (`internal/argocd/wrapper.go:125`). A multi-source app reports `liveRef: ""`.
- **Target ref.** `GetManifests` sends `ApplicationManifestQuery.Revision` for both sides (`wrapper.go:178`). For a multi-source app ArgoCD ignores `Revision` and renders every source at its own `targetRevision`. The diff is therefore always empty, whatever the pushed change was. This is a silent false negative, which is the worst failure for a "what will this PR change?" tool.
- **Inputs.** One `targetRef` per run: `--target-ref` on the CLI, `targetRef` in the UI query and `DiffsRequest.targetRef` on the API. Nothing says which repo the ref belongs to.
- **Helm-repo sources.** A single-source app whose source is a Helm repo chart gets the branch name as a chart version, and manifest generation fails (`helm-repo` fixture). The CLI counts that as a failure.

What ArgoCD needs (confirmed by `TestE2E_SourceFixtures`): `Revisions[i]` + `SourcePositions[i]` (1-based) for each source to override; a source left out renders at its own `targetRevision`. Rendering a multi-source app with explicit `Revisions` also differs from its default render in ways the change didn't cause: Helm test-hook Pods appear, `namespace` is dropped and resources reorder. So both sides of a diff must be rendered the same way.

## Goals

1. A multi-source app shows exactly the change the pushed ref makes to the sources that ref belongs to, and nothing else.
2. A caller that changes nothing gets the same results as today for single-source apps: same requests, same response fields, same CLI files and exit codes.
3. When Tangle can't tell which sources a ref applies to, it says so instead of returning an empty diff.
4. Which revision went to which source position is visible in logs, traces and the response, so a surprising diff can be explained.

Non-goals: source hydrator, ApplicationSets (Tangle only sees the generated Applications), and different refs for different repos in one run (deferred, see [Deferred](#deferred)).

## Core idea: the ref belongs to a repo

A git ref only means something in one repo. Tangle needs one or more **target repos** alongside the **target ref**, and applies the ref to every source whose `repoURL` matches any of them. Every other source stays at its own `targetRevision`. More than one target repo covers a change made under the same branch name in several repos (D6).

The target ref can be a branch, a tag or a full commit SHA (D7). Tangle passes it to ArgoCD unchanged, so the rules for choosing source positions are the same whatever kind of ref it is.

- **Explicit:** the caller passes them (`--target-repo`, repeatable, on the CLI; `targetRepo`, repeatable, in the UI query; `targetRepos` on the API). In CI the current repo is usually already known (`GITHUB_REPOSITORY`, `CI_PROJECT_URL`).
- **Inferred (opt-in, D1):** with `--infer-target-repo` (`inferTargetRepo` in the UI and API), when no target repo is passed and every git source of an app is in the same repo, that repo is the target. This covers `helm-repo-git-values` and `git-chart-git-values`.
- **Unresolved:** a multi-source app with no target repo, where inference is off or the app's git sources span more than one repo (`two-repos`). The app gets the new status "needs target repo", not an empty diff.

Matching has to normalize `repoURL`: scheme (`https://` vs `ssh://` vs `git@host:`), the `.git` suffix, a trailing slash and case of the host. Helm and OCI sources never match a git target repo.

## Use cases

Each row is something a user does and what they should see. "Today" is the current result.

### Single source (backwards compatibility)

| # | Use case | Expected | Today |
| --- | --- | --- | --- |
| S1 | Git source, no target repo given | Same as today: `Revision` = target ref | Same |
| S2 | Git source, target repo given and matching | Same as S1 | n/a |
| S3 | Git source, target repo given, a different repo | App is "not affected": no render, empty diff, not counted as an error | Renders, diff likely empty or the branch is missing |
| S4 | Helm repo chart, no target repo | Unchanged: error (D3) | Error: branch is not a chart version |
| S4b | Helm repo chart, target repo given | "Not affected" (D3) | Error |
| S5 | CMP source (Tanka) | Same as S1: CMPs take a revision like any git source | Works |

### Multi source

| # | Use case | Fixture | Expected |
| --- | --- | --- | --- |
| M1 | Helm-repo chart + values `ref` from git, change in values, target repo given or inferred | `helm-repo-git-values` | Ref goes to position 2 only. Diff shows the values change |
| M1b | As M1, no target repo and inference off | `helm-repo-git-values` | "Needs target repo" (D1) |
| M2 | Git chart + `ref` source with its own `path`, same repo | `git-chart-git-values` | Ref goes to positions 1 and 2. Diff shows changes from both |
| M3 | Two repos, change in one, target repo given | `two-repos` | Ref goes only to the matching position. Diff shows that change |
| M4 | Two repos, no target repo | `two-repos` | "Needs target repo", with the candidate repos listed |
| M5 | Target repo given, matches none of the app's sources | any | "Not affected", as in S3 |
| M6 | Same repo in two sources at different `targetRevision`s (say `main` and `v1.2`) | `same-repo-pinned` (new) | Ref goes to both (D4) |
| M7 | Target ref doesn't exist in the matched repo | any, with a missing ref | Manifest generation error naming the repo and ref, as for single source today |
| M8 | One source is a CMP | `multi-source-cmp` (new) | Same as M1 to M3. CMPs accept a revision per position |
| M9 | Three repos, change in one, target repo given | `three-repos` (new) | Ref goes only to the matching position (3). The other two stay pinned, and the diff shows only that change |
| M10 | Three repos, no target repo (inference on or off) | `three-repos` (new) | "Needs target repo", listing all three candidate repos in source order |
| M11 | Three repos, target repo is one with no such ref | `three-repos` (new) | Manifest generation error naming that repo and ref (M7). The error isn't hidden by the other two sources rendering fine |
| M12 | Several target repos, only one used by the app | `two-repos`, target repos = the manifests repo and `ivanklee86/tangle` | Same as M3: an extra repo the app doesn't use changes nothing |
| M13 | Several target repos, more than one used by the app | `three-repos`, target repos = the manifests repo and `ivanklee86/tangle` | The ref goes to positions 2 and 3. `tangle` has no `test_gitops`, so the result is the M11 error naming `tangle`, which shows that both positions got the ref. The successful case (the same branch existing in two repos) is unit-tested only, because no second repo we control has a change branch |

### Ref kinds

The target ref can be a branch, a tag or a commit SHA (D7). These cases hold the result fixed and change only how the ref is spelled.

| # | Use case | Fixture | Expected |
| --- | --- | --- | --- |
| R1 | Single source, target ref is a tag on `test_gitops`'s commit | `test-1`, tag `fixture-target` (new) | Same diff as `test_gitops`: one added `Ingress` |
| R2 | Single source, target ref is the full SHA of `test_gitops`'s commit | `test-1` | Same as R1 |
| R3 | Multi source, target ref is a tag, target repo given | `two-repos`, `fixture-target` | Same as M3 |
| R4 | Multi source, target ref is the full SHA, target repo given | `two-repos` | Same as M3 |
| R5 | Target ref is an abbreviated SHA | `test-1` | Whatever ArgoCD does with it, Tangle reports it: the same diff, or a manifest generation error naming the ref. Never an empty diff. The live test records which, so a change in ArgoCD's behavior shows up |
| R6 | Target ref is a SHA that isn't in the matched repo | `two-repos`, a SHA from `ivanklee86/tangle` | Manifest generation error naming the repo and ref, as in M7 |

### Live side

| # | Use case | Expected |
| --- | --- | --- |
| L1 | Multi-source app on the applications page | Shows each source's repo, path or chart, and `targetRevision`, not a blank live ref |
| L2 | Diff of a multi-source app with no real change | Empty diff. Both sides are rendered with explicit `Revisions` (the live side pinned to each source's own `targetRevision`), so render artifacts cancel out |
| L3 | Live ref for a multi-source app in the CLI and API | `liveRef` stays a string for old callers. Per-source detail goes in a new field |

### Filtering

| # | Use case | Expected |
| --- | --- | --- |
| F1 | CI wants to diff only the apps that read from its repo | Opt-in: with a target repo, apps marked "not affected" can be hidden (UI) or left out of the output files (CLI flag) |
| F2 | Existing label filters | Unchanged. Repo filtering is applied after labels |

## Compatibility requirements

- **API.** Add fields; never rename, remove or change the meaning of one. `DiffsRequest` gains an optional `targetRepos` list (D6). Without it a request behaves as today for single-source apps. An empty list means the same as leaving it out. `DiffsResponse` gains an optional `status` (`ok`, `notAffected`, `needsTargetRepo`) and per-source detail (position, repo, path or chart, live revision, target revision used). `ApplicationLinks` gains `sources`, and `liveRef` stays.
- **The one deliberate behavior change** is for multi-source apps with no target repo. Today they return a false empty diff. Now they return "needs target repo", which fails a CLI run under `--fail-on-errors` (D1, D2). This is a bug fix, but a CI pipeline that passed will fail until it passes `--target-repo` or `--infer-target-repo`. The release notes must say so and give the one-line fix.
- **CLI.** `--target-ref` keeps working on its own. New `--target-repo` flag, repeatable (D6). File names stay `diff-<argocd>-<app>.yaml`. New `--infer-target-repo` flag (D1). `--fail-on-errors` doesn't count "not affected" as a failure, and does count "needs target repo" (D2).
- **UI.** Existing URLs (`?targetRef=…&labels=…`) still load and show the same result for single-source apps. `targetRepo` (repeatable, D6) and `inferTargetRepo` (D5) are new optional query parameters, so shared links stay stable and reproduce the same result.
- **`pkg/client`.** Existing functions keep their signatures. Add a variant or an options field for the target repo.

## Observability

- Log and trace attributes per diff: `source.count`, the resolved target repos, whether they were explicit or inferred, and the `positions` and `revisions` sent to ArgoCD for each side.
- Metric: count of diffs by `status` (`ok`, `notAffected`, `needsTargetRepo`, `error`), so a rise in ambiguous apps shows up.
- The response's per-source detail is also the user-facing explanation of what was compared.

## Test cases to plan for

Unit tests, with fakes:

- Repo URL normalization: https vs ssh vs scp-style, `.git`, trailing slash, host case. Non-git sources never match.
- Position selection: single source, M1, M1b, M2, M3, M4, M5, M6 and M9 to M13, with inference on and off. S4 and S4b for Helm-only apps.
- Several target repos: two repos both used by the app (the successful M13 case, with a fake that has the branch in both), duplicate entries, and two spellings of one repo collapsing to one entry after normalization.
- Ref kinds: a branch, a tag, a full SHA and an abbreviated SHA reach ArgoCD byte-for-byte unchanged, on both the single-source and multi-source paths.
- Both sides use `Revisions`/`SourcePositions` for multi-source apps. Single source still sends `Revision`, so the S1 request is identical to today.
- Handler: a request without `targetRepos` decodes and behaves as before. New response fields are omitted or empty for single source, if we choose `omitempty`.
- CLI: a repeated `--target-repo` and `--infer-target-repo` are passed through, the exit code for each status, and `--target-ref` alone is unchanged for single-source apps.
- UI: a repeated `targetRepo` round-trips through the URL, in order. Old URLs parse. Each status renders its state.

Live e2e: un-skip the multi-source cases in `TestE2E_ServerSourceFixtures` and `web/e2e/live/sources.spec.ts` and tighten them to exact matches. Add the live cases, the R1 to R6 ref-kind cases and the new fixtures listed in [Fixture changes](#fixture-changes). The CLI's live suite (`cmd/tangle-cli/main_e2e_test.go`) gets R1 and R2 too, since a SHA is what CI is most likely to pass.

## Fixture changes

The fixtures span two repos. Manifest content lives in [`tangle-integration-manifests`](https://github.com/ivanklee86/tangle-integration-manifests), on its `main` and `test_gitops` branches. The Application CRs live in this repo under `integration/kubernetes/sources/`, in AppProject `sources`. Existing fixtures already cover M1 to M5 and L2, so most use cases need no new content.

### Covered by existing fixtures

| Use case | How |
| --- | --- |
| M1, M1b | `helm-repo-git-values`, with a target repo given or inferred, and with neither |
| M2 | `git-chart-git-values` |
| M3, M4 | `two-repos`, with and without `--target-repo` |
| M5, S3, S4b | Any fixture, with a target repo none of its sources use (say `stefanprodan/podinfo` for `tanka`) |
| M7 | Any multi-source fixture, with a target ref that doesn't exist in the matched repo (say `does-not-exist`) |
| L2 | Any multi-source fixture, with `main` as the target ref: both sides are rendered at the same content with explicit `Revisions`, so the diff must be empty. `helm-repo-git-values` matters most, because the podinfo chart has the Helm test hooks that show up in explicit renders |
| S4 | `helm-repo`, with no target repo (today's error, unchanged) |

### New content in `tangle-integration-manifests`

1. **`same-repo-pinned/` for M6** (same repo, different `targetRevision`s). There are two plain-ConfigMap directories, `same-repo-pinned/branch/` and `same-repo-pinned/tag/`, and a new tag `fixture-pinned-v1` on the `main` commit. On `test_gitops`, both ConfigMaps change. Under D4, a diff with `--target-repo` must show both changes, including the one in the source pinned to the tag. ConfigMaps keep the fixture free of pods, so it can't hit image-architecture problems like the guestbook one.
2. **`multi-source-cmp/` for M8** (a CMP source inside a multi-source app). A small Tanka environment with its own `jsonnetfile.json`, plus a values-style ConfigMap directory in the same repo. On `test_gitops` the jsonnet output changes. It can't reuse `tanka/`: a second Application rendering the same resources in `default` would make ArgoCD report a shared resource and fight over sync.
3. **`three-repos/` for M9 to M11.** One plain ConfigMap directory, whose `greeting` changes on `test_gitops`. It's the third source of the `three-repos` app (see below). The other two sources come from repos with no `test_gitops` branch.
4. **Tag `fixture-target` on `test_gitops`'s commit, for R1 to R6.** No new content: it's a second name for the existing change.
5. **The tags are new long-lived refs.** `fixture-pinned-v1` is on `main` and never moves. `fixture-target` has to move every time `test_gitops` is rebased, because the rebase gives that commit a new SHA. Add both tags to the ruleset that blocks force-pushes and deletion, and add "move `fixture-target` to the new `test_gitops` commit" to the README's rebase steps.
6. **The SHA isn't pinned anywhere.** The live tests resolve it at run time (`git ls-remote` on `refs/heads/test_gitops`) and check that `fixture-target` peels to the same commit, so a missed tag move fails with a clear message instead of a confusing diff.
7. **README updates.** Add the new rows to the source-type table. Fix the existing `two-repos` row, which still describes `argoproj/argocd-example-apps` `guestbook/`. Since `ee82beb` the first source is `stefanprodan/podinfo` `kustomize/` at `6.17.0`.
8. **`test_gitops` stays one commit ahead of `main`.** Add the new content to `main` first, then rebase `test_gitops` and add the changes to its single commit, following the README's "Changing this repo" steps (the ruleset is disabled for the rebase).

### New Application CRs in this repo

- `application-same-repo-pinned.yaml`, `application-multi-source-cmp.yaml` and `application-three-repos.yaml` in `integration/kubernetes/sources/`, labelled `suite: sources` and `source: multi-source`, auto-synced like the other fixtures.
- **`three-repos` sources**, in this order:
  1. `stefanprodan/podinfo` `kustomize/` at tag `6.17.0`, as in `two-repos`. Its image is multi-arch.
  2. `ivanklee86/tangle` `integration/kubernetes/example/manifests/4/` pinned to `309da00`, the last commit before #286 moved the manifests out. It holds one ConfigMap (`example-cm-4`), and this repo no longer has a `test_gitops` branch.
  3. `tangle-integration-manifests` `three-repos/` at `main`.
- **`three-repos` gets its own namespace** (`three-repos`, with `CreateNamespace=true`), because its podinfo Deployment and its ConfigMap would otherwise share names with `two-repos` and `test-4` in `default`. Every other fixture deploys to `default`.
- **Why the tangle repo as the second repo:** it's pinned to a SHA, it has no pods and it's already trusted by the test stack. The other options were a new fixture repo, which is more to maintain, or another third-party repo, which needs a pod-free directory and a pinned ref. A SHA on `main` here survives as long as history isn't rewritten, which the `main` branch ruleset already prevents.
- **URL normalization in the live stack.** One source in one fixture should spell the manifests repo differently from the others, say without the `.git` suffix in `same-repo-pinned`. Then a live test proves that both spellings match one `--target-repo`. ssh and scp-style URLs need credentials in the test stack, so they stay unit-test only.
- Update `SOURCES` and `MULTI_SOURCE` in `web/e2e/live/sources.spec.ts`, the expected lists in `TestE2E_ServerSourceFixtures`, and `TestE2E_SourceFixtures`. Adding apps changes the counts in the `suite:sources` and `source:multi-source` queries.

### Order of changes

The manifests repo is read live from GitHub by every Tangle branch, so new content there must be additive: new directories and new tags, with nothing existing changed except the README. Then it can land before the Tangle PR that adds the CRs, without breaking `main` or open PRs.

## Design brief for Claude Design

Questions for a UI pattern:

1. **Target repo input.** Where does it sit next to the target ref in the query editor? It's optional, usually one repo, sometimes several (D6), usually set once per team, and often inferable. Should it be a field, a multi-value combo with suggestions from the repos the listed apps use, or hidden behind "advanced"?
2. **Sources on the applications page.** A row today shows one live ref. How should it show one to N sources (repo, path or chart, `targetRevision`, `ref` name) without making single-source rows heavier?
3. **Diff detail.** How to show which positions got the target ref and which stayed at live, next to the diff itself.
4. **New statuses in the diff list.** "Not affected" (quiet, possibly hidden by default) and "needs target repo" (actionable: pick the repo here, which re-runs that app or all ambiguous apps).
5. **A list of mixed apps.** Many single-source apps plus a few multi-source ones. The common case must not get noisier.
6. **The inference toggle.** Inference is opt-in (D1) and lives in the URL as `inferTargetRepo` (D5). Where does the switch sit in the query editor, and how does the page show that it's on?

## Design (2026-11-26)

The Claude Design canvas is [Tangle — multi-source Applications](https://claude.ai/artifact/QfXVUKyGpP1Z7no8Eav1RN). It covers all six brief items, plus the "needs target repo" detail and a dark-mode check. Single-source rows and detail panes look as they do today, and every status pairs its colour with a word or an icon.

### Decisions from the design review

The mockups implied three behaviours the decisions below didn't settle. These were accepted on 2026-11-26 and are listed with the other decisions as D8 to D10.

- **D8. Fixing "needs target repo" changes the query, not one app.** The mockup offers "Re-run both apps" and "Re-run this app". Adding a target repo changes the whole query: single-source apps in other repos become "not affected" (S3). So a per-app re-run would need an override that isn't in the URL, which breaks D5. **Decision:** one action, "Add `<repo>` to target repos". It updates the URL and re-runs the query, and the panel says that other apps may change status. "Re-run this app" is dropped.
- **D9. Moved sources say where they moved from, with no "pinned tag" label.** Tangle only sees the `targetRevision` string, so it can't tell the tag `v1.2` from the branch `release-4.2`. **Decision:** every moved source shows "moved from `<live revision>`" (D4 already accepts that pinned sources move). Rejected: guessing from the string (semver or 40-character hex counts as pinned), which is wrong for branches like `v2`; and asking ArgoCD's repository `ListRefs` API, which costs a call per repo and needs a new `repositories, get` RBAC grant.
- **D10. "All" counts what's visible.** In the mockup, "Show not affected (4)" is on but the segment reads "All 8". **Decision:** "Not affected" apps are hidden unless the toggle is on, and the toggle label always carries their count. "All" counts the rows the list shows, so it's 12 with the toggle on and 8 with it off. The query bar's total counts every app the labels matched, with "not affected" as its own count.

### Data the design needs

Beyond the per-source detail already listed under [Compatibility requirements](#compatibility-requirements):

| UI element | Needs | Source |
| --- | --- | --- |
| Source kind badge (Helm, Git) | Whether a source is a chart or a git path | `chart` set on the source. CMP sources show as Git, and OCI charts as Helm. No separate badge for these yet |
| "Used by source 1" on a `ref` source | Which sources read `$<ref>/…` | Parse `helm.valueFiles` (and `helm.fileParameters`) for `$<ref>/` prefixes: a pure function of the spec, unit-tested on its own |
| "2 of 3 moved" | Number of positions that got the target ref | The per-source detail |
| Drawer suggestions ("Used by matching apps · 9 apps") and the "no matching app uses this repo" hint | Each app's source repos for the labels being edited | `ApplicationLinks.sources` (L1). The drawer calls `/applications` once when the field gets focus and caches the result. It never refetches per keystroke, because that call fans out to every ArgoCD (the concern behind ADR 0008) |
| Needs-repo candidates ("live `main` · used by 6 other apps") | Per-source detail plus the application list | Worked out in the browser |
| Not-affected detail ("Uses `org/billing` at `main`") | Per-source detail for an app that wasn't rendered | The diffs response carries `sources` for every status, including `notAffected` and `needsTargetRepo`, not only for rendered diffs |

### Mapping onto existing code

- **Target repos input:** the chip input from `LabelsInput`, plus a keyboard-accessible suggestion list (ARIA combobox and listbox). Flowbite has no chip multi-select, so this is the largest new component. A shared helper shortens `https://host/org/repo.git` to `org/repo`, with the full URL on hover. The query bar, drawer, sources table and candidate list all use it.
- **Infer switch:** Flowbite `Toggle`. The mockup's switch stands in for it.
- **Query bar:** extend the existing summary snippet with "in `<repo>`…" chips, the "repo inferred" badge and a "+N more" overflow from three repos up.
- **Diffs list:** new outcomes in `outcomeDot` and the outcome text: "Not affected" gets a dashed hollow dot and grey text, "Needs repo" a warning triangle. Add a "Needs repo" segment, shown only when its count is above zero, and the "Show not affected" toggle (D10).
- **Detail header:** for multi-source apps, "N sources → `<ref>`" replaces `liveRef → targetRef` (`DiffDetail.svelte:42`).
- **Sources panel:** a new collapsible component above the tabs, rendered only for apps with more than one source.
- **Needs-repo panel:** an Alert with a fieldset of candidate repos as checkboxes, and the single action from D8.
- **Applications table:** an expandable sub-row inside the existing `Table`. Collapsed, the Live ref cell only says "N sources": a bare list of revisions reads as nothing without their sources (decided while reviewing PR 3). The mockup also shortens SHAs, including for single-source rows (`3f9c2a1` with a copy button). That's a small change to today's output, so call it out in the PR.

Ignore what the mockup tool brought in. The design uses Flowbite's blue (`#1A56DB`) as its accent, but the app's `primary` is indigo (`web/src/app.css`), so use the `primary-*` classes, never the hex values. It uses Inter and JetBrains Mono, but the app keeps its current fonts. It adds a "Tangle" wordmark in the query bar, a white left pane and a blue selected row, none of which the brief asked for. Today's styling stays.

### Phases

1. **Core:** the API and CLI changes, the target repos input (no suggestions), the infer switch, the query bar summary, the two new outcomes and the toggle, the sources panel, the needs-repo panel with D8's action, and the applications sub-rows.
2. **Polish:** drawer suggestions and the unused-repo hint, "+N more" in the query bar, "used by N other apps" on candidates, "used by source N" on `ref` sources, and shortened SHAs. Done: shortened SHAs shipped with phase 1 (#292), and the rest in the phase 2 PR. One deviation from the data table above: suggestions come from the applications the page has already loaded, not a fresh `/applications` call when the field gets focus. That spares a fan-out to every ArgoCD, at the cost of suggestions not following label edits made in the drawer.

## Decisions (2026-11-26)

- **D1. Inference is opt-in.** Tangle infers the target repo only with `--infer-target-repo` (`inferTargetRepo` in the UI and API). Without it, a multi-source app with no target repo is "needs target repo". Nothing is guessed silently.
- **D2. "Needs target repo" fails CI.** It counts as a failure under `--fail-on-errors`, like a manifest generation error, so the old false negative doesn't come back as a quiet warning.
- **D3. Helm-only apps are "not affected" only when a target repo is given.** With a target repo, a single-source Helm repo app is "not affected", because a git ref can't apply to it. With none, it keeps today's error, so existing results don't change.
- **D4. The ref goes to all matching sources.** Every source whose `repoURL` matches any target repo gets the target ref, whatever its own `targetRevision`. This needs no base-branch input. A source pinned to a tag in the same repo also moves to the branch. That's accepted, and the per-source detail in the response shows it.
- **D5. Inference in the UI is a query parameter.** `inferTargetRepo=true` sits in the URL next to `targetRef` and `targetRepo`, so a shared link reproduces the same result. It's off when absent, as on the CLI and API.
- **D6. The target repo is a list, with one ref.** `--target-repo` repeats, `targetRepo` repeats in the URL, and the API takes `targetRepos`. This covers a change made under the same branch name in several repos. Taking a list from the start avoids adding a second field later. One repo is the common case, and it's simply a list of one.
- **D7. The target ref may be a branch, a tag or a commit SHA,** passed to ArgoCD unchanged. Use cases R1 to R6 test it. This closes gap G3 from the [manifests-repo plan](integration-manifests-repo.md).
- **D8. Fixing "needs target repo" adds the repo to the query.** It updates the URL and re-runs every app. There's no per-app re-run. See [Decisions from the design review](#decisions-from-the-design-review).
- **D9. Moved sources show "moved from `<live revision>`",** with no "pinned tag" label, because Tangle can't tell a tag from a branch.
- **D10. "All" counts the visible rows.** "Not affected" apps are hidden by default, and their toggle label always shows how many there are.

## Deferred

- **A different ref per repo** (`repo=ref` pairs: a coordinated change whose PRs use different branch names). It would need a repeatable `--target repo=ref`, `targets: [{repo, ref}]` on the API with the single-ref fields as shorthand (sending both forms is a 400), a URL encoding that survives the `:` and `/` in repo URLs, a row editor in the UI, and a second repo we control with its own change branch for fixtures. The hard part sits outside Tangle: the CI run for one PR has to know the other PR's branch. D6 doesn't rule it out. Pick it up when someone needs it.

D1 to D7 are recorded in [ADR 0032](../../adrs/0032-choose-multi-source-revisions-by-target-repo.md), and D8 to D10 in [Design](#design-2026-11-26). Follow-ups: #293 (show which sources got the ref when a render fails) and #294 (screenshot tests).
