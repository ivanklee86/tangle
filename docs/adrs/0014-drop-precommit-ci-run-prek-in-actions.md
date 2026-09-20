---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Drop pre-commit.ci, run prek's checks as a GitHub Actions job instead

## Context and Problem Statement

The repo currently relies on [pre-commit.ci](https://pre-commit.ci), a hosted GitHub App, to run `.pre-commit-config.yaml`'s hooks on every PR and to auto-update hook `rev`s on its own weekly schedule; `README.md` carries its status badge, and `.pre-commit-config.yaml`'s `ci: skip: [web-lint]` key is pre-commit.ci-specific configuration (it's read only by the hosted service, not by local `pre-commit`/`prek` runs). Since [ADR 0012](0012-pre-commit-ts-and-markdown-checks.md), local development has moved to [`prek`](https://github.com/j178/prek) (a faster, Rust-based, drop-in-compatible reimplementation of `pre-commit` — see `.devcontainer`'s `postCreateCommand`), and the project no longer wants a third-party hosted service in the loop: the goal is to run the same hook suite directly in `ci.yaml`, on infrastructure already trusted for everything else in this pipeline.

This also changes the reasoning ADR 0012 built its `web-lint` design around. That ADR's central distinction — `language: node` hooks (like `markdownlint-cli2`) get real CI enforcement because pre-commit.ci runs them, while `language: system`/`script` hooks (like `web-lint`, and `go-fmt` from `dnephin/pre-commit-golang`) are silently skipped by pre-commit.ci's sandbox and become local-only convenience — stops being about a sandbox limitation once pre-commit.ci is gone. A self-hosted GitHub Actions job has no such restriction; whether a hook gets CI enforcement is now purely our own choice of what to wire up in the new job, not a platform constraint.

## Decision Drivers

- Removing a hosted third-party GitHub App reduces external dependencies and one more place a repo integration can silently break or fall out of sync (badge pointing at a service no longer gating anything).
- `prek` is already the local tool of record ([ADR 0012](0012-pre-commit-ts-and-markdown-checks.md), `.devcontainer`'s `postCreateCommand`) — running it in CI too means exactly one implementation of "run these hooks," not two (prek locally, pre-commit.ci's own runner remotely) that could theoretically diverge.
- `go-fmt` and `web-lint` already have a real, path-filtered CI gate elsewhere (the `go` job's own `gofmt` check, the `ts` job's `task ts:lint`) — the new job doesn't need to re-provision a Node/npm toolchain just to re-run `web-lint`, and should stay lean (checkout + Go, for `go-fmt` only) rather than duplicating the `ts` job's setup wholesale.
- AGENTS.md: versions should always be pinned — `j178/prek-action` and the `prek` version it installs both need explicit pins, matching this repo's existing SHA-pin-with-version-comment convention for lesser-known third-party actions (`k1LoW/octocov-action`).

## Considered Options

- **Keep pre-commit.ci, do nothing**: rejected — the explicit ask is to stop using it now that `prek` covers the same ground locally.
- **New CI job via a hand-rolled `pip install pre-commit` step**: rejected — reintroduces the slower classic `pre-commit`, when the project has already standardized on `prek`.
- **New CI job via [`j178/prek-action`](https://github.com/j178/prek-action)** (chosen): the same author's own GitHub Action, documented in `prek`'s README, with built-in caching and no separate install step to hand-maintain.
- **Job scope**: run every hook including `web-lint` (rejected for this change — would need a full Node/npm-install setup purely to re-run a check the `ts` job already gates, for no coverage gain) vs. run everything except `web-lint`, matching pre-commit.ci's own prior behavior (chosen, via `SKIP=web-lint`) vs. run only the hooks with no other CI gate at all, i.e. also drop `go-fmt` from the new job (rejected — `go-fmt` needs a Go toolchain regardless, and the `go` job already pays that setup cost, but the new job is meant to be pre-commit.ci's replacement, and `go-fmt` was never skipped there).
- **Job gating**: path-filtered like `go`/`ts`/`docs` (rejected — most of these hooks, e.g. `trailing-whitespace`/`check-yaml`/`markdownlint-cli2`, are repo-wide with no single ecosystem path to filter on) vs. always-run, same as `e2e` (chosen).

## Decision Outcome

Chosen options:

1. Remove the pre-commit.ci badge from `README.md` and the `ci:` key from `.pre-commit-config.yaml` (dead configuration once nothing reads it).
2. Add a new, always-run `pre-commit` job to `.github/workflows/ci.yaml`: `actions/checkout@v4` + `actions/setup-go@v5` (pinned `1.27`, for `go-fmt`) + `j178/prek-action@4e14d07f9231acabce116ccfca13b13dd9755ece # v3.0.0` (pinned `prek-version: 0.5.3`), with `SKIP: web-lint` in the step's `env` so `web-lint` keeps its existing local-only status — same behavior pre-commit.ci already had via `ci: skip:`, just expressed as a `SKIP` env var instead since there's no longer a pre-commit.ci-specific config key to read.
3. Rewrite the `web-lint` hook's explanatory comment in `.pre-commit-config.yaml` — the "pre-commit.ci skips `language: system` hooks" framing is no longer accurate; the real reason it's excluded from the new CI job is to avoid duplicating the `ts` job's own `task ts:lint` gate.
4. Manually uninstall/disable the pre-commit.ci GitHub App on this repository (`https://github.com/apps/pre-commit-ci` → Configure, or the repo's Installed GitHub Apps settings) — not a file change, so it's a follow-up step outside this plan's diff, but necessary so the app stops attempting to run/comment on PRs after the badge and its rationale are gone.
5. Amend [ADR 0012](0012-pre-commit-ts-and-markdown-checks.md)'s "Superseded by" pointer to reference this ADR — its `web-lint`/`markdownlint-cli2` split decision still stands, only the "why pre-commit.ci treats them differently" reasoning is superseded.

### Consequences

- Good, because one less hosted third-party integration is in the loop, and `prek` becomes the single implementation of "run these hooks," used identically locally and in CI.
- Good, because `go-fmt` now gets CI enforcement through this new job too (in addition to the `go` job's own `gofmt` check) — mild, harmless duplication, not a gap.
- Neutral, because `web-lint` still has no *pre-commit-driven* CI enforcement, same as today — its real gate remains the `ts` job's `task ts:lint`, unchanged by this ADR.
- Neutral, because the new job re-installs a Go toolchain independently of the `go` job (GitHub Actions jobs run in separate VMs with no shared state) — this mirrors the `e2e` job's already-established pattern of re-provisioning toolchains rather than depending on another job's setup.
- Bad, because `rev:` auto-updates that pre-commit.ci previously ran on its own weekly schedule stop happening automatically — Renovate's `pre-commit` manager is deliberately disabled by upstream Renovate policy (see ADR 0013, "Renovate weekly grouped updates," for the full reasoning), so those hook `rev`s (`pre-commit-hooks`, `pre-commit-golang`, `markdownlint-cli2`, `renovatebot/pre-commit-hooks`) now need a manual bump when a maintainer notices they're stale, with no automated nudge from either system.

## More Information

- Plan: [Drop pre-commit.ci, run prek in CI](../agents/plans/drop-precommit-ci-run-prek-in-actions.md)
- [`prek`](https://github.com/j178/prek)
- [`j178/prek-action`](https://github.com/j178/prek-action)
- [pre-commit.ci](https://pre-commit.ci) (being dropped)
- [ADR 0012](0012-pre-commit-ts-and-markdown-checks.md) (superseded in part — see above)
- Superseded by, if adopted later: none.
