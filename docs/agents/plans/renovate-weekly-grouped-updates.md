# Renovate weekly grouped updates

Status: done · 2026-09-20

Restructure `renovate.json` from an ungrouped, biweekly config into four weekly, per-ecosystem
grouped PRs (`go`, `ts`, `k8s`, `ci`), gated by a 5-day `minimumReleaseAge` (`timestamp-optional`)
and automerged (`pr`, `platformAutomerge`) for everything except major bumps. Along the way, add
`customManagers` entries for the pinned versions Renovate cannot see today (four `go install`
lines, `K3D_VERSION`, `PLAYWRIGHT_VERSION`, the gateway-api release URL), fix a Go-swagger version
drift and two unpinned Docker images found during this review, and add
`renovatebot/pre-commit-hooks`' `renovate-config-validator` so `renovate.json` gets validated the
same way TypeScript/Svelte and Markdown already are. See
[ADR 0013](../../adrs/0013-renovate-weekly-grouped-updates.md) for the decision record and the
options considered.

Land the four workstreams below in order; each is independently useful and independently
testable before moving to the next.

## 1. Core `renovate.json` restructure — schedule, `minimumReleaseAge`, groups, automerge

Replace `renovate.json` in full:

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:recommended"],
  "schedule": ["on friday"],
  "minimumReleaseAge": "5 days",
  "minimumReleaseAgeBehaviour": "timestamp-optional",
  "packageRules": [
    {
      "matchDatasources": ["go"],
      "groupName": "go"
    },
    {
      "matchManagers": ["dockerfile"],
      "matchPackageNames": ["golang", "ghcr.io/ivanklee86/devcontainer/go"],
      "groupName": "go"
    },
    {
      "matchDatasources": ["npm"],
      "groupName": "ts"
    },
    {
      "matchManagers": ["dockerfile"],
      "matchPackageNames": ["node"],
      "groupName": "ts"
    },
    {
      "matchManagers": ["helmv3"],
      "groupName": "k8s"
    },
    {
      "matchManagers": ["dockerfile"],
      "matchPackageNames": [
        "rancher/kubectl",
        "alpine/helm",
        "derailed/k9s",
        "quay.io/argoproj/argocd"
      ],
      "groupName": "k8s"
    },
    {
      "matchDatasources": ["github-releases"],
      "groupName": "k8s"
    },
    {
      "matchManagers": ["github-actions"],
      "groupName": "ci"
    },
    {
      "matchManagers": ["devcontainer"],
      "groupName": "devcontainer dependencies",
      "automerge": true,
      "platformAutomerge": true
    },
    {
      "matchUpdateTypes": ["major"],
      "automerge": false
    },
    {
      "matchUpdateTypes": ["minor", "patch", "pin", "digest"],
      "automerge": true,
      "platformAutomerge": true
    }
  ]
}
```

**Why each piece is shaped this way**

- `"schedule": ["on friday"]` — same day the repo already uses, biweekly modifier dropped. Renovate
  only opens/updates branches during this window, so in practice each group gets at most one PR a
  week (nothing to do most Fridays still means no PR at all, which is correct).
- `minimumReleaseAge`/`minimumReleaseAgeBehaviour` sit at the config root, not inside a
  `packageRules` entry, so they apply to every manager including the four groups, the
  `devcontainer` group, and anything ungrouped. `timestamp-optional` (vs. the default
  `timestamp-required`) means a datasource that doesn't report a release timestamp is treated as
  already stable, rather than permanently blocked — some of the `customManagers`-tracked
  datasources added in workstream 2 (`github-releases`) don't reliably return one.
- The `go`/`ts` groups are matched by `matchDatasources` (`"go"`, `"npm"`) rather than
  `matchManagers`, because `gomod`'s dependencies and the new `go install` custom-manager entries
  (workstream 2) both resolve through Renovate's `go` datasource, and the same is true for `npm`
  vs. the new `PLAYWRIGHT_VERSION` entry — one match field cleanly unions the native manager and
  the custom one instead of needing to enumerate `matchManagers: ["gomod", "custom.regex"]` and
  then separately disambiguate by file.
- The `dockerfile`-manager sub-rules (`matchPackageNames`) exist because a single Dockerfile
  contains images from all three of go/ts/k8s — `matchManagers: ["dockerfile"]` alone can't tell
  them apart, so each sub-rule scopes by image name instead. `alpine:latest` (the `Dockerfile`
  runtime stage) is deliberately left out of all four groups — it has no clear ecosystem owner, so
  it stays in Renovate's default "Other Dependencies" bucket rather than being force-fit somewhere.
- The `k8s` group's `matchDatasources: ["github-releases"]` sub-rule is safe as a bare datasource
  match (no `matchManagers` needed) because nothing else in this repo uses that datasource — only
  the `K3D_VERSION` and gateway-api custom managers from workstream 2 do. If that stops being true
  later, tighten it to `matchManagers: ["custom.regex"]` + `matchDatasources`.
- `ci` = `matchManagers: ["github-actions"]`, which covers all three workflow files
  (`ci.yaml`, `release.yaml`, `release-docs.yaml`), not just `ci.yaml` — there's no finer split
  requested, and `release*.yaml` action bumps are exactly as low-risk/reviewable as `ci.yaml`'s.
- The existing `devcontainer` group is otherwise untouched, but the two trailing `matchUpdateTypes`
  rules now apply to it too (packageRules apply in array order, later rules win per-field) — this
  is a deliberate tightening: today it automerges *any* update including majors, and after this
  change majors are excluded like everywhere else. Call this out explicitly in the PR description
  since it's a behavior change to an existing rule, not just new rules being added.
- The major/non-major automerge split is repo-wide (not scoped to the four groups via
  `matchManagers`/`matchDatasources`) deliberately — it's simpler to read as "majors are always
  manual, everything else automerges once 5 days old," and it applies uniformly to anything
  Renovate finds, grouped or not.

**Steps**

1. Replace `renovate.json` with the block above.
2. `npx --yes --package renovate@44.103.6 -- renovate-config-validator` locally (the same version
   workstream 4's pre-commit hook pins) — confirm it passes before relying on the pre-commit hook
   or the Renovate app to catch a mistake.
3. Open the repository's Renovate Dependency Dashboard issue (enabled by default via
   `config:recommended`) after the next scheduled run (or trigger a manual run via the Renovate
   app/GitHub UI if available) and confirm: four grouped branches/PRs appear when there's something
   to update in each ecosystem, `alpine:latest` and any other ungrouped dep still appear
   individually under "Other Dependencies," and a `renovate/stability-days` pending check appears
   on a branch with an update younger than 5 days.

**Rollback**: revert `renovate.json` to the pre-restructure version (git history); nothing else in
this plan depends on this workstream's exact grouping mechanics, only on `customManagers` existing
(workstream 2) and the pre-commit hook (workstream 4), which are independent files.

## 2. Track the previously Renovate-invisible pins

**Problem, found during this review**: `go install` lines in `tasks/go.yaml` (4) and `Dockerfile`
(1, a *different, older* `go-swagger` version — `v0.33.1` there vs. `v0.36.6` in
`tasks/go.yaml`'s `install-ci`, an existing drift this workstream also fixes), `ARG K3D_VERSION`
and `ARG PLAYWRIGHT_VERSION` in `.devcontainer/Dockerfile`, and the `gateway-api` release URL in
`tasks/k8s.yaml` are all pinned versions with no Renovate manager watching them — they can only go
stale, never get bumped, which is the opposite of what pinning is for. `.devcontainer/Dockerfile`'s
"Go extras not in the central go image" section (`go install github.com/air-verse/air@latest` and
`...swagger@latest`) is worse still — not stale-but-pinned, but entirely unpinned, an AGENTS.md
violation this workstream also fixes by pinning both to explicit versions (`v1.67.4`, `v0.36.6` —
the latter matching every other `go-swagger` pin in the repo after this workstream) before wiring
the same `go install` custom manager up to that file too.

**`renovate.json` addition** — a new top-level `customManagers` array:

```json
{
  "customManagers": [
    {
      "customType": "regex",
      "managerFilePatterns": [
        "/^tasks/go\\.yaml$/",
        "/^Dockerfile$/",
        "/^\\.devcontainer/Dockerfile$/"
      ],
      "matchStrings": ["go install (?<depName>[^\\s@]+)@(?<currentValue>[^\\s]+)"],
      "datasourceTemplate": "go"
    },
    {
      "customType": "regex",
      "managerFilePatterns": ["/^\\.devcontainer/Dockerfile$/"],
      "matchStrings": [
        "ARG (?:K3D|PLAYWRIGHT)_VERSION=(?<currentValue>.*) # (?<datasource>.*?)/(?<depName>.*?)\\s"
      ]
    },
    {
      "customType": "regex",
      "managerFilePatterns": ["/^tasks/k8s\\.yaml$/"],
      "matchStrings": ["gateway-api/releases/download/(?<currentValue>v[0-9.]+)/"],
      "datasourceTemplate": "github-releases",
      "depNameTemplate": "kubernetes-sigs/gateway-api"
    }
  ]
}
```

The first entry, once `.devcontainer/Dockerfile` is pinned (below), needs no further file changes —
`depName`/`currentValue` are already fully present in each `go install module@version` line, and it
matches all seven occurrences across the three files (`go-junit-report`, `golangci-lint`,
`gomplate`, both `swagger` pins, and `air-verse/air`), routing them into the `go` group via
`datasourceTemplate: "go"` (workstream 1's `matchDatasources: ["go"]` rule). The third entry
likewise needs no file changes — the version is already embedded in the URL. The second entry
follows Renovate's own documented `ENV X_VERSION=... # datasource/depName` convention (adapted to
`ARG`), which does need a trailing comment added to each `ARG` line:

