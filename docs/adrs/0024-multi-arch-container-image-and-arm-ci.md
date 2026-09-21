---
status: "proposed"
date: 2026-09-21
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Publish a multi-arch container image and test the full pyramid on ARM

## Context and Problem Statement

`ghcr.io/ivanklee86/tangle` is a single-platform `linux/amd64` image. `release.yaml`'s `docker` job runs `docker/build-push-action@v7` with no `platforms:` input and no `docker/setup-buildx-action` step at all, so the build uses the runner's default `docker` driver and produces exactly one image manifest, for the architecture of the `ubuntu-latest` runner it happened to run on. Anyone pulling that image onto an arm64 node — an Apple Silicon laptop running `docker run`, a Graviton/Ampere Kubernetes node, a Raspberry Pi — gets either QEMU emulation (Docker Desktop, silently slow) or a hard `exec format error` (a real arm64 cluster, at pod start). The `tangle-cli` binaries are not affected: GoReleaser already cross-compiles the default matrix, and [ADR 0021](0021-automate-tangle-deployments-chart-version-bump.md)'s Homebrew cask picks up `darwin/arm64` from it. The container image is the one artifact that is amd64-only, and it is the artifact `tangle-deployments` installs.

CI has a matching gap in the other direction. Every job in `ci.yaml` runs on `ubuntu-latest`, so no test in this repository has ever executed on ARM. Nothing here is obviously architecture-sensitive — there is no `import "C"` anywhere, no build-tagged assembly, no `unsafe` pointer arithmetic over struct layouts — but "obviously" is doing load-bearing work in that sentence, and the usual portability traps (`int` width, unaligned 64-bit atomics, map or goroutine scheduling order leaking into a test's expectations) are exactly the kind of thing a test suite finds and a code review does not. Publishing an arm64 image means claiming the server works on arm64, and today nothing in the pipeline substantiates that claim.

The claim being made is also bigger than "the Go unit suite passes." What ships is a *container image* that talks to a real ArgoCD over gRPC-Web and serves a frontend that a real browser drives. [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) built a full test pyramid precisely because the layers catch different things; an arm64 image validated only at the pyramid's base is validated at the layer least likely to be where an architecture problem actually bites. The interesting failures — a gRPC-Web client behaving differently, a timing-sensitive interaction with a real cluster, the arm64 image itself being subtly wrong in a way no unit test touches — live in `e2e`.

There is a third problem, orthogonal to both. Even once the Dockerfile builds for two platforms, nothing would exercise the *cross-compilation* path until a release is cut. A Dockerfile edit that breaks it — an added `RUN` in the runtime stage, a build-stage tool that needs to execute a target-platform binary — would pass every PR check and fail at release time, after a tag exists and a GitHub Release is already published. That is the worst possible moment to discover it, and, importantly, an arm64 `e2e` job does *not* catch it: on an arm64 runner `BUILDPLATFORM` equals `TARGETPLATFORM`, so `GOOS`/`GOARCH` are a no-op and the cross-build is never attempted.

The repository is public, which matters concretely: GitHub's hosted arm64 runners (`ubuntu-24.04-arm`) are free for public repositories, so ARM coverage costs wall-clock time and nothing else.

## Decision Drivers

- The image is the artifact `tangle-deployments` consumes; an arm64 node cannot run it at all today, and the failure surfaces at pod start rather than at install.
- Nothing in this build needs a native toolchain. The Go code is pure Go with no cgo, so `GOOS`/`GOARCH` cross-compilation is exact, not approximate; the frontend build emits static assets that are byte-identical regardless of which architecture ran `npm run build`. Any design that pays for native arm64 build capacity is paying for something this repo does not need.
- Emulation is the thing to avoid, not multi-arch itself. A QEMU-emulated `go build` of a project this size is minutes, not seconds, and it would land on the critical path of every release. A build that cross-compiles costs approximately one extra `go build`.
- Confidence should match the claim. Shipping an arm64 image is a statement about the server running against a real ArgoCD on arm64, so the evidence should come from the layer that tests that, not only from the hermetic unit layer.
- The prerequisites that previously argued against ARM `e2e` have been checked and are not blockers (see **More Information** for the commands and results): `argo-cd` v3.5.3 publishes an `argocd-linux-arm64` asset; k3d publishes `k3d-linux-arm64` and its `install.sh` detects the architecture itself; Playwright 1.63.0 recognizes `ubuntu24.04-arm64` as a host platform and serves both `chromium-linux-arm64.zip` and `chromium-headless-shell-linux-arm64.zip`; and `quay.io/argoproj/argocd:v3.5.3`, `alpine`, `golang:1.27-alpine`, and `node:24-alpine` are all manifest lists including `linux/arm64`. What remained was one hardcoded asset name, not a missing capability.
- Matrix legs run concurrently, so the cost of an ARM leg is paid in runner-minutes (free here) rather than in wall-clock, up to arm64 runner availability. Doubling `e2e` is not the same as making every PR wait twice as long.
- Shared Actions caches are keyed by content today, not by architecture — and three separate keys are affected. `go-tools-${{ hashFiles('tasks/go.yaml') }}` (in `go` and `e2e`), `cli-tools-k3d-…-argocd-…` (in `e2e`), and `playwright-${{ runner.os }}-1.63.0` (in `ts` and `e2e`) all cache *compiled binaries* under keys with no architecture component; `runner.os` is `Linux` on both, so it does not discriminate. Adding an arm64 writer to any of them poisons it for the amd64 readers. This is a correctness prerequisite, not a nicety.
- `report`'s centralized octocov run reads fixed artifact filenames (`.octocov.yml`'s `coverage/coverage.out`, `coverage/coverage-e2e.out`). Matrixed jobs that upload from every leg collide on artifact name, and `merge-multiple: true` would silently overwrite same-named files inside them.

