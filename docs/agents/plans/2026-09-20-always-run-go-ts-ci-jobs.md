# Always run go/ts CI jobs

Status: proposed · 2026-09-20

Drop the path-conditional gating on `ci.yaml`'s `go` and `ts` jobs so both run on every push and
pull request, the same way `e2e` already does; leave `docs` conditional. See
[ADR 0017](../../adrs/0017-always-run-go-and-ts-ci-jobs.md) for the decision record.

## 1. Simplify the `filter` job

**`.github/workflows/ci.yaml`** — drop the `go` and `ts` filter definitions and outputs; keep
`docs`:

```diff
   filter:
     runs-on: ubuntu-latest
     outputs:
-      go: ${{ steps.filter.outputs.go }}
-      ts: ${{ steps.filter.outputs.ts }}
       docs: ${{ steps.filter.outputs.docs }}
     steps:
     - uses: actions/checkout@v4
     - uses: dorny/paths-filter@v4
       id: filter
       with:
         filters: |
-          go:
-            - '**/*.go'
-            - 'go.mod'
-            - 'go.sum'
-            - 'Taskfile.yaml'
-            - 'tasks/go.yaml'
-            - 'tasks/docker.yaml'
-            - 'tasks/k8s.yaml'
-            - 'tasks/argocd.yaml'
-            - 'Dockerfile'
-            - '.github/workflows/ci.yaml'
-          ts:
-            - 'web/**'
-            - 'Taskfile.yaml'
-            - 'tasks/ts.yaml'
-            - '.github/workflows/ci.yaml'
           docs:
             - 'docs/**'
             - '!docs/agents/**'
             - 'mkdocs.yml'
             - 'requirements.txt'
             - '.github/workflows/ci.yaml'
```

**Why keep `filter` at all**: `docs` still needs it. Removing `dorny/paths-filter` entirely is a
separate, out-of-scope decision (ADR 0017's "Considered Options" — extending this to `docs` too).

## 2. Make `go` and `ts` unconditional

**`go` job** — drop `needs`/`if`, otherwise unchanged:

```diff
   go:
-    needs: [filter]
-    if: needs.filter.outputs.go == 'true'
     runs-on: ubuntu-latest
     steps:
     - name: Checkout code
       uses: actions/checkout@v4
       ...
```

**`ts` job** — same change:

```diff
   ts:
-    needs: [filter]
-    if: needs.filter.outputs.ts == 'true'
     runs-on: ubuntu-latest
     steps:
       ...
```

**`docs` job** — unchanged: keeps `needs: [filter]` / `if: needs.filter.outputs.docs == 'true'`.

**`report` job** — unchanged: `needs: [go, ts, e2e]` with `if: always()` already tolerates a
missing artifact from any of its three inputs; that behavior was written for a job being *skipped*
by the old path filter, but works identically if a job instead fails outright, so nothing here
needs to change now that `go`/`ts` no longer skip for path reasons.

## 3. Update `docs/agents/ci.md`

- Pipeline diagram: drop the `go`/`ts` conditional labels (`if: go paths changed` /
  `if: web/** changed`) from the `go_job`/`ts_job` nodes, and drop the `filter_job --> go_job` /
  `filter_job --> ts_job` edges (only `filter_job --> docs_job` remains).
- Update the prose paragraph currently reading "`go`/`ts`/`docs` are gated on `filter`'s path
  outputs and skip cleanly ... when their paths aren't touched" to state that only `docs` is now
  gated that way, and that `go`/`ts` always run, alongside `e2e`, for the reasons in
  [ADR 0017](../../adrs/0017-always-run-go-and-ts-ci-jobs.md).
- Update the "Gated on" column in the test-pyramid table (`Path filter` for the unit/integration
  rows) to reflect that `go`/`ts` unit and integration tests now run unconditionally; only `docs`
  retains a path-filter gate.
- Leave the `e2e`, `report`, and `pre-commit` sections as-is — none of this changes their behavior.

## 4. Validate

1. Push a branch touching only `docs/agents/**` (excluded from the `docs` filter) — confirm `go`
   and `ts` both run (no longer skip), `docs` skips (unchanged), and `e2e` still runs.
2. Push a branch touching only `docs/**` (not excluded) — confirm `docs` now runs too, alongside
   `go`/`ts`/`e2e`.
3. Confirm branch protection's required-checks list (if it names `go`/`ts` by job name) doesn't
   need any change — an always-run job satisfies the same required-check name a conditionally
   skipped one did.

## Rollback

Revert the `filter` job's `go`/`ts` filter definitions and outputs, and re-add
`needs: [filter]` / `if: needs.filter.outputs.<go|ts> == 'true'` to the `go`/`ts` jobs; this
restores ADR 0009's original path-conditional behavior exactly, since the `docs` job and
everything else in `ci.yaml` are untouched by this plan.
