# Multi-arch container image and ARM CI coverage

Status: proposed · 2026-09-21

Publish `ghcr.io/ivanklee86/tangle` as a `linux/amd64` + `linux/arm64` manifest list, and back that
claim by running the whole test pyramid on ARM — the hermetic `go` suite *and* the live
ArgoCD/browser `e2e` suite — with a non-pushing cross-build job guarding the release path. See
[ADR 0024](../../adrs/0024-multi-arch-container-image-and-arm-ci.md) for the decision record.

## Background: why cross-compilation, not emulation

Adding `platforms: linux/amd64,linux/arm64` to `docker/build-push-action` is a one-line change that
produces a correct manifest list and a bad build. Without a `--platform` pin on the build stages,
buildx runs every instruction of the arm64 leg under QEMU — `go mod download`, `go install`,
`swagger generate spec`, `go build ./...`, `npm install`, `npm run build` — none of which needs to
be native.

It does not need to be native because nothing here is architecture-bound:

```console
grep -rn 'import "C"' --include="*.go" .
# no matches: pure Go, so GOOS/GOARCH cross-compilation is exact
```

and the `node` stage's output (`/app/build`) is static assets that get *copied* into the runtime
image, never executed. So both expensive stages can be pinned to the runner's own architecture and
told what to emit, which is what buildx's
[automatic platform args](https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope)
exist for: `BUILDPLATFORM` is where the build runs, `TARGETOS`/`TARGETARCH` are what it is building
for.

One instruction genuinely cannot be cross-compiled away. The runtime stage runs
`RUN addgroup -S tangle && adduser -S tangle -G tangle`, and a `RUN` in an arm64 stage needs an
arm64 `busybox` to execute it. That is what `docker/setup-qemu-action` is for here, and it is the
*only* thing it is for — two `busybox` calls appending a line each to `/etc/passwd` and
`/etc/group`. Note that the arm64 `e2e` leg needs no QEMU: it builds natively, where
`BUILDPLATFORM == TARGETPLATFORM`. That is also exactly why it does not substitute for the `image`
job, which is the only place the cross-build is ever exercised.

## Background: the prerequisites, checked

The ARM `e2e` leg was initially deferred over three unknowns. All three were checked, and none is
a blocker — the only real gap was one hardcoded release-asset name in `ci.yaml`:

```console
gh api repos/argoproj/argo-cd/releases/tags/v3.5.3 --jq '.assets[].name' | grep linux
# argocd-linux-amd64 / argocd-linux-arm64 / argocd-linux-ppc64le / argocd-linux-s390x

gh api repos/k3d-io/k3d/releases/tags/v5.9.0 --jq '.assets[].name' | grep linux
# k3d-linux-386 / k3d-linux-amd64 / k3d-linux-arm / k3d-linux-arm64

grep -roh 'ubuntu2[0-9]\.04-arm64' web/node_modules/playwright-core/lib/ | sort -u
# ubuntu20.04-arm64 / ubuntu22.04-arm64 / ubuntu24.04-arm64

curl -s -o /dev/null -w '%{http_code}\n' -L \
  https://cdn.playwright.dev/dbazure/download/playwright/builds/chromium/1243/chromium-linux-arm64.zip
# 200   (and 200 for chromium-headless-shell-linux-arm64.zip; 1243 is 1.63.0's pinned revision)

docker buildx imagetools inspect quay.io/argoproj/argocd:v3.5.3 | grep Platform
# linux/amd64 / linux/arm64 / linux/ppc64le / linux/s390x
```

`alpine:latest`, `golang:1.27-alpine`, and `node:24-alpine` are likewise manifest lists including
`linux/arm64`. k3d's `install.sh` selects its own architecture, so only the `argocd` download needs
touching.

## 1. Rewrite the Dockerfile to cross-compile

**`Dockerfile`** — three changes: pin both build stages to `$BUILDPLATFORM`, take `TARGETOS`/
`TARGETARCH` as args, and scope `GOOS`/`GOARCH` to the `go build` command alone.

