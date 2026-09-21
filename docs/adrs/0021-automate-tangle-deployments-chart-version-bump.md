---
status: "accepted"
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
   parallel ones). **Revised during implementation** (see "Found during a live test" below): this
   half also needs a second fine-grained PAT, scoped to `tangle-deployments` itself
   (`CHART_BUMP_PAT`, `Contents` + `Pull requests: Read and write`), used for *both* opening the PR
   and merging it — the default `GITHUB_TOKEN` turned out not to work for either. **Revised again
   after three automated releases** (see "Revised: `image.tag` and minor bumps" below): the bump
   also has to write `values.yaml`'s `image.tag`, and the chart `version` bump is minor, not patch.
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

Chart `version` handling deliberately stays narrow: the automation only ever patch-bumps it
(**superseded — it minor-bumps; see "Revised: `image.tag` and minor bumps" below**), because
bumping `appVersion` is itself a chart content change and Helm/`chart-releaser-action` both
require *some* version bump to register a new release — it does not try to infer minor/major chart
version changes from what changed in `tangle`, since the two have already diverged in practice and
the maintainer already bumps the chart's own semver by hand when the chart's templates change
independent of a `tangle` release.

Left out of this decision, tracked as a follow-up rather than bundled in (**now resolved, the other
way round — see below**): collapsing `values.yaml`'s explicit `image.tag: "v0.1.0"` override so it
falls back to the chart's own `{{ .Chart.AppVersion }}` default (the comment above it already says
that's the intent), which would remove the second, Renovate-tracked copy of the version this ADR's
automation makes redundant. Doing that safely requires first confirming
`ghcr.io/ivanklee86/tangle` actually publishes an image tag matching whatever exact string ends up
in `appVersion` (with or without the `v` prefix — the two currently disagree: `appVersion: "0.1.0"`
vs. `image.tag: "v0.1.0"`), so it's left as a separately verified change rather than assumed safe
here.

### Consequences

- Good, because publishing a `tangle` release now fully drives a chart release with no manual
  edit in a second repository — the stated goal, verified end to end on a real release (`v0.2.0`
  → chart `0.0.13`, `gh-pages` index updated) with no manual intervention.
- Good, because `tangle-deployments`' `release.yaml` finally runs the way
  `chart-releaser-action` is documented to run (`push` to `main`), rather than needing a
  human-cut release as a workaround trigger.
- Good, because `tangle-deployments` gains PR-time CI (`helm lint`/`kubeconform`) it didn't have
  before, closing a real gap independent of this automation.
- Neutral, because two fine-grained PATs now exist (one per repo — see "Found during a live test"
  below) instead of the one originally planned, each has to be created and stored by hand (not
  scriptable), and each expiry has to be tracked and rotated — a small, recurring maintenance cost
  in exchange for not granting a long-lived, broadly-scoped credential.
- Bad, because a cross-repo credential exists at all — its blast radius is scoped to exactly one
  repository each, but they're secrets that didn't need to exist before this.
- Bad, because auto-merge on the bump PR means a `tangle` release now indirectly cuts a public
  chart release with no human in the loop by default — mitigated by the new CI gate (and, since the
  live test, by branch protection actually enforcing it), but a bug that CI doesn't catch (e.g. a
  bad `appVersion` string) would still ship. Turning off auto-merge trades this for requiring a
  manual click on every `tangle` release.

### Found during a live test

Three things the design above got wrong, each only visible by actually cutting a release and
watching the chain run (not from reading the workflow YAML):

1. **`main` had no branch protection.** `gh pr merge --auto` has nothing to wait for without a
   required status check, so it merged the first test PR in ~2 seconds — before the `chart` CI job
   had even started, which then failed orphaned (its branch was already deleted). Fixed by adding
   branch protection on `tangle-deployments`' `main` requiring the `chart` check, and by enabling
   the repo's "Allow auto-merge" setting (`allow_auto_merge`), which was also off and made
   `gh pr merge --auto` fail outright (`GraphQL: Auto merge is not allowed for this repository`).
2. **A bot-authored PR is gated behind manual approval.** `peter-evans/create-pull-request`'s
   default `GITHUB_TOKEN` makes the PR's author `github-actions[bot]`, whose `author_association`
   on this public repo resolves to `CONTRIBUTOR` — GitHub gates `pull_request`-triggered workflow
   runs (i.e. the new `chart` check) behind manual "Approve and run" for any non-collaborator
   author. That's the right default for real outside contributors, but it silently blocks this
   repo's own automation from ever completing unattended. Fixed by passing the `CHART_BUMP_PAT`
   (repo permission `Contents` + `Pull requests: Read and write`) to `create-pull-request`'s
   `token:` input, making the PR's author the token's owner instead.
3. **GitHub suppresses events from its own token, and this applies to more than `push`.** Even
   after (1) and (2), `release.yaml`'s `push: main` trigger never fired once the bump PR merged.
   GitHub's anti-recursion rule ("events triggered using `GITHUB_TOKEN` won't create a new workflow
   run") isn't limited to raw `git push` — it also suppressed the `pull_request: closed` event that
   would otherwise fire once `gh pr merge --auto`'s *merge itself* completed, because that step was
   still using `GITHUB_TOKEN`. The first fix attempt (add `workflow_dispatch` to `release.yaml` and
   an explicit `pull_request: closed`-triggered job calling it) treated the symptom and didn't work
   either, for the same underlying reason: the `pull_request: closed` event it depended on was
   itself suppressed. The actual fix was simpler than either workaround: use `CHART_BUMP_PAT` for
   the merge step too, not just for opening the PR. A PAT-authenticated merge behaves like an
   ordinary user merge and triggers `release.yaml` via its plain `push: main` trigger with no
   special-casing needed — proven by every merge in this chain performed by a real account (the
   PAT, or a human clicking merge) correctly triggering it, and every one performed by
   `GITHUB_TOKEN` alone not triggering it.

