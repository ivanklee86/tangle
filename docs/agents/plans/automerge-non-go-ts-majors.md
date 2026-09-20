# Automerge non-Go/TS majors

Status: done · 2026-09-20

Flip the repo-wide `matchUpdateTypes: ["major"]` rule in `renovate.json` from `automerge: false` to
`automerge: true`, then layer three narrower rules back on top, scoped to Go and TypeScript, that
restore `automerge: false` for majors in those two ecosystems specifically. See
[ADR 0016](../../adrs/0016-automerge-major-updates-outside-go-and-ts.md) for the decision record.

## 1. Restructure the major-update rule

**`renovate.json`** — replace the single repo-wide major rule with four:

```diff
     {
       "matchUpdateTypes": ["major"],
-      "automerge": false
+      "automerge": true,
+      "platformAutomerge": true
+    },
+    {
+      "matchDatasources": ["go"],
+      "matchUpdateTypes": ["major"],
+      "automerge": false
+    },
+    {
+      "matchDatasources": ["npm"],
+      "matchUpdateTypes": ["major"],
+      "automerge": false
+    },
+    {
+      "matchManagers": ["dockerfile"],
+      "matchPackageNames": ["golang", "ghcr.io/ivanklee86/devcontainer/go", "node"],
+      "matchUpdateTypes": ["major"],
+      "automerge": false
     },
     {
       "matchUpdateTypes": ["minor", "patch", "pin", "digest"],
       "automerge": true,
       "platformAutomerge": true
     }
```

**Why this shape**

- The first rule (bare `matchUpdateTypes: ["major"]`) now sets the *default* for majors — automerge
  on — matching the same shape as the pre-existing minor/patch/pin/digest rule right below it.
  packageRules apply in array order and later matches win per-field, so this default only holds for
  packages the three rules after it don't also match.
- `matchDatasources: ["go"]` and `matchDatasources: ["npm"]` reuse exactly the matchers from the
  `go`/`ts` groupName rules earlier in the file (`renovate.json:9`, `renovate.json:24`) — this covers
  `gomod` plus the `go install` custom managers, and the native `npm` manager plus the
  `PLAYWRIGHT_VERSION` custom manager, respectively, so the automerge scope tracks the grouping
  scope exactly rather than drifting from it.
- The `dockerfile`-manager rule reuses the same `matchPackageNames` as the existing `go`/`ts`
  dockerfile sub-rules (`renovate.json:19-20`, `renovate.json:29`) — `golang`,
  `ghcr.io/ivanklee86/devcontainer/go`, and `node` — merged into one rule since all three get
  identical treatment here (unlike the groupName rules, which have to keep `go` and `ts` separate).
- `k8s` (`helmv3`, the k8s-scoped `dockerfile` images, `github-releases`), `ci`
  (`github-actions`), and `devcontainer` are deliberately *not* matched by any of the three new
  scoped rules, so they fall through to the new default and automerge majors — this is the intended
  behavior change. `devcontainer`'s own `automerge: true` (`renovate.json:55-58`, no
  `matchUpdateTypes` restriction) was already unconditional; nothing after it in the array
  overrides that anymore, so majors automerge there too, same as everything else it manages.

## 2. Validate

1. `npx --yes --package renovate@44.103.6 -- renovate-config-validator` — confirm the restructured
   rules parse. (Ran with `npm_config_cache` pointed at a writable scratch dir in this session,
   working around a pre-existing root-owned `~/.npm` cache unrelated to this change.)
2. `prek run renovate-config-validator --all-files` — confirm the pre-commit hook passes.
3. After the next scheduled Friday run (or a manual trigger), confirm via the Dependency Dashboard
   or PR list: a pending `k8s`/`ci` major (if any) opens a PR marked for automerge rather than held
   for manual review; a pending Go or TypeScript major, if any, still opens a PR that is *not*
   marked for automerge, same as before this change.

## Rollback

Revert the four-rule block back to the single `matchUpdateTypes: ["major"]` → `automerge: false`
rule; every ecosystem's majors go back to requiring manual review, as established by ADR 0013.