```diff
 # Stage 1: Build Go server
-FROM golang:1.27-alpine AS go
+#
+# Pinned to $BUILDPLATFORM (the runner's own architecture), not the target's.
+# This stage both cross-compiles the server and *executes* a tool it just
+# built (`swagger`, below), so it has to run natively. Nothing in this repo
+# uses cgo, so GOOS/GOARCH cross-compilation is exact rather than a best
+# effort.
+FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS go

 WORKDIR /app
 COPY go.mod go.sum ./
 RUN go mod download -x
 RUN go install github.com/go-swagger/go-swagger/cmd/swagger@v0.36.6
 COPY . .
 RUN swagger generate spec -o ./internal/tangle/swagger.json --scan-models
-RUN go build -v -ldflags "-X main.version=docker" -o . ./...
+
+# Declared here rather than at the top of the stage on purpose: buildx
+# invalidates cache from the point an ARG is *used*, so keeping these below
+# the dependency download, tool install, and swagger generation lets both
+# platform legs share those layers verbatim instead of building them twice.
+ARG TARGETOS
+ARG TARGETARCH
+
+# GOOS/GOARCH are set on this one command, deliberately not as a stage-wide
+# ENV — the `go install` above has to produce a binary this stage can run,
+# and a stage-wide GOARCH would cross-compile `swagger` and then fail to
+# execute it. CGO_ENABLED=0 is belt-and-braces: there's no cgo to disable,
+# but it makes a future cgo dependency fail here rather than silently
+# producing a dynamically-linked binary alpine can't run.
+RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
+    go build -v -ldflags "-X main.version=docker" -o . ./...

 # Stage 2: Build web assets
-FROM node:24-alpine AS node
+#
+# Also $BUILDPLATFORM: `npm run build` emits static assets that the runtime
+# image serves, never runs, so the output is identical whichever architecture
+# produced it. Emulating this stage would cost minutes for a byte-identical
+# result.
+FROM --platform=$BUILDPLATFORM node:24-alpine AS node

 WORKDIR /app
 COPY ./web .
 RUN npm install
 RUN npm run build

 # Stage 3 - Runtime Image
+#
+# No --platform pin: this one resolves per target, which is the whole point.
+# It is also the only stage with a RUN that executes on the target platform
+# (the adduser below), so it's the only reason a *cross*-build needs QEMU —
+# see the `Set up QEMU` steps in ci.yaml's `image` job and release.yaml. A
+# native arm64 build (ci.yaml's `e2e` arm64 leg) needs none.
 FROM alpine:latest AS runtime
```

The rest of the runtime stage is unchanged. `go build -o . ./...` keeps writing both binaries into
`/app` when cross-compiling, so the `COPY --from=go /app/tangle-server` / `/app/tangle-cli` lines
below need no adjustment.

## 2. Build both architectures in the release

**`.github/workflows/release.yaml`**, `docker` job — the `platforms:` input is the visible change;
the buildx setup step is the one that makes it work at all.

```diff
       - name: Checkout repository
         uses: actions/checkout@v7

+      - name: Set up QEMU
+        # Solely for the runtime stage's `adduser`, which runs on the target
+        # platform. Every compile stage is pinned to $BUILDPLATFORM and
+        # cross-compiles, so no build work is emulated.
+        uses: docker/setup-qemu-action@v3
+        with:
+          platforms: arm64
+
+      - name: Set up Docker Buildx
+        # Not optional housekeeping: the default `docker` driver cannot
+        # produce a multi-platform result, so `platforms:` below fails the
+        # build without this rather than being quietly ignored.
+        uses: docker/setup-buildx-action@v4
+
       - name: Log in to the Container registry
         uses: docker/login-action@v4
@@
       - name: Build and push Docker image
         uses: docker/build-push-action@v7
         with:
           context: .
+          platforms: linux/amd64,linux/arm64
           push: true
           tags: ${{ steps.meta.outputs.tags }}
           labels: ${{ steps.meta.outputs.labels }}
```

`docker/metadata-action`'s configuration is untouched. A manifest list carries the same tags a
single-platform image did — the registry picks the matching architecture at pull time — so
`tangle-deployments`' `image.tag` bump ([ADR 0021](../../adrs/0021-automate-tangle-deployments-chart-version-bump.md))
keeps working with no change on that side.

