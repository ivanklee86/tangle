---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Restructure Renovate into weekly grouped updates with gated automerge

## Context and Problem Statement

`renovate.json` today has three problems now that [ADR 0003](0003-svelte-e2e-testing-strategy.md)/the e2e work has landed and dependency PRs are cheap to trust again: it fires only every two weeks (`"every 2 weeks on friday"`), which lets updates pile up and makes each PR riskier to review; it groups only `gomod` deps (`"go.mod dependencies"`) and leaves `npm`, `helmv3`, and `github-actions` deps ungrouped, so every individual TypeScript/Helm/Action bump opens its own PR; and it has no `minimumReleaseAge`, so a PR can land the same day a package is released, before a supply-chain problem or a bad release would typically surface. Separately, a real review of every file Renovate can reach in this repo found several pinned tool versions that Renovate cannot see at all today — `go install` lines in `tasks/go.yaml` and `Dockerfile`, `ARG K3D_VERSION`/`ARG PLAYWRIGHT_VERSION` in `.devcontainer/Dockerfile`, and the `gateway-api` release URL in `tasks/k8s.yaml` — which is at odds with AGENTS.md's "versions should always be pinned" standard, since a pin nobody bumps just goes stale in place. Two Docker image references (`alpine/helm`, `derailed/k9s` in `.devcontainer/Dockerfile`) are not pinned to a tag at all, so Renovate has nothing to bump even after the fix above. `.devcontainer/Dockerfile`'s "Go extras not in the central go image" section (added after this ADR was first drafted, by since-merged work) goes further still: `go install github.com/air-verse/air@latest` and `...swagger@latest` are pinned to nothing at all — the same failure mode as the untracked pins above, just with no version string for Renovate's `go install` custom manager to even capture.

## Decision Drivers

- Weekly cadence with per-ecosystem grouping (go/ts/k8s/ci) keeps each PR reviewable and predictable, instead of a biweekly grab-bag.
- A 5-day `minimumReleaseAge` with `timestamp-optional` behavior gives a real safety buffer against yanked/compromised releases, without permanently blocking updates from datasources that don't report release timestamps.
- `automerge`/`platformAutomerge` (already used for the `devcontainer` group) removes babysitting for low-risk bumps, but major-version bumps carry real breaking-change risk (Sveltekit, `argo-cd`/`traefik` chart majors, Go major bumps) and should stay a manual merge.
- A repo-wide "review" was asked for, not just the four named groups — leaving known-stale, Renovate-invisible pins in place while restructuring everything else around them would be inconsistent with the pinning standard this same change is meant to uphold.
- `renovatebot/pre-commit-hooks`' `renovate-config-validator` hook is `language: node`, so (unlike the repo's existing `web-lint` hook) it runs under pre-commit.ci per [ADR 0012](0012-pre-commit-ts-and-markdown-checks.md), giving `renovate.json` a real, CI-enforced syntax/schema gate instead of only local, pre-push convenience.

## Considered Options

