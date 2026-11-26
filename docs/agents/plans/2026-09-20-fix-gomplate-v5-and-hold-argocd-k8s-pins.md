# Fix gomplate v5 and hold argo-cd/k8s.io pins

Status: proposed · 2026-09-20

Fixes two unrelated stuck Renovate PRs: [#220](https://github.com/ivanklee86/tangle/pull/220)
(gomplate v5's Go install path) and [#216](https://github.com/ivanklee86/tangle/pull/216) (the
weekly grouped `go` minor/patch, blocked on `k8s.io/*`/`gitops-engine` pins that mirror argo-cd
v3.5.3's own vendoring). See [ADR
0020](../../adrs/0020-hold-k8s-io-and-gitops-engine-updates-to-manual-argo-cd-bumps.md) for the
decision record and both reproductions.

## 1. `tasks/go.yaml` — gomplate v5's module path

```yaml
- go install github.com/hairyhenderson/gomplate/v5/cmd/gomplate@v5.2.0
```

Go's semantic-import-versioning convention means a v5 release lives under a `/v5/` module path, not
`/v4/` — `go install .../v4/cmd/gomplate@v5.2.0` errors `invalid version: unknown revision
v4/cmd/gomplate/v5.2.0`. Renovate's `go install`-line regex `customManager` (`renovate.json`'s
`customManagers` entry matching `/^tasks/go\.yaml$/`) captures the whole matched string, including
`/v4/`, as an opaque `depName`, and only substitutes what follows `@` — it has no way to know the
path itself needs to change on a major bump. `go-junit-report/v2` and `golangci-lint/v2` are
tracked by the same regex and will need the identical by-hand fix on their own next majors; this is
a standing limitation of the pattern, not something this change tries to generalize a fix for.

Verified in a worktree of #220's branch: `task go:install-ci`, `task go:generate`, `go build
./...`, `go test ./...` all pass with only this line changed.

## 2. `renovate.json` — stop independent `k8s.io/*`/`gitops-engine` bumps

Added immediately after the existing `argo-cd/v3` minor/patch rule from ADR 0015:

```json
{
  "matchManagers": ["gomod"],
  "matchPackageNames": ["github.com/argoproj/argo-cd/v3"],
  "matchUpdateTypes": ["minor", "patch"],
  "enabled": false
},
{
  "description": "k8s.io/* and the gitops-engine replace mirror argo-cd v3.5.3's own vendoring pins; only a deliberate argo-cd bump should move them",
  "matchDatasources": ["go"],
  "matchPackageNames": ["k8s.io/**", "github.com/argoproj/argo-cd/gitops-engine"],
  "enabled": false
},
```

Verified the glob actually does what it's meant to, rather than trusting the schema validator alone
(which only checks config shape, not matching behavior): pulled the `minimatch` matcher Renovate
uses for `matchPackageNames` glob patterns out of a scratch `npx renovate` install and confirmed
directly —

```text
k8s.io/apimachinery -> true
k8s.io/api          -> true
sigs.k8s.io/yaml     -> false
github.com/argoproj/argo-cd/gitops-engine -> true
```

`sigs.k8s.io/*` correctly falls outside the pattern — it isn't part of the mirrored `replace (...)`
block and isn't implicated in either #216 failure, so it keeps updating normally.

## 3. Docs

[ADR 0020](../../adrs/0020-hold-k8s-io-and-gitops-engine-updates-to-manual-argo-cd-bumps.md) —
no `ci.md` changes needed; this doesn't change job structure, only Renovate's proposal scope and
one tool-install line.

## Verification performed

- `task go:install-ci` / `task go:generate` / `go build ./...` / `go test ./...` — all pass in a
  worktree of #220's branch with the corrected install line.
- `prek run renovate-config-validator --all-files` — schema-valid.
- Direct `minimatch` test against the new `matchPackageNames` glob (above) — confirms it matches
  the intended packages and nothing else.
- No `go.mod` changes needed on `main` — both problems live in *proposed*, unmerged PRs; this fix
  is entirely `renovate.json` + one line of `tasks/go.yaml`.

## Follow-up (operational, not part of this change)

1. Close #216 — the config change removes `k8s.io/*` and the `gitops-engine` replace from future
   proposals entirely, the same reasoning as #221 (ADR 0019): not a rebase-and-retry, a different
   future proposal.
2. Close #220 once this branch's `tasks/go.yaml` fix lands, or push the identical one-line
   correction directly onto `renovate/major-go` and merge it as-is — unlike #216, nothing here
   depends on a `renovate.json` rule, just this one file.
3. Whenever a real argo-cd version bump happens, a human re-derives the `gitops-engine` replace
   commit and the `k8s.io/*` mirror block from that release's own `go.mod`, exactly as today — this
   decision doesn't create or remove that process, only stops Renovate from proposing a partial,
   broken version of it in the meantime.
