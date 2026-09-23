# CI use-case e2e tests

Status: implemented · 2026-09-23

Turn the two product scenarios in [test_cases.md](../test_cases.md) ("CI" and "CI with failure") into e2e tests that run against the live stack (`task services:cicd`). Each Gherkin step becomes a test that can be traced back to it. The CLI steps are tested in Go. The "user clicks the URL" steps are tested in the live Playwright suite.

## Fixtures: repurposing what's already there

Instead of pushing a real change for every run, the tests aim at existing Applications with a `targetRef` whose contents are already known. The `test_gitops` branch on `origin` already acts as "the user's pushed change". All four example Applications (`integration/kubernetes/example/application-*.yaml`) track `main` on GitHub, and `test_gitops` differs from `main` as follows (checked against the running stack on 2026-09-23):

| Application | ArgoCD | Labels | Change on `test_gitops` | Tangle result |
| --- | --- | --- | --- | --- |
| `test-1` | `test` | `env:test, foo:bar, bazz:buzz` | `manifests/1/values.yaml`: `ingress.enabled: true` | Diff adds one `Ingress`, no error |
| `test-2` | `test` | `env:preprod, foo:bar, bazz:buzz` | none | Empty diff, no error |
| `test-3` | `prod` | `env:prod, foo:bar` | `manifests/3/values.yaml`: `replicaCount 1` (invalid YAML, "Sabatoge #3") | `manifestGenerationError` from `helm template`, empty manifests |
| `test-4` | `prod` | `env:infra, foo:bar` | none | Empty diff, no error |

Each scenario maps to a label query:

- **CI (success)**: `bazz:buzz` → `test-1` (changed) plus `test-2` (unchanged). This covers "diff present" and "no diff" in one run, and every app succeeds.
- **CI with failure**: `foo:bar` → all four. `test-3` fails, and the other three show that a partial failure still writes output for the healthy apps.

A pure `env:prod` query (only `test-3`) is deliberately not used for the failure case. It can't show that good output survives next to a bad app.

Stated in each test file's header comment: these tests depend on the remote `test_gitops` branch and on `main`'s `integration/kubernetes/example/manifests` on GitHub, not on the local checkout. Rebasing or "fixing" `test_gitops` breaks them. Add a short note to the top of `test_cases.md` and to `internals.md` that `test_gitops` is a test fixture.

## Assumptions

- **The Application and Diffs URLs are rendered by an external CLI in the user's CI**, not by `tangle-cli`. The Playwright tests therefore build the URLs themselves, the same way the UI's link preview does (served `domain` + `/applications?labels=…` / `/diffs?labels=…&targetRef=…`). That's the contract the home-page live test already pins.
- **CI invokes `tangle-cli` with `--server-address` set**, plus `--fail-on-error` in the failure scenario.

## Gaps found while planning (flagged, not enshrined)

1. **`NewWithConfig`/`New` default `Err` to `os.Stdin`** (`internal/cli/tanglecli.go`). The binary isn't affected because `PersistentPreRunE` overwrites it with `cmd.ErrOrStderr()`. Library callers are affected, and `printToStreamWithColor` panics on a write error. It's a one-line fix, done in workstream 1 because the tests read stderr.
2. **Found during implementation: `--folder` didn't default to the working directory.** `New()` set `Folder` to the cwd, but registering the `--folder` flag reset it to `""`, so a run without `--folder` tried to write `/diff-….yaml` and failed with `permission denied`. Fixed in `TangleCLI.Configure`, with a unit test in `cmd/tangle-cli/main_test.go`. The scenario tests leave `--folder` out so they cover this path.
3. **`TangleCLI.Error` calls `os.Exit(1)`**, so the CLI can't be driven in-process from a test without killing the test binary. The Go e2e test builds and execs the real binary. That's also the only honest way to observe the exit code.

## Test cases

Test names follow the scenario steps, e.g. `TestE2E_CIScenario/cli_writes_manifests_matching_argocd`, so a failure points to the broken promise.

### Go: `cmd/tangle-cli/main_e2e_test.go` (`//go:build e2e`, `TestE2E_` prefix)

`TestMain`, or a `sync.Once` helper, runs `go build -o $TMP/tangle-cli ./cmd/tangle-cli` once. Each subtest runs the binary with `--server-address localhost:8081 --insecure --target-ref test_gitops --retries 2`, with a fresh `t.TempDir()` as its working directory (no `--folder`, see gap 2), capturing stdout, stderr and the exit code (`exec.ExitError.ExitCode()`).

**Oracle for "matches manifests from ArgoCD"**: `argocd app manifests <app> --revision test_gitops --server localhost:8080 --plaintext --auth-token $ARGOCD_TOKEN`, via the `argocd` CLI that the devcontainer and the CI `e2e` job already install. The oracle is ArgoCD itself, independent of Tangle's own API. Comparison: split both on `---`, parse each document with `sigs.k8s.io/yaml` (already in `go.mod` through the k8s deps; verify at implementation), key by `apiVersion/kind/namespace/name`, and `assert.Equal` the maps. That ignores ordering and formatting and still catches content drift. On 2026-09-23 both produced ServiceAccount/Service/Deployment/Ingress/Pod for `test-1`.

`TestE2E_CIScenario` (labels `bazz=buzz`, no `--fail-on-error`):

