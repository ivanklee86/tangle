# Move live-test manifests to `tangle-integration-manifests`

Status: in progress · 2026-11-26

The live e2e suites run against four ArgoCD Applications. Their source is this repo: `main`'s `integration/kubernetes/example/manifests/` is the live state, and the long-lived `test_gitops` branch stands in for "the change the user pushed". Move what ArgoCD renders into a new, minimal repo, `ivanklee86/tangle-integration-manifests`, and point the Applications at it.

## Review of the current implementation

How it works today:

- `task k8s:bootstrap:application` applies `integration/kubernetes/example/{project,application-*}.yaml`. Each Application points `repoURL` at `github.com/ivanklee86/tangle.git`, `targetRevision: main`, with its path under `integration/kubernetes/example/manifests/<n>`.
- Tangle takes `liveRef` from each Application's `targetRevision` (`handlers.go:172`), so `main` on GitHub is the live side. The tests pass `test_gitops` as `targetRef`.
- What `test_gitops` actually changes is two lines: `manifests/1/values.yaml` (`ingress.enabled: true`) and `manifests/3/values.yaml` (`replicaCount 1`, invalid YAML).
- The `test` and `prod` ArgoCD entries in `integration/tangle.yaml` are the same ArgoCD instance reached with two tokens. RBAC in `integration/kubernetes/argocd/values.yaml` scopes `automation` to project `default` (test-1 and test-2) and `automationProd` to `my-project` (test-3 and test-4).

Problems:

1. **The fixture is a full, stale fork of the product repo.** `test_gitops` carries a whole old copy of tangle (old `go.mod`, ingress-nginx instead of Traefik, an old `tangle.yaml`). It has needed periodic "Merge branch 'main' into test_gitops" commits, and `git diff main...test_gitops` is mostly noise, so it's hard to see what the fixture actually is.
2. **The live side is tangle's `main`.** Editing an example manifest in a tangle PR does nothing in that PR's CI, because ArgoCD reads `main` on GitHub. After merge it silently moves the baseline for every later run and every open PR. The test code and its data are versioned in two different places.
3. **"Don't rebase or fix this branch"** is a rule that's only enforced by documentation (`internals.md`, test header comments), on a branch sitting next to dozens of real feature branches.
4. **Product-repo noise:** pre-commit has to exclude `manifests/{1,3}/templates`, and the `helm create` boilerplate (`NOTES.txt`, HPA, `tests/test-connection.yaml`) lives in the product tree.

What's not a problem: the unit tests that mention `test_gitops` (`handlers_test.go`, `client_test.go`, `wrapper_test.go`, `tanglecli_test.go`, `main_test.go`, `fake_wrapper_test.go`) all use fakes. There the string is only a label, so they need no change.

## Use cases the fixtures provide

These are the behaviors the live suites assert, and the fixture property each one relies on. The new repo has to keep every row true.

