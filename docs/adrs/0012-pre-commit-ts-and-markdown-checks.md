---
status: "accepted"
date: 2026-09-20
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Add pre-commit checks for TypeScript/Svelte and Markdown

## Context and Problem Statement

`.pre-commit-config.yaml` only checked whitespace/EOF/YAML (`pre-commit/pre-commit-hooks`) and Go formatting (`dnephin/pre-commit-golang`). Nothing ran against `web/`'s TypeScript/Svelte code or the repo's Markdown (`README.md`, `CONTRIBUTING.md`, `docs/**/*.md`, including ADRs and plans) before a commit landed, even though both are already linted in real CI: the `ts` job in `.github/workflows/ci.yaml` runs `task ts:lint` (`svelte-check` + `prettier --check` + `eslint`) on every PR touching `web/`, and nothing lints Markdown anywhere. This ADR covers how to add both, given one constraint specific to this repo: [pre-commit.ci](https://pre-commit.ci) is enabled (see the badge in `README.md`), which runs hooks on PRs and auto-updates `rev`s weekly, but does not execute `language: system` (or `script`) hooks — it silently skips them.

## Decision Drivers

- The `ts` job in CI is already the enforcement gate for TypeScript/Svelte; a pre-commit hook only needs to give a developer the same feedback before they push, not duplicate CI as a second gate.
- `web/`'s ESLint config is flat-config with `typescript-eslint`, `eslint-plugin-svelte`, and `eslint-config-prettier` — mirroring that toolchain via `pre-commit/mirrors-eslint`'s `additional_dependencies` means hand-duplicating every plugin version from `web/package.json` and keeping the two in sync by hand indefinitely.
- pre-commit.ci doesn't run `language: system` hooks, so any hook that shells out to `web/`'s own `npm` scripts can only ever be local, pre-push convenience — not something to rely on as a CI gate.
- Markdown has no existing tooling to reuse or duplicate against, so the mirroring-cost trade-off above doesn't apply there.
- AGENTS.md asks for full-length lines in docs (no hard-wrapping), which conflicts with markdownlint's default line-length rule (`MD013`) — `docs/adrs/*.md` already runs lines past 1000 characters by design.

## Considered Options

- **TypeScript/Svelte**: `pre-commit/mirrors-eslint` (+ a separate Prettier mirror), duplicating plugin versions into `additional_dependencies`.
- **TypeScript/Svelte**: a `repo: local`, `language: node` hook installing the same dependency set via `additional_dependencies` (works under pre-commit.ci, still duplicates versions).
- **TypeScript/Svelte**: a `repo: local`, `language: system` hook that runs `web/`'s own `npm run lint` (chosen) — no duplicated dependency declarations, but skipped by pre-commit.ci.
- **Markdown**: `pre-commit/mirrors-eslint` with `eslint-plugin-markdown`, or a bespoke Prettier-based check.
- **Markdown**: [`DavidAnson/markdownlint-cli2`](https://github.com/DavidAnson/markdownlint-cli2)'s own pre-commit hook (chosen) — `language: node`, manages its own dependencies, no coupling to `web/package.json`, works fine under pre-commit.ci.

## Decision Outcome

Chosen options: a `repo: local` `web-lint` hook (`language: system`, running `cd web && npm run lint`) for TypeScript/Svelte, added to `ci.skip` in `.pre-commit-config.yaml` with an inline comment explaining why; and `DavidAnson/markdownlint-cli2`'s hook for Markdown, configured via a new root `.markdownlint-cli2.yaml`.

`web-lint` trades CI enforcement (which it never provides — pre-commit.ci skips it, and the real gate is the `ts` GitHub Actions job either way) for zero duplicated tooling configuration: it always lints with exactly the ESLint/Prettier setup `web/package.json` and `web/eslint.config.js` already define, so it can't drift from what CI actually runs. The `.markdownlint-cli2.yaml` config disables three default rules to match established conventions rather than fighting them:

- `MD013` (line-length) — conflicts with AGENTS.md's full-length-lines instruction and how `docs/adrs/*.md` is already written.
- `MD010` (hard tabs), scoped to `code_blocks: false` only — `docs/agents/plans/*.md` quote real TS/Svelte source verbatim, and `web/.prettierrc` sets `useTabs: true`, so those snippets are legitimately tab-indented; prose outside code fences is still checked.
- `MD036` (emphasis-as-heading) — `docs/agents/plans/*.md` use a bold `**Steps**`-style label as an inline marker inside an already-headed workstream, not as a substitute document heading.
- `MD041` (first-line-heading) — most of `docs/*.md` is rendered by mkdocs-material (see `mkdocs.yml`'s `nav`), which already supplies each page's title from the nav entry; those pages deliberately don't repeat it as an in-page H1.

Everything else runs at markdownlint's defaults; genuine violations found on the first full-repo run (missing blank lines around headings/fences, un-annotated code fences, bare URLs, two files with two real top-level H1s) were fixed in the files themselves rather than suppressed.

### Consequences

- Good, because both checks run at commit time with no new tooling to hand-maintain — `web-lint` always matches whatever `web/package.json`/`eslint.config.js` currently define, and `markdownlint-cli2` manages its own dependencies.
- Good, because Markdown gets a real CI-enforced gate for the first time (via pre-commit.ci), on top of local, pre-commit feedback.
- Neutral, because `web-lint` provides no new CI enforcement of its own — it's strictly pre-push convenience layered on top of the `ts` job, which remains the actual gate for TypeScript/Svelte.
- Neutral, because `web-lint` requires `web/node_modules` to exist locally (`npm install` in `web/`) for the hook to run at all — the same prerequisite `task ts:install`/`task ts:lint` already have, and already documented in `docs/contributing.md`'s local development workflow.
- Bad, because a contributor who skips `npm install` in `web/` gets no pre-commit signal on TypeScript/Svelte changes at all (the hook errors rather than silently passing) until they push and CI runs.

## Pros and Cons of the Options

### `pre-commit/mirrors-eslint` (+ Prettier mirror), duplicated dependencies

- Good, because it runs under pre-commit.ci, giving TypeScript/Svelte a second CI-enforced gate alongside the `ts` job.
- Bad, because every ESLint/Prettier plugin version in `web/package.json` (`typescript-eslint`, `eslint-plugin-svelte`, `eslint-config-prettier`, `prettier-plugin-svelte`, `prettier-plugin-tailwindcss`, …) has to be re-declared in `additional_dependencies` and manually kept in sync — a drift source with no corresponding benefit, since the `ts` job already runs the real thing.

### Local `language: node` hook with duplicated `additional_dependencies`

- Good, because it also runs under pre-commit.ci.
- Bad, because it has the same duplication problem as the mirror option, just declared differently.

### Local `language: system` hook running `npm run lint` (chosen)

- Good, because it can never drift from `web/`'s actual lint configuration — there's only one definition of "how to lint `web/`," and both this hook and CI call it.
- Bad, because pre-commit.ci skips `language: system` hooks, so it adds no CI enforcement beyond what the `ts` job already provides.

### `markdownlint-cli2` pre-commit hook (chosen)

- Good, because it's self-contained (`language: node`, own dependency resolution), runs identically locally and under pre-commit.ci, and needs no coupling to `web/`'s toolchain.
- Neutral, because its default rule set needed three targeted overrides (above) to match conventions already established in this repo's docs, rather than being usable entirely out of the box.

## More Information

- Plan: [Pre-commit checks for TypeScript and Markdown](../agents/plans/pre-commit-ts-markdown-checks.md)
- [markdownlint-cli2](https://github.com/DavidAnson/markdownlint-cli2)
- [pre-commit/mirrors-eslint](https://github.com/pre-commit/mirrors-eslint) (considered and rejected)
- [pre-commit.ci](https://pre-commit.ci)
- Superseded by, if adopted later: [ADR 0014](0014-drop-precommit-ci-run-prek-in-actions.md) (the
  `language: system` vs. `language: node`/pre-commit.ci distinction this ADR's `web-lint` design
  leans on no longer applies once pre-commit.ci is dropped — the split itself still stands, just
  for a different reason: avoiding duplication with the `ts` job, not a sandbox limitation).
