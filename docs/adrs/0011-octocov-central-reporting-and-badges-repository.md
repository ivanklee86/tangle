---
status: "proposed"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Stand up a dedicated octocov "central mode" repository for coverage badges and durable reporting

## Context and Problem Statement

[ADR 0010](0010-replace-codecov-with-octocov.md) gives `tangle` its own PR-diff coverage comment by persisting each run's octocov report to a GitHub Actions Artifacts datastore (`artifact://${GITHUB_REPOSITORY}`). That solves the diff-vs-base-branch comment, but it doesn't solve two things Codecov also used to provide: a publicly embeddable coverage badge (e.g. for `README.md`) and a durable, browsable coverage history — Actions artifacts are private-by-default, require authentication to fetch, and expire on GitHub's retention window (90 days by default). [octocov](https://github.com/k1LoW/octocov) has a built-in answer for this: "central mode," where a separate, dedicated repository periodically collects reports from one or more source repositories' datastores and republishes them as a generated dashboard (an `index.html`) and a set of badge SVGs, typically hosted via GitHub Pages. This ADR decides whether and how to stand one up.

## Decision Drivers

- A README badge needs a stable, public, unauthenticated URL to embed (`https://<pages-url>/badges/coverage.svg`) — Actions artifacts can't serve that directly (private API, auth required, expiring).
- octocov ships this exact pattern natively as "central mode" — a config section (`central:`) and a scheduled workflow, no custom dashboard code to write or maintain.
- octocov's documented pattern for collecting from a GitHub Actions Artifacts source is a scheduled (`on.schedule`) workflow in the central repo, not a push-triggered one — artifact-based collection isn't event-driven the way a git-backed datastore is, so freshness is bounded by the schedule interval, not immediate on every `tangle` push.
- A generically-named repository (not `tangle`-specific) lets this same piece of infrastructure aggregate coverage/metrics from other personal repos later without renaming or migrating anything, matching the pattern octocov's own maintainer uses for their open source projects' dashboard.
- GitHub Pages — the natural place to host a generated dashboard and badge SVGs — requires the hosting repository to be public (or a paid plan for private Pages), which is consistent with `tangle` itself already being public.

## Considered Options

- No central repository: rely solely on ADR 0010's artifact datastore for the PR-diff comment, and either skip a public badge entirely or point it at [octocov.dev](https://octocov.dev) (a free hosted viewer octocov links coverage values to when reports live in Actions artifacts) instead of standing up new infrastructure.
- A central-mode repository reading from octocov's `github://owner/repo/path` (git-backed) datastore instead of artifacts: source repos push their reports as commits directly into the central repo, letting it trigger `on.push` instead of a schedule — but this means `tangle` would need a *second* write path (a cross-repo write credential, pushing reports into `octocov-central`'s git history) in addition to the `artifact://` datastore ADR 0010 already configures it to write to.
- A central-mode repository reading from the `artifact://` datastore ADR 0010 already populates, collected on a schedule (chosen).

## Decision Outcome

Chosen option: "A central-mode repository reading from the `artifact://` datastore, collected on a schedule," standing up a new public repository, `ivanklee86/octocov-central`. It reuses exactly the datastore ADR 0010 already configures `tangle` to write to — no second write path, no additional credential needed from `tangle`'s own CI — at the cost of badge/dashboard freshness being bounded by the central repo's own schedule (recommend daily) rather than updating on every `tangle` push, which is an acceptable trade-off for something whose whole purpose is a slow-moving badge, not a live value.

### Consequences

- Good, because badges and a dashboard get a stable public URL (GitHub Pages) embeddable in `tangle`'s `README.md`, and in any future repo's README once added as another source.
- Good, because it reuses ADR 0010's artifact datastore as-is — `tangle`'s CI doesn't gain a second reporting destination or a new outbound credential.
- Good, because adding another source repo later is a one-line config change (another `artifact://owner/repo` entry in `central.reports.datastores`), not new infrastructure.
- Neutral, because reading `tangle`'s artifacts from `octocov-central`'s own workflow needs a cross-repo *read* credential — the default `GITHUB_TOKEN` in a workflow is scoped to the repo it runs in, so `octocov-central` needs a PAT (classic or fine-grained, `actions:read` scope on `tangle`) stored as a secret in `octocov-central` (e.g. `TANGLE_ARTIFACTS_TOKEN`). This is new secret-management surface — notably the opposite direction of ADR 0010's "drop a token" motivation — though it's narrowly scoped to read-only artifact access on one repository, unlike Codecov's broader upload token.
- Bad, because the badge/dashboard is only as fresh as the schedule interval (recommend daily), not live — a coverage change lands in `tangle`'s own PR-diff comment (ADR 0010) immediately, but won't reach the public badge until the next scheduled run.
- Bad, because it's another repository to maintain going forward — its own dependency/action-version pinning (per AGENTS.md), its own occasional breakage to notice and fix — even though its content is almost entirely generated and it needs no application code of its own.

## Pros and Cons of the Options

### No central repository

- Good, because it's zero additional infrastructure beyond ADR 0010.
- Bad, because it leaves no public badge and no durable history — exactly the two things this ADR exists to provide; ADR 0010's artifact datastore alone is bounded by GitHub's retention window and isn't publicly embeddable.

### Central repository via the `github://` (git-backed) datastore

- Good, because collection can be `on.push`-triggered — fresher than a schedule.
- Bad, because it requires `tangle` to gain a second report write path (pushing into `octocov-central`'s git history) in addition to ADR 0010's `artifact://` datastore, duplicating bookkeeping for no benefit specific to this repo's own needs.

### Central repository via the `artifact://` datastore, scheduled (chosen)

See Decision Outcome.

## More Information

- [octocov central mode](https://github.com/k1LoW/octocov#central)
- [octocov.dev](https://octocov.dev) (the hosted viewer octocov links to for artifact-stored reports — considered and rejected in favor of a fully self-hosted badge/dashboard, since it's a third-party service outside this repo's control)
- Implementation notes: [CI pipeline restructure, workstream 11](../agents/plans/ci-pipeline-restructure.md) (most of this workstream's steps live in the new `octocov-central` repository, not in `tangle`)
- Related: [ADR 0010](0010-replace-codecov-with-octocov.md) (the artifact datastore this central repository reads from)
- Superseded by, if adopted later: none.