## Considered Options

### How to build the image for two platforms

- **One `docker` job, `platforms: linux/amd64,linux/arm64`, Dockerfile rewritten to cross-compile** (chosen) — pin the Go and Node build stages to `--platform=$BUILDPLATFORM` and compile with `GOOS`/`GOARCH` taken from buildx's `TARGETOS`/`TARGETARCH` args. Both platforms' expensive work runs natively on the amd64 runner.
- **One job, `platforms:` added and the Dockerfile left as-is** — rejected: correct output, wrong cost. With no `--platform=$BUILDPLATFORM` pin, buildx runs the entire `golang:1.27-alpine` and `node:24-alpine` stages under QEMU for the arm64 leg — `go mod download`, `go install swagger`, `swagger generate spec`, `go build ./...`, and `npm install && npm run build`, all emulated, on every release.
- **Native matrix (amd64 + `ubuntu-24.04-arm`), push by digest, merge with `buildx imagetools create`** — rejected: this is the right pattern when a build genuinely cannot cross-compile, and it buys nothing here. It turns one job into three, introduces digest-passing between jobs via artifacts, splits the layer cache across two runners, and makes the release's success depend on three jobs coordinating instead of one succeeding. Note that the arm64 `e2e` leg does build the image natively on arm64 — but as a test fixture, not as a release artifact, which is a different job with different failure consequences.

### How to handle the runtime stage's `RUN`

The runtime stage executes one command on the *target* platform: `RUN addgroup -S tangle && adduser -S tangle -G tangle`. Cross-compilation does not help here — a `RUN` in an arm64 stage needs an arm64 `busybox` to execute.

- **Add `docker/setup-qemu-action@v3` and keep the user creation as-is** (chosen) — registers `binfmt_misc` handlers so that one trivial busybox invocation can run emulated. The emulated work is two `busybox` calls writing a few lines to `/etc/passwd` and `/etc/group`; the setup step itself is the larger cost, and both are seconds. Only the jobs that *cross*-build need it: the arm64 `e2e` leg builds natively and needs no QEMU at all.
- **Drop the `RUN` and use a numeric `USER 65532:65532` with `COPY --chown=65532:65532`** — rejected for now, though genuinely tempting: it removes QEMU from the pipeline entirely and produces a distroless-style image. It also changes the container's runtime UID (alpine's `adduser -S` allocates a low system UID; 65532 is not it) and removes the `tangle` entry from `/etc/passwd`. Whether that breaks anything depends on `tangle-deployments`' `securityContext`, which is outside this repository. Worth doing deliberately, with that chart checked, rather than as a side effect of a multi-arch change.
- **Move user creation into a build-stage-generated `/etc/passwd` copied into the runtime stage** — rejected: it achieves the same thing as the numeric-UID option with more machinery and the same unverified UID question.

### How far to take ARM testing in CI

