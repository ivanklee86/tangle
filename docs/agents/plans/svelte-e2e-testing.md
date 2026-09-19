# Svelte frontend e2e testing

Status: proposed · 2026-09-19

Turn `web/`'s Playwright setup from a single non-passing, CI-unused spec (`e2e/demo.test.ts` waits for an `h1` no page renders) into a real e2e suite, wire Playwright into the devcontainer, add Taskfile tasks for it, and run it in CI. See [ADR 0003](../../adrs/0003-svelte-e2e-testing-strategy.md) for why this is two suites — a mocked-API suite that's the required, fast, every-PR check, and a live-stack smoke suite that's opt-in — rather than one.

Land the four workstreams below in order; each is independently useful and independently testable before moving to the next.

## 1. Write the mocked e2e suite

**Layout change** (`web/e2e/`)

| | Current | Target |
|---|---|---|
| `web/e2e/demo.test.ts` | Checks for an `h1` (fails — no page has one) | Deleted |
| `web/e2e/fixtures/` | — | New: fixture JSON matching `$lib/data.ts` types |
| `web/e2e/mocked/*.spec.ts` | — | New: the real suite, network-mocked |
| `web/playwright.config.ts` | Points `testDir: 'e2e'` at the whole (empty-ish) directory | `testDir: 'e2e/mocked'`, unchanged `webServer` (`npm run build && npm run preview`) |

**Fixtures** (`web/e2e/fixtures/applications.json`, `web/e2e/fixtures/diff.json`) — plain JSON matching the wire shapes `TangleAPIClient` (`web/src/lib/client.ts`) expects raw from the API, i.e. `ApplicationsResponse` (`{ results: ArgoCDApplicationResults[] }`) and `ApplicationDiffResponse` (`{ liveManifests, targetManifests, diffs, manifestGenerationError }`) from `web/src/lib/data.ts`. Model them on what this session's manual verification actually saw from the live cluster (two ArgoCDs `test`/`prod`, two apps each, one healthy+synced and one unhealthy+out-of-sync per ArgoCD) so the fixtures exercise both the "happy" and "alert" rendering paths (health/sync icons, the tab-title warning icon in `diffs`, sortable rows with real name/health/syncStatus variance to sort on).

**Mocking approach**: `page.route('**/api/applications*', route => route.fulfill({ json: fixture }))` and `page.route('**/api/argocd/*/applications/*/diffs', route => route.fulfill({ json: diffFixture }))` per spec, set up before `page.goto()`. `PUBLIC_BASE_URL` in `.env.production` is empty (relative fetch), so these requests are same-origin as the Playwright-driven `npm run preview` server and the glob patterns above match them without needing to know the exact origin.

**Specs** (each a `test.describe` block; use `test.beforeEach` for the common route-mock setup):

- `e2e/mocked/home.spec.ts` — `/`: both cards render with their labeled inputs; submitting "Diffs" with an empty Target Ref shows the `role="alert"` toast with the exact copy ("You must provide a target git ref to generate a diff!") and it dismisses on clicking its close button; submitting either card with a malformed label (no `:`) shows the invalid-label toast; submitting "Applications" with valid labels navigates to `/applications/?labels=...` (assert via `page.waitForURL`); dark-mode toggle button flips `html.dark` and persists across a reload (it writes to `localStorage`, per `flowbite-svelte`'s `DarkMode` component).
- `e2e/mocked/applications.spec.ts` — `/applications/?refresh=false`: tabs render one per ArgoCD with the `(count)` suffix from the fixture; switching tabs shows that ArgoCD's own table; **column sort**: click the "Applications" header once asserts row order matches ascending `name`, with the ▲ indicator appended to the header text, click again asserts descending order with ▼ — this is the custom sort reimplementation from the Tailwind/Flowbite upgrade ([ADR 0002](../../adrs/0002-upgrade-svelte-frontend-to-tailwind-v4-and-flowbite-svelte-v1.md)) and has no other test coverage; health/sync status cells render the right icon+text per fixture value (`Healthy`/`Missing`, `Synced`/`OutOfSync`); the refresh-period `Select` and refresh-toggle `Button` are present and toggling the button changes its color (`primary` vs `alternative`) — don't assert the actual polling interval fires (that's a `setInterval`, not worth e2e time; leave interval-restart logic to a future unit test per this plan's "unit tests tbd" scope note).
- `e2e/mocked/diffs.spec.ts` — `/diffs/`: nested tabs (ArgoCD → application) render; an application whose fixture `syncStatus`/`health` marks it unhealthy shows the rose `ExclamationCircleSolid` icon on its inner tab title, a healthy one doesn't; "Status" section renders the right `ArgoCDHealthStatus`/`ArgoCDSyncStatus` text; "(More Info)" link `href` matches the fixture's `url` and opens in a new tab (`target="_blank"`); the "Manifests" accordion is collapsed by default and expanding it reveals the fixture's YAML (via `svhighlight`'s `CodeBlock`); the refresh-diff `GradientButton` re-issues the mocked diff POST (assert the route handler's call count, or swap the mock's `route.fulfill` payload between clicks and assert the rendered diff text changes).

