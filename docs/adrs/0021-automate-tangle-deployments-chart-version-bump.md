---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Automate `tangle-deployments`' chart version bump on a `tangle` release

## Context and Problem Statement

`ivanklee86/tangle-deployments` hosts the Helm chart for `tangle` (`charts/tangle`, published as a
Helm repo at `https://ivanklee86.github.io/tangle-deployments/` via
[`helm/chart-releaser-action`](https://github.com/helm/chart-releaser-action)). Today, keeping the
chart in sync with a `tangle` release is entirely manual: `Chart.yaml`'s `appVersion` (currently
`0.1.0`, matching `tangle`'s latest tag `v0.1.0`) and its own chart `version` (currently `0.0.9`,
tracked independently) both have to be hand-edited and committed, and
`tangle-deployments/.github/workflows/release.yaml` — the workflow that actually packages the
chart and publishes it — is wired to `on: release: types: [published]`, so a GitHub Release also
has to be cut *on `tangle-deployments` itself* by hand to trigger it. `chart-releaser-action`'s own
documented usage is `on: push: branches: [main]`; it diffs `Chart.yaml`'s `version` between commits
to decide what to package, so gating it behind a manual release-publish step is a repurposing of
the action, not its intended trigger.

This gap is visible in `tangle-deployments`' own history: Renovate already opens a PR bumping the
`ghcr.io/ivanklee86/tangle` Docker tag in `charts/tangle/values.yaml`'s `image.tag` field (its
default `config:recommended` docker-datasource manager finds it there), and a separate, manual
"Update versions." commit follows a few minutes after that PR merges — someone bumping
`Chart.yaml` by hand to match. There is no CI on `tangle-deployments` today beyond a local
`.pre-commit-config.yaml` and the release workflow — no `helm lint`/`kubeconform` gate runs on
pull requests, even though the Taskfile already defines `template:default` and `tests` tasks that
could run in one.

The goal: publishing a release in `tangle` should automatically update and tag the chart in
`tangle-deployments`, without a human re-typing a version number in a second repository.

## Decision Drivers

- GitHub Actions workflows in one repository cannot react to another repository's events without
  an explicit bridge — some mechanism has to carry the release version from `tangle` to
  `tangle-deployments`.
- `chart-releaser-action` should run the way it's documented (`push` to `main`), not behind a
  manual release-publish gate that exists only as a workaround for the lack of automation.
- A cross-repo write needs a credential broader than the default `GITHUB_TOKEN` (which is scoped
  to the repo the workflow runs in), so whatever mechanism is chosen has a real security surface —
  minimize the scope and lifetime of whatever token gets used.
- `tangle-deployments` has no PR-time CI today; landing an automated version-bump PR without also
  landing a safety net that validates the resulting chart (`helm lint`/`kubeconform`, already
  scripted in its `Taskfile.yaml`) would automate publishing a broken chart just as readily as a
  good one.
- Chart-releaser needs `Chart.yaml`'s `version` to change to detect a new release; `appVersion`
  syncing alone isn't sufficient to trigger a chart publish.
- The chart's own `version` and `appVersion` have already diverged in practice (`0.0.9` vs.
  `0.1.0`) — the maintainer bumps the chart's minor/major version by hand when the chart's
  templates change (see the "Add support for configurable parallelism and bump app/chart version"
  commit), independent of `tangle` releases. Automation should own only the mechanical,
  every-release case (`appVersion` sync + a patch bump), not decide chart semver on the
  maintainer's behalf.

## Considered Options

- **`repository_dispatch` from `tangle` → a new workflow in `tangle-deployments`** (chosen) — the
  event GitHub Actions defines specifically for one repo's workflow to trigger another's, carrying
  an arbitrary `client_payload`.
- **`tangle-deployments` polls `tangle`'s releases on a schedule** — rejected: not event-driven,
  adds a cron job that's idle almost all the time, and still needs to diff against some
  previously-seen state to avoid reacting to the same release twice.
- **`tangle`'s workflow pushes a commit directly to `tangle-deployments`'s `main`, no PR** —
  rejected: skips the CI safety net entirely (there being none yet is exactly the problem to fix,
  not compound), and a malformed dispatch payload or bug in the bump script would land straight on
  `main`, where `chart-releaser-action` would immediately package and publish it as a real,
  public chart release with no review step in between.
- **A GitHub App installation token instead of a personal access token** — noted as the more
  "enterprise" answer (short-lived, not tied to a personal account, survives the account's own
  permission changes), but rejected for now as more machinery than two personal repositories
  need; revisit if `tangle`/`tangle-deployments` ever move to an organization.

## Decision Outcome

Chosen option: **`repository_dispatch`, feeding a PR-based bump workflow gated by new CI**.

1. `tangle`'s `release.yaml` gets a new job, gated on `github.event.release.prerelease == false`,
   that fires a `repository_dispatch` event of type `tangle-release` at `tangle-deployments` with
   `client_payload: { version: <release tag> }`, authenticated with a fine-grained personal access
   token scoped to *only* `tangle-deployments`, `Contents: Read and write` (the minimum GitHub
   documents for triggering a dispatch), stored as a `tangle`-repo secret
   (`TANGLE_DEPLOYMENTS_DISPATCH_TOKEN`) with an expiry date set and a calendar reminder to rotate
   it, not "no expiration."
2. `tangle-deployments` gets a new workflow, `bump-chart-version.yaml`, triggered on that
   `repository_dispatch` type, which strips the `v` prefix into `appVersion`, patch-bumps
   `version`, and opens a PR via `peter-evans/create-pull-request` against a fixed branch name
   (so a second dispatch before the first PR merges updates the same PR instead of piling up
   parallel ones) using the workflow's own default `GITHUB_TOKEN` — no PAT needed for this half,
   since it's a same-repo commit.