**`.devcontainer/Dockerfile`**

```diff
- ARG K3D_VERSION=v5.9.0
+ ARG K3D_VERSION=v5.9.0 # github-releases/k3d-io/k3d
```

```diff
- ARG PLAYWRIGHT_VERSION=1.63.0
+ ARG PLAYWRIGHT_VERSION=1.63.0 # npm/playwright
```

And, in the same file's "Go extras not in the central go image" section, pin the two `@latest`
installs so the `go install` custom manager above has an actual version to track:

```diff
- RUN go install github.com/air-verse/air@latest && \
-     go install github.com/go-swagger/go-swagger/cmd/swagger@latest
+ RUN go install github.com/air-verse/air@v1.67.4 && \
+     go install github.com/go-swagger/go-swagger/cmd/swagger@v0.36.6
```

(`v1.67.4` is `air-verse/air`'s latest release as of 2026-09-20, checked via
`gh api repos/air-verse/air/releases/latest` at plan-review time; `v0.36.6` matches every other
`go-swagger` pin in the repo after this workstream.)

**`Dockerfile`** — fix the drifted `go-swagger` pin to match `tasks/go.yaml`'s (both will then be
tracked, and kept aligned, by the same `go` group going forward):

```diff
- RUN go install github.com/go-swagger/go-swagger/cmd/swagger@v0.33.1
+ RUN go install github.com/go-swagger/go-swagger/cmd/swagger@v0.36.6
```

