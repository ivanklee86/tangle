# CI pipeline restructure

Status: proposed · 2026-09-20

Build out the full test pyramid for both Go and the frontend — fast unit tests at the base, mock-backed integration tests in the middle, real-ArgoCD-and-browser e2e tests at the top — split Go tests accordingly, make `ci.yaml`'s `go`/`ts`/`docs` jobs run only when their own paths change, fold the standalone `format` job into `go`, land one combined `e2e` job (Go live tests + the frontend's live-stack Playwright suite) that always runs, replace Codecov with [octocov](https://github.com/k1LoW/octocov) for the `go` job's coverage reporting: a GitHub Actions Artifacts datastore gives `tangle` its own diff-vs-base-branch PR comment, and a new dedicated repository (`ivanklee86/octocov-central`, running octocov's central mode) turns that into a durable, publicly embeddable coverage badge and dashboard, and cache Go's module/build cache and installed tool binaries, npm's package cache, Playwright's downloaded browser, the pinned k3d/argocd CLI binaries, and the Docker image build's layers (via buildx and the GitHub Actions cache backend) across CI runs. Overall CI wall-clock time is explicitly not a constraint for the `e2e` job's own test breadth — the always-run `e2e` job is allowed to be slow if that's what real coverage costs — but the caching workstream trims the repeated-download/rebuild overhead every job pays regardless of that breadth. See [ADR 0009](../../adrs/0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) for the taxonomy/conditional-jobs decision and how it changes [ADR 0003](../../adrs/0003-svelte-e2e-testing-strategy.md)'s cadence, [ADR 0010](../../adrs/0010-replace-codecov-with-octocov.md) for the coverage-tooling swap and its artifact datastore, and [ADR 0011](../../adrs/0011-octocov-central-reporting-and-badges-repository.md) for the central badges/dashboard repository.

This plan absorbs and supersedes [docs/agents/plans/svelte-e2e-testing.md](svelte-e2e-testing.md) — its research (especially the devcontainer Playwright/font findings) is still good, but its four workstreams are folded into workstreams 4–6 below, widened from a two-file "smoke" suite to full live coverage of the same user flows the mocked suite covers.

Land the twelve workstreams below in order; each is independently useful and independently testable before moving to the next.

## The test pyramid (target state)

```
                     ┌─────────────────────────────┐
                     │   e2e (real ArgoCD + real    │  slow, few tests,
                     │   browser) — always runs     │  real infra, top of
                     │   e2e job                    │  the pyramid
                     ├─────────────────────────────┤
                     │  mock/fixture-backed         │  fast, no live infra,
                     │  integration — go job's      │  most of the coverage
                     │  fakes + ts job's mocked     │  lives here
                     │  Playwright suite            │
                     ├─────────────────────────────┤
                     │  unit — go job's pure        │  fastest, most numerous,
                     │  package tests + ts job's    │  path-conditional
                     │  vitest component/lib tests  │
                     └─────────────────────────────┘
```

| Layer | Go | TS | Runs in | Gated on |
|---|---|---|---|---|
| Unit | `pkg/client`, `internal/tangle` (server/loader/manifests), `internal/cli/output_test.go`, `cmd/tangle-cli`'s help-text case | `*.spec.ts`, `*.svelte.test.ts` (vitest) | `go` / `ts` jobs | Path filter |
| Integration (mock/fixture-backed) | `internal/argocd/wrapper_test.go`, `internal/tangle/handlers_test.go`, `internal/cli`/`cmd/tangle-cli`'s CLI-against-`httptest.Server` cases | `web/e2e/mocked/*.spec.ts` (Playwright, network-mocked) | `go` / `ts` jobs | Path filter |
| E2E (real ArgoCD, real browser) | `internal/argocd/client_e2e_test.go`, `internal/tangle/server_e2e_test.go` | `web/e2e/live/*.spec.ts` (Playwright, real cluster) | `e2e` job | Always runs |

## Test taxonomy (target state) — Go

| File | Today | Target bucket | Why |
|---|---|---|---|
| `pkg/client/client_test.go` | Pure (URL string building) | Unit | Already no external dependency. |
| `internal/tangle/server_test.go` | Pure | Unit | Already no external dependency. |
| `internal/tangle/loader_test.go` | Pure (reads `integration/tangle.yaml` from disk) | Unit | Reads a checked-in fixture file, no network. |
| `internal/tangle/manifests_test.go` | Pure (YAML assembly/diff logic) | Unit | Already no external dependency. |
| `internal/cli/output_test.go` | Pure (string formatting) | Unit | Already no external dependency. |
| `cmd/tangle-cli/main_test.go`: `TestCli/root Command` | Pure (help text) | Unit | Already no external dependency. |
| `internal/argocd/client_test.go`: `TestNewArgoCDClient/creates client with invalid options` | Depends on `.env` existing (shared `setup(t)`) but doesn't need live ArgoCD — missing-token-env-var validation only | Unit (rewritten to use `t.Setenv`, not `.env`) | Tests our own validation, not ArgoCD's wire protocol. |
| `internal/argocd/wrapper_test.go` (all cases) | Needs real ArgoCD at `localhost:8080` via `NewArgoCDClient` | Integration (rewritten against a fake `IArgoCDClient`) | Tests `ArgoCDWrapper`'s pooling/label-filtering/error-propagation logic, not ArgoCD's wire protocol — a fake client exercises the same code paths. |
| `internal/tangle/handlers_test.go` (all cases) | Needs real ArgoCD at `localhost:8080` (via `tangle.New(&config,...)`) | Integration (rewritten to inject fake `IArgoCDWrapper`s directly into `Tangle.ArgoCDs`) | Tests label-parsing/routing/JSON-shape logic, not ArgoCD's wire protocol. |
| `internal/cli/tanglecli_test.go`: `TestTangleCLIHappyPaths` | Needs a live `tangle-server` (real container) at `localhost:8081`, backed by real ArgoCD | Integration (rewritten to run `tangle.New(...)`'s handler in-process via `httptest.NewServer`, with fake wrappers injected) | `pkg/client` is a plain `net/http` client against a domain string — an `httptest.Server`'s address works identically to a real deployed server for everything this test actually exercises (label parsing, manifest-file writing, retries). |
| `cmd/tangle-cli/main_test.go`: `TestCli/generate manifests` | Same as above | Integration (same rewrite) | Same reasoning. |
| `internal/argocd/client_test.go`: `TestNewArgoCDClient/creates client with valid options`, `TestArgoCDClient_List`, `TestArgoCDClient_GetApplicationManifests`, `TestArgoCDClient_Get` | Needs real ArgoCD at `localhost:8080` | **E2E** (moved to `internal/argocd/client_e2e_test.go`, `//go:build e2e`) | This is `ArgoCDClient` itself — the thing that talks real gRPC-web to ArgoCD. There's nothing left to fake underneath it; testing it against a fake would only test our own assumptions about ArgoCD's behavior, not ArgoCD's actual behavior. |
| *(new)* `internal/tangle/server_e2e_test.go` | Doesn't exist | **E2E** (`//go:build e2e`) | With `handlers_test.go` and `wrapper_test.go` moved to fakes, nothing exercises the *real* `handlers → wrapper → client → live ArgoCD` path end-to-end anymore. Give this real breadth now that CI speed isn't a constraint — cover listing applications (all four ArgoCD/app combinations from `integration/kubernetes/example/`, not just one), label/exclude-label filtering against the real cluster, and a full diff generation, using the real `integration/tangle.yaml` config against the live cluster `task services:cicd` brings up. |

**Net effect**: `go test ./...` (no tags) needs no `.env`, no live ArgoCD, and no Docker/k3d — it's a pure, fast unit+integration suite. `go test -tags=e2e ./...` needs `task services:cicd` up and `.env` populated by `task argocd:token`, exactly like today's `go` job.

## 1. Build the fake ArgoCD client/wrapper

**New package**: `internal/argocd/argocdfakes/` (regular `.go` files, not `_test.go` — both `internal/argocd`'s own tests and `internal/tangle`'s tests need to import these, and Go doesn't allow importing `_test.go`-only symbols across packages).

- `fake_client.go`: a hand-rolled type implementing `argocd.IArgoCDClient` (`List`, `GetApplicationManifests`, `Get`, `GetUrl`, `GetScheme`) backed by an in-memory fixture: a configurable `[]v1alpha1.Application` list (filtered by the incoming `ApplicationQuery.Selector` the same way real ArgoCD's label selector would be interpreted — reuse whatever subset of Kubernetes label-selector parsing is simplest, e.g. `k8s.io/apimachinery/pkg/labels` — confirmed already present at `v0.36.1` in `go.sum`, pulled in transitively via `argo-cd`, so this needs no new `go.mod` dependency) and a configurable per-application manifest response, plus injectable errors (`ErrOnGet`, `ErrOnList`, `ErrOnGetManifests`, keyed by application name where relevant) to reproduce the current "error"/"not found" test cases.
- `fake_wrapper.go`: a hand-rolled type implementing `argocd.IArgoCDWrapper` directly (`ListApplicationsByLabels`, `GetManifests`, `GetUrl`, `GetScheme`) for `internal/tangle`'s handler tests, which operate one layer above the client and don't need the pooling logic exercised — just canned per-ArgoCD results.
- Model the fixture data on `integration/kubernetes/example/application-*.yaml` (names `test-1`/`test-2`/`test-3`/`test-4`, labels, project, health/sync status) so the fake's behavior matches what today's live-cluster tests assert (e.g. `wrapper_test.go`'s `foo=bar` → `test-2`, `env=test` exclude → 1 result) without changing any test's expected values — this is a mechanical swap of "how the data gets there," not a rewrite of what's asserted.

**Steps**

1. Read `internal/argocd/wrapper_test.go`, `internal/tangle/handlers_test.go`, and `internal/argocd/client_test.go`'s existing assertions closely to extract the exact fixture shape needed (which apps, which labels, which projects, which manifest content) so behavior is preserved byte-for-byte where the tests assert exact values.
2. Write `argocdfakes.NewFakeClient(fixture ...)` and `argocdfakes.NewFakeWrapper(fixture ...)` with table-driven fixture construction.
3. Unit-test the fakes themselves minimally (they're test infrastructure now, not throwaway — a bug in the fake's label-matching would silently make real bugs invisible).

**Rollback**: delete `internal/argocd/argocdfakes/`; nothing else depends on it until workstream 2 wires it in.

## 2. Rewrite the mock-backed Go tests, add the `e2e` build tag

**Per AGENTS.md's testing philosophy**: for each rewritten file, first list the test cases being preserved (they should map 1:1 to today's cases — this workstream changes *how* dependencies are provided, not what's being verified), then write them; flag anything that turns out to be untestable or already buggy rather than encoding it.

1. `internal/argocd/wrapper_test.go` — replace `NewArgoCDClient(...)` + `setup(t)`/`.env` with `argocdfakes.NewFakeClient(...)`. Same subtests (`pool`, `error`, `get manifests from pool`, `not found getting manifests`), same assertions.
2. `internal/tangle/handlers_test.go` — replace `New(&config, "testing")`'s real wrapper construction by building `Tangle{Config: &config, ArgoCDs: map[string]argocd.IArgoCDWrapper{"test": argocdfakes.NewFakeWrapper(...), "prod": argocdfakes.NewFakeWrapper(...)}}` directly (or add a small test-only constructor path if `New()`'s other side effects — router setup, logger — are still needed; router setup doesn't touch `ArgoCDs` until request time, so overwriting the map after `New()` returns is simplest and needs no production code change). Same label-filter test matrix, same assertions.
3. `internal/argocd/client_test.go` — drop the shared `setup(t)`/`.env` dependency for `TestNewArgoCDClient/creates client with invalid options`; use `t.Setenv(...)`-style per-case env instead (that subtest already expects *no* token to be found, so no `t.Setenv` call at all may be simplest — confirm at implementation time that `os.LookupEnv` behaves correctly with zero ambient env pollution from other tests). Move everything else in this file to `internal/argocd/client_e2e_test.go` with `//go:build e2e`, keeping `godotenv.Load("../../.env")` there since those genuinely need the live-minted token.
4. `internal/cli/tanglecli_test.go` and `cmd/tangle-cli/main_test.go` — rewrite to build a real `*tangle.Tangle` with fake wrappers injected (per point 2), wrap `.Server.Handler` in `httptest.NewServer(...)`, and point `Config.ServerAddr`/the `generate-manifests` command's `--server-address` flag at the test server's host:port (`strings.TrimPrefix(server.URL, "http://")`) with `Insecure: true`. Same fixture data as `handlers_test.go` so the existing `assert.FileExists`/`assert.Contains` expectations (app names, counts) keep passing unchanged.
5. New `internal/tangle/server_e2e_test.go` (`//go:build e2e`): build a real `Tangle` from `integration/tangle.yaml` + live `.env` (same pattern `loader_test.go` and the current `handlers_test.go` use for config). Give this broad coverage per the pyramid table above — list applications across all four seeded apps and both ArgoCDs, exercise label/exclude-label filtering against real data, and generate at least one real diff — asserting against whatever `task services:cicd`'s seeded cluster actually contains.
6. Confirm `go build ./...` (no tags) and `go vet ./...` still pass with `argocdfakes` present but unused by production code (it's only ever imported from `_test.go` files, so it won't ship in `tangle-server`/`tangle-cli` binaries — verify with `go build` output size / `go list -deps` if in doubt).

**Steps**

1. Land points 1–3 (argocd package) as one commit; `go test ./...` in `internal/argocd` should pass with zero external dependencies.
2. Land point 2 for `internal/tangle` as one commit; same check for that package.
3. Land points 4–5 (`internal/cli`, `cmd/tangle-cli`, new e2e file) as one commit.
4. Full local check: `go test ./...` (no `.env`, no cluster running) passes end to end; separately, with `task services:cicd` up and `.env` populated, `go test -tags=e2e ./...` passes too.

**Rollback**: each commit is independently revertable; reverting all of them returns to today's fully-live-dependent test suite.

## 3. Go Taskfile wiring

**`tasks/go.yaml`**

1. `test` (default, used locally and in the fast `go` CI job) stays `go test -v ./... 2>&1` — untagged, so it now runs the unit+integration bucket only, with no behavior change to the command itself (the tests moved, the task didn't).
2. `test-ci` stays as-is for the unit/integration bucket's CI invocation (JUnit + coverage), also unchanged.
3. Add `test:e2e` and `test:e2e:ci` as their own flat, colon-named keys (Task has no nested-subtask syntax — a `:` in a task's key is just a literal namespacing convention, the same way `tasks/k8s.yaml` has `cluster:create`/`cluster:delete`/`cluster:config` as sibling top-level keys). These mirror `test`/`test-ci` but with `-tags=e2e` and (for `test:e2e:ci`) a separate report/coverage filename (`report-e2e.xml`, `coverage-e2e.out`/`.html`) so the two suites' reports don't collide when both run in the same job (workstream 7):
   ```yaml
   tasks:
     test:
       desc: Run Golang tests.
       cmds:
         - go test -v ./... 2>&1

     test-ci:
       desc: Run Golang tests and generate coverage report.
       cmds:
         - go test --coverprofile {{.COVERAGE_RAW}} -v ./... 2>&1 | tee >(go-junit-report > report.xml)
         - go tool cover -html={{.COVERAGE_RAW}} -o {{.COVERAGE_REPORT}}

     test:e2e:
       desc: Run Golang e2e tests (needs a live ArgoCD — see `task services:cicd`).
       cmds:
         - go test -tags=e2e -v ./... 2>&1

     test:e2e:ci:
       desc: Run Golang e2e tests and generate coverage report, for CI.
       vars:
         COVERAGE_RAW: coverage-e2e.out
         COVERAGE_REPORT: coverage-e2e.html
       cmds:
         - go test -tags=e2e --coverprofile {{.COVERAGE_RAW}} -v ./... 2>&1 | tee >(go-junit-report > report-e2e.xml)
         - go tool cover -html={{.COVERAGE_RAW}} -o {{.COVERAGE_REPORT}}
   ```
   (`test`/`test-ci` shown unchanged, for context — this plan doesn't rename them, only adds the two `test:e2e*` siblings alongside them, matching the frontend's `ts:test:e2e:live` naming from workstream 6.)

**Steps**

1. Add the two tasks.
2. `task go:test` locally with no `.env`/cluster — confirm it passes and doesn't try to reach the network.
3. With `task services:cicd` up, `task go:test:e2e` — confirm it exercises exactly the moved e2e files.

**Rollback**: revert the task additions; `go:test`/`go:test-ci` are untouched either way.

## 4. Frontend mocked (integration-layer) Playwright suite

Turn `web/`'s Playwright setup from a single non-passing, CI-unused spec (`e2e/demo.test.ts` waits for an `h1` no page renders) into the integration layer of the pyramid: real browser, real app code, network-mocked API responses.

**Layout change** (`web/e2e/`)

| | Current | Target |
|---|---|---|
| `web/e2e/demo.test.ts` | Checks for an `h1` (fails — no page has one) | Deleted |
| `web/e2e/fixtures/` | — | New: fixture JSON matching `$lib/data.ts` types |
| `web/e2e/mocked/*.spec.ts` | — | New: the mocked suite, network-mocked |
| `web/playwright.config.ts` | Points `testDir: 'e2e'` at the whole (empty-ish) directory | `testDir: 'e2e/mocked'`, unchanged `webServer` (`npm run build && npm run preview`) |

**Fixtures** (`web/e2e/fixtures/applications.json`, `web/e2e/fixtures/diff.json`) — plain JSON matching the wire shapes `TangleAPIClient` (`web/src/lib/client.ts`) expects raw from the API: `ApplicationsResponse` (`{ results: ArgoCDApplicationResults[] }`) and `ApplicationDiffResponse` (`{ liveManifests, targetManifests, diffs, manifestGenerationError }`) from `web/src/lib/data.ts`. Model them on two ArgoCDs (`test`/`prod`), two apps each, one healthy+synced and one unhealthy+out-of-sync per ArgoCD, so the fixtures exercise both the "happy" and "alert" rendering paths.

**Mocking approach**: `page.route('**/api/applications*', route => route.fulfill({ json: fixture }))` and `page.route('**/api/argocd/*/applications/*/diffs', route => route.fulfill({ json: diffFixture }))` per spec, set up before `page.goto()`. `PUBLIC_BASE_URL` in `.env.production` is empty (relative fetch), so these requests are same-origin as the Playwright-driven `npm run preview` server.

**Specs** (each a `test.describe` block; `test.beforeEach` for the common route-mock setup):

- `e2e/mocked/home.spec.ts` — `/`: both cards render with their labeled inputs; submitting "Diffs" with an empty Target Ref shows the `role="alert"` toast with the exact copy and dismisses on clicking its close button; submitting either card with a malformed label shows the invalid-label toast; submitting "Applications" with valid labels navigates to `/applications/?labels=...`; dark-mode toggle flips `html.dark` and persists across a reload.
- `e2e/mocked/applications.spec.ts` — `/applications/?refresh=false`: tabs render one per ArgoCD with the `(count)` suffix; switching tabs shows that ArgoCD's own table; column sort (click "Applications" header once → ascending `name` with ▲, again → descending with ▼); health/sync status cells render the right icon+text; refresh-period `Select` and refresh-toggle `Button` are present and toggling changes the button's color.
- `e2e/mocked/diffs.spec.ts` — `/diffs/`: nested tabs (ArgoCD → application) render; an unhealthy fixture application shows the rose `ExclamationCircleSolid` icon on its inner tab title; "Status" section renders the right text; "(More Info)" link `href`/`target` match the fixture; "Manifests" accordion is collapsed by default and expanding reveals the fixture's YAML; the refresh-diff `GradientButton` re-issues the mocked diff POST.

**Steps**

1. Delete `web/e2e/demo.test.ts`, create `web/e2e/fixtures/*.json` and `web/e2e/mocked/*.spec.ts` per above.
2. Update `web/playwright.config.ts`'s `testDir` to `'e2e/mocked'`, and add a JUnit reporter (`reporter: [['list'], ['junit', { outputFile: 'test-results/junit.xml' }]]`) for workstream 8's unified reporting.
3. `npm run test:e2e` locally (needs Playwright's browser installed — see workstream 5) until green.
4. Pin `@playwright/test`'s version in `web/package.json` exactly per AGENTS.md's pinning rule, since workstream 5 needs to install the exact matching browser build.

**Rollback**: revert `web/e2e/`, `web/playwright.config.ts`, and the `@playwright/test` pin; nothing outside `web/` changes in this workstream.

## 5. Devcontainer: install Playwright's browser

**Status: done.** `.devcontainer/Dockerfile` already has this exact `ARG PLAYWRIGHT_VERSION=1.63.0` block (matching `@playwright/test`'s pin in `web/package.json`) landed ahead of the rest of this plan. Steps below are kept for rollback/history; nothing left to do here before moving on to workstreams 4 and 6, which can now assume a working local Playwright browser.

**Problem found in prior manual verification** (carried over from the superseded plan — still accurate): `npx playwright install --with-deps chromium` fails outright on this devcontainer's Debian base — `ttf-ubuntu-font-family`/`ttf-unifont` aren't available from this image's apt sources. The browser binary downloads fine either way; without a font package installed afterward, Chromium renders real content but **all text is genuinely zero-height/invisible** (icons and colors still show), which looks exactly like a CSS bug.

**Steps**

1. In `.devcontainer/Dockerfile`, after the existing Node setup and before switching `USER vscode` back (the apt install needs root):
   ```dockerfile
   # Playwright (for web/ e2e tests) — keep the version in sync with
   # @playwright/test in web/package.json.
   ARG PLAYWRIGHT_VERSION=1.63.0
   RUN npx --yes playwright@${PLAYWRIGHT_VERSION} install chromium chromium-headless-shell && \
       apt-get update && apt-get install -y --no-install-recommends \
           libglib2.0-0 libnss3 libnspr4 libdbus-1-3 libatk1.0-0 \
           libatk-bridge2.0-0 libatspi2.0-0 libx11-6 libxcomposite1 \
           libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 \
           libxkbcommon0 libasound2 libcups2 libpango-1.0-0 libcairo2 \
           fonts-liberation fonts-dejavu-core && \
       rm -rf /var/lib/apt/lists/*
   ```
   Deliberately not `playwright install --with-deps` (that's what failed above) — install the browser binary and the runtime library/font list explicitly instead. If `libcups2`/`libpango-1.0-0`/`libcairo2` turn out already covered by another dependency's install step by the time this lands, that's fine — redundant `apt-get install` of an already-present package is a no-op.
2. Confirm `ARG PLAYWRIGHT_VERSION` matches the exact version pinned in `web/package.json` (workstream 4, step 4) — Playwright's browser binary and npm package versions must match exactly or the test run refuses to launch.
3. `task devcontainer` (builds `.devcontainer/Dockerfile`) to confirm the image still builds.
4. Inside a container built from the new image, `cd web && npm ci && npx playwright test` (the mocked suite from workstream 4) to confirm browsers launch and render text correctly without any manual apt steps.

**Rollback**: revert the `.devcontainer/Dockerfile` hunk; nothing else depends on it.

## 6. Frontend live (e2e-layer) Playwright suite

The top of the frontend pyramid: same user flows as workstream 4's mocked specs, run against the real `task services:cicd` stack instead of fixtures. Assertions here are structural rather than exact-value where real cluster state varies (application names/health/sync are whatever `integration/kubernetes/argocd` actually contains at run time), but the flow coverage should match the mocked suite's breadth — not a thin "smoke" subset — since CI time isn't the constraint here.

**New files**: `web/e2e/live/*.spec.ts`, `web/playwright.live.config.ts` (`testDir: 'e2e/live'`, `use: { baseURL: 'http://localhost:8081' }`, no `webServer` block — the caller is responsible for `task services:cicd` already being up).

- `e2e/live/home.spec.ts` — `/` loads, both cards render, submitting "Applications" with no labels navigates to `/applications/?...` without error, dark-mode toggle works.
- `e2e/live/applications.spec.ts` — `/applications/?refresh=false` loads, every configured ArgoCD tab is present with a non-empty table, clicking the "Applications" column header re-renders without throwing, health/sync cells render a recognized icon+text pair (don't assert which one — real state), no `pageerror` events fire during the whole flow.
- `e2e/live/diffs.spec.ts` — `/diffs/` loads, nested tabs render for every real application, the Manifests accordion expands and shows non-empty YAML for at least one application, the refresh-diff button re-issues a real diff request and the content updates, no `pageerror` events fire.

**Steps**

1. Write the three live specs and `playwright.live.config.ts`.
2. Add Task wiring in `tasks/ts.yaml`:
   ```yaml
   test:e2e:live:
     desc: Run the live-stack Playwright suite against task services:cicd.
     dir: web
     cmds:
       - npx playwright test --config=playwright.live.config.ts
   ```
   and a root-level convenience task in `Taskfile.yaml`:
   ```yaml
   e2e:live:
     desc: Full live-stack e2e run (brings up services, runs Playwright, tears down).
     cmds:
       - task: services:cicd
       - task: ts:test:e2e:live
       - task: k8s:cluster:delete
   ```
3. `task e2e:live` locally to confirm the full bring-up/test/teardown cycle passes against a real cluster.
4. Add a JUnit reporter to `playwright.live.config.ts` too, for workstream 8.

**Rollback**: delete `web/e2e/live/`, `web/playwright.live.config.ts`, and the two new tasks; workstreams 4–5 are unaffected.

## 7. `ci.yaml` restructure

**Path filter** (new first job, using [`dorny/paths-filter`](https://github.com/dorny/paths-filter) pinned to a specific version per AGENTS.md's pinning rule — `v4`, not `v3`: checked via `gh api repos/dorny/paths-filter/releases/latest` at plan-review time, `v4.0.3` is current and includes a security fix (escaping multi-line filenames in list-files output) over `v3`, with no filter-syntax changes affecting this config):

```yaml
jobs:
  filter:
    runs-on: ubuntu-latest
    outputs:
      go: ${{ steps.filter.outputs.go }}
      ts: ${{ steps.filter.outputs.ts }}
      docs: ${{ steps.filter.outputs.docs }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v4
        id: filter
        with:
          filters: |
            go:
              - '**/*.go'
              - 'go.mod'
              - 'go.sum'
              - 'Taskfile.yaml'
              - 'tasks/go.yaml'
              - 'tasks/docker.yaml'
              - 'tasks/k8s.yaml'
              - 'tasks/argocd.yaml'
              - 'Dockerfile'
              - '.github/workflows/ci.yaml'
            ts:
              - 'web/**'
              - '.github/workflows/ci.yaml'
            docs:
              - 'docs/**'
              - '!docs/agents/**'
              - 'mkdocs.yml'
              - 'requirements.txt'
              - '.github/workflows/ci.yaml'
```

Changing `.github/workflows/ci.yaml` itself trips all three filters deliberately — a CI logic change should re-verify every path, not rely on whatever else happened to be touched in the same PR.

**`go` job** — gains `if: needs.filter.outputs.go == 'true'`, needs: `[filter]`, absorbs the old `format` job as its first step (fail fast before installing the rest of the Go toolchain-adjacent tools), drops the `task services:cicd`/live-cluster steps entirely (those move to `e2e`), and swaps Codecov for octocov (workstream 10) — same `coverage.out`/`coverage.html` outputs from `task go:test-ci`, no format-conversion step needed:

```yaml
  go:
    needs: [filter]
    if: needs.filter.outputs.go == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: 1.27 }
      - name: Check formatting
        run: |
          gofmt -d .
          test -z "$(gofmt -l .)"
      - uses: arduino/setup-task@v2
      - run: task go:install-ci
      - run: task go:generate
      - run: task go:lint
      - run: task go:test-ci
      - uses: EnricoMi/publish-unit-test-result-action@v2
        if: always()
        with: { files: report.xml }
      - uses: k1LoW/octocov-action@v1
        with: { config: .octocov.yml }
      - uses: actions/upload-artifact@v4
        if: always()
        with: { name: go-coverage-report, path: ./coverage.html }
      - uses: actions/upload-artifact@v4
        if: always()
        with: { name: go-junit-report, path: ./report.xml }
```

**`ts` job** — gains `if: needs.filter.outputs.ts == 'true'`, `needs: [filter]`; add the mocked Playwright suite (workstream 4) after the existing build step, and upload its JUnit output too:

```yaml
  ts:
    needs: [filter]
    if: needs.filter.outputs.ts == 'true'
    runs-on: ubuntu-latest
    steps:
      # ...existing checkout/setup-node/install/lint/build steps, unchanged...
      - name: Install Playwright browser
        run: node node_modules/playwright/cli.js install --with-deps chromium chromium-headless-shell
        working-directory: web
      - name: Run unit tests
        run: task ts:test:unit  # existing `npm run test:unit -- --run`, exposed at Task level if not already
      - name: Run mocked e2e tests
        run: task ts:test:e2e
      - uses: actions/upload-artifact@v4
        if: always()
        with: { name: ts-junit-report, path: web/test-results/junit.xml }
```

**`docs` job** — gains `if: needs.filter.outputs.docs == 'true'`, `needs: [filter]`; steps unchanged otherwise.

**`e2e` job** — new, **no `if:`** (always runs on every push/PR per this plan's decision), combines the Go live-ArgoCD suite and the frontend live Playwright suite against one cluster bring-up:

```yaml
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: 1.27 }
      - uses: actions/setup-node@v4
        with: { node-version: '24' }
      - uses: arduino/setup-task@v2
      - name: Install k3d
        run: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.9.0 bash
      - name: Install argocd
        run: |
          curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/download/v3.5.3/argocd-linux-amd64
          sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
          rm argocd-linux-amd64
      - run: task go:install-ci
      - run: task go:generate
      - name: Bring up live stack
        run: task services:cicd
      - name: Run Go e2e tests
        run: task go:test:e2e:ci
      - name: Install Playwright browser
        run: node node_modules/playwright/cli.js install --with-deps chromium
        working-directory: web
      - name: Run frontend live e2e suite
        run: task ts:test:e2e:live
      - uses: EnricoMi/publish-unit-test-result-action@v2
        if: always()
        with: { files: report-e2e.xml }
      - uses: actions/upload-artifact@v4
        if: always()
        with: { name: e2e-junit-report, path: ./report-e2e.xml }
```

`tangle-server`'s container already serves the built SPA on the same `:8081` the API is on (`internal/tangle/server.go`'s `http.FileServer(http.Dir("./build"))`), which is exactly what `playwright.live.config.ts` (`baseURL: 'http://localhost:8081'`, no `webServer` block) assumes — `task services:cicd`'s `docker:build` step already builds `./build` from the current checkout, so the live suite always tests the same code the rest of the PR does.

Note the deliberate omission of `crazy-max/ghaction-setup-docker@v4` here, even though today's `go` job has it ahead of its own `services:cicd` call: `ubuntu-latest` already ships Docker Client/Server 28.0.4 and Docker-Buildx 0.37.0 preinstalled (per [`actions/runner-images`'s Ubuntu 24.04 software manifest](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md), checked via `gh api` at plan-review time) — there's no version gap that action was ever documented as closing in this repo (no comment, no ADR, and its own history shows it was scaffolded in at project bootstrap with no rationale recorded), and dropping it is what `docker/setup-buildx-action`'s own usage example assumes: a plain `ubuntu-latest` runner, no separate Docker install step. This also removes workstream 12's open compatibility caveat about the two actions' interaction outright, rather than leaving it to verify at runtime.

**Steps**

1. Add the `filter` job.
2. Rewrite `go`, `ts`, `docs` with `needs`/`if`; confirm locally (`act` or just careful reading) that the filter expressions match intent.
3. Add the `e2e` job; confirm `task go:test:e2e:ci` and `task ts:test:e2e:live` both exist (workstreams 3 and 6) before wiring them in.
4. Delete the standalone `format` job.
5. Push a branch touching only `docs/agents/**` — confirm `go`/`ts`/`docs` all skip (green, skipped) and `e2e` still runs. Push a branch touching only `web/**` — confirm `go`/`docs` skip, `ts` and `e2e` run. Push a branch touching only `internal/**` — confirm `ts`/`docs` skip, `go` and `e2e` run.

**Rollback**: revert `ci.yaml` to the pre-restructure version (git history); workstreams 1–6 (test rewrites, Task wiring) are unaffected and still valid on their own — only this workstream's CI wiring reverts.

## 8. Unified test reporting

Each job (`go`, `ts`, `e2e`) already uploads its own JUnit-format report as a build artifact (workstream 7). Add a final job that waits on all three and publishes one combined result:

```yaml
  report:
    needs: [go, ts, e2e]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          pattern: '*-junit-report'
          path: reports
          merge-multiple: true
      - uses: EnricoMi/publish-unit-test-result-action@v2
        if: always()
        with:
          files: reports/*.xml
          check_name: 'Unified Test Results'
```

`needs: [go, ts, e2e]` with `if: always()` means `report` still runs even when `go`/`ts` were skipped by the path filter (a skipped `needs` job doesn't fail the dependent job by default) — `download-artifact`'s `pattern` glob simply finds fewer files on those runs, and `publish-unit-test-result-action` reports on whatever it's given.

**If this turns out not to work cleanly** (e.g., `EnricoMi/publish-unit-test-result-action` doesn't merge Playwright's JUnit format and Go's cleanly into one meaningful summary, or the per-job publishes already give enough visibility and a fourth job is just noise) — per the original ask, drop this workstream. The per-job `EnricoMi/publish-unit-test-result-action` calls (workstream 7) already give PR-level test annotations without it; this workstream is additive polish, not load-bearing.

**Steps**

1. Confirm `web/playwright.config.ts` (workstream 4) and `web/playwright.live.config.ts` (workstream 6) both have a JUnit reporter configured.
2. Add the `report` job.
3. Push a branch, confirm one combined check appears summarizing Go + TS + e2e results, including on a run where `go`/`ts` were skipped.
4. If the combined view isn't actually useful in practice, delete this workstream's job and keep the three per-job publishes from workstream 7.

**Rollback**: delete the `report` job; the three per-job `EnricoMi/publish-unit-test-result-action` steps from workstream 7 are independent and keep working.

## 9. Update `docs/agents/ci.md`

Once workstreams 1–8 and 12 land, rewrite `docs/agents/ci.md`'s pipeline diagram and prose to reflect: the `filter` job and its outputs; `go`/`ts`/`docs` as conditional; `format` folded into `go`; the always-run `e2e` job combining Go's live-ArgoCD tests and the frontend live Playwright suite; the `report` job if workstream 8 is kept; the full test pyramid shape (unit/integration/e2e for both languages); the Go-module/build cache, npm cache, Playwright-browser cache, k3d/argocd CLI cache, and Docker-layer (buildx + GHA backend) cache from workstream 12; and, once workstreams 10–11 land too, the `go` job's coverage reporting switching from Codecov to octocov (no more `gcov2lcov` conversion step or `CODECOV_TOKEN`, plus the new `artifact://` datastore and its diff comment) and a brief pointer to the separate `octocov-central` repository behind the coverage badge. Update its "Things worth revisiting" section too — this restructure directly resolves the "no job dependencies/ordering," "format provides no fail-fast benefit," and "no caching of Go modules, npm packages, or the k3d/ArgoCD CLI downloads" items it already lists (the last one fully now, image layers included); note what's still open (`services:cicd` still stands up a whole k3d+ArgoCD cluster every `e2e` run — workstream 12 makes the pieces of that cheaper to rebuild, but doesn't remove the cluster bring-up itself, which [ADR 0009](../../adrs/0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) already accepts as a deliberate trade-off) and that the `e2e` job's wall-clock cost overall is accepted for coverage's sake, not an oversight.

## 10. Replace Codecov with octocov

Per [ADR 0010](../../adrs/0010-replace-codecov-with-octocov.md), swap the `go` job's coverage reporting from Codecov (external SaaS, `CODECOV_TOKEN` secret, `jandelgado/gcov2lcov-action` format-conversion step) to [octocov](https://github.com/k1LoW/octocov) via [`k1LoW/octocov-action`](https://github.com/k1LoW/octocov-action) — a self-contained binary that reads Go's native coverage profile directly, posts PR comments using the workflow's own `GITHUB_TOKEN`, and persists each run's report to a GitHub Actions Artifacts datastore so PR comments can show a diff against the base branch's coverage, not just the current run's total.

**New file**: `.octocov.yml` at the repo root:

```yaml
coverage:
  path: coverage.out
report:
  datastores:
    - artifact://${GITHUB_REPOSITORY}
  if: is_default_branch
diff:
  datastores:
    - artifact://${GITHUB_REPOSITORY}
  if: is_pull_request
comment:
  if: is_pull_request
```

`coverage.path` matches `tasks/go.yaml`'s `COVERAGE_RAW` var (`coverage.out`), unchanged by this workstream. `report.if: is_default_branch` persists a report only from `main` runs — that's the baseline everything else diffs against, so PR runs don't pollute the datastore with their own transient reports. `diff.if: is_pull_request` reads that persisted baseline back and compares it against the current PR's coverage. `comment.if: is_pull_request` limits the PR-comment step to pull request runs, so a push straight to `main` doesn't try to comment on a nonexistent PR. `artifact://${GITHUB_REPOSITORY}` needs no owner/repo literal — `${GITHUB_REPOSITORY}` is already set in the Actions environment to `ivanklee86/tangle`.

**Steps**

1. Add `.octocov.yml`.
2. In `ci.yaml`'s `go` job (workstream 7), replace the `jandelgado/gcov2lcov-action` + `codecov/codecov-action` steps with a single `k1LoW/octocov-action@v1` step (the action itself pinned per AGENTS.md), configured via `with: { config: .octocov.yml, version: v0.79.0 }` — the `version:` input pins the octocov *binary* the action installs, which otherwise defaults to `latest` and would violate AGENTS.md's pinning rule on its own even with the action pinned. (`v0.79.0` checked against `gh api repos/k1LoW/octocov/releases/latest` at plan-review time — it's current, so no bump needed before implementation.)
3. Drop the `CODECOV_TOKEN` reference entirely — `octocov-action` authenticates both the PR comment and the artifact datastore reads/writes with the workflow's default `GITHUB_TOKEN` (the action's own `github-token` input already defaults to `${{ github.token }}`), which `ci.yaml` already grants `pull-requests: write` at the top-level `permissions:` block (used today by `EnricoMi/publish-unit-test-result-action`) — artifact read/write needs no additional scope beyond the default token's own repo access, so no new permission is needed.
4. Merge a trivial Go change to `main` first (so `report.if: is_default_branch` has something to persist), then push a branch that changes coverage and confirm the `go` job's PR comment shows a diff against that baseline, and that `coverage.html` still uploads as a build artifact unchanged.
5. Once confirmed, remove the now-unused `CODECOV_TOKEN` secret from the repository's GitHub settings, and consider removing/archiving the Codecov project integration itself — both are manual follow-ups outside this repo's version control, not file changes this plan makes.

**Out of scope**: a publicly embeddable coverage badge and durable (beyond GitHub's Actions-artifact retention window, 90 days by default) coverage history — the `artifact://` datastore is private and expiring by nature. See [workstream 11](#11-stand-up-the-octocov-central-repository) and [ADR 0011](../../adrs/0011-octocov-central-reporting-and-badges-repository.md).

**Rollback**: revert `.octocov.yml` and the `ci.yaml` step swap; `jandelgado/gcov2lcov-action`/`codecov/codecov-action` keep working as before as long as `CODECOV_TOKEN` hasn't been deleted from repo secrets yet.

## 11. Stand up the octocov central repository

**Status: done**, with one deliberate deviation from the design below — see "What actually shipped."

Per [ADR 0011](../../adrs/0011-octocov-central-reporting-and-badges-repository.md), create a new public repository, `ivanklee86/octocov-central`, running octocov's "central mode" on a schedule to turn workstream 10's `artifact://` reports into a publicly embeddable coverage badge and a browsable dashboard, published via GitHub Pages. **Almost none of this workstream's steps touch the `tangle` repository** — it's a separate repo, created and maintained independently; the one `tangle`-side change is step 6 below.

**In the new `octocov-central` repository**

1. Create the repository (`ivanklee86/octocov-central`, public) — a manual/one-time step, not part of this plan's file changes to `tangle`. Done via `gh repo create`.
2. Add `.octocov.yml`:
   ```yaml
   central:
     root: .
     reports:
       datastores:
         - artifact://ivanklee86/tangle
     badges:
       datastores:
         - local://badges
   ```
   (`central.reports.datastores` is a list — more source repos are added later as more `artifact://owner/repo` entries, no structural change needed. No `push:` key — see "What actually shipped.")
3. Add a scheduled workflow, `.github/workflows/central.yaml` — see "What actually shipped" for the real (not `k1LoW/octocov-action`-based) version that ended up working.
4. Create a fine-grained PAT scoped to read-only `actions` access on `ivanklee86/tangle` only, and store it as the `TANGLE_ARTIFACTS_TOKEN` secret in `octocov-central`'s repo settings — a manual step the user did themselves (PAT creation has no API; it's web-UI-only).
5. Enable GitHub Pages on `octocov-central`, serving the generated dashboard (`central.root`'s output) and badge SVGs (`central.badges.datastores`' `local://badges` path). Done via `gh api repos/ivanklee86/octocov-central/pages` — live at `https://ivanklee86.github.io/octocov-central/`.
6. **In `tangle`**: add a coverage badge to `README.md` linking to `octocov-central`'s published badge URL — **not done yet**, deliberately: `badges/coverage.svg` doesn't exist yet, since it needs `tangle`'s `go` job to have run on `main` at least once first (workstream 10's `report.if: is_default_branch`), which hasn't happened (this plan's PR hadn't merged as of this writing). Land this once that badge is confirmed to actually render.

**What actually shipped, and why it differs from the design above**

The original design ran octocov via `k1LoW/octocov-action` with `central.push` configured to have octocov commit and push its own generated dashboard/badges back to the repo. That never worked: octocov's `central.push` (`gh.PushUsingLocalGit`, a go-git-based push) consistently failed with `authorization failed: Permission to ivanklee86/octocov-central.git denied to ivanklee86`, reproducibly, regardless of:

- which token backed `GITHUB_TOKEN` (the workflow's default token, with `permissions: contents: write` and the repo's workflow-permissions ceiling explicitly raised to `write` — both checked and fixed along the way; still failed),
- whether `GITHUB_TOKEN` was set at step level vs. job level (ruling out one guess about composite-action env relay — a same-job A/B test running the identical octocov binary+config directly in a plain step, bypassing `k1LoW/octocov-action` entirely, produced the exact same failure once there was actually something new to push, disproving that theory once a vacuous first "success" — nothing to commit that run — was caught and re-tested properly), or
- `actions/checkout`'s `persist-credentials` setting (ruling out a credential-header conflict).

A plain `git push` using the identical `GITHUB_TOKEN` value, from the same job, succeeded immediately — with either `octocov` (octocov's own hardcoded username) or `x-access-token` as the Basic Auth username, ruling that out too. The failure is specific to go-git's HTTP transport in this environment, not to any of the more obvious suspects (permissions, token identity, or username).

Given that, `.octocov.yml` omits `push:` entirely — octocov then just writes `README.md`/`badges/*.svg` to the local checkout and skips pushing (the same graceful-skip path already used when `central.reReport` is unset; logged as `Skip commit and push central report: ...`), and the workflow commits and pushes those files itself with plain `git`, using a token it already proved works. The real `.github/workflows/central.yaml`:

```yaml
name: Central
on:
  schedule:
    - cron: '0 6 * * *'  # daily
  workflow_dispatch: {}
permissions:
  contents: write
jobs:
  central:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install octocov
        run: |
          curl -sSL https://github.com/k1LoW/octocov/releases/download/v0.79.0/octocov_v0.79.0_linux_amd64.tar.gz -o /tmp/octocov.tar.gz
          tar -xzf /tmp/octocov.tar.gz -C /tmp octocov
          sudo install -m 755 /tmp/octocov /usr/local/bin/octocov
      - name: Run octocov (central mode)
        env:
          OCTOCOV_GITHUB_TOKEN: ${{ secrets.TANGLE_ARTIFACTS_TOKEN }}
        run: octocov --config=.octocov.yml
      - name: Commit and push the regenerated dashboard/badges
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          if git diff --quiet && git diff --cached --quiet; then
            echo "Nothing to commit."
            exit 0
          fi
          git add -A
          git commit -m "Update by octocov [skip ci]"
          git push "https://x-access-token:${GITHUB_TOKEN}@github.com/ivanklee86/octocov-central.git" HEAD:main
```

Also needed along the way, both real gaps found by actually running this rather than by reading docs:

- `central.badges.datastores: local://badges` needs the `badges/` directory to already exist in the repo — octocov's local datastore does `os.Stat(root)` at construction time and errors (`stat .../badges: no such file or directory`) rather than creating it; only writes to files *inside* an already-validated root get `MkdirAll`'d. Fixed with a checked-in `badges/.gitkeep`.
- The repo's own "Workflow permissions" setting (`gh api repos/.../actions/permissions/workflow`) defaulted to `read`, silently capping whatever the workflow file's own `permissions:` block requested — raised to `write` via the same API. (This turned out not to be the actual fix for the push failure above, but it's a real prerequisite regardless, and worth knowing about for any repo whose account-wide default is read-only.)

Confirmed working end to end: `gh run view` shows a successful commit+push (`[main 0f29350] Update by octocov [skip ci]`), and `https://ivanklee86.github.io/octocov-central/` serves the regenerated dashboard live (HTTP 200, Jekyll-rendered from the committed `README.md`). The dashboard's repository table is currently empty — expected, not a bug: it reflects `ivanklee86/tangle` having no persisted `artifact://` report yet, since that only happens on a `main`-branch run of `tangle`'s `go` job (workstream 10), which hasn't happened yet.

**Steps**

1. ~~Land `octocov-central`'s `.octocov.yml` and workflow, confirm `workflow_dispatch` runs successfully end-to-end~~ Done, per above (with the plain-git-push design, not `central.push`).
2. ~~Enable the `on.schedule` trigger once the manual run is confirmed working~~ Done — it was in the workflow file from the start; no separate enablement step turned out to be needed.
3. ~~Enable GitHub Pages, confirm the badge URL resolves publicly (unauthenticated) and renders~~ Done — Pages is live; no badge exists yet since there's no coverage data yet (see above).
4. Add the badge to `tangle`'s `README.md` — **remaining**: do this once `tangle`'s `main` branch has run the `go` job at least once and a subsequent `octocov-central` run (manual `workflow_dispatch` is fine, no need to wait for the daily schedule) produces a real `badges/coverage.svg`.

**Rollback**: delete or archive the `octocov-central` repository and the `TANGLE_ARTIFACTS_TOKEN` secret; remove the badge line from `tangle`'s `README.md` (once added). Workstream 10's artifact datastore and PR-diff comment are unaffected either way — they don't depend on `octocov-central` existing.

## 12. CI dependency caching

Closes the last open item on `docs/agents/ci.md`'s "Things worth revisiting" list: "No caching of Go modules, npm packages, or the k3d/ArgoCD CLI downloads across runs." Five pieces: the first four are pure additions to the job YAML from workstream 7 (no Task or application code changes); the fifth (Docker layer caching) does touch `tasks/docker.yaml`, deliberately, per its own section below.

**Go modules/build cache — already on by default.** `actions/setup-go@v5`'s `cache` input defaults to `true` (confirmed by reading the action's `action.yml` via `gh api repos/actions/setup-go/contents/action.yml`) and caches `~/go/pkg/mod` plus the build cache, keyed on `go.sum`'s hash. This has been silently active in `ci.yaml`'s `go` job since it adopted `setup-go@v5` — no code change needed beyond making it explicit for anyone reading the workflow:

```yaml
  - uses: actions/setup-go@v5
    with:
      go-version: 1.27
      cache: true   # default; explicit for clarity
```

What that cache does *not* cover: the four pinned tool binaries `task go:install-ci` builds via `go install` (`go-junit-report`, `golangci-lint`, `gomplate`, `swagger`) — these compile from source on every run regardless. Cache `~/go/bin` separately, keyed on the exact pinned versions in `tasks/go.yaml` so a version bump there invalidates the cache automatically, in both `go` and `e2e` (both run `task go:install-ci`):

```yaml
  - uses: actions/cache@v6
    with:
      path: ~/go/bin
      key: go-tools-${{ hashFiles('tasks/go.yaml') }}
```

**npm cache — opt-in, unlike Go.** `actions/setup-node@v4`'s `cache` input is *not* on by default (same `action.yml`-read method confirms this) and needs an explicit dependency-file path since `package.json` isn't at the repo root:

```yaml
  - uses: actions/setup-node@v4
    with:
      node-version: '24'
      cache: 'npm'
      cache-dependency-path: web/package-lock.json
```

`task ts:install` still runs plain `npm install`, unchanged — this only warms npm's own tarball cache, it doesn't skip the install step or require switching to `npm ci`.

**Playwright browser cache.** The "Install Playwright browser" step downloads Chromium + chromium-headless-shell (100MB+) fresh every run, in both `ts` and `e2e`. Cache `~/.cache/ms-playwright`, keyed on the exact pinned version so a `@playwright/test` bump invalidates it automatically, and skip the install step outright on a hit:

```yaml
  - uses: actions/cache@v6
    id: playwright-cache
    with:
      path: ~/.cache/ms-playwright
      key: playwright-${{ runner.os }}-1.63.0
  - name: Install Playwright browser
    if: steps.playwright-cache.outputs.cache-hit != 'true'
    run: node node_modules/playwright/cli.js install --with-deps chromium chromium-headless-shell
    working-directory: web
```

Using the literal version string in the key (rather than hashing `web/package.json`) is deliberate — it needs to match exactly what's installed, and a stray unrelated `package.json` change (say, an eslint plugin bump) shouldn't accidentally bust this cache. `--with-deps` still apt-installs system libraries on every run regardless of cache-hit, since those live outside `~/.cache/ms-playwright` — accepted as-is; apt-get is fast enough on `ubuntu-latest` that a second caching mechanism for it isn't worth the complexity.

**k3d / argocd CLI cache.** `e2e`'s "Install k3d"/"Install argocd" steps curl-download pinned-version binaries every run. Cache `/usr/local/bin/k3d` and `/usr/local/bin/argocd` under one combined key (simplest, since the two install steps are adjacent and neither cache is useful alone) and skip both installs on a hit:

```yaml
  - uses: actions/cache@v6
    id: cli-tools-cache
    with:
      path: |
        /usr/local/bin/k3d
        /usr/local/bin/argocd
      key: cli-tools-k3d-v5.9.0-argocd-v3.5.3
  - name: Install k3d
    if: steps.cli-tools-cache.outputs.cache-hit != 'true'
    run: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.9.0 bash
  - name: Install argocd
    if: steps.cli-tools-cache.outputs.cache-hit != 'true'
    run: |
      curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/download/v3.5.3/argocd-linux-amd64
      sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
      rm argocd-linux-amd64
```

Both version strings are already hardcoded in this job's steps and duplicated in `.devcontainer/Dockerfile`'s `K3D_VERSION` arg and `argocd` base-image tag — nothing enforces the cache key staying in sync if either tool's pin is bumped later, so land this with a comment in the workflow itself calling that out, the same way the Dockerfile already comments "keep in sync."

**Docker layer cache (buildx + the GitHub Actions cache backend).** `task docker:build` (`tasks/docker.yaml`) runs a plain `docker build -f Dockerfile -t tangle .` — every `e2e` run rebuilds `Dockerfile`'s three stages (`go mod download` + Go build, `npm install` + `npm run build`, the Alpine runtime copy) from scratch. Unlike the other four pieces, this one changes *how the image gets built*, not just what wraps it — but `docker:build` is the same task a laptop runs locally (`task services:cicd` depends on it there too), so the change has to work identically, cache-less, for local dev.

Per [Docker's own GHA-cache-backend docs](https://docs.docker.com/manuals/build/cache/backends/gha/) (fetched directly via `gh api repos/docker/docs/contents/content/manuals/build/cache/backends/gha.md` to confirm the exact mechanics before committing to this design): the `type=gha` cache backend (a) is not supported by the default `docker` build driver — it needs a `docker-container`-driver builder, which [`docker/setup-buildx-action`](https://github.com/docker/setup-buildx-action) creates and activates by default; and (b) needs `ACTIONS_CACHE_URL`/`ACTIONS_RUNTIME_TOKEN` env vars that `docker/build-push-action` sets automatically but a bare `docker buildx build` CLI invocation does not — Docker's docs explicitly recommend [`crazy-max/ghaction-github-runtime`](https://github.com/crazy-max/ghaction-github-runtime) to expose them for exactly this "calling buildx directly from an inline step" case. Since `task docker:build` is a bare CLI invocation (by design — it's also the local-dev path), this plan uses that action rather than switching the build step over to `docker/build-push-action`, which would mean the `e2e` job no longer calls `task services:cicd`'s own build step at all.

**`tasks/docker.yaml`**, `build` task — swap `docker build` for `docker buildx build --load`, with an optional cache-flags passthrough via a plain shell env var (not a Task `vars:` default) so the exact same command line runs locally (empty, i.e. today's behavior via the default `docker`-driver builder) and in CI (populated):

```yaml
tasks:
  build:
    desc: Build Dockerfile.
    cmds:
      - docker buildx build -f Dockerfile -t tangle --load ${DOCKER_BUILD_CACHE_FLAGS:-} .
```

`--load` is required with `buildx` (unlike classic `docker build`, which loads into the local daemon implicitly) so `docker run tangle` / `task services:cicd`'s `docker run` step keeps working unchanged. `${DOCKER_BUILD_CACHE_FLAGS:-}` is deliberately unquoted — when unset it expands to nothing, and when CI sets it to two flags, unquoted expansion is what word-splits them into separate arguments.

**`ci.yaml`'s `e2e` job** — add a builder and the runtime-exposing action right after `arduino/setup-task@v2` (no separate Docker-install step needed first — see workstream 7's note on why `crazy-max/ghaction-setup-docker@v4` is deliberately not carried into this job), then set the cache flags as an env var on the step that calls `task services:cicd`:

```yaml
      - uses: arduino/setup-task@v2
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v4
      - name: Expose GitHub Actions cache variables for buildx
        uses: crazy-max/ghaction-github-runtime@v4
      # ...Install k3d / Install argocd / go:install-ci / go:generate, unchanged...
      - name: Bring up live stack
        run: task services:cicd
        env:
          DOCKER_BUILD_CACHE_FLAGS: --cache-from type=gha --cache-to type=gha,mode=max
```

`mode=max` (rather than the default `mode=min`) is the point here specifically because `Dockerfile` is multi-stage: `mode=min` only caches the final exported image's own layers, missing the `go`/`node` builder stages entirely — exactly where the expensive `go mod download`/`npm install` steps live. `mode=min` would build a working cache that never actually speeds up the two slowest steps.

**Caveats worth knowing going in** (from the same docs page): GitHub's cache eviction/usage limits still apply (same family of constraint as workstream 12's other `actions/cache` entries), and cache writes are read-only in some default-branch trigger contexts — neither is a reason not to do this, both are just why a cache "miss" occasionally happens even right after a "hit" run.

**Steps**

1. Add the explicit `cache: true` to both `setup-go@v5` steps (`go`, `e2e`) — no functional change, documents the existing default.
2. Add the `~/go/bin` tool cache before `task go:install-ci` in both jobs.
3. Add `cache: 'npm'`/`cache-dependency-path` to `ts`'s `setup-node@v4` step.
4. Add the Playwright browser cache (with its conditional install step) to both `ts` and `e2e`.
5. Add the k3d/argocd CLI cache (with its conditional install steps) to `e2e`.
6. Change `tasks/docker.yaml`'s `build` task to `docker buildx build --load ${DOCKER_BUILD_CACHE_FLAGS:-}`; run `task docker:build` locally (no env var set) and confirm the image still builds and `task services:cicd` still runs it, unchanged from today.
7. Add `docker/setup-buildx-action@v4` and `crazy-max/ghaction-github-runtime@v4` to `e2e`, and `DOCKER_BUILD_CACHE_FLAGS` to its "Bring up live stack" step's `env:`.
8. Push a branch and run CI twice in a row without touching anything cache-relevant; confirm the second run's logs show cache hits (`Cache restored from key: ...` for the `actions/cache` entries, `CACHED` layers and/or a shorter "exporting to GitHub Actions Cache" duration in the buildx build output) and that the tool/browser/CLI install steps are skipped or measurably faster.
9. Deliberately bump one pinned version in a throwaway test (e.g. `golangci-lint`'s pin in `tasks/go.yaml`, or a line in `Dockerfile`'s Go stage) and confirm only that cache misses on the next run while the others still hit — this verifies the keys/layers are scoped correctly rather than always regenerating or never invalidating.

**Rollback**: revert the added cache steps and the `if: steps.*.outputs.cache-hit != 'true'` conditionals back to always-run installs; revert `tasks/docker.yaml`'s `build` task to plain `docker build` and drop the two buildx-related actions from `e2e`. No other workstream depends on caching being present — everything still works, just slower. The pieces are independently revertable (e.g. keep the Go/npm/Playwright/CLI caches while reverting just the Docker layer cache, if that one turns out flaky).

## Sequencing notes

- Do 1 → 2 → 3 in order per package (argocd, then tangle, then cli/cmd) so each commit's `go test ./...` stays green throughout, matching this repo's one-workstream-per-commit convention ([kubernetes-stack-upgrade.md](kubernetes-stack-upgrade.md)).
- Workstream 5 is already done (see its Status note) — it landed ahead of schedule, which is fine since 4 and 6 only *depend on* it, they don't need to land after it in the same pass. Do 4 → 6 next in order: the mocked suite (4) needs the working local Playwright browser 5 already provides; 6 reuses both.
- Workstreams 1–3 (Go) and 4–6 (frontend) are independent of each other — land whichever side first, including interleaved.
- Workstream 7 is the highest-risk step to get right in CI specifically (path-filter expressions, `needs`/`if` wiring) and depends on workstreams 3 and 6 for the tasks it calls — test with throwaway branches touching exactly one area at a time (per its own step 5) before merging, since a mistake here silently skips real checks rather than failing loudly.
- Workstream 8 is optional per the original ask ("if that's not possible we can drop it") — don't let it block 1–7 from landing.
- Workstreams 10–11 (octocov) are independent of everything else in this plan and can land before, after, or interleaved with any other workstream — workstream 10 only touches the `go` job's last few steps and a new `.octocov.yml` file, and neither interacts with the test-taxonomy or path-filtering work in workstreams 1–9.
- Do 10 → 11 in order: workstream 11's central repository reads the `artifact://` reports workstream 10 starts producing, so there's nothing for it to collect until workstream 10 is live on `main`.
- Workstream 12 (dependency caching) only touches job YAML that workstream 7 also touches, and is otherwise independent of every other workstream — land it in the same pass as 7 (simplest, since you're already editing those exact blocks) or as an immediate follow-up once 7 is on `main`. If it lands *before* 7 for some reason, apply the Go-tool/npm/Playwright/k3d-argocd cache blocks to the current (pre-restructure) `go`/`ts` jobs' existing install steps first, then carry the same blocks into the new `e2e` job when 7 lands.
- Explicitly out of scope: `release.yaml`/`release-docs.yaml` changes (ci.md's "Things worth revisiting" flags a couple of independent issues there — not part of this ask), and reducing `services:cicd`'s own k3d/ArgoCD cluster bring-up cost (workstream 12 caches its inputs, not the bring-up itself — see ADR 0009's accepted trade-off).