## 3. Matrix the `go` job

**`.github/workflows/ci.yaml`**, `go` job:

```diff
   go:
-    runs-on: ubuntu-latest
+    strategy:
+      # An arm64-only failure should still report the amd64 result, and vice
+      # versa — telling the two apart is the entire diagnostic value of the
+      # matrix, and fail-fast would throw it away exactly when it matters.
+      fail-fast: false
+      matrix:
+        include:
+          - arch: amd64
+            runner: ubuntu-latest
+          - arch: arm64
+            # Free for public repositories. There is no `ubuntu-latest-arm`;
+            # the version has to be named explicitly.
+            runner: ubuntu-24.04-arm
+    runs-on: ${{ matrix.runner }}
     steps:
```

The `arch` values are Go's `GOARCH` spellings deliberately — step 4 substitutes them straight into
an ArgoCD release-asset name.

Then the reporting steps. Every artifact upload becomes amd64-only, and the check gets an explicit,
unique name:

```diff
     - name: Publish Unit Test Results
       uses: EnricoMi/publish-unit-test-result-action@v2
       if: always()
       with:
         files: report.xml
+        # Explicit and unique. The action's default is the literal string
+        # `Test Results`, and `go` and `e2e` both currently take that default
+        # — so two check runs already share one name today, and four would
+        # after this change. `report`'s `Unified Test Results` stays the
+        # canonical merged view; these are the per-leg detail views.
+        check_name: Go Test Results (${{ matrix.arch }})
     - name: Save coverage report.
       uses: actions/upload-artifact@v7
-      if: always()
+      # amd64 only: both legs run the same suite over the same source, and
+      # `report` merges fixed artifact names (see .octocov.yml's
+      # coverage.paths). Coverage is a property of the source, not the
+      # runner, so there is nothing to merge from the arm64 leg — and
+      # uploading it would collide on artifact name.
+      if: always() && matrix.arch == 'amd64'
       with:
         name: go-coverage-report
         path: ./coverage.html
```

…and the same `&& matrix.arch == 'amd64'` on `Save coverage profile.` and `Save JUnit report.`.
`.octocov.yml` needs no change.

**Leave the `gofmt` check on both legs.** It is two shell commands and duplicating it costs
nothing; carving out an exception would add a conditional for no benefit.

## 4. Matrix the `e2e` job

**`.github/workflows/ci.yaml`**, `e2e` job — the same matrix block as step 3, plus the one
substantive fix.

```diff
   e2e:
-    runs-on: ubuntu-latest
+    strategy:
+      fail-fast: false
+      matrix:
+        include:
+          - arch: amd64
+            runner: ubuntu-latest
+          - arch: arm64
+            runner: ubuntu-24.04-arm
+    runs-on: ${{ matrix.runner }}
     env:
       K3D_VERSION: v5.9.0 # github-releases/k3d-io/k3d
       ARGOCD_VERSION: v3.5.3 # github-releases/argoproj/argo-cd
```

The `argocd` install is the only step that hardcodes an architecture. `k3d`'s `install.sh` detects
its own (and `k3d-linux-arm64` exists), so the step above it is left alone:

```diff
     - name: Install argocd
       if: steps.cli-tools-cache.outputs.cache-hit != 'true'
       run: |
-        curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/download/${{ env.ARGOCD_VERSION }}/argocd-linux-amd64
-        sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
-        rm argocd-linux-amd64
+        # argo-cd names its release assets by GOARCH (`argocd-linux-amd64`,
+        # `argocd-linux-arm64`), which is exactly what matrix.arch holds —
+        # that's why the matrix uses GOARCH spellings rather than, say,
+        # `x64`/`aarch64`.
+        asset=argocd-linux-${{ matrix.arch }}
+        curl -sSL -o "$asset" https://github.com/argoproj/argo-cd/releases/download/${{ env.ARGOCD_VERSION }}/"$asset"
+        sudo install -m 555 "$asset" /usr/local/bin/argocd
+        rm "$asset"
```

