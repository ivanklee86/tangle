---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Replace Codecov with octocov for Go coverage reporting

## Context and Problem Statement

`.github/workflows/ci.yaml`'s `go` job converts Go's native coverage profile (`coverage.out`, produced by `task go:test-ci`) to lcov via `jandelgado/gcov2lcov-action@v1.1.1`, then uploads it to Codecov via `codecov/codecov-action@v5`, authenticated with a `CODECOV_TOKEN` repository secret. This is an external SaaS dependency with a secret to provision and rotate, plus a format-conversion step that exists only because Codecov's action doesn't consume Go's native coverage format directly. [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md)/[the CI pipeline restructure plan](../agents/plans/ci-pipeline-restructure.md) already touches this exact job's later steps (JUnit publishing, artifact upload) while restructuring `ci.yaml`, making this a natural point to also drop the external dependency in favor of [octocov](https://github.com/k1LoW/octocov), a self-contained Go coverage/test-reporting tool that reads Go's coverage profile format directly, posts PR comments using the workflow's own `GITHUB_TOKEN`, and — the question this ADR resolves — can persist reports across runs so that PR comments show a coverage *diff* against the base branch, not just the current run's total.

## Decision Drivers

- octocov reads Go's native coverage profile format directly, so the `jandelgado/gcov2lcov-action` conversion step becomes unnecessary.
- octocov posts PR coverage comments using the workflow's own `GITHUB_TOKEN`, so there's no external account to sign up for and no `CODECOV_TOKEN` secret to provision, store, or rotate.
- A diff-vs-base-branch coverage comment (the thing a Codecov PR comment gives today) needs octocov's `report`/`diff` config to persist each run's report to a datastore it can read back later — a plain PR-comment-only setup can't show that diff, since it has nothing to compare against.
- octocov supports a GitHub Actions Artifacts datastore (`artifact://owner/repo`) out of the box: it stores/reads reports as workflow artifacts on the same repository, authenticated with the same `GITHUB_TOKEN` already in scope, no new credentials, no new repository, no cloud account. This is the cheapest way to unblock the diff comment for *this* repo specifically.
- Fewer externally-hosted dependencies in CI's trust boundary is a general win, and is consistent with this project's existing bias toward self-hosted infrastructure for testing (k3d, ArgoCD, and now the mock-backed Go tests from ADR 0009, rather than reaching for hosted equivalents).
- AGENTS.md requires pinned versions — `k1LoW/octocov-action` is pinned to a specific tag the way `codecov/codecov-action`/`jandelgado/gcov2lcov-action` are today, and the octocov binary itself (installed by the action, `latest` by default) additionally needs its own explicit `version:` pin to actually satisfy that rule.

## Considered Options

- Keep Codecov as-is (`gcov2lcov` conversion + `codecov/codecov-action` + `CODECOV_TOKEN`).
- Switch to octocov, coverage-percentage PR comment only, no `report`/`diff` datastore — no historical trend or diff-vs-base-branch comparison.
- Switch to octocov with the GitHub Actions Artifacts datastore (`artifact://${GITHUB_REPOSITORY}`) for `report`/`diff` — diff-vs-base-branch PR comments and a short-lived history, using only this repo's own existing Actions artifact storage.
- Switch to octocov with a dedicated git "report" repository (octocov's `github://owner/repo/path` datastore) or a cloud datastore (S3/GCS/BigQuery) for `report`/`diff` — permanent, unbounded history, at the cost of standing up and authenticating to a second repository or cloud account just for this.

## Decision Outcome

Chosen option: "Switch to octocov with the GitHub Actions Artifacts datastore for `report`/`diff`," because it gets the one thing a plain PR-comment-only setup can't — a diff-vs-base-branch coverage comment — using infrastructure this repo already has (Actions artifacts, `GITHUB_TOKEN`) instead of standing up a second repository or a cloud account just to unblock it. Artifacts have GitHub's own retention window (org/repo-level setting, 90 days by default), so this datastore is a rolling window, not a permanent archive — that's an accepted, bounded trade-off for *this repo's* own PR-diff feature specifically. A permanent, cross-run, publicly-browsable coverage history and badge set is a separate concern, addressed by standing up a dedicated octocov "central mode" repository — see [ADR 0011](0011-octocov-central-reporting-and-badges-repository.md), which reads reports from this same `artifact://` datastore on a schedule and republishes them somewhere durable.

### Consequences

- Good, because there's no more `CODECOV_TOKEN` secret to provision, store, or rotate, and one less external SaaS account in CI's trust boundary.
- Good, because it drops the `jandelgado/gcov2lcov-action` conversion step entirely — octocov reads `coverage.out` (the same file `task go:test-ci` already produces, unchanged by this decision) natively.
- Good, because PR comments and the artifact datastore both authenticate with the workflow's default `GITHUB_TOKEN`, and `ci.yaml` already grants `pull-requests: write` at the top-level `permissions:` block for `EnricoMi/publish-unit-test-result-action` — no new permission scope or secret is needed for either.
- Good, because PRs now get a real coverage *diff* against the base branch's most recent report (not just a bare current-run total), which is the actually-useful signal a coverage comment exists to give a reviewer.
- Neutral, because the `artifact://` datastore's history is bounded by GitHub's artifact retention window (90 days by default, configurable per-repo) — this is a rolling window suitable for base-branch-diffing, not a permanent trend archive. [ADR 0011](0011-octocov-central-reporting-and-badges-repository.md)'s central repo is what turns this into durable history.
- Bad, because whatever historical coverage data already exists on Codecov's dashboard for this repository doesn't carry over — this is a one-way migration for that history. Removing the `CODECOV_TOKEN` secret and/or the Codecov project integration itself is a manual follow-up in GitHub/Codecov settings, outside this repo's version control and outside this ADR's scope.
- Bad, because there are now two octocov-related workstreams instead of one drop-in swap (this ADR's artifact datastore, plus [ADR 0011](0011-octocov-central-reporting-and-badges-repository.md)'s central repository) — accepted because the alternative (PR-comment-only, no diff) doesn't actually deliver the feature Codecov's comment gave today.

## Pros and Cons of the Options

### Keep Codecov as-is

- Good, because zero migration work and no change in behavior.
- Bad, because it's the exact dependency (external SaaS + secret + conversion step) this decision exists to remove, and doesn't get any simpler by being revisited later — the conversion step and token management cost keeps accruing either way.

### Switch to octocov, PR comment only, no datastore

- Good, because it's the smallest possible change — no new config sections, no artifact-retention caveat to explain.
- Bad, because it silently drops the diff-vs-base-branch comparison Codecov's comment already gave — a plain "here's the current total" comment is a real regression in the information a reviewer sees on a PR, not a lateral move.

### Switch to octocov with the GitHub Actions Artifacts datastore (chosen)

See Decision Outcome.

### Switch to octocov with a dedicated report repository or cloud datastore

- Good, because it's a strict superset of the chosen option's coverage visibility for *this* repo alone — permanent trend history with no retention window, no separate central repo needed just to keep old reports around.
- Bad, because it requires provisioning and authenticating to a second repository or a cloud account (S3/GCS/BigQuery credentials) purely to store `tangle`'s own reports — new CI infrastructure and a new credential this repo doesn't otherwise have, when the artifact datastore already unblocks the diff comment with zero new credentials. This option's permanent-storage benefit is exactly what [ADR 0011](0011-octocov-central-reporting-and-badges-repository.md)'s central repository provides anyway, so paying for it twice (once here, once there) is redundant.

## More Information

- [octocov](https://github.com/k1LoW/octocov) and [`k1LoW/octocov-action`](https://github.com/k1LoW/octocov-action)
- [octocov's GitHub Actions Artifacts datastore (`artifact://` scheme)](https://github.com/k1LoW/octocov#artifact)
- Implementation plan: [CI pipeline restructure, workstream 10](../agents/plans/ci-pipeline-restructure.md)
- Related: [ADR 0009](0009-ci-pipeline-test-taxonomy-and-conditional-jobs.md) (the broader `go` job restructure this coverage-tooling swap rides along with)
- Related: [ADR 0011](0011-octocov-central-reporting-and-badges-repository.md) (the dedicated central-mode repository that turns this ADR's artifact datastore into durable, publicly-browsable history and badges)
- Current-state reference: [docs/agents/ci.md](../agents/ci.md)
- Superseded by, if adopted later: none.
