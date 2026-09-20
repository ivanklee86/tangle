---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Pin and cache the Task CLI in CI behind a local composite action

## Context and Problem Statement

Every CI job that runs a `task` command installs the Task CLI with `arduino/setup-task@v3.0.0` and no `version:` input — six call sites across `ci.yaml` (`go`, `ts`, `docs`, `e2e`), `release.yaml`, and `release-docs.yaml`. The action's default version is the range `3.x`, and resolving a range is not a download: [`installer.ts`'s `computeVersion()`](https://github.com/arduino/setup-task/blob/main/src/installer.ts) lists every tag in `go-task/task` through `https://api.github.com/repos/go-task/task/git/refs/tags`, unauthenticated unless the action's optional `repo-token` input is supplied. Anonymous GitHub API requests are limited to 60/hour *per source IP*, and hosted runners share a small pool of Azure egress IPs with every other repository building at that moment, so the budget is spent by strangers. CI now fails on it outright — run [35515503330](https://github.com/ivanklee86/tangle/actions/runs/35515503330)'s `e2e` job died 22 seconds in with `##[error]API rate limit exceeded for 4.236.173.160.` at the `Run arduino/setup-task@v3.0.0 / with: version: 3.x` step, taking the whole job down before it installed anything. This is also the one remaining CI dependency with no caching at all: [workstream 12](../agents/plans/ci-pipeline-restructure.md#12-ci-dependency-caching) cached Go modules, Go tool binaries, npm packages, the Playwright browser, and the k3d/argocd CLIs, but Task itself is re-fetched from the network by all four `ci.yaml` jobs on every single run.

Two independent things are going wrong, and only one of them is what the failure message names. The tag-listing API call is what actually breaks the build, and it exists solely because the version is a range. The 16 MB release-tarball download is the part that is merely wasteful — release assets are served from a different, far more forgiving path than the REST API, so caching alone would have made the failure rarer without removing it, and pinning alone would have removed the failure without making the install cheaper.

## Decision Drivers

- The failing call is the API tag listing, not the binary download, so the fix has to remove that call rather than merely make it less frequent — a cache still misses on every new runner image, cache eviction, or first run on a branch, and a miss under the status quo is another anonymous API request that can fail exactly the same way.
- An exact semver pin removes the call entirely rather than working around it: `computeVersion()` returns `v${version}` immediately when `semver.valid(version)` passes, so a pinned version never reaches `fetchVersions()` at all.
- `AGENTS.md` already requires that versions always be pinned; `3.x` is the only unpinned toolchain version left in CI, and it is the one that broke.
- Six call sites across three workflows means an inline fix is six copies of the same pin, cache step, and cache key to keep in sync — the same duplication problem `ci.yaml`'s `K3D_VERSION`/`ARGOCD_VERSION` job-level `env:` was introduced to avoid, but spanning files, where an `env:` block cannot reach.
- Caching is still worth doing on its own terms, independent of the rate limit: it removes a 16 MB download from four jobs per `ci.yaml` run, and it is the one dependency workstream 12 did not cover.
- A pinned version must not become a stale version — Renovate has to keep tracking it, in the "ci" group with the rest of the CI tooling rather than the "k8s" group that `renovate.json`'s `github-releases` catch-all would otherwise sweep it into.

## Considered Options

- **Cache the Task binary at each call site, leaving `version: 3.x`** — rejected: caches the wrong layer. It skips the download on a hit but still runs `arduino/setup-task`, which still resolves `3.x` through the anonymous API on *every* run, hit or miss — the failing request is the one call caching cannot remove.
- **Pass `repo-token: ${{ github.token }}` at each call site and change nothing else** — rejected as the whole answer: it fixes the rate limit (an authenticated request draws on this repo's own 5,000/hour budget), but keeps resolving a range over the network on every job when the resolution is unnecessary, keeps the version unpinned against `AGENTS.md`, and caches nothing.
- **Pin the version and add a cache step inline at all six call sites** — rejected: correct behavior, but six copies of a pinned version, a cache path, and a cache key across three files, with no single place to change them and nothing stopping them from drifting apart.
- **Pin the version, cache the tool-cache directory, and pass `repo-token`, all behind one local composite action** (chosen) — one file owns the version, the cache key, and the rationale; all six call sites become `uses: ./.github/actions/setup-task`.
- **Drop `arduino/setup-task` and `curl` the release tarball directly** — rejected: hand-rolls platform/arch detection, extraction, checksum handling, and PATH setup that the action already does correctly, to avoid a dependency that is not itself the problem.

## Decision Outcome

Chosen option: "Pin the version, cache the tool-cache directory, and pass `repo-token`, all behind one local composite action." Add `.github/actions/setup-task/action.yml`, a composite action that restores `${{ runner.tool_cache }}/task` via `actions/cache@v6` keyed on OS, arch, and version, then runs `arduino/setup-task@v3.0.0` with an exact pinned `version` (`3.53.1`) and `repo-token`. Replace all six `uses: arduino/setup-task@v3.0.0` references in `ci.yaml`, `release.yaml`, and `release-docs.yaml` with `uses: ./.github/actions/setup-task`. Add a `renovate.json` custom manager for the pinned version and a package rule putting `go-task/task` in the "ci" group. See [the implementation plan](../agents/plans/cache-task-binary-in-ci.md) for the exact diff.

The three parts do different jobs and the ordering matters. The pin is what fixes the failure: it short-circuits `computeVersion()` so no `api.github.com` request is made on any run, cached or not. The cache is what makes the install cheap: on a hit, `arduino/setup-task` finds the binary through `tc.find()` and does nothing but add it to PATH. `repo-token` is defense in depth only — it is unused while the version stays pinned, and matters if the pin is ever loosened back to a range, at which point the tag listing becomes an authenticated request against this repo's own budget instead of the shared anonymous one.

Caching the tool cache rather than a directory of this action's own choosing is deliberate: `arduino/setup-task` installs via `@actions/tool-cache`, which writes `<tool cache>/task/<version>/<arch>/task` alongside a sibling `<arch>.complete` marker file that its own `tc.find()` checks for. Restoring into the same layout is what lets the unmodified upstream action find the cached binary and skip the download, with no need to replicate its PATH handling here.

### Consequences

- Good, because the API call that is actually failing CI is gone entirely rather than made less frequent — a cache miss now costs a release-asset download, not another anonymous API request that can fail the job.
- Good, because four `ci.yaml` jobs per run stop downloading a 16 MB tarball each, closing the last gap in workstream 12's dependency caching.
- Good, because the Task version is pinned as `AGENTS.md` requires, in exactly one place, and a CI run no longer silently changes toolchain version the moment upstream tags a release.
- Good, because the six call sites collapse to one line each, and the next change to how Task is installed is a one-file change rather than a six-site sweep across three workflows.
- Neutral, because `repo-token` is dead configuration while the pin holds — kept deliberately, and the action's comment says so, since the failure mode it guards is exactly the one that caused this ADR.
- Bad, because the cache only pays off once `main` has populated it: GitHub scopes a cache to the branch that created it plus its descendants, so a PR gets hits from `main`'s entry but the very first run after this lands, and the first run after any version bump, still downloads on every job.
- Bad, because pinning means Task upgrades arrive as Renovate PRs rather than automatically — intended (that is what pinning is), at the cost of one more thing in the weekly update batch, and the "ci" group rule keeps it batched with the rest of the CI tooling rather than landing as its own PR.
- Bad, because a local composite action is a level of indirection a reader of `ci.yaml` has to follow to find out which Task version a job actually installs, where previously the answer was on the line in front of them (`3.x` — wrong and not obviously so, which is the trade being made).

## Pros and Cons of the Options

### Cache the binary, leave `version: 3.x`

- Good, because it is the smallest change and removes the repeated download.
- Bad, because it does not remove the failing request: the range still resolves through the anonymous API on every run whether or not the cache hits, so CI keeps failing at the same step for the same reason.

### Pass `repo-token` only

- Good, because it addresses the error message directly, and an authenticated 5,000/hour budget is not realistically exhaustible by this repo.
- Bad, because it leaves the version unpinned against `AGENTS.md`, keeps making an avoidable network call per job to resolve a range whose answer changes only when upstream releases, and caches nothing.

### Pin and cache inline at all six call sites

- Good, because it needs no new file and each workflow stays readable on its own.
- Bad, because the version and cache key are duplicated six times across three files, with the usual consequence: a future bump updates five of them.

### Pin, cache, and pass `repo-token` behind a local composite action (chosen)

See Decision Outcome.

### Download the release tarball directly

- Good, because it removes a third-party action from the critical path of every job.
- Bad, because it reimplements platform/arch mapping, extraction, and PATH setup for no benefit — the action is not what failed here, its default version input is.

## More Information

- Implementation plan: [Cache the Task binary in CI](../agents/plans/cache-task-binary-in-ci.md)
- Failing run: [`e2e` on run 35515503330](https://github.com/ivanklee86/tangle/actions/runs/35515503330) — `API rate limit exceeded for 4.236.173.160.` at the `arduino/setup-task@v3.0.0` step
- Related: [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) and [ADR 0017](0017-always-run-go-and-ts-ci-jobs.md) — every job that installs Task now runs on every push and PR, which is why the per-run count of these installs is six rather than "however many jobs the path filter let through"
- Related: [workstream 12](../agents/plans/ci-pipeline-restructure.md#12-ci-dependency-caching) (CI dependency caching) — this closes the one dependency it did not cover
- Current-state reference: [docs/agents/ci.md](../agents/ci.md)
- Superseded by, if adopted later: none.