- exits 0; stdout lists `Applications found: 2`; the table shows both apps with `False ✅`.
- `cli_writes_manifests_matching_argocd`: `manifests-test-test-1.yaml` and `manifests-test-test-2.yaml` exist, are non-empty and equal the argocd oracle per app.
- `cli_writes_diffs_matching_change`: `diff-test-test-1.yaml` contains an added (`+`) `kind: Ingress` document, and no line removes (`-`) a resource kind. `diff-test-test-2.yaml` exists and is empty. No `error-*.txt` files.
- Also run the same invocation with `--fail-on-error` and assert it still exits 0. That pins that the flag doesn't fire when nothing failed.

`TestE2E_CIFailureScenario` (labels `foo=bar`, `--fail-on-error`):

- exits 1; stderr contains `Failures found in manifest generation!`; stdout still renders the table, with `True 🔥` only on the `test-3` row.
- `cli_writes_manifests_matching_argocd`: manifests for `test-1`, `test-2` and `test-4` equal the oracle. `manifests-prod-test-3.yaml` exists and is empty. The argocd oracle for `test-3` also fails. Assert that its error and `error-prod-test-3.txt` both mention `helm template`, so "matches ArgoCD" holds for the failure too.
- `cli_writes_diffs_matching_change`: same `test-1` Ingress assertion; `test-2`/`test-4` diffs empty; `diff-prod-test-3.yaml` empty; `error-prod-test-3.txt` non-empty.
- Also the same invocation without `--fail-on-error` → exits 0 with identical files. That pins `--fail-on-error` as the only thing that turns a generation error into a non-zero exit.

Edge and error cases that are cheap to add at this layer: an unreachable `--server-address` exits 1 with `Error getting applications`, and a label that matches nothing (`env=foobar`) exits 0 with `Applications found: 0` and no files. Both were checked against the live stack and behave as described; `pond.NewResultPool(0)` is fine.

### Playwright: `web/e2e/live/ci-scenarios.spec.ts`

A small helper reads `domain` from `/api/config`, the same way `home.spec.ts` does, and builds `${domain}/applications?labels=…` and `${domain}/diffs?labels=…&targetRef=test_gitops`. Every test collects `pageerror` and asserts none. Diff-generation waits use `timeout: 60_000`, since each app is a real ArgoCD manifest render. Health waits use `expect(...).toPass()`, because `test-1` may briefly be `Progressing` right after bring-up.

`CI → Applications URL shows high-level state` (`bazz:buzz`): exactly two rows. `test-1` shows `Healthy`/`Synced` and `test-2` shows `Missing`/`OutOfSync` (it has no automated sync policy, so this is stable by design).

`CI → Diffs URL shows high-level state` (`bazz:buzz`): the header count reads `1 changed` with no error badge; the `Applications` nav lists `test-1` as `Changed` and `test-2` as `No changes`. Selecting `test-1` shows a diff containing `kind: Ingress`.

`CI with failure → Applications URL` (`foo:bar`): four rows, both ArgoCDs represented.

`CI with failure → Diffs URL` (`foo:bar`): header shows `1 changed` and `1 error`. The `test-3` row reads `Error`, and its detail pane shows the manifest generation error mentioning `helm template`. The `Errors` outcome filter narrows the list to `test-3` alone. `test-1` still shows its Ingress diff, so a failure in one app doesn't blank the others.

Role/label selectors should come from the current `ui-refactor` pages (`nav[aria-label="Applications"]`, `Filter by diff outcome`, the outcome text in `web/src/routes/diffs/+page.svelte`). Land this after the in-flight UI refactor so the selectors don't churn.

## Workstreams

### 1. Go CLI e2e suite

1. Fix `Err: os.Stdin` → `os.Stderr` in `New`/`NewWithConfig` (gap 1), with a unit test asserting library callers' errors reach stderr.
2. Add `cmd/tangle-cli/main_e2e_test.go` per above, plus a small `argocdManifests(t, app, ref)` helper and a YAML-document-set comparer. Load `.env` with `godotenv`, like `internal/tangle/server_e2e_test.go` does.
3. `task go:test:e2e` locally against the running stack. It's already selected by `-run '^TestE2E_'`, so no Taskfile or CI change is needed. Coverage: build the binary with `-cover` and set `GOCOVERDIR` so `report`'s octocov picks up CLI coverage. That's optional, so decide at implementation.

### 2. Playwright scenario spec

1. Add `web/e2e/live/ci-scenarios.spec.ts`, with the fixture summary and URL builders at the top of the file (no separate `scenarios.ts`: only one spec uses them).
2. `task ts:test:e2e:live` locally. Already covered by the existing `e2e` CI job, so no workflow change.

### 3. Docs

- `docs/agents/test_cases.md`: link each scenario to its Go and Playwright tests, note the `test_gitops` fixture dependency.
- `docs/agents/internals.md`: describe the CLI e2e layer and the `test_gitops` fixture.
- `docs/agents/ci.md`: add the CLI e2e tests to the test-pyramid table's Go e2e row.

## Risks

- **Remote fixture drift**: someone "fixes" `test_gitops`, or `main`'s example manifests change. Mitigation: the fixture note in docs, and assertions keyed to the specific changed resource (`Ingress`, `helm template`) rather than whole-diff snapshots.
- **ArgoCD manifest cache**: the `test-3` error comes back as `(cached)`. That's harmless for assertions (substring match), but the oracle and Tangle may disagree on wording, so match on `helm template` only.
- **Runtime**: the failure scenario renders four apps and the oracle renders four more. Expect around 30–60s per Go scenario; set `go test -timeout` accordingly.
- **Parallel runs**: both Go and Playwright hit the same stack in the same CI job, sequentially. The tests are read-only against ArgoCD (no syncs), so there's no cross-test interference.

## Sequencing

1 → 2 → 3, one commit each. Rollback for each is deleting the new test files; only workstream 1's `os.Stderr` fix touches production code.