**Note on `PLAYWRIGHT_VERSION` vs. `web/package.json`'s `@playwright/test`/`playwright`**: both are
now in the `ts` group (same `npm` datasource), so they land in the same weekly PR *when both have
an update that week* — but Renovate doesn't force them to bump in lockstep. If only one has a new
release, the "these must match exactly" comment already in `.devcontainer/Dockerfile` can be
briefly out of sync for up to a week. This is a real, accepted limitation (see ADR 0013's
Consequences) — no config change closes it; a reviewer merging a `ts` PR that touches one but not
the other should hold it until the following week's PR catches the other up, or bump the other by
hand in the same PR.

**Steps**

1. Add the `customManagers` block to `renovate.json` (append to the file from workstream 1).
2. Add the two trailing comments to `.devcontainer/Dockerfile`'s `ARG` lines.
3. Fix `Dockerfile`'s `go-swagger` pin to `v0.36.6`.
4. `npx --yes --package renovate@44.103.6 -- renovate-config-validator` again — confirm the new
   `customManagers` entries parse.
5. `task devcontainer` (or equivalent local build) to confirm `.devcontainer/Dockerfile` still
   builds with the comment additions (comments after the `ARG` value are valid Dockerfile syntax,
   but worth confirming rather than assuming).
6. `docker buildx build -f Dockerfile -t tangle .` (or `task docker:build`) to confirm the
   `go-swagger` version bump doesn't break `swagger generate spec` in the build.