- **Matrix `go` *and* `e2e` over amd64/arm64, plus a new always-run `image` job that cross-builds both platforms without pushing** (chosen) — `go` covers the hermetic unit/integration layer, `e2e` covers the arm64 image running against a real ArgoCD with a real browser, and `image` covers the one thing neither does: that the amd64-hosted cross-build still works.
- **Matrix `go` only, plus the `image` job** — rejected: cheaper, and it was the initial direction, but it validates an arm64 image at the layer least likely to expose an arm64 problem. The prerequisites that made `e2e` look expensive turned out to be one hardcoded `argocd-linux-amd64` asset name (see **More Information**), which is a two-line fix, not a project.
- **Matrix `ts`** — rejected: the frontend's test and build output does not depend on the runner's architecture, and its mocked Playwright suite has no native component whose behavior could differ. The live browser suite *is* covered on ARM, via the `e2e` matrix, which is the place where a real browser on a real arm64 kernel actually gets exercised.
- **Matrix `go`/`e2e` but skip the `image` job** — rejected: leaves cross-compilation untested until release. The arm64 `e2e` leg builds the image with `BUILDPLATFORM == TARGETPLATFORM`, so it never exercises the cross-build the release depends on.

### How matrixed jobs should report results

- **Only the amd64 leg uploads artifacts; the arm64 leg publishes its own separate check** (chosen) — keeps `report`'s merge inputs exactly as they are today (fixed filenames, no collisions, no double-counted coverage) while still surfacing *which test* failed on ARM rather than only *that* the job went red.
- **Both legs upload, with arch-suffixed artifact names** — rejected: the artifact names would be unique but the files inside them are not (`report-e2e.xml` from both legs), and `report`'s `merge-multiple: true` download would silently overwrite one with the other. Fixing that means renaming files in a shell step before upload, which is more machinery than a `check_name` input.
- **Only the amd64 leg reports anything at all** — rejected as the whole answer: simplest, and fine for `go`, but an arm64-only `e2e` failure would then offer nothing but raw job logs, which is exactly the situation where test-level detail is worth the most.

## Decision Outcome

Chosen: **cross-compile a two-platform image from one release job, and prove it across the whole test pyramid — `go` and `e2e` both matrixed over amd64/arm64 — with a non-pushing `image` job guarding the cross-build itself.**

Concretely:

1. **`Dockerfile`** — pin the `go` and `node` stages to `--platform=$BUILDPLATFORM`, declare `ARG TARGETOS TARGETARCH` immediately before the build command, and set `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH` on the `go build` invocation only. The `GOOS`/`GOARCH` assignment is scoped to that one command deliberately: the same stage also runs `go install github.com/go-swagger/go-swagger/cmd/swagger` and then *executes* the result to generate the Swagger spec, so a stage-wide `ENV GOARCH` would cross-compile the `swagger` binary and then fail to run it.
2. **`release.yaml`'s `docker` job** — add `docker/setup-qemu-action@v3` and `docker/setup-buildx-action@v4`, and add `platforms: linux/amd64,linux/arm64` to the build step. The buildx setup step is not optional housekeeping: the default `docker` driver cannot produce a multi-platform result, so without it the `platforms:` input fails the build rather than being ignored.
3. **`ci.yaml`'s `go` job** — matrix over `ubuntu-latest` (amd64) and `ubuntu-24.04-arm` (arm64), `fail-fast: false`.
4. **`ci.yaml`'s `e2e` job** — the same matrix, plus one substantive fix: the `Install argocd` step hardcodes the `argocd-linux-amd64` release asset and becomes `argocd-linux-${{ matrix.arch }}`. The matrix values are chosen to be Go's `GOARCH` spellings (`amd64`/`arm64`) precisely so they can be substituted straight into that asset name. `k3d`'s installer detects the architecture itself and needs no change.
5. **`ci.yaml`'s new `image` job** — `docker/setup-qemu-action` + `docker/setup-buildx-action`, then a `--platform linux/amd64,linux/arm64` build with `push: false` and its own GitHub Actions cache scope.
6. **Cache keys** — add `${{ runner.os }}-${{ runner.arch }}` to the `go-tools-` key (in `go` and `e2e`), the `cli-tools-` key (in `e2e`), and the `playwright-` key (in `ts` and `e2e`); give `e2e`'s buildx cache a per-arch scope and the `image` job its own.
7. **Reporting** — amd64 legs upload artifacts exactly as today; arm64 legs publish a separate, per-job check instead.
8. **`tasks/docker.yaml`** — add a `docker:build:multiarch` task so the cross-build is reproducible locally, keeping `docker:build` as the fast single-platform `--load` build the e2e stack depends on.