| App | ArgoCD (project) | Labels | Source | Sync policy | Live state | On target ref | Result |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `test-1` | `test` (`default`) | `env:test, foo:bar, bazz:buzz` | Helm chart | automated | Healthy / Synced | `ingress.enabled: true` | Diff adds one `Ingress` and removes nothing |
| `test-2` | `test` (`default`) | `env:preprod, foo:bar, bazz:buzz` | Plain dir (ConfigMap) | automated | Healthy / Synced | unchanged | Empty diff |
| `test-3` | `prod` (`my-project`) | `env:prod, foo:bar` | Helm chart | none | Missing / OutOfSync (its manifests at `main` fail to apply, so a manual sync doesn't change this) | invalid `values.yaml` | `helm template` error, empty manifests |
| `test-4` | `prod` (`my-project`) | `env:infra, foo:bar` | Plain dir (ConfigMap) | automated | Healthy / Synced | unchanged | Empty diff |

The **Fixture dependency** column says what each use case needs from the manifests repo. "Application CRs" means it only needs the labels and projects, which stay in tangle. "Repo content" means it needs the manifests at `main` and `test_gitops`.

### Listing and label queries

| # | Use case | Asserted by | Fixture dependency |
| --- | --- | --- | --- |
| L1 | List across several ArgoCD instances, each scoped by its token's RBAC | `TestE2E_ServerApplications/no_tags` (2 + 2), `CI with failure` Applications link (4 rows, test and prod) | Application CRs |
| L2 | Include filter that matches all / one / none | `tags_match_all`, `tags_match_one`, `tags_match_none`, `TestE2E_ArgoCDClient_List` | Application CRs |
| L3 | Several include labels are ANDed | `multiple_tags` (`env:test,bazz:buzz` → test-1) | Application CRs |
| L4 | Exclude, exclude-only, several excludes, include + exclude on the same key | `exclude*`, `cross_map_different_values` | Application CRs |
| L5 | Malformed or contradictory query → 400 with a message | `TestE2E_ServerApplicationsBadRequest` | None |
| L6 | Health and sync status shown per app (synced vs. never-synced) | `CI scenario` Applications link (test-1 and test-2 Healthy/Synced), `CI with failure` Applications link (test-3 Missing/OutOfSync), `applications.spec.ts` | Repo content at `main` must deploy and become Healthy (test-1, test-2, test-4), and must fail to apply for test-3 |

### Diffs

| # | Use case | Asserted by | Fixture dependency |
| --- | --- | --- | --- |
| D1 | Changed app → diff shows exactly the added resource | `assertDiffAddsOnlyIngress`, `expectIngressDiff`, Diffs "1 changed" | test-1 Helm chart, ingress gated by a value that differs between refs |
| D2 | Unchanged app → empty diff, on both ArgoCDs | `diff-test-test-2.yaml` / `diff-prod-test-4.yaml` empty, "No changes" | test-2 and test-4 identical on both refs |
| D3 | One app fails to generate; the others still render | `CI with failure` (CLI and UI), Diffs "1 error", outcome filter | test-3 Helm chart, invalid YAML only on the target ref |
| D4 | Diffs work on each ArgoCD instance | `TestE2E_ServerDiffs` (test-1 on test, test-4 on prod) | test-1 and test-4 render on both refs |
| D5 | Diff pages fill progressively, reload re-issues the POST, switching apps swaps the pane | `diffs.spec.ts` | At least 2 apps under `foo:bar` |

### CLI (`generate-manifests`)

| # | Use case | Asserted by | Fixture dependency |
| --- | --- | --- | --- |
| C1 | Writes `manifests-*`, `diff-*` (and `error-*` on failure) to the working directory | `TestE2E_CIScenario`, `TestE2E_CIFailureScenario` | D1–D3 |
| C2 | The manifests written match what ArgoCD itself renders (the `argocd app manifests` oracle) | `assertManifestsMatchArgoCD` | Any renderable app. The oracle compares, so chart contents can change freely |
| C3 | `--fail-on-error`: exit 0 when clean, 1 on failure. Without the flag: exit 0 with the same files | both scenario tests | D3 |
| C4 | No matches → exit 0, no files. Unreachable server → exit 1 | `TestE2E_CLIEdgeCases` | None |

### Gaps (not covered today; optional, not part of this move)

- **G1 Modify or remove in a diff.** Only a pure addition is tested. One extra changed line (for example a ConfigMap value in a fifth app, so test-2 and test-4 stay unchanged) would cover `-` lines. That needs a new Application CR and changes the expected counts in L1–L4.
- **G2 Target ref that doesn't exist.** Test-only change, no repo content needed.
- **G3 Target ref as a tag or commit SHA.** Cheap once the new repo exists: tag `test_gitops`'s commit. Now planned as use cases R1 to R6 in the [multi-source plan](2026-11-26-multi-source-applications.md) (D7).
- **G4 Kustomize source**, **G5 Degraded health.** Mention only if Tangle should promise these.

I'd do the move first with no new use cases, so the PR is a pure change of source and every expected value stays the same. Then take G2, G3 and the Helm trim as follow-ups, since they need no new fixture apps.

## Design of the new repo

```text
tangle-integration-manifests/
├── README.md
├── test-1/          # Helm chart: Deployment, Service, Ingress (gated by ingress.enabled)
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── test-2/configmap.yaml
├── test-3/          # Helm chart, minimal (Chart.yaml, values.yaml, one template)
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
└── test-4/configmap.yaml
```

- **Directories are named after their Application**, so each manifest is easy to trace to its app.
- **Refs:** `main` is the live baseline, and the Applications keep `targetRevision: main`. `test_gitops` is a branch that is **exactly one commit** on top of `main`, containing the two `values.yaml` edits. Keeping the name means no test, README, or screenshot-script strings change. The kludge was the branch being a fork of the product repo, not the name.
- **Copy the charts verbatim.** The `helm create` boilerplate (`NOTES.txt`, `hpa.yaml`, `tests/test-connection.yaml`, the ServiceAccount) moves unchanged, so the rendered manifests are byte-identical and the move can't change test outcomes. Trimming is a follow-up. It's safe whenever it happens, because C2 compares against ArgoCD rather than a snapshot and D1 only checks for an added `Ingress` with no removals.
- **README** (one of only two non-manifest files, with an MIT `LICENSE` matching the other public repos): what the repo is for, the app table above (app → labels → what differs on `test_gitops` → expected Tangle result), and the rules: "`test_gitops` = `main` + one commit; change both together; tangle's live e2e reads this repo from GitHub, so changes take effect immediately for every tangle branch."
- **Repo settings:** public, so ArgoCD needs no credentials. No CI, Renovate, or Dependabot; if the Renovate app is installed org-wide, exclude this repo, because an image or chart bump would change rendered manifests. Add a ruleset on `main` and `test_gitops` that blocks deletion and force-push and requires a PR on `main`. No octocov enrollment, no devcontainer.

## Steps

### 1. Create `tangle-integration-manifests` (before any tangle change)

Tangle's CI reads it from GitHub, so it has to exist with both refs first.

1. Create the repo. Commit `test-1..4` from tangle `main`'s `integration/kubernetes/example/manifests/{1..4}` (verbatim), `README.md`, and `LICENSE` to `main`.
2. Branch `test_gitops` from it and commit the two `values.yaml` edits. Confirm `git diff main test_gitops` is exactly those two lines.
3. Add the ruleset.

### 2. Tangle PR: point the Applications at the new repo

- `integration/kubernetes/example/application-{1..4}.yaml`: set `repoURL: https://github.com/ivanklee86/tangle-integration-manifests.git` and `path: test-<n>`. Labels, projects, sync policies, and `project.yaml` stay. They're cluster setup, and L1–L4 depend on them.
- Delete `integration/kubernetes/example/manifests/`.
- `.pre-commit-config.yaml`: drop the two `manifests/{1,3}/templates` excludes.
- Docs and comments: `docs/agents/internals.md` "Live test fixtures", the `docs/agents/test_cases.md` intro, the header comments in `cmd/tangle-cli/main_e2e_test.go` and `web/e2e/live/ci-scenarios.spec.ts`, and the comments in `internal/tangle/server_e2e_test.go:50` and `internal/argocd/argocdfakes/fixtures.go:6`. Each should say that the fixtures live in `tangle-integration-manifests` and link its README, instead of restating the table.
- Optionally fold this plan's use-case tables into `docs/agents/test_cases.md`, so the use cases are listed in one place.
- No test code changes: `test_gitops`, the app names, the labels, and every expected count stay the same.

Verify: `task e2e:live` and `task go:test:e2e` locally against a fresh `task services:cicd`, then the CI `e2e` job (amd64 and arm64) on the PR.

### 3. Cleanup (after the tangle PR merges)

- Delete `test_gitops` from `ivanklee86/tangle`. Ask first: after this, CI can no longer be re-run on older commits that still point at it.
- Mark this plan implemented.

## Risks

- **Ordering:** if the tangle PR merges before step 1, `main` CI breaks. Avoid this by doing step 1 first.
- **Drift in the new repo:** a direct push to `main` moves the baseline. The ruleset and the README's one-commit rule cover this.

## Decisions (2026-11-26)

1. **Branch name:** keep `test_gitops` for now. A rename (`pushed-change` was the candidate) can come later as its own change.
2. **Helm charts:** copy verbatim; trim in a follow-up.
3. **Repo contents:** `README.md` and `LICENSE` plus the manifests, nothing else.

## Follow-up: source-type fixtures (2026-11-26)

Test infrastructure for the ways an Application can get its manifests that Tangle doesn't handle yet. This adds the fixtures and a contract test against ArgoCD. It doesn't change Tangle's behavior.

| Application | Covers | Revision ArgoCD needs for the pushed change |
| --- | --- | --- |
| `tanka` | A config management plugin sidecar (Tanka, as in homeops). Its diff modifies and removes lines (G1) | `Revision` |
| `helm-repo` | A chart from a Helm repo, with no git source | None. A branch fails, because the revision is a chart version |
| `helm-repo-git-values` | A Helm-repo chart, values from git through `ref` | `Revisions`/`SourcePositions` = position 2 |
| `git-chart-git-values` | A git chart, plus a `ref` source with its own `path` (same repo) | Positions 1 and 2 |
| `two-repos` | Sources from two repos, with the change in one | Position 2. Sending it to every position fails |

The apps live in `integration/kubernetes/sources/`, in AppProject `sources`, behind the `sources` ArgoCD (token `ARGOCD_SOURCES_TOKEN`), so existing expectations don't change. `TestE2E_SourceFixtures` asserts each row directly against ArgoCD.

Not covered yet: source hydrator (it needs a commit-server and push credentials), duplicate resources across sources, a CMP failure, and tag or SHA target refs (G3, now planned in the [multi-source plan](2026-11-26-multi-source-applications.md)).

Tangle-level tests on these fixtures are in `TestE2E_ServerSourceFixtures` and `web/e2e/live/sources.spec.ts`. Their multi-source cases are skipped until the gap below closes.

Next: make Tangle send `Revisions`/`SourcePositions` for multi-source apps. That needs a way to say which repo the pushed ref belongs to. Rendered by hand (2026-11-26), a multi-source app at explicit `Revisions` also differs from its default render in ways the pushed change didn't cause: Helm test-hook Pods appear, `namespace` is dropped and resources reorder. So the fix should probably render both sides with `Revisions`, then tighten the skipped tests to exact matches and un-skip them.