**Rollback**: revert the `customManagers` block, the two comment additions, and the `go-swagger`
version bump independently — none of the three depends on the others.

## 3. Pin the two floating Docker images

**Problem, found during this review**: `.devcontainer/Dockerfile`'s `COPY --from=alpine/helm` and
`COPY --from=derailed/k9s` have no tag at all (implicitly `:latest`), which violates AGENTS.md's
"versions should always be pinned" rule and means Renovate has no version to track even after
workstream 1's `k8s` group is wired up to catch `matchPackageNames: ["alpine/helm", "derailed/k9s"]`.
Current tags as of 2026-09-20 (checked via Docker Hub's API at plan-review time):

```diff
- COPY --from=alpine/helm /usr/bin/helm /usr/local/bin
+ COPY --from=alpine/helm:4.3.0 /usr/bin/helm /usr/local/bin
```

```diff
- COPY --from=derailed/k9s /bin/k9s /usr/local/bin
+ COPY --from=derailed/k9s:v0.50.18 /bin/k9s /usr/local/bin
```

**Steps**

1. Make both edits in `.devcontainer/Dockerfile`.
2. `task devcontainer` (build the devcontainer image) to confirm both tags resolve and `helm`/`k9s`
   still install correctly — run `helm version`/`k9s version` inside a shell in the built image.

**Rollback**: revert the two tag additions; the images go back to floating on `:latest`.

## 4. Add `renovate-config-validator` as a pre-commit hook

Per ADR 0013 and ADR 0012's precedent: `renovatebot/pre-commit-hooks`' `renovate-config-validator`
hook is `language: node` (confirmed via its `.pre-commit-hooks.yaml`), so — unlike the repo's
existing `web-lint` hook — it runs under pre-commit.ci, giving `renovate.json` a real CI-enforced
gate, not just local convenience.

**`.pre-commit-config.yaml` addition**:

```diff
     - repo: https://github.com/DavidAnson/markdownlint-cli2
       rev: v0.23.3
       hooks:
           - id: markdownlint-cli2
+    - repo: https://github.com/renovatebot/pre-commit-hooks
+      rev: 44.103.6
+      hooks:
+          - id: renovate-config-validator
```

The hook's own `.pre-commit-hooks.yaml` already sets `additional_dependencies: [renovate@44.103.6]`
and `files: '(^|/).?renovate(?:rc)?(?:\.json5?)?$'`, which matches only the repo's root
`renovate.json` — no further configuration needed.

**Steps**

1. Add the hook block to `.pre-commit-config.yaml`.
2. `prek run renovate-config-validator --all-files` (or `pre-commit run renovate-config-validator
   --all-files`) locally — confirm it passes against the `renovate.json` from workstreams 1–2.
3. Push a branch with an intentionally broken `renovate.json` (e.g. an unknown top-level key) to
   confirm pre-commit.ci actually runs this hook (not silently skipped, unlike `web-lint`) and
   fails the check; then fix it back.

**Rollback**: remove the added block from `.pre-commit-config.yaml`; `renovate.json` goes back to
being validated only by the Renovate app itself, after the fact.

## 5. Track `ci.yaml`'s own k3d/argocd version copies (CodeRabbit finding)

**Problem, found by CodeRabbit review on the PR**: workstream 2's `K3D_VERSION` custom manager only
tracks `.devcontainer/Dockerfile`'s copy. `.github/workflows/ci.yaml`'s `e2e` job independently
hardcodes the same k3d version twice (its cache key and its `TAG=` install value) and the argocd
version twice (same cache key, and the release-download URL) — four more untracked literals, with
only a code comment ("keep in sync") holding the devcontainer and CI copies together. A Renovate
bump to `.devcontainer/Dockerfile` would silently leave `ci.yaml` on the old version.

**Fix**: collapse `ci.yaml`'s four literals to two, via a job-level `env:` block, then track those
two lines with a new `customManagers` entry — grouped into the same `k8s` `groupName` (via
workstream 1's blanket `matchDatasources: ["github-releases"]` rule) as `.devcontainer/Dockerfile`'s
copy, so a bump to either file's `k3d-io/k3d`/`argoproj/argo-cd` pin moves every occurrence
together in one PR instead of relying on a human to notice a stale comment.

**`.github/workflows/ci.yaml`**:

```diff
   e2e:
     runs-on: ubuntu-latest
+    env:
+      # Single source of truth for both CLI versions in this job (cache key
+      # and install steps below all reference these instead of repeating the
+      # literal) — tracked by renovate.json's "k8s" group alongside
+      # .devcontainer/Dockerfile's own K3D_VERSION/argocd copies, so a
+      # Renovate bump moves every occurrence together in one PR.
+      K3D_VERSION: v5.9.0 # github-releases/k3d-io/k3d
+      ARGOCD_VERSION: v3.5.3 # github-releases/argoproj/argo-cd
     steps:
     ...
         path: |
           /usr/local/bin/k3d
           /usr/local/bin/argocd
-        # Neither version is derived from a hashed file — both are hardcoded
-        # here and duplicated in .devcontainer/Dockerfile's K3D_VERSION arg
-        # and argocd base-image tag. Keep this key in sync if either pin
-        # is bumped.
-        key: cli-tools-k3d-v5.9.0-argocd-v3.5.3
+        key: cli-tools-k3d-${{ env.K3D_VERSION }}-argocd-${{ env.ARGOCD_VERSION }}
     - name: Install k3d
-      # Keep in sync with K3D_VERSION in .devcontainer/Dockerfile.
       if: steps.cli-tools-cache.outputs.cache-hit != 'true'
-      run: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.9.0 bash
+      run: curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=${{ env.K3D_VERSION }} bash
     - name: Install argocd
-      # Keep in sync with the argocd CLI version in .devcontainer/Dockerfile.
       if: steps.cli-tools-cache.outputs.cache-hit != 'true'
       run: |
-        curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/download/v3.5.3/argocd-linux-amd64
+        curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/download/${{ env.ARGOCD_VERSION }}/argocd-linux-amd64
         sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
         rm argocd-linux-amd64
```

**`renovate.json`** — new `customManagers` entry:

```json
{
  "customType": "regex",
  "managerFilePatterns": ["/^\\.github/workflows/ci\\.yaml$/"],
  "matchStrings": [
    "(?:K3D|ARGOCD)_VERSION: (?<currentValue>.*) # (?<datasource>.*?)/(?<depName>.*?)\\s"
  ]
}
```

Produces `k3d-io/k3d`/`github-releases` (identical `depName`/`datasource` to `.devcontainer/
Dockerfile`'s entry — verified by running both regexes against the actual files with Node's
`RegExp` before landing this) and a *new* `argoproj/argo-cd`/`github-releases` dependency, previously
untracked anywhere — closing the same gap for argocd that CodeRabbit only flagged for k3d, since
the exact same "cache key + hardcoded value, comment-only sync" pattern applied to both.

**Steps**

1. Make both file edits.
2. `npx --yes --package renovate@44.103.6 -- renovate-config-validator` — confirm it still passes.
3. Verify extraction manually (`node -e` with the two regexes against the real file contents) —
   confirm `ci.yaml`'s `K3D_VERSION` match produces the identical `depName`/`datasource` as
   `.devcontainer/Dockerfile`'s, which is what makes Renovate's shared `groupName: "k8s"` combine
   them into one PR rather than two independent ones.
4. `prek run --files renovate.json .github/workflows/ci.yaml` — confirm `check-yaml` and
   `renovate-config-validator` both still pass.

**Rollback**: revert both file edits; `ci.yaml` goes back to four hardcoded literals and a
comment-only sync obligation, and `.devcontainer/Dockerfile`'s copy stays the only Renovate-tracked
one — reintroducing the drift risk this workstream closes.