Net effect on the design: `CHART_BUMP_PAT` is used for both the `create-pull-request` step and the
`gh pr merge --auto` step in `bump-chart-version.yaml`; no `workflow_dispatch` workaround job was
needed in the end. Branch protection (`chart` required) and `allow_auto_merge: true` are both now
set on `tangle-deployments`.

### Revised: `image.tag` and minor bumps

Two changes to the above, made 2026-09-21 after watching three automated bumps land
(`tangle-deployments` [#25](https://github.com/ivanklee86/tangle-deployments/pull/25), with the
one-time catch-up of the drift it left behind in
[#27](https://github.com/ivanklee86/tangle-deployments/pull/27)).

1. **The bump writes `values.yaml`'s `image.tag` too, and the follow-up above is resolved the
   opposite way from how it was framed.** Bumping `Chart.yaml` alone was never enough: the chart's
   deployment template renders
   `image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"`,
   and `values.yaml` *pins* `image.tag`, so the `.Chart.AppVersion` fallback never applies and the
   published chart deploys whatever image the last hand-edit left there. Three automated releases
   ran before anyone noticed: `appVersion` reached `"0.3.0"` while `image.tag` sat at `"v0.1.0"`.

   The verification the follow-up asked for was done and came back **negative**:
   `ghcr.io/ivanklee86/tangle` publishes v-prefixed tags *only* (`v0.0.0` … `v0.3.0`, read from the
   registry's own tag list), so the `appVersion` string (`0.3.0`, `v` stripped) is not a pullable
   tag and collapsing the override onto `.Chart.AppVersion` would break the chart outright. It
   would need either the template to re-add the `v` prefix or `tangle` to publish unprefixed tags;
   neither is worth doing to delete one line. So the second copy of the version stays, and the
   automation keeps it in sync instead — writing the **raw** dispatch tag (`v0.3.0`), not
   `appVersion`.

   Mechanically this is a targeted `awk` substitution on the `tag:` line inside the top-level
   `image:` block, *not* `yq -i` as used for `Chart.yaml`. `yq -i` round-trips the whole document:
   on `Chart.yaml` (nine plain lines) that is invisible, but on `values.yaml` it strips every blank
   line and re-anchors the commented-out example blocks to their parent's indentation, turning a
   one-line bump into a ~60 line diff. `yq` is still used immediately afterwards to *read the value
   back and fail the run if it didn't land*, so a future `values.yaml` restructure that the `awk`
   no longer matches is loud rather than a PR that silently bumps nothing.

2. **The chart `version` bump is minor, not patch** (`{$NF+=1}` → `{$2+=1; $3=0}`, so `0.0.16` →
   `0.1.0`). The original narrow rule reasoned only about *needing some bump at all* for
   `chart-releaser-action` to publish; it picked patch as the smallest that satisfies that. In
   practice a `tangle` release is a new set of app features, and the chart revision that ships it
   is what consumers upgrade to get them — minor is the honest signal, and it keeps patch free to
   mean what it should: a chart-only fix, still bumped by hand. The rest of the original rule
   stands: nothing tries to infer *major* chart bumps from what changed in `tangle`.

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
- Original state (now superseded by this ADR): `tangle-deployments/.github/workflows/release.yaml`
  (`on: release: published`), `tangle-deployments/charts/tangle/Chart.yaml`
  (`appVersion: "0.1.0"`, `version: 0.0.9`), `tangle-deployments/charts/tangle/values.yaml`
  (`image.tag: "v0.1.0"`, overriding the chart's own `.Chart.AppVersion` default)
- `tangle` PR: [#228](https://github.com/ivanklee86/tangle/pull/228) (the `notify-tangle-deployments`
  dispatch job)
- `tangle-deployments` PRs, in the order they actually landed:
  [#11](https://github.com/ivanklee86/tangle-deployments/pull/11) (receiver workflow + CI +
  release-trigger retarget), [#14](https://github.com/ivanklee86/tangle-deployments/pull/14)
  (first, incomplete fix attempt for finding 3 above), [#16](https://github.com/ivanklee86/tangle-deployments/pull/16)
  (`CHART_BUMP_PAT` for opening the PR — finding 2), [#18](https://github.com/ivanklee86/tangle-deployments/pull/18)
  (`CHART_BUMP_PAT` for the merge too — the actual fix for finding 3, superseding #14's approach),
  [#25](https://github.com/ivanklee86/tangle-deployments/pull/25) (`values.yaml`'s `image.tag` and
  the switch to minor bumps — see "Revised" above), [#27](https://github.com/ivanklee86/tangle-deployments/pull/27)
  (the one-time `image.tag` catch-up for the drift the missing bump left behind, alongside an
  unrelated `manifestsWorkers` key fix; replaces #26, auto-closed by a branch rename)
- Related: [ADR 0013](0013-renovate-weekly-grouped-updates.md) — the automerge-non-major policy
  this ADR's auto-merge choice mirrors, applied here to a fully mechanical bump instead of a
  Renovate PR
- Superseded by, if adopted later: a GitHub App-based token replacing the fine-grained PATs.