The division of labor is the point, and each piece covers something the others cannot. Cross-compilation removes emulation from the expensive stages, which is what makes a two-platform release build affordable. QEMU stays only for the one target-platform `RUN` that no amount of cross-compilation can eliminate. The matrixed `go` job catches portability bugs cheaply and early; the matrixed `e2e` job is the only place the arm64 image itself runs against a real ArgoCD and a real browser; and the `image` job is the only place the amd64-hosted cross-build is exercised at all.

### Consequences

- Good, because `ghcr.io/ivanklee86/tangle:<tag>` becomes a manifest list serving both `linux/amd64` and `linux/arm64`. Consumers change nothing — the tag is the same, and the registry hands each node its own architecture — so `tangle-deployments`' chart and ADR 0021's version bump keep working untouched.
- Good, because the release build stays fast. The only emulated instruction in the whole pipeline is one `busybox adduser`; every compile runs natively.
- Good, because arm64 support is evidenced at the same depth as amd64: the same unit/integration suite, the same live-ArgoCD Go suite, and the same live Playwright suite, all on a real arm64 kernel, on every push and PR.
- Good, because a Dockerfile change that breaks cross-compilation fails in the `image` job on the PR that introduced it, with nothing published and no tag to clean up.
- Good, because three latent cache bugs get fixed. `go-tools-`, `cli-tools-`, and `playwright-` all cache compiled binaries under architecture-blind keys today; nothing but the absence of an ARM job keeps them from being wrong, and this change would have made them wrong in a confusing, intermittent way.
- Neutral, because the frontend build stays on the build platform in the image. That is correct rather than a compromise: `npm run build` emits static assets, and the `node` stage's output is copied, never executed, in the runtime image.
- Bad, because matrixing renames status checks: `go` becomes `go (amd64)`/`go (arm64)` and `e2e` becomes `e2e (amd64)`/`e2e (arm64)`. Branch-protection rules naming `go` or `e2e` silently stop being satisfiable by anything, which fails closed (PRs block) rather than open — but it blocks every PR until the rules are updated. This needs doing in the same change window, and it is the one step that cannot be done from this repository's files.
- Bad, because CI roughly doubles its runner-minutes: two full k3d + ArgoCD bring-ups per run instead of one, plus a Docker build in the new `image` job. Free on a public repository, but not free in the abstract, and worth revisiting if this ever goes private.
- Bad, because wall-clock is now hostage to arm64 runner availability. The legs run concurrently, so in the normal case the pipeline is no slower than its slowest single leg — but if the arm64 pool queues, every PR waits on it, and `e2e` was already the critical path.
- Bad, because arm64 failures are reported through a different mechanism than amd64 ones (a separate check rather than the unified `report` output). That asymmetry is deliberate, and documented, but it is one more thing a contributor has to know when a PR goes red.
- Bad, because the release `docker` job grows two setup steps for a build that used to be one action invocation. `setup-qemu-action` in particular is there for a single `RUN`, which reads as disproportionate until you know what it is for; the workflow comment has to carry that.

## Pros and Cons of the Options

### One job, cross-compiled (chosen)

- Good, because it keeps the release workflow's shape — one `docker` job — while doubling what it produces, and because a pure-Go, no-cgo codebase makes the cross-compile exact rather than a best effort.
- Good, because the layer cache stays in one place instead of being split across two runners with different architectures.
- Bad, because the Dockerfile stops being a plain Dockerfile: `$BUILDPLATFORM`, `TARGETOS`, and `TARGETARCH` are buildx concepts, so `docker build` without buildx no longer reproduces what CI does.

### One job, `platforms:` only, Dockerfile unchanged

- Good, because it is a one-line change and produces a correct manifest list.
- Bad, because every expensive build stage runs under QEMU for the arm64 leg on every release — `go mod download`, `go install`, `swagger generate`, `go build ./...`, `npm install`, `npm run build` — for no benefit, since none of that work needs to be native.

### Native matrix + `imagetools create` merge

