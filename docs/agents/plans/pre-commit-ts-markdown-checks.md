# Pre-commit checks for TypeScript and Markdown

Status: implemented · 2026-09-20

Before this change, `.pre-commit-config.yaml` only checked whitespace/EOF/YAML (`pre-commit/pre-commit-hooks`) and Go formatting (`dnephin/pre-commit-golang`). Nothing ran against `web/`'s TypeScript/Svelte code or the repo's Markdown (`README.md`, `CONTRIBUTING.md`, `docs/**/*.md`, including ADRs and plans) before a commit lands. This plan adds both, informed by two constraints specific to this repo:

- **pre-commit.ci is active** (badge in `README.md:3`), which auto-runs hooks on PRs and auto-updates `rev`s weekly. pre-commit.ci does not support `language: system` (or `script`) hooks — it silently skips them — so any hook that shells out to the project's own `npm` scripts only provides local, pre-push feedback; it can't be the thing enforcing the check in CI.
- **TS is already linted in real CI.** `.github/workflows/ci.yaml`'s `ts` job runs `task ts:lint` (`npm run check` + `npm run lint`, i.e. `svelte-check` + `prettier --check` + `eslint`) on every PR touching `web/`. That's the actual gate. The pre-commit hook's job is just to catch the same issues *before* a developer pushes, cheaply.

## 1. Markdown: `markdownlint-cli2`

Add [`DavidAnson/markdownlint-cli2`](https://github.com/DavidAnson/markdownlint-cli2)'s pre-commit hook (`language: node`, manages its own deps — works fine under pre-commit.ci, no coupling to `web/package.json`).

```yaml
- repo: https://github.com/DavidAnson/markdownlint-cli2
  rev: <pinned tag>
  hooks:
    - id: markdownlint-cli2
```

Needs a `.markdownlint-cli2.yaml` at the repo root. The one rule that **must** be disabled out of the box: `MD013` (line-length) — AGENTS.md asks for "full-length lines when writing docs," and `docs/adrs/*.md` already runs lines past 1000 characters by design. Default config otherwise; then:

- Run it against the full tree, read what else fires (candidates likely to need tuning based on this repo's docs: `MD033` if any raw HTML shows up in `docs/`, `MD041`/first-line-heading on files like `CONTRIBUTING.md` if they don't open with an H1, `MD024` for ADRs that reuse heading names like "Decision" across files — MD024 is per-file by default so that one's probably fine).
- Fix genuine issues in existing files rather than disabling rules to route around them; only disable a rule repo-wide if it conflicts with a real convention (like `MD013` does).
- Exclude `dist/`, `site/` (mkdocs build output), `web/build/`, `web/node_modules/` via `.markdownlint-cli2.yaml`'s `globs`/`ignores`.

## 2. TypeScript/Svelte: local hook wrapping the existing `web/` scripts

Skip `pre-commit/mirrors-eslint` / `mirrors-prettier`. This project's ESLint is flat-config with `typescript-eslint`, `eslint-plugin-svelte`, and `eslint-config-prettier`; mirroring it means hand-duplicating every plugin version into `additional_dependencies` and keeping that in sync with `web/package.json` by hand forever — exactly the maintenance trap the mirror's own docs warn about. `web/` already has working `lint`/`check` npm scripts; reuse them instead of re-describing the toolchain in YAML.

```yaml
- repo: local
  hooks:
    - id: web-lint
      name: web lint (prettier + eslint)
      entry: bash -c 'cd web && npm run lint'
      language: system
      files: ^web/.*\.(ts|js|svelte|css|json)$
      pass_filenames: false
```

- `pass_filenames: false` because `npm run lint` (`prettier --check .` + `eslint .`) already scopes itself to `web/` and checks the whole directory, matching what CI runs — partial/staged-file linting would diverge from the CI gate.
- Deliberately **not** including `npm run check` (`svelte-check`, type-aware): it's slower (spins up the TS program) and is still enforced in the `ts` CI job. Pre-commit stays fast; full type-checking remains CI's job.
- Because this is `language: system`, pre-commit.ci will skip it. Add it to `ci.skip` in `.pre-commit-config.yaml` with a comment explaining why (so it doesn't look like an oversight), and note in the hook's `name` or a nearby comment that enforcement lives in the `ts` GitHub Actions job.
- Requires `web/node_modules` to exist locally (`npm install` in `web/`) for the hook to run at all — same prerequisite `task ts:install`/`task ts:lint` already have. Worth a one-line mention in `CONTRIBUTING.md` if it doesn't already say to run `npm install` in `web/`.

## 3. Wire up `.pre-commit-config.yaml` and verify

- Add both hooks, plus a top-level `ci: skip: [web-lint]` block (with comment).
- Run `pre-commit run --all-files` locally; fix whatever markdownlint and the web lint surface in existing files rather than suppressing.
- Confirm `pre-commit run --all-files` is clean, and that `task ts:lint` (the real CI gate) still passes unchanged.

## 4. Optional: record the decision as an ADR

`docs/adrs/0009`–`0011` already record CI/tooling decisions in this repo (test taxonomy, Codecov→octocov). The pre-commit.ci constraint above (local TS hook can't run there, so it's local-only defense-in-depth rather than a CI gate) is a real, non-obvious tradeoff future contributors could reasonably question or "fix" by trying to mirror ESLint — worth a short ADR (`0012-...`) so nobody re-litigates it. Skipping this is fine too if it's considered too small to warrant one; flagging it as a judgment call for confirmation before implementing.