The rest of the bring-up needs nothing: `quay.io/argoproj/argocd:v3.5.3` and the k3s/Traefik images
are all manifest lists with `linux/arm64`, and `task services:cicd` builds the tangle image
natively on whichever runner it lands on.

Reporting mirrors step 3 — a unique check name on both legs, artifacts from amd64 only:

```diff
     - name: Publish Unit Test Results
       uses: EnricoMi/publish-unit-test-result-action@v2
       if: always()
       with:
         files: |
           report-e2e.xml
           web/test-results/junit-live.xml
+        check_name: E2E Test Results (${{ matrix.arch }})
     - name: Save JUnit report.
       uses: actions/upload-artifact@v7
-      if: always()
+      if: always() && matrix.arch == 'amd64'
```

…plus the same condition on `Save coverage profile.`. The arm64 leg's own JUnit output is still
fully visible in its `E2E Test Results (arm64)` check — it just doesn't enter `report`'s merge.

## 5. Fix three architecture-blind cache keys

This is the step most likely to be skipped and most expensive to skip. Three `actions/cache` keys
cache **compiled binaries** with no architecture component, and each is shared across jobs that
will now run on different architectures. `runner.os` is `Linux` on both, so it does not
discriminate — `runner.arch` (`X64` / `ARM64`) is what does.

**`go-tools-`** — written by `go` (both legs) and `e2e` (both legs), caching `~/go/bin`:

```diff
     - name: Cache Go tool binaries
       uses: actions/cache@v6
       with:
         path: ~/go/bin
-        key: go-tools-${{ hashFiles('tasks/go.yaml') }}
+        # os/arch is load-bearing, not decoration: ~/go/bin holds compiled
+        # binaries and this key is shared across four matrix legs on two
+        # architectures. Without it, whichever leg saves first hands its
+        # binaries to every other leg.
+        key: go-tools-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('tasks/go.yaml') }}
```

Apply this in **both** the `go` job and the `e2e` job.

**`cli-tools-`** — `e2e` only, caching the `k3d` and `argocd` binaries:

```diff
     - name: Cache k3d/argocd CLIs
       id: cli-tools-cache
       uses: actions/cache@v6
       with:
         path: |
           /usr/local/bin/k3d
           /usr/local/bin/argocd
-        key: cli-tools-k3d-${{ env.K3D_VERSION }}-argocd-${{ env.ARGOCD_VERSION }}
+        key: cli-tools-${{ runner.arch }}-k3d-${{ env.K3D_VERSION }}-argocd-${{ env.ARGOCD_VERSION }}
```

Without this the arm64 leg would restore amd64 `k3d`/`argocd` binaries, skip its install steps
because the cache hit, and then fail with `exec format error` — after the cluster bring-up, several
minutes in.

**`playwright-`** — written by `ts` (amd64 only) and `e2e` (both legs), caching browser binaries:

```diff
     - name: Cache Playwright browser
       id: playwright-cache
       uses: actions/cache@v6
       with:
         path: ~/.cache/ms-playwright
-        key: playwright-${{ runner.os }}-1.63.0
+        # runner.os alone is `Linux` on both architectures; browser binaries
+        # are not portable between them.
+        key: playwright-${{ runner.os }}-${{ runner.arch }}-1.63.0
```

Apply this in **both** the `ts` job and the `e2e` job — they share the key, so fixing only one
leaves the collision in place.

Finally, `e2e`'s buildx layer cache needs a per-arch scope, or the two legs export mutually useless
layers over each other:

```diff
     - name: Bring up live stack
       run: task services:cicd
       env:
-        DOCKER_BUILD_CACHE_FLAGS: --cache-from type=gha --cache-to type=gha,mode=max
+        # Per-arch scope: the two matrix legs build different-architecture
+        # layers, and the `image` job caches a two-platform build. Sharing
+        # buildx's default `buildkit` scope would have all three evicting
+        # each other on every run.
+        DOCKER_BUILD_CACHE_FLAGS: --cache-from type=gha,scope=e2e-${{ matrix.arch }} --cache-to type=gha,scope=e2e-${{ matrix.arch }},mode=max
```