- Good, because it never emulates anything and is the standard answer for builds with native-toolchain requirements (cgo, platform-specific native npm modules that ship in the output).
- Bad, because it triples the job count, adds digest-passing plumbing between jobs, splits the cache, and solves a problem this build does not have.

### Matrix `go` + `e2e` + a non-pushing `image` job (chosen)

- Good, because every layer of ADR 0009's pyramid runs on both architectures, and the one gap none of them covers (cross-compilation) gets its own job.
- Good, because the prerequisites turned out to be one hardcoded asset name rather than missing upstream support, so the cost is far below what it looked like before checking.
- Bad, because it renames two required checks, doubles the pipeline's heaviest job in runner-minutes, and makes wall-clock depend on arm64 runner availability.

### Matrix `go` only

- Good, because it is the cheapest credible ARM signal and touches nothing about the `e2e` job's already-delicate bring-up.
- Bad, because it validates an arm64 *image* without ever running that image, at the pyramid layer least likely to surface an architecture problem.

### Matrix `ts`

- Good, because it would be consistent with matrixing everything else.
- Bad, because vitest and a network-mocked Playwright suite produce architecture-independent results; the real browser on real arm64 is already covered by `e2e`'s live suite.

## More Information

- Implementation plan: [Multi-arch container image and ARM CI coverage](../agents/plans/multi-arch-image-and-arm-ci.md)
- Current-state reference: [docs/agents/ci.md](../agents/ci.md) — the pipeline diagram and per-job walkthroughs this changes
- Related: [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) — the test pyramid this now replicates on a second architecture, and the ADR that accepts `e2e`'s cost
- Related: [ADR 0017](0017-always-run-go-and-ts-ci-jobs.md) — `go` and `e2e` run on every push and PR, so their matrices double on every run rather than occasionally
- Related: [ADR 0021](0021-automate-tangle-deployments-chart-version-bump.md) — the consumer of this image; a manifest list keeps its `image.tag` bump working unchanged
- Related: [ADR 0018](0018-pin-and-cache-the-task-cli-in-ci.md) — `./.github/actions/setup-task` already keys its cache on `runner.arch`, so it needs no change for an ARM runner; the three keys that do need fixing predate that habit
- Upstream: [buildx automatic platform ARGs](https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope) (`BUILDPLATFORM`, `TARGETOS`, `TARGETARCH`)
- Upstream: GitHub's `ubuntu-24.04-arm` hosted runners, free for public repositories

### Prerequisite verification

Every arm64 dependency of the `e2e` job was checked before choosing to matrix it, rather than assumed:

| Dependency | Check | Result |
| --- | --- | --- |
| `argocd` CLI v3.5.3 | `gh api repos/argoproj/argo-cd/releases/tags/v3.5.3 --jq '.assets[].name'` | `argocd-linux-arm64` present — the pipeline's only real gap was `ci.yaml` hardcoding `argocd-linux-amd64` |
| `k3d` v5.9.0 | `gh api repos/k3d-io/k3d/releases/tags/v5.9.0 --jq '.assets[].name'` | `k3d-linux-arm64` present; `install.sh` selects by architecture itself, so the install step needs no change |
| Playwright 1.63.0 host platform | `grep -roh 'ubuntu2[0-9]\.04-arm64' web/node_modules/playwright-core/lib/` | `ubuntu24.04-arm64` recognized |
| Playwright Chromium (rev 1243) | `curl -o /dev/null -w '%{http_code}' -L https://cdn.playwright.dev/dbazure/download/playwright/builds/chromium/1243/chromium-linux-arm64.zip` | `200` |
| Playwright headless shell (rev 1243) | same, `chromium-headless-shell-linux-arm64.zip` | `200` |
| ArgoCD server image | `docker buildx imagetools inspect quay.io/argoproj/argocd:v3.5.3` | manifest list including `linux/arm64` |
| Image base layers | `docker buildx imagetools inspect` on `alpine:latest`, `golang:1.27-alpine`, `node:24-alpine` | all manifest lists including `linux/arm64` |
| `EnricoMi/publish-unit-test-result-action` | `docker buildx imagetools inspect ghcr.io/enricomi/publish-unit-test-result-action:v2.20.0` | `linux/amd64` + `linux/arm64` — so the amd64-only artifact gating is about filename collisions, not about the action's architecture support |

- Superseded by, if adopted later: none.