3. `tangle-deployments` also gets a minimal CI workflow (new) that runs `task template:default`
   and `task tests` (`helm template` + `kubeconform`, already defined in its `Taskfile.yaml`) on
   every pull request — the safety net the bump PR relies on, and a gap worth closing regardless
   of this automation.
4. The bump PR is set to auto-merge (`gh pr merge --auto --squash`, from within the same bump
   workflow) once the new CI passes — this is a fully deterministic, mechanical edit with a real
   CI gate behind it, in the same spirit as this repo's own `automerge` policy for non-major
   Renovate updates, so it doesn't need a human to click merge every time `tangle` cuts a release.
5. `tangle-deployments`' `release.yaml` trigger changes from `on: release: types: [published]` to
   `on: push: branches: [main]`, matching `chart-releaser-action`'s documented usage — the merge
   from step 4 is what actually cuts the chart release and updates the `gh-pages` index from here
   on, and the old manual "cut a release by hand to trigger packaging" step goes away entirely.

Chart `version` handling deliberately stays narrow: the automation only ever patch-bumps it,
because bumping `appVersion` is itself a chart content change and Helm/`chart-releaser-action` both
require *some* version bump to register a new release — it does not try to infer minor/major chart
version changes from what changed in `tangle`, since the two have already diverged in practice and
the maintainer already bumps the chart's own semver by hand when the chart's templates change
independent of a `tangle` release.

Left out of this decision, tracked as a follow-up rather than bundled in: collapsing
`values.yaml`'s explicit `image.tag: "v0.1.0"` override so it falls back to the chart's own
`{{ .Chart.AppVersion }}` default (the comment above it already says that's the intent), which
would remove the second, Renovate-tracked copy of the version this ADR's automation makes
redundant. Doing that safely requires first confirming `ghcr.io/ivanklee86/tangle` actually
publishes an image tag matching whatever exact string ends up in `appVersion` (with or without the
`v` prefix — the two currently disagree: `appVersion: "0.1.0"` vs. `image.tag: "v0.1.0"`), so it's
left as a separately verified change rather than assumed safe here.

### Consequences

- Good, because publishing a `tangle` release now fully drives a chart release with no manual
  edit in a second repository — the stated goal.
- Good, because `tangle-deployments`' `release.yaml` finally runs the way
  `chart-releaser-action` is documented to run (`push` to `main`), rather than needing a
  human-cut release as a workaround trigger.
- Good, because `tangle-deployments` gains PR-time CI (`helm lint`/`kubeconform`) it didn't have
  before, closing a real gap independent of this automation.
- Neutral, because a new fine-grained PAT has to be created and stored by hand (not scriptable),
  and its expiry has to be tracked and rotated — a small, recurring maintenance cost in exchange
  for not granting a long-lived, broadly-scoped credential.
- Bad, because a cross-repo credential exists at all — its blast radius is scoped to exactly one
  repository's contents, but it's still a secret that didn't need to exist before this.
- Bad, because auto-merge on the bump PR means a `tangle` release now indirectly cuts a public
  chart release with no human in the loop by default — mitigated by the new CI gate, but a
  bug that CI doesn't catch (e.g. a bad `appVersion` string) would still ship. Turning off
  auto-merge trades this for requiring a manual click on every `tangle` release.

## Pros and Cons of the Options

### `repository_dispatch` (chosen)

- Good, because it's the mechanism GitHub Actions documents specifically for this: one repo's
  workflow triggering another's, with an arbitrary payload.
- Bad, because it still needs a PAT (or GitHub App token) — no dispatch mechanism avoids a
  cross-repo credential entirely.

### Scheduled polling

- Good, because it needs no token stored in `tangle` at all — `tangle-deployments`' own workflow
  can read `tangle`'s public releases with an unauthenticated or repo-scoped read.
- Bad, because it's not event-driven (delay up to the poll interval), and needs its own
  "have I already handled this release" state to avoid reacting twice — solving a problem
  `repository_dispatch` doesn't have.

### Direct push, no PR

- Good, because it's the fewest moving parts — one commit, no PR, no auto-merge wiring.
- Bad, because it removes the one review/CI opportunity between a dispatch payload and a public
  chart release, which is the opposite of the safety net this ADR is trying to add.

### GitHub App token

- Good, because it's short-lived, not tied to a personal GitHub account, and considered the
  stronger long-term answer for cross-repo automation.
- Bad, because standing up a GitHub App (registration, installation, permission review) is more
  setup than two personal repositories currently justify; revisit if either repo moves to an
  organization.

## More Information

- Implementation plan: [Automate the `tangle-deployments` chart bump](../agents/plans/automate-tangle-deployments-chart-bump.md)
- `chart-releaser-action`'s documented trigger: `on: push: branches: [main]` —
  <https://github.com/helm/chart-releaser-action>
- `repository_dispatch` token requirements — <https://docs.github.com/en/rest/repos/repos#create-a-repository-dispatch-event>
- Current state: `tangle-deployments/.github/workflows/release.yaml` (`on: release: published`),
  `tangle-deployments/charts/tangle/Chart.yaml` (`appVersion: "0.1.0"`, `version: 0.0.9`),
  `tangle-deployments/charts/tangle/values.yaml` (`image.tag: "v0.1.0"`, overriding the chart's own
  `.Chart.AppVersion` default)
- Related: [ADR 0013](0013-renovate-weekly-grouped-updates.md) — the automerge-non-major policy
  this ADR's auto-merge choice mirrors, applied here to a fully mechanical bump instead of a
  Renovate PR
- Superseded by, if adopted later: a GitHub App-based token replacing the fine-grained PAT.
