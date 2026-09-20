# Restrict argo-cd's Go module updates to major-only

Status: done · 2026-09-20

Add one `packageRule` to `renovate.json` that disables minor/patch Renovate updates for
`github.com/argoproj/argo-cd/v3` specifically, leaving major updates (and every other direct Go
dependency's minor/patch updates) on today's behavior. See
[ADR 0015](../../adrs/0015-restrict-argo-cd-go-module-updates-to-major-only.md) for the decision
record and the options considered — in short: argo-cd's own transitive dependency tree
(`k8s.io/kubernetes` and its staging modules, `gitops-engine`, etc.) is large enough that a routine
minor/patch bump forces `go mod tidy` to reconcile dozens of indirect module versions in the same
commit, ballooning the weekly `go` group PR's diff even though indirect dependencies are already
excluded from Renovate's own update proposals by default. Scoping the fix to argo-cd (rather than
disabling minor/patch for all direct Go dependencies) keeps that behavior change targeted to the
one dependency actually causing it.

## 1. Add the `packageRule`

**`renovate.json`** — insert a new rule immediately after the existing `go` datasource group rule
(so the two `go`-related rules stay adjacent and readable):

```diff
   "packageRules": [
     {
       "matchDatasources": ["go"],
       "groupName": "go"
     },
+    {
+      "matchManagers": ["gomod"],
+      "matchPackageNames": ["github.com/argoproj/argo-cd/v3"],
+      "matchUpdateTypes": ["minor", "patch"],
+      "enabled": false
+    },
     {
       "matchManagers": ["dockerfile"],
       "matchPackageNames": ["golang", "ghcr.io/ivanklee86/devcontainer/go"],
       "groupName": "go"
     },
     ...
```

**Why this shape**

- `matchManagers: ["gomod"]` (not `matchDatasources: ["go"]`) scopes the rule to the actual
  `go.mod`-parsed dependency, not the unrelated `go install` custom-manager pins (`golangci-lint`,
  `go-swagger`, `gomplate`, `go-junit-report`, `air-verse/air`) that also resolve through the `go`
  datasource and share the `go` groupName — those should keep updating normally, since they're
  isolated single-line pins with no transitive-graph blast radius.
- `matchPackageNames: ["github.com/argoproj/argo-cd/v3"]` is the exact module path as it appears in
  `go.mod`'s `require` block (`go.mod:8`) — this is what Renovate reports as the dependency's name
  for `gomod`-managed deps, so it's also what a Dependency Dashboard entry or existing PR title for
  this dependency will show, useful for confirming the rule matched correctly after landing it.
- `matchUpdateTypes: ["minor", "patch"]` deliberately excludes `major` — a major argo-cd bump still
  flows through the existing `go` group and ADR 0013's repo-wide `matchUpdateTypes: ["major"]` →
  `automerge: false` rule, so it still opens a PR, just not one that's automerged, same as today.
- `enabled: false` on this scoped rule doesn't affect `gitops-engine`/`pkg/v2` — they're already
  `// indirect` in `go.mod` (`go.mod:45-46`) and excluded from independent Renovate proposals by
  gomod's own default (`matchDepTypes: ["indirect"]` is off unless explicitly re-enabled) — only the
  mechanical `go mod tidy` sweep triggered by argo-cd's own version bump touches them, and that sweep
  no longer happens for minor/patch once this rule lands.
- Placement right after the `go` groupName rule (rather than at the end of `packageRules`, where the
  repo-wide `matchUpdateTypes` automerge rules live) keeps the two `go`-specific rules visually
  adjacent; ordering doesn't change behavior here since `enabled: false` makes the update proposal
  disappear entirely; there's nothing left for the later automerge rules to apply to.

## 2. Validate

1. `npx --yes --package renovate@44.103.6 -- renovate-config-validator` (same pinned version the
   `renovate-config-validator` pre-commit hook from ADR 0013 uses) — confirm the new rule parses.
2. `prek run renovate-config-validator --all-files` — confirm the pre-commit hook itself passes
   against the updated file.
3. Open the repository's Renovate Dependency Dashboard issue after the next scheduled Friday run
   (or trigger a manual run) and confirm: if argo-cd has a pending minor/patch release that week, it
   no longer appears as an open/proposed update (it may still be listed under a "disabled" or
   "ignored" section of the dashboard, which is expected); a pending argo-cd *major* release, if any,
   still opens a PR in the `go` group as before.
4. If an argo-cd minor/patch release is available at review time, confirm directly: run Renovate
   locally against this repo (or inspect the next scheduled run's logs) and verify no branch is
   created for it, while other direct `go` deps with pending minor/patch releases still get one.

## Rollback

Remove the added `packageRule`; argo-cd goes back to being grouped and updated like every other
direct Go dependency, including minor/patch automerge, and the weekly `go` PR's diff size goes back
to today's behavior.