`./.github/actions/setup-task` already keys on `runner.arch`
([ADR 0018](../../adrs/0018-pin-and-cache-the-task-cli-in-ci.md)), and `actions/setup-go` and
`actions/setup-node` include OS and arch in their own cache keys, so those need nothing. The three
keys above predate that habit.

## 6. Add a non-pushing multi-arch `image` job

**`.github/workflows/ci.yaml`** — new always-run job. This is *not* made redundant by the arm64
`e2e` leg: that leg builds natively, so `GOOS`/`GOARCH` are a no-op there and the cross-build the
release depends on is never attempted.

```yaml
  image:
    # Builds exactly what release.yaml publishes, minus the push — and it is
    # the only job that exercises *cross*-compilation at all (the arm64 e2e
    # leg builds natively, where BUILDPLATFORM == TARGETPLATFORM). Without
    # this, a Dockerfile change that breaks the cross-build passes every PR
    # check and fails at release time, after a tag exists and a GitHub
    # Release is already published.
    runs-on: ubuntu-latest
    steps:
    - name: Checkout code
      uses: actions/checkout@v7
    - name: Set up QEMU
      # Only for the runtime stage's `adduser` — see the Dockerfile.
      uses: docker/setup-qemu-action@v3
      with:
        platforms: arm64
    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v4
    - name: Build both architectures
      uses: docker/build-push-action@v7
      with:
        context: .
        platforms: linux/amd64,linux/arm64
        push: false
        cache-from: type=gha,scope=image-multiarch
        cache-to: type=gha,scope=image-multiarch,mode=max
```

`report` deliberately does **not** gain `image` in its `needs:` — that job produces no JUnit or
coverage artifact, and adding it would make the unified report wait on a Docker build for nothing.
`report`'s existing `needs: [go, ts, e2e]` needs no edit either: a `needs:` on a matrixed job
already waits for every leg.

## 7. Make the cross-build reproducible locally

**`tasks/docker.yaml`** — `docker:build` stays exactly as it is (single-platform, `--load`, feeding
`services:cicd`); add a sibling for verifying the release build by hand:

```yaml
  build:multiarch:
    desc: Cross-build the image for both release architectures (verification only — no load, no push).
    cmds:
      # No --load, deliberately: the local image store can't hold a
      # multi-platform result, so this builds both and discards the output.
      # Needs a docker-container builder (`docker buildx create --use`) —
      # the default `docker` driver refuses --platform with more than one
      # entry, which is the same reason CI needs setup-buildx-action.
      - docker buildx build -f Dockerfile --platform linux/amd64,linux/arm64 ${DOCKER_BUILD_CACHE_FLAGS:-} .
```

## 8. Update `docs/agents/ci.md`

- Mark `go` and `e2e` as two-leg matrices in the pipeline mermaid diagram, and add `image` to the
  `ci_wf` subgraph with no edge to `report` — it feeds nothing downstream.
- Note in the `release_wf` subgraph that `docker` now emits a two-platform manifest list.
- In the `go` and `e2e` walkthroughs, record the matrix, that only the amd64 leg uploads artifacts,
  and that each leg publishes its own named check.
- Extend the `e2e` walkthrough's step 1 to note the `argocd` asset name now follows `matrix.arch`.
- Add a short subsection on the three cache keys that are architecture-scoped and why — this is the
  detail most likely to be undone by a future edit that does not know it matters.
