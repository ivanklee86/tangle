---
status: "accepted"
date: 2026-11-26
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Choose which sources of a multi-source Application get the target ref by its repo

## Context and Problem Statement

Tangle diffs an Application by asking ArgoCD for its manifests twice, at the live ref and at the target ref, through `ApplicationManifestQuery.Revision`. ArgoCD ignores `Revision` for an Application with `spec.sources`. It renders every source at its own `targetRevision`, so both sides are identical and Tangle reports an empty diff, whatever the pushed change was. Tangle also reads the live ref only from `spec.source`, so these apps show a blank live ref.

To render a change, ArgoCD needs `Revisions[i]` with `SourcePositions[i]` for each source to override (`TestE2E_SourceFixtures`). A git ref only means something in one repo, and Tangle takes one `targetRef` with nothing saying which repo it came from. So Tangle can't tell which positions to override. The `two-repos` fixture shows that sending the ref to every source fails, because the other repo has no such branch. Helm repo sources take chart versions, not git refs.

The requirements and use cases are in [the multi-source plan](../agents/plans/multi-source-applications.md).

## Decision Drivers

- A multi-source app must show the change the pushed ref makes, or say why it can't. An empty diff that is really "not computed" is the worst outcome for a "what will this PR change?" tool.
- Existing callers (CLI, UI URLs, API clients) and single-source apps must keep working unchanged.
- Tangle shouldn't guess silently. When it picks positions, the caller should be able to see which positions and why.
- It should need no information that CI doesn't already have.

## Considered Options

- An optional target repo, matched against each source's `repoURL`, with opt-in inference and a failing "needs target repo" status when it's unresolved (chosen).
- The same, with inference on by default.
- Send the target ref to every git source.
- Keep sending `Revision` (today's behavior).
- A list of `repo=ref` pairs per run.

## Decision Outcome

Chosen option: **an optional target repo, matched by `repoURL`**, because it's the only option that gives the right answer for every fixture shape without guessing, and it adds nothing a single-source caller has to change.

1. **Target repos.** The caller may pass one or more target repos: `--target-repo` (CLI, repeatable), `targetRepo` (UI query, repeatable) or `targetRepos` (API). Tangle sends the target ref to every source whose normalized `repoURL` matches any of them. A list covers a change made under the same branch name in several repos. Taking a list from the start avoids adding a second field later. Normalization covers the scheme (`https`, `ssh`, scp-style `git@host:`), the `.git` suffix, a trailing slash and the host's case. Every other source is pinned to its own `targetRevision`. For a multi-source app, both sides are rendered with explicit `Revisions`/`SourcePositions`, so artifacts of an explicit render cancel out. Single-source apps keep sending `Revision`, exactly as today.
2. **Inference is opt-in.** Only with `--infer-target-repo` (`inferTargetRepo`) does Tangle treat an app's single git repo as the target, when no target repo is passed.
3. **Unresolved apps fail CI.** A multi-source app with no target repo and no inference, or whose git sources span several repos, gets the status "needs target repo" instead of a diff. `--fail-on-errors` counts it as a failure.
4. **Apps the repo doesn't touch are "not affected".** With a target repo given, an app with no matching source isn't rendered and doesn't count as a failure. That includes a single-source Helm repo app. Without a target repo, such an app keeps today's error.
5. **All matching sources move.** Every source in a target repo gets the target ref, whatever its own `targetRevision`, including one pinned to a tag.
6. **Any kind of ref.** The target ref may be a branch, a tag or a commit SHA. Tangle passes it to ArgoCD unchanged, so rules 1 to 5 don't depend on the kind of ref.
7. **Inference in the UI is a query parameter** (`inferTargetRepo=true`), so a shared link reproduces the same result.

### Consequences

- Good, because multi-source apps stop returning false empty diffs, and every case Tangle can't resolve is named.
- Good, because single-source behavior is unchanged when no target repo is passed: the same requests to ArgoCD, the same response fields and the same CLI exit codes.
- Good, because the response's per-source detail (position, repo, revision used on each side) explains every diff, and the same detail goes to logs and traces.
- Bad, because a CI pipeline that diffs multi-source apps and passes today, on a false empty diff, fails until it adds `--target-repo` or `--infer-target-repo`. The release notes must say so and give the fix.
- Bad, because a source pinned to a tag in the target repo is moved to the branch too (rule 5). The per-source detail makes this visible, but the diff may include changes the PR didn't intend for that source.
- Neutral, because with a target repo, Helm-only apps go from an error to "not affected". That's more correct, and callers only see it when they opt in by passing the repo.

### Confirmation

- Unit tests cover URL normalization and position selection for every row in the plan's use-case tables, with inference on and off and with one or several target repos.
- Live tests diff by tag and by full SHA (the plan's R1 to R6), and they resolve the SHA at run time, so rebasing the fixture branch doesn't break them.
- A single-source request without `targetRepos` produces the same ArgoCD request as before, asserted against the fake client.
- The multi-source cases in `TestE2E_ServerSourceFixtures` and `web/e2e/live/sources.spec.ts`, skipped today, are turned back on and tightened to exact matches.

## Pros and Cons of the Options

### Target repo with inference on by default

- Good, because `helm-repo-git-values`, `git-chart-git-values` and similar apps would diff correctly in existing CI with no new flags.
- Bad, because Tangle would pick positions for the caller without being asked, and an app that gains a second repo later would silently change from "diffed" to "needs target repo".

### Send the target ref to every git source

- Good, because it needs no new input.
- Bad, because it fails whenever the sources span repos (`two-repos`): the other repo has no such branch.
- Bad, because it would move pinned sources in repos the PR didn't touch.

### Keep sending `Revision`

- Good, because nothing changes.
- Bad, because every multi-source app reports an empty diff whatever the change was.

### A `repo=ref` list per run

- Good, because it could express a coordinated change across a chart repo and a values repo whose PRs use different branch names.
- Bad, because no use case needs it yet, and it's a larger change to every input surface: a URL encoding that survives repo URLs, a row editor in the UI, and fixtures with a second change branch in another repo. The CI run for one PR would also have to know the other PR's branch. The list of target repos with one ref covers the same-branch-name case, and doesn't rule pairs out later.