- **Schedule**: keep biweekly vs. move to weekly (chosen) vs. daily (too noisy for a grouped-PR model).
- **Grouping**: leave ungrouped (today) vs. one group per Renovate *manager* (chosen: go=`gomod`, ts=`npm`, k8s=`helmv3`, ci=`github-actions`, each also absorbing the same-ecosystem `dockerfile`/custom-manager deps) vs. one single "everything" group (rejected — defeats reviewability, and mixes unrelated blast radii in one PR).
- **Automerge scope**: automerge everything including majors (rejected — a major bump automerging unattended is exactly the risk `minimumReleaseAge` doesn't fully cover) vs. automerge minor/patch/pin/digest only, majors always manual (chosen) vs. no automerge at all (rejected — loses the point of the existing `devcontainer` precedent and doubles review load for no real benefit on low-risk bumps).
- **Renovate-invisible pins** (`go install` lines, `K3D_VERSION`, `PLAYWRIGHT_VERSION`, gateway-api URL): leave as-is, out of scope (rejected — see Decision Drivers) vs. add `customManagers` (`customType: "regex"`) entries so they're tracked and grouped like everything else (chosen).
- **Config validation**: no validation, rely on the Renovate app's own dashboard errors after the fact (today's state) vs. `renovatebot/pre-commit-hooks`' `renovate-config-validator` (chosen, `language: node`, runs under pre-commit.ci) vs. a bespoke CI job step running `npx renovate-config-validator` (rejected — duplicates what the pre-commit hook already gives for free under pre-commit.ci, per the `markdownlint-cli2` precedent in ADR 0012, without the local pre-commit feedback loop).
- **`pre-commit` manager** (Renovate updating `.pre-commit-config.yaml`'s own `rev:` pins): considered and rejected — it's disabled by default upstream (a deliberate, indefinite decision by the Renovate maintainers, not a gap), and pre-commit.ci already auto-updates those `rev`s on its own weekly schedule per ADR 0012, so enabling it would just create two systems fighting over the same file.

## Decision Outcome

Chosen options, landed together in one `renovate.json` restructure (see [the implementation plan](../agents/plans/renovate-weekly-grouped-updates.md) for the exact config and file-by-file changes):

1. **Weekly schedule**: `"schedule": ["on friday"]` (same day as today, biweekly modifier dropped).
2. **Four ecosystem groups** — `go`, `ts`, `k8s`, `ci` — each grouping its native Renovate manager (`gomod`, `npm`, `helmv3`, `github-actions` respectively) together with the same-ecosystem `dockerfile` images (by `matchPackageNames`) and the new `customManagers` entries below (by `matchDatasources`).
3. **`minimumReleaseAge: "5 days"` with `minimumReleaseAgeBehaviour: "timestamp-optional"`**, set at the config root so it applies to every manager, including datasources that don't return a release timestamp.
4. **Automerge (`automerge: true`, `platformAutomerge: true`) for every non-major update**, and an explicit `matchUpdateTypes: ["major"]` rule keeping `automerge: false` — applied repo-wide, not just to the four groups, which also tightens the existing `devcontainer` group (today it automerges majors unconditionally; after this change it doesn't).
5. **New `customManagers` entries** (regex, following Renovate's own documented `ENV X_VERSION=... # datasource/depName` convention, adapted to `ARG` and to bare `go install` lines) so the `go install` pins, `K3D_VERSION`, `PLAYWRIGHT_VERSION`, and the gateway-api release URL are tracked and grouped like every other dependency.
6. **Pin `alpine/helm` and `derailed/k9s` to explicit tags** (`4.3.0`, `v0.50.18` as of 2026-09-20) in `.devcontainer/Dockerfile` — a prerequisite for Renovate to track them at all, and a direct fix for an existing AGENTS.md pinning-rule violation found during this review.
7. **Pin `air-verse/air` and the devcontainer's own `go-swagger` install to explicit versions** (`v1.67.4`, `v0.36.6` as of 2026-09-20, the latter matching `tasks/go.yaml`'s pin) in `.devcontainer/Dockerfile` — same rationale as 6, for the `@latest` pins found in that file's "Go extras" section.
8. **`renovatebot/pre-commit-hooks`' `renovate-config-validator` hook**, added to `.pre-commit-config.yaml`, pinned to `44.103.6`.
9. **Track `.github/workflows/ci.yaml`'s own `k3d`/`argocd` version copies** (a CodeRabbit finding on the PR) — the `e2e` job hardcoded both versions a second time (cache key, install commands) with only a comment asking a human to keep them in sync with `.devcontainer/Dockerfile`. Collapsed to a job-level `env:` block and tracked by a new `customManagers` entry, joining the same `k8s` group so a bump moves every occurrence together.

### Consequences

- Good, because each of the four weekly PRs (when there's anything to update) stays scoped to one ecosystem, and low-risk bumps land without manual merges.
- Good, because previously-invisible pins (Go tool installs, k3d, Playwright, gateway-api) are now tracked and will actually get bumped instead of silently going stale.
- Good, because `renovate.json` gets a real syntax/schema check before merge, the same way TypeScript/Svelte and Markdown already do (ADR 0012).
- Neutral, because `PLAYWRIGHT_VERSION` (`.devcontainer/Dockerfile`) and `@playwright/test`/`playwright` (`web/package.json`) are grouped into the same `ts` PR but are not forced to bump together — if only one has a new release in a given week, the "these must match exactly" invariant documented in `.devcontainer/Dockerfile` can be briefly out of sync until the next PR catches the other one up. Grouping narrows this window to at most a week; it doesn't close it.
- Neutral, because the `ci` group covers the whole `github-actions` manager, i.e. `release.yaml` and `release-docs.yaml` too, not just `ci.yaml` — there was no finer-grained split requested, and splitting further would need per-workflow `matchFileNames` rules with no clear benefit today.
- Bad, because `minimumReleaseAge` delays every update by 5 days regardless of how urgent it is (e.g. a security patch); `dependencyDashboard` (already on via `config:recommended`) remains the way to see what's pending and, if needed, manually bypass the wait for an individual PR.

## Pros and Cons of the Options

### Automerge everything including majors

- Good, because it matches the literal wording of the existing `devcontainer` precedent with no carve-out.
- Bad, because a major-version automerge (e.g. Sveltekit, `argo-cd`/`traefik` chart majors) can break the build or runtime behavior with nobody having looked at the diff first, and `minimumReleaseAge` alone doesn't mitigate a bad *design*, only a bad or yanked *release*.

### Leave Renovate-invisible pins out of scope

- Good, because it's a smaller, more literal implementation of "go/ts/k8s/ci groups."
- Bad, because those pins (four `go install` tool versions, `k3d`, Playwright, gateway-api) stay permanently stale unless someone remembers to bump them by hand — the exact failure mode AGENTS.md's pinning rule exists to prevent, and this review was the natural point to catch it.

## More Information

- Plan: [Renovate weekly grouped updates](../agents/plans/renovate-weekly-grouped-updates.md)
- [Renovate: `minimumReleaseAge`](https://docs.renovatebot.com/configuration-options/#minimumreleaseage) / [`minimumReleaseAgeBehaviour`](https://docs.renovatebot.com/configuration-options/#minimumreleaseagebehaviour)
- [Renovate: `customManagers`](https://docs.renovatebot.com/configuration-options/#custommanagers)
- [`renovatebot/pre-commit-hooks`](https://github.com/renovatebot/pre-commit-hooks)
- [ADR 0012](0012-pre-commit-ts-and-markdown-checks.md) (pre-commit.ci's `language: node` vs. `language: system` distinction this decision relies on)
- Superseded by, if adopted later: none.