**Steps**

1. Delete `web/e2e/demo.test.ts`, create `web/e2e/fixtures/*.json` and `web/e2e/mocked/*.spec.ts` per above.
2. Update `web/playwright.config.ts`'s `testDir` to `'e2e/mocked'`.
3. `npm run test:e2e` locally (needs Playwright's browser installed — see workstream 2) until green.
4. Pin `@playwright/test`'s version in `web/package.json` exactly (currently `^1.45.3`; latest as of 2026-09-19 is 1.63.0 — re-check at implementation time) per AGENTS.md's pinning rule, since workstream 2 needs to install the exact matching browser build.

**Rollback**: revert `web/e2e/`, `web/playwright.config.ts`, and the `@playwright/test` pin; nothing outside `web/` changes in this workstream.

## 2. Devcontainer: install Playwright's browser

**Problem found during this session's manual verification**: `npx playwright install --with-deps chromium` fails outright on this devcontainer's Debian base — two of the font packages it tries to apt-install (`ttf-ubuntu-font-family`, `ttf-unifont`) aren't available from this image's apt sources. The browser binary itself downloads fine either way; without a font package installed afterward, Chromium renders real content but **all text is genuinely zero-height/invisible** (icons and colors still show), which looks exactly like a CSS bug and burned real time to diagnose in this session — worth a comment in the Dockerfile so it isn't rediscovered.

**Steps**

1. In `.devcontainer/Dockerfile`, after the existing Node setup (Playwright needs `npm`/`npx` on `PATH`, already true by that point) and before switching `USER vscode` back (the apt install needs root):
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
   Deliberately not `playwright install --with-deps` (that's what failed above) — install the browser binary and the runtime library/font list explicitly instead, confirmed working in this session (`sudo dpkg --configure -a` was needed once to recover from the failed `--with-deps` attempt's interrupted apt state; a clean image build shouldn't hit that).

   **Corrected during implementation of [the component unit-tests plan](svelte-component-unit-tests.md)** (2026-09-19), which shares this exact devcontainer prerequisite: `chromium-headless-shell` is a separate download from `chromium` (needed by `@vitest/browser-playwright`, and Playwright's own e2e runner can use it too for faster headless runs), and `libcups2`/`libpango-1.0-0`/`libcairo2` were missing from the original researched list — only surfaced by Playwright's `Host system is missing dependencies` warning when actually launching a browser, not by an install-only check. If this workstream is implemented after that plan already landed the Dockerfile change, this step is already done — skip to confirming it, don't redo the research.
2. Confirm `ARG PLAYWRIGHT_VERSION` matches the exact version pinned in `web/package.json` (workstream 1, step 4) — Playwright's browser binary and npm package versions must match exactly or the test run refuses to launch.
3. `task devcontainer` (builds `.devcontainer/Dockerfile`) to confirm the image still builds.
4. Inside a container built from the new image, `cd web && npm ci && npx playwright test` (the mocked suite from workstream 1) to confirm browsers launch and render text correctly without any manual apt steps.

**Rollback**: revert the `.devcontainer/Dockerfile` hunk; nothing else depends on it.

## 3. Taskfile wiring

**`tasks/ts.yaml`**

1. Add `test:e2e` (mocked suite; effectively what `npm run test:e2e` already does, exposed at the Task level for consistency with the other `ts:*` tasks and so CI can call it the same way as everything else):
   ```yaml
   test:e2e:
     desc: Run the mocked-API Playwright e2e suite.
     dir: web
     cmds:
       - npx playwright test
   ```
   (`ts:test` already runs `npm run test:unit -- --run && npm run test:e2e`, i.e. it already covers this — this task exists so CI/devs can run just the e2e leg without the unit leg.)
2. Add `test:e2e:smoke` for the live-stack suite from [ADR 0003](../../adrs/0003-svelte-e2e-testing-strategy.md):
   ```yaml
   test:e2e:smoke:
     desc: Run the live-stack Playwright smoke suite against task services:cicd.
     dir: web
     cmds:
       - npx playwright test --config=playwright.smoke.config.ts
   ```
   This task assumes `task services:cicd` (root `Taskfile.yaml`) is already up — `tangle-server` embeds and serves `web/build` itself (`internal/tangle/server.go`'s `http.FileServer(http.Dir("./build"))` on `/*`), on the same `:8081` the API is on, so the smoke config points straight at `http://localhost:8081` and needs no `webServer`/dev-server of its own.
3. Add a convenience root-level task in `Taskfile.yaml` that sequences the live smoke run end-to-end (bring up the stack, run the suite, tear down), since `test:e2e:smoke` alone assumes the stack is already running:
   ```yaml
   e2e:smoke:
     desc: Full live-stack e2e smoke run (brings up services, runs Playwright, tears down).
     cmds:
       - task: services:cicd
       - task: ts:test:e2e:smoke
       - task: k8s:cluster:delete
   ```
   Cross-namespace task calls from inside `tasks/ts.yaml` back to root-level tasks (`services:cicd`, `k8s:cluster:delete`) use Task's root-namespace prefix (`:services:cicd`) if called from within `ts:`'s own task definitions — this top-level sequencing task avoids needing that by living in the root `Taskfile.yaml` instead, calling `ts:test:e2e:smoke` the normal namespaced way.

**Steps**

1. Add the three tasks above.
2. `task ts:test:e2e` locally — should pass using workstream 1's mocked suite and workstream 2's devcontainer browser install.
3. `task e2e:smoke` locally — brings up a real cluster, runs the (not-yet-written — workstream 4 writes the actual smoke specs) suite, tears down; safe to run with zero smoke specs present (Playwright passes trivially on an empty test dir) to confirm the plumbing before workstream 4 adds real specs.

**Rollback**: revert the three task additions; no other task depends on them yet.

## 4. Write the live smoke suite and wire CI

**Smoke suite** (`web/e2e/smoke/*.spec.ts`, `web/playwright.smoke.config.ts`)

Reuses the same user flows as workstream 1's mocked specs but with loose, structural assertions instead of exact fixture values, since this runs against whatever `integration/kubernetes/argocd` actually contains at test time:

- `e2e/smoke/applications.spec.ts` — `/applications/?refresh=false` loads, at least one tab is present, its table has at least one row, clicking the "Applications" column header doesn't throw and re-renders (don't assert order — real data may already be in any order), no `pageerror` events fire during the whole flow.
- `e2e/smoke/diffs.spec.ts` — `/diffs/` loads, nested tabs render, the Manifests accordion expands and shows non-empty YAML content for at least one application, no `pageerror` events fire.

`web/playwright.smoke.config.ts`: `testDir: 'e2e/smoke'`, `use: { baseURL: 'http://localhost:8081' }`, no `webServer` block (the caller — `task e2e:smoke` — is responsible for `task services:cicd` already being up, per workstream 3).

**CI** (`.github/workflows/ci.yaml`)

1. In the `ts` job, after the existing `Build website` step, add Playwright browser install + the mocked suite:
   ```yaml
   - name: Install Playwright browser
     # Invoke the locally installed `playwright` package directly rather
     # than `npx playwright@<version>` — this project also depends on
     # `@playwright/test`, which provides its own (older-pinned) `playwright`
     # bin under the same name, and `npx` resolves to that one instead of
     # the version requested, silently installing the wrong browser
     # revision. `node node_modules/playwright/cli.js` bypasses the bin
     # collision and always uses the exact version in web/package.json.
     run: node node_modules/playwright/cli.js install --with-deps chromium
     working-directory: web
   - name: Run e2e tests
     run: task ts:test:e2e
   ```
   `ubuntu-latest` GitHub-hosted runners are on Playwright's officially supported list, so `--with-deps` (unlike the devcontainer's Debian base in workstream 2) should work as documented here — verify at implementation time rather than assuming, and fall back to the devcontainer's explicit package list if not. This invokes `node_modules/playwright/cli.js` directly instead of `npx playwright@1.63.0` to avoid a bin-name collision with `@playwright/test`'s own `playwright` binary (see the identical fix and rationale in the `ts` job's own "Install Playwright browser" step in `.github/workflows/ci.yaml`); the version is read from `web/package.json` itself, so there's nothing here to keep in sync.
2. Add a new, separate job for the live smoke suite, gated so it does **not** run on every push/PR (per [ADR 0003](../../adrs/0003-svelte-e2e-testing-strategy.md), this is opt-in, not a required check):
   ```yaml
   on:
     push:
       branches: ['main']
     pull_request:
     workflow_dispatch: {}
     schedule:
       - cron: '0 6 * * *'   # nightly

   jobs:
     # ...existing go/ts/format/docs jobs unchanged...

     e2e-smoke:
       if: github.event_name == 'workflow_dispatch' || github.event_name == 'schedule'
       runs-on: ubuntu-latest
       steps:
       - uses: actions/checkout@v4
       - uses: actions/setup-go@v5
         with:
           go-version: 1.27
       - uses: actions/setup-node@v4
         with:
           node-version: '24'
       - uses: arduino/setup-task@v2
       - uses: crazy-max/ghaction-setup-docker@v4
       - name: Install k3d
         run: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.9.0 bash
       - name: Install argocd
         run: |
           curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/download/v3.5.3/argocd-linux-amd64
           sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
           rm argocd-linux-amd64
       - name: Install Playwright browser
         # See the `ts` job's own "Install Playwright browser" step above
         # (and its counterpart in `.github/workflows/ci.yaml`) for why this
         # invokes `node_modules/playwright/cli.js` directly instead of
         # `npx playwright@<version>`.
         run: node node_modules/playwright/cli.js install --with-deps chromium
         working-directory: web
       - name: Run live-stack e2e smoke suite
         run: task e2e:smoke
   ```
   This duplicates the `go` job's k3d/argocd/Docker setup steps rather than depending on that job, since GitHub Actions jobs don't share a filesystem/running services across jobs — `needs: go` would only sequence them, not hand off the live cluster. Revisit at implementation time whether that duplication is worth collapsing into a reusable composite action; not required for this to work.

**Steps**

1. Write the two smoke specs and `playwright.smoke.config.ts`.
2. `task e2e:smoke` locally to confirm the full bring-up/test/teardown cycle passes against a real cluster.
3. Add the CI step to the `ts` job and the new `e2e-smoke` job; update the `on:` triggers to add `workflow_dispatch` and the nightly `schedule` (additive — doesn't change when the existing required jobs run, since their `if:` stays implicit/unconditional).
4. Push a branch and confirm: the required jobs (`go`, `ts`, `format`, `docs`) still run and pass on the PR as before; `e2e-smoke` does *not* run on the PR; manually trigger it via `workflow_dispatch` (Actions tab → "Run workflow") and confirm it passes.

**Rollback**: revert the CI workflow changes and delete `web/e2e/smoke/` + `web/playwright.smoke.config.ts`; workstreams 1–3 are unaffected (the mocked suite and its CI step stand alone).

## Sequencing notes

- Do 1 → 2 → 3 → 4 in order: the mocked suite (1) needs a working local Playwright browser to develop against, which is exactly what 2 provides system-wide in the devcontainer (a contributor without the devcontainer's apt packages can still install a browser manually, but 2 is what makes it work out of the box); 3 exposes both suites as Task commands, which 4's CI steps then call rather than reinventing `npx playwright test` invocations inline.
- Commit each workstream separately, matching this repo's existing convention ([kubernetes-stack-upgrade.md](kubernetes-stack-upgrade.md), [svelte-frontend-upgrade.md](svelte-frontend-upgrade.md)) of one workstream per commit within a single PR — easy to bisect a regression to "the fixtures/specs," "the devcontainer image," "the task wiring," or "the CI workflow" specifically.
- Workstream 4's CI job is the one most likely to need iteration (real cluster bring-up in CI is exactly the kind of thing that's fine locally and flaky in a fresh runner) — don't be surprised if the smoke job needs a couple of `workflow_dispatch` runs to shake out timing issues (e.g. `tangle-server` not yet listening on `:8081` when Playwright's first request lands) that `task services:cicd`'s existing `argocd:healthcheck` step doesn't already guard against.