- In "Things worth revisiting", add: ARM coverage does not extend to `ts` (deliberate — its output
  is architecture-independent, and the real browser on arm64 is covered by `e2e`'s live suite), and
  the pipeline's runner-minutes roughly doubled, which is free only while the repo is public.

## 9. Renovate

No configuration change needed. `docker/setup-qemu-action` is matched by the existing
`github-actions` manager rule and lands in the "ci" group with the other action bumps.

## Validate

1. **Parse first.** `yaml.safe_load` over `ci.yaml` and `release.yaml` — a matrix typo is a
   workflow that never starts.
2. **Cross-build locally**, before pushing anything:

   ```console
   docker buildx create --use --name tangle-multiarch   # if not already on a container driver
   task docker:build:multiarch
   ```

   This catches a Dockerfile mistake in seconds instead of in a CI queue.
3. **Confirm the arm64 binary is actually arm64.** A cross-compile that silently produced an amd64
   binary would still build, push, and then fail at pod start — check the artifact, not the exit
   code:

   ```console
   docker buildx build --platform linux/arm64 --target go -o type=local,dest=/tmp/arm64 .
   file /tmp/arm64/app/tangle-server   # expect: ELF 64-bit LSB executable, ARM aarch64

   # `file` isn't in the devcontainer; read the ELF e_machine field directly
   # instead. b700 = AArch64, 3e00 = x86-64.
   od -An -tx1 -j18 -N2 /tmp/arm64/app/tangle-server
   ```

4. **Open the PR** and confirm all six checks: `go (amd64)`, `go (arm64)`, `e2e (amd64)`,
   `e2e (arm64)`, `image`, and `report` still producing one `Unified Test Results` from three
   artifact sets.
5. **Watch the first arm64 `e2e` run specifically.** It is the one leg with no prior art in this
   repo. The two steps most likely to surprise: `Install argocd` (should fetch
   `argocd-linux-arm64`, not `-amd64` — and should *miss* the CLI cache the first time, because
   the key now includes `ARM64`), and `Install Playwright browser` (should download the arm64
   Chromium and headless shell rather than restoring an x64 cache).
6. **Check the caches did the right thing** on the second run. Expect distinct entries per
   architecture — `go-tools-Linux-ARM64-<hash>` alongside `go-tools-Linux-X64-<hash>`,
   `cli-tools-ARM64-…` alongside `cli-tools-X64-…`, `playwright-Linux-ARM64-1.63.0` alongside
   `playwright-Linux-X64-1.63.0`. A single shared entry anywhere means step 5 was partially
   applied, and the symptom later will be an `exec format error` deep into a job.
7. **After the first release**, verify the published manifest and run the other architecture:

   ```console
   # expect two Manifests entries: linux/amd64 and linux/arm64
   docker buildx imagetools inspect ghcr.io/ivanklee86/tangle:<tag>

   docker run --rm --platform linux/arm64 --entrypoint /usr/bin/tangle-cli \
       ghcr.io/ivanklee86/tangle:<tag> --version
   ```

   `imagetools inspect` may also list `unknown/unknown` entries — those are buildx's provenance
   attestations, not a broken platform, and they are expected with `build-push-action`'s defaults.

## Manual step that is not in any file

**Update branch protection**, in the same change window as the merge. Two things move:

- Matrixing renames `go` → `go (amd64)`/`go (arm64)` and `e2e` → `e2e (amd64)`/`e2e (arm64)`. A
  required-check rule naming `go` or `e2e` stops being satisfiable by anything. It fails closed —
  PRs block rather than merge unchecked — but it blocks *every* PR until the rule lists the new
  names.
- Steps 3 and 4 give the per-leg test-result checks explicit names (`Go Test Results (amd64)` and
  so on), replacing the action's default `Test Results`. If any rule names `Test Results`, it needs
  updating too.

Consider adding `image` as a required check at the same time; it is the one that protects the
release build.

## Rollback

Each step is independently revertible, and they are not equally risky:

- **Step 5 (cache keys) should survive a rollback of everything else.** It is correct on its own
  merits, and reverting it re-creates three latent cross-architecture cache collisions.
- **Steps 3, 4, and 6 (CI)** are additions plus the `argocd` asset fix — safe to revert at any
  time, at the cost of the ARM signal. Remember to put the branch-protection rules back. The
  `argocd-linux-${{ matrix.arch }}` line is worth keeping either way; with the matrix gone it just
  resolves to `amd64` if you substitute a literal.
- **Steps 1 and 2 (the image itself)** are the ones with a published consequence. Reverting the
  Dockerfile and `release.yaml` makes the *next* release single-platform again, but any manifest
  list already pushed to GHCR stays multi-arch — harmless for amd64 consumers, but it means an
  arm64 user who upgraded can silently regress on a later tag. If that matters, hold the tag rather
  than reverting the build.
