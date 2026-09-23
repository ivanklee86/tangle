---
status: "proposed"
date: 2026-09-22
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Reject malformed and duplicate label pairs with 400 instead of silently dropping them

## Context and Problem Statement

`GET /api/applications` takes `labels` and `excludeLabels` as comma-separated `key:value` pairs and turns them into one Kubernetes label selector. Two shapes of bad input were accepted with a `200` and a result set that did not match what the caller asked for ([#241](https://github.com/ivanklee86/tangle/issues/241)):

- A segment that did not split into exactly two parts on `:` was logged at error level and **skipped**, and the request carried on with whatever remained. `?labels=env` therefore returned *every* application, and `?labels=env,team:platform` returned everything with `team=platform`. Nothing in the response said half the filter had been dropped.
- Both parameters parse into a `map[string]string`, so a repeated key silently took the last value: `?labels=env:test,env:prod` became `{env: prod}`. Since the pairs are ANDed into one selector, the two are contradictory and the caller almost certainly meant something else.

A third shape is reachable across the two parameters: `?labels=env:test&excludeLabels=env:test` builds the selector `env=test,env!=test`, which no application can ever satisfy, and returned an empty `200` with no indication why.

The web UI already refuses to *produce* the first shape (`web/src/lib/ui/validation.ts`), but the README treats the JSON API and `tangle-cli` as first-class interfaces, and both got a `200` for all three. A filter that silently matches more than the caller wrote is worse than an error in a CI/CD context, where the result set decides which applications get diffed.

## Decision Drivers

- A response that doesn't match the request, with no way for the caller to detect it, is the worst available outcome — worse than either a narrower result or an outright failure.
- The web UI will validate the same rules client-side. The server has to be the authority, or the two disagree and the UI's rules become decorative.
- Rejecting input that used to be accepted is a breaking change to a documented endpoint, so the rule needs to be narrow enough to justify and written down somewhere a future reader will find it.
- Argo CD selector semantics, not our own taste, decide which inputs are actually impossible to satisfy.

## Considered Options

- Keep dropping bad pairs, but report what was dropped in the response body alongside the results.
- Reject malformed segments and within-map duplicate keys with a `400`, and leave cross-parameter conflicts alone.
- Reject malformed segments, within-map duplicate keys, and a key shared between both parameters **with the same value**, with a `400`.
- Reject all of the above plus any key appearing in both parameters, regardless of value.

## Decision Outcome

Chosen option: "Reject malformed segments, within-map duplicate keys, and a key shared between both parameters with the same value", because it refuses exactly the inputs that cannot be turned into the selector the caller described, and nothing else.

The three rules, enforced by `parseLabels` and `conflictingLabels` in `internal/tangle/labels.go`:

1. Every comma-separated segment must be exactly `key:value` with both parts non-empty, the key a valid Kubernetes label key and the value a valid label value (apimachinery's `IsQualifiedName` and `IsValidLabelValue`). This covers `env`, `env:test:extra`, `:value`, `key:`, the empty segment a trailing comma produces, and values Argo CD can't parse such as `env:te=st`, which used to come back as a `500` from every instance. It also rejects a key padded with whitespace (`env:test, env:prod` has the key `" env"`), which would otherwise slip past rule 2 while the selector parser trimmed it back to `env`.
2. A key may appear at most once within `labels`, and at most once within `excludeLabels`.
3. A key may not appear in both parameters with the **same** value.

Rule 3 is deliberately narrow. `labels=env:test&excludeLabels=env:prod` produces `env=test,env!=prod`, which is redundant but perfectly satisfiable, and Argo CD will happily serve it — rejecting it would refuse a working query on the grounds that we find it untidy. Only the identical pair is impossible, and impossibility is the thing being detected. This keeps the rule statable in one sentence to an API consumer and to the UI's client-side check.

The error-level server log is kept, since operators find it useful, but the `400` with an `ErrorResponse` body naming the offending segment and parameter is now the contract. Validation runs before any fan-out, so a rejected request costs nothing on any configured Argo CD instance.

### Consequences

- Good, because a caller can no longer receive a result set wider than the filter they wrote without being told.
- Good, because the message names the segment and the parameter, so `tangle-cli` and `curl` users can fix the input from the response alone.
- Good, because a malformed query no longer costs a list call to every configured Argo CD.
- Bad, because it is a breaking change: a request that returned `200` and a full list now returns `400`. That result was wrong, but a caller depending on it — a CI job passing `?labels=env` and treating the full list as intentional — will start failing. That is the intended outcome, and it is why this is written down rather than just fixed.
- Bad, because the rules now live in three places that must agree: `parseLabels`, the OpenAPI description in `internal/docs/docs.go`, and the UI's client-side validation. The server is the authority; the other two mirror it.
- Neutral, because `tangle-cli` can send some of the rejected shapes itself. `labelStringsToMap` drops anything without exactly one `=` and builds a map, so it never sends a duplicate key, but it doesn't check the key or value: `--label env=a:b`, `--label =x` and `--label env=` all reach the server and are refused. The CLI then exits with the `400`, which `pkg/client` surfaces with the server's message naming the bad segment, rather than a bare status code.

## Pros and Cons of the Options

### Report dropped pairs in the response body

- Good, because it is not a breaking change — existing callers keep their `200`.
- Bad, because it puts a warning in a body that callers already parse for results, and nothing forces them to read it. The failure mode stays silent for anyone who doesn't.
- Bad, because it means shipping a second, parallel shape of response error alongside `ErrorResponse`, for input that is simply invalid.

### Reject within-parameter problems only

- Good, because it is the smallest change that fixes both reported bugs.
- Bad, because `labels=env:test&excludeLabels=env:test` keeps returning an empty `200`, which is the same class of silent wrongness — a result that looks like an answer and is actually a contradiction.

### Reject any key appearing in both parameters

- Good, because it is the simplest rule to state and to mirror client-side: a key belongs to one parameter or the other.
- Bad, because it rejects `env=test,env!=prod`, which Argo CD would serve correctly. Refusing a satisfiable query is a new bug traded for an old one, and there is no evidence anyone finds that shape confusing.

## More Information

[#240](https://github.com/ivanklee86/tangle/issues/240) — an exclude-only query dropping its selector entirely and returning every application — is a plain bug in `internal/argocd/wrapper.go` rather than a decision, and is fixed alongside this without its own ADR. The two were found together and share the implementation plan in [docs/agents/plans](../agents/plans/label-query-validation-and-exclude-only-selector.md).
