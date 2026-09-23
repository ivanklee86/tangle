# Label query fixes: exclude-only selectors (#240) and label validation (#241)

Status: implemented · 2026-09-22

Two backend-only bugs in how `GET /api/applications` turns `labels`/`excludeLabels` into an Argo CD label
selector. [#240](https://github.com/ivanklee86/tangle/issues/240) is a silently dropped selector;
[#241](https://github.com/ivanklee86/tangle/issues/241) is silently dropped *input*. They're independent, but
they share a code path and a test surface, so this plan covers both. Nothing in `web/` changes.

## Review of the reports

Both reproduce against the code as written. Confirmed details beyond what the issues say:

**#240** — `internal/argocd/wrapper.go:78-97` builds `labelsSlice` from both maps, then gates attaching the
selector on `len(labels) > 0`. Exclude-only queries therefore send an `ApplicationQuery` with no `Selector` at
all, which Argo CD reads as "list everything". The condition wants to be `len(labelsSlice) > 0` so that the
no-filter case (both maps empty, e.g. a bare `GET /api/applications`) still sends an empty query.

**#241** — `internal/tangle/handlers.go:73-102` has the two parse loops the issue quotes. Both drop malformed
segments with an error-level log and continue, and both use `map[string]string`, so a repeated key takes the
last value. Confirmed side effects the issue doesn't mention:

- `internal/tangle/handlers_test.go` has an `invalid_tags` case (`?labels=foobar`) asserting 200 and a full
  result set, and `internal/tangle/server_e2e_test.go:71` has the identical case. Both **encode the bug** and
  must be rewritten, not merely extended — exactly the kind of thing `AGENTS.md` asks be flagged rather than
  preserved.
- The web UI needs no change to cope with the new 400: `web/src/lib/backend/http.ts`'s `fetchEnvelope` already
  treats any non-200 as an error envelope and parses an `ErrorResponse` body out of it, and
  `web/src/lib/ui/validation.ts`'s `LABEL_FORMAT` regex already refuses to *produce* a malformed pair. The UI
  can still produce a duplicate key (the structured key/value inputs don't dedupe); it will now get a legible
  400 instead of a wrong result set, which is an improvement even before the UI-side check lands.
- `tangle-cli` can't produce a duplicate key: `internal/cli/tanglecli.go` (`labelStringsToMap`) drops anything without exactly one `=` and builds a map. It doesn't check the key or value, though, so `--label env=a:b`, `--label =x` and `--label env=` reach the server as malformed segments and get the 400. That's why receiving the 400 well matters (see Step 4). *Corrected during review of #242; the original claim was that the CLI could produce neither bad shape.*

## Decisions

- A key present in both `labels` and `excludeLabels` is a 400 **only when the values are identical**. That pair
  produces `key=value,key!=value`, which can never match anything. Different values (`labels=env:test` +
  `excludeLabels=env:prod`) are redundant but perfectly valid to Argo CD, and stay a 200.
- Malformed segments and within-map duplicate keys are always a 400, per the issue.
- `pkg/client` surfaces the 400's `ErrorResponse` body as the error text, and stops retrying on any 4xx.

## Test layers

"Integration" and "e2e" have specific meanings in this repo (`docs/agents/ci.md:89`, `tasks/go.yaml`), and both
fixes need coverage at each. Reading the request as the *Go* layers, since both bugs are backend-only; the
TypeScript suites (`web/e2e/mocked`, `web/e2e/live`) are untouched.

| layer | where | how it runs | backing |
| --- | --- | --- | --- |
| unit | `internal/argocd/wrapper_test.go`, new `internal/tangle/labels_test.go`, `pkg/client/client_test.go` | `task go:test` | `argocdfakes.FakeClient`, `httptest` |
| integration | `internal/tangle/handlers_test.go` | `task go:test` | real `ArgoCDWrapper` over `argocdfakes.FakeClient` |
| e2e | `internal/tangle/server_e2e_test.go` | `task go:test:e2e`, needs `task services:cicd` + `.env` | live Argo CD, four `integration/kubernetes/example/application-*.yaml` |

**The integration layer needs a new harness, and this is the crux of testing #240.**
`argocdfakes.FakeWrapper.matchesLabels` (`fake_wrapper.go:113`) applies the two maps *directly* rather than
building a selector string, so it already behaves correctly for exclude-only input. Every current handler test
goes through `FakeWrapper` via `newTestTangle()`, which means **no handler-level test can fail on #240 today,
before or after the fix** — the bug is invisible from there.

So add a second harness alongside `newTestTangle`, in `handlers_test.go`:

```go
// newIntegrationTangle builds a *Tangle whose ArgoCDs are real
// argocd.ArgoCDWrapper values over argocdfakes.FakeClient, so the handler's
// parsed label maps travel the production path — wrapper builds one k8s
// selector string, FakeClient parses it with k8s.io/apimachinery/pkg/labels.
// newTestTangle's FakeWrapper short-circuits that by matching the maps
// directly, so it cannot observe selector-construction bugs (#240).
func newIntegrationTangle(t *testing.T) *Tangle
```

`FakeClient` already carries `Url`/`Scheme` (`fake_client.go:34-36`) backing the wrapper's `GetUrl`/`GetScheme`,
so it drops straight in. Split `argocdfakes.ExampleApplications()` `[:2]` / `[2:]` across the `test` / `prod`
instances, matching the RBAC scoping the live fixtures have and the split `newTestTangle` already uses. Pass
`DoNotInstrumentWorkers: true` to `argocd.New`, as `wrapper_test.go` does, or a second subtest panics
re-registering pool metrics.

Keep `newTestTangle` for `TestHandlersError` — `FakeWrapper.ErrOnListApplicationsByLabels` is the only way to
inject a wrapper-level failure, and `FakeClient.ErrOnList` isn't quite the same test.

**Both bad-request tests should go through the router, not the bare handler.** Every existing test calls
`http.HandlerFunc(tangle.applicationsHandler)` directly, so nothing covers the handler as actually mounted.
That's tolerable for 200s; for a new status code it's worth proving no middleware (`Recoverer`, `Timeout`, CORS)
rewrites it. The router isn't exported, but `New` stores it as `tangle.Server.Handler`
(`internal/tangle/server.go:129-134`), so `httptest.NewServer(tangle.Server.Handler)` reaches the real stack at
`/api/applications?...`.

## Step 1 — #240: attach the selector whenever either map is non-empty

`internal/argocd/wrapper.go`:

- Change the gate to `len(labelsSlice) > 0`, and collapse the `if`/`else` accordingly.
- Sort `labelsSlice` before `strings.Join`. Map iteration order is random, so the selector string is currently
  non-deterministic; k8s selector semantics don't care, but a stable string is what makes the assertions below
  possible. One `slices.Sort(labelsSlice)` after both loops is enough.

### Unit — `internal/argocd/wrapper_test.go`

The `FakeClient`-backed suite, the only place the selector is really parsed:

| case | input | expectation |
| --- | --- | --- |
| includes only | `labels{env=test}` | `test-1` only (existing `pool` subtest) |
| **excludes only** | `excludeLabels{env=test}`, empty includes | `test-2` only — **fails before the fix** |
| **excludes only, multiple** | `excludeLabels{env=test, bazz=buzz}` | empty — both fixtures carry `bazz=buzz` |
| both | `labels{foo=bar}`, `excludeLabels{env=test}` | `test-2` only (existing `exclude` subtest) |
| **neither** | both empty | both applications, and no selector sent |

Add a `LastQuery *application.ApplicationQuery` field to `FakeClient`, recorded at the top of `List`. That
turns these from transitive result-count checks into direct assertions on the promise: the selector string is
`"env!=test"` for exclude-only, `"env!=test,foo=bar"` (sorted) for both, and `Selector` is nil for neither.
Without it, "the selector was built correctly" is only ever inferred from which applications came back.

### Integration — `internal/tangle/handlers_test.go`

Re-run the existing `TestHandlers` label matrix against `newIntegrationTangle`, plus the new rows. This is the
table that would have caught #240 end to end:

| case | url | test | prod |
| --- | --- | --- | --- |
| **exclude_only** | `?excludeLabels=env:test` | 1 | 2 |
| **exclude_only_multiple** | `?excludeLabels=env:test,bazz:buzz` | 0 | 2 |
| exclude_and_include | `?labels=foo:bar&excludeLabels=env:test` | 1 | 2 |
| **cross_map_different_values** | `?labels=env:test&excludeLabels=env:prod` | 1 | 0 |

`exclude_only_multiple`'s prod count of 2 is worth stating explicitly rather than leaving as a magic number: the
`prod` fixtures carry no `bazz` label at all, and a k8s `!=` requirement matches an absent key. That's real
selector semantics `FakeWrapper` would also have got right by accident — it's the *first* two rows that need the
real wrapper.

Rather than duplicate the table, parameterize the existing `TestHandlers` over both harnesses and skip the
error-injection case for the integration one. If that reads worse than a second table, a separate
`TestHandlersIntegration` is fine — the rows matter more than the arrangement.

### E2E — `internal/tangle/server_e2e_test.go`

Add the same four rows to `TestE2E_ServerApplications`'s table. Counts are identical (the live fixtures are what
`ExampleApplications` mirrors), and this is the only layer where a real Argo CD, not apimachinery, interprets the
selector — which is the actual promise #240 breaks.

## Step 2 — #241: one `parseLabels` helper, 400 on bad input

New unexported helper in `internal/tangle` (its own file, `labels.go`, since `handlers.go` is already long):

```go
// parseLabels splits a comma-separated "key:value,key:value" query parameter
// into a map, rejecting anything that can't be turned into a well-formed
// Kubernetes selector rather than silently dropping it.
func parseLabels(param, raw string) (map[string]string, error)
```

Rules, in the order they're checked, with the `param` name (`labels` / `excludeLabels`) interpolated into every
message so the caller knows which parameter was wrong:

1. Empty `raw` → empty map, no error (a bare `GET /api/applications` stays valid).
2. Each comma-separated segment must split into exactly two parts on `:`, and neither part may be empty →
   otherwise `invalid label %q in %s: expected key:value`. This covers `env`, `env:test:extra`, the empty
   segment from a trailing comma, `:value` and `key:`.
3. A key already in the map → `duplicate label key %q in %s: each key may appear at most once`.

Then in `applicationsHandler`, replacing both loops:

```go
labels, err := parseLabels("labels", query.Get("labels"))
if err != nil { /* 400 + ErrorResponse{Error: err.Error()} */ }

excludeLabels, err := parseLabels("excludeLabels", query.Get("excludeLabels"))
if err != nil { /* 400 + ErrorResponse{Error: err.Error()} */ }

for _, key := range slices.Sorted(maps.Keys(labels)) {
    if excluded, ok := excludeLabels[key]; ok && excluded == labels[key] {
        // 400: label %q=%q appears in both labels and excludeLabels; no application can match
    }
}
```

Keep an error-level `t.Log.Error` on each rejection — the issue explicitly asks that operators not lose the
signal — but the 400 is now the contract. Factor the "log, set status, encode `ErrorResponse`" trio into a small
`t.respondError(w, status, err)` helper if it reads better than three copies; the handler already repeats that
shape for the 500 path. The cross-map loop iterates sorted keys so that a request with several conflicts reports
a deterministic one, which is what makes the test below assertable.

### Unit — `internal/tangle/labels_test.go`

Table-driven `TestParseLabels`, one row per promise to callers. Each error row asserts the **message text**,
since the message is the API contract here, not just the status code:

| case | input | expectation |
| --- | --- | --- |
| empty | `""` | empty map, no error |
| single | `env:test` | `{env: test}` |
| multiple | `env:test,foo:bar` | both pairs |
| no separator | `env` | `invalid label "env" in labels: expected key:value` |
| too many separators | `env:test:extra` | invalid-label error |
| trailing comma | `env:test,` | invalid-label error naming the empty segment |
| empty key | `:test` | invalid-label error |
| empty value | `env:` | invalid-label error |
| duplicate key | `env:test,env:prod` | `duplicate label key "env" in labels: …` |
| duplicate, same value | `env:test,env:test` | duplicate-key error — still a typo, still ambiguous |

Run the whole table for both `param` values to prove the parameter name is interpolated, not hard-coded.

### Integration — bad-request rows in `internal/tangle/handlers_test.go`

New `TestHandlersBadRequest`, through `httptest.NewServer(tangle.Server.Handler)` so the response is what a real
client sees. Each row asserts `400`, a decodable `ErrorResponse`, and a substring of `.Error`:

| case | url | error names |
| --- | --- | --- |
| malformed include | `?labels=env` | `env`, `labels` |
| malformed exclude | `?excludeLabels=env` | `env`, `excludeLabels` |
| duplicate include key | `?labels=env:test,env:prod` | `env`, `labels` |
| duplicate exclude key | `?excludeLabels=env:test,env:prod` | `env`, `excludeLabels` |
| contradictory pair | `?labels=env:test&excludeLabels=env:test` | `env`, both parameters |
| malformed alongside valid | `?labels=env,team:platform` | `env` — the valid pair must not rescue the request |

Plus two rows that must stay **200**, guarding against over-rejection: `?labels=env:test&excludeLabels=env:prod`
(the cross-map decision above) and a bare `/api/applications`.

One case worth asserting beyond the status code: for a rejected request, **nothing reaches Argo CD**. Give
`FakeClient` a call counter (or reuse `LastQuery` from Step 1, asserting it stays nil) and check it's zero after
a 400. The issue's wording is "Nothing is sent to Argo CD" — that's a separate promise from "the caller gets a
400", and it's the one that keeps a malformed query from costing a fan-out across every configured instance.

Then delete the `invalid_tags` row from `TestHandlers`. It asserts 200 and a full result set for `?labels=foobar`
— the exact behavior being fixed.

### E2E — bad-request rows in `internal/tangle/server_e2e_test.go`

New `TestE2E_ServerApplicationsBadRequest` with the same six 400 rows, against the live-stack `Tangle` from
`newE2ETangle`, through `tangle.Server.Handler`. The existing `TestE2E_ServerApplications` asserts
`http.StatusOK` for every row, so these can't be folded into it.

Be clear about what this layer adds: a rejected request never reaches Argo CD, so this is not testing the live
integration the way the Step 1 rows are. What it *does* prove is that the validation holds when `Tangle` is built
from `integration/tangle.yaml` through the real `LoadConfig` path with real wrappers and real middleware, rather
than from a hand-built test config — which is where a config-dependent difference (timeouts, CORS, the
instrumentation middleware that unit tests disable) would show up. The 200 rows added in Step 1 are the ones
doing the live work.

Also delete `server_e2e_test.go:71`'s `invalid_tags` row, for the same reason as its handler-test twin.

## Step 3 — swagger

`internal/tangle/swagger.json` is generated (`internal/tangle/.gitignore` ignores it) by `swagger generate spec`
under `task go:generate`, so the edit goes in the annotations in `internal/docs/docs.go`, and regeneration is
just re-running the task:

- Expand the `Labels` / `ExcludeLabels` parameter descriptions to state the format and the rules:
  comma-separated `key:value` pairs, each key at most once per parameter, and a key may not appear in both with
  the same value.
- Add `400: errorResponse` to the `swagger:route GET /api/applications` response list, alongside the existing
  `200` and `500`. The `errorResponse` swagger response and `tangle.ErrorResponse` model already exist, so this
  is one line.
- While in there: `Instance` is documented as a query parameter but nothing in `applicationsHandler` reads it.
  Out of scope for these two issues — worth a separate issue rather than a drive-by removal.

Because the spec is generated and gitignored, a stale one is invisible in review — CI regenerates it, but
nothing asserts its content. Add a small integration test in `internal/tangle` unmarshalling the package's
embedded `spec` var (`server.go:40-41`) and asserting `paths./api/applications.get.responses` has a `400`. It's
a handful of lines and it's the only thing that would catch the annotation being dropped in a later refactor.

## Step 4 — `pkg/client`

`pkg/client/client.go`:

- `GetApplications` currently returns `fmt.Errorf("unexpected status code: %d", resp.StatusCode)` and discards
  the body, so a caller sees `unexpected status code: 400` with no idea which label was wrong. Read the body,
  decode it into `tangle.ErrorResponse`, and include `Error` in the returned error when it's non-empty; fall
  back to the bare status code when the body isn't the expected shape.
- `GetApplicationWithRetries` retries any error, so a 400 would burn all five backoff periods (up to 66s) on a
  request that can never succeed. Give the status-code error a type (e.g. `type StatusError struct { Code int;
  Message string }`) so the retry loop can `errors.As` it and return immediately on a 4xx, while keeping the
  existing behavior for 5xx and transport errors.
- `GetDiffs` has the identical bare-status-code line; give it the same treatment for consistency, even though
  the diffs handler has no 400 today.

### Unit — `pkg/client/client_test.go`

Against `httptest.NewServer`:

| case | server responds | expectation |
| --- | --- | --- |
| 400 with error body | `400` + `{"error":"invalid label \"env\" in labels: …"}` | error text contains the message |
| 400 with non-JSON body | `400` + `not json` | error still names the status code |
| 4xx is not retried | always `400` | exactly **1** request, error returned immediately |
| 5xx is still retried | always `500` | `retries+1` requests, existing behavior unchanged |
| success after a 500 | `500` then `200` | result returned, no error |

The request-count assertions are the point of the middle rows — a `time.Sleep`-free way to prove the retry loop
changed behavior is to count handler invocations, with a short explicit `Backoff` (`[]int{0}`) so the test
doesn't actually wait.

No change to `internal/cli` — it already prints whatever error `GetApplications` returns via `t.Error(...)`,
which is precisely why fixing the error text is worth doing.

## Step 5 — docs and ADR

- `docs/usage.md`: the two `/applications` and `/diffs` parameter tables describe `labels`/`excludeLabels` as
  `key1:value1,key2:value2` but say nothing about what happens to bad input. Add a sentence under each table
  naming the 400 cases.
- `docs/agents/internals.md:57-58` describes the endpoint as "translating labels into a single Kubernetes
  selector (`k=v,k!=v`)" — accurate once #240 is fixed, misleading today. Add the validation step to that
  description.
- ADR (next number is **0026**) recording the #241 decision: rejecting malformed and duplicate label pairs is a
  deliberate, breaking change to a documented API contract, and the cross-map rule ("400 only when the pair is
  identical") is a judgement call that a future reader will otherwise have to re-derive. #240 is a plain bug fix
  and needs no ADR.

## Verification

- `task go:test` — unit + integration, no live dependencies. Every new test except the `TestE2E_` ones runs here.
- `task services:cicd`, then `task go:test:e2e` with `.env` present — the e2e rows in Steps 1 and 2.
- `task go:generate` then `task go:lint` — regenerates the spec and checks the annotations still parse.
- Confirm the two `invalid_tags` deletions are reflected in the commit message; a removed assertion is the kind
  of thing a reviewer should see called out rather than discover.

## Not in scope

Noted while reading, deliberately left alone:

- **`baseLink` ignores `excludeLabels`.** `handlers.go:113-122` builds the deep link into the Argo CD UI from
  `labels` only, so an exclude-only query now returns correct results behind a link that shows everything.
  Real, adjacent, and in neither issue — worth its own issue.
- **Label values aren't URL-encoded** in `pkg/client`'s `GenerateApplicationsUrl*`. A value containing `&` or
  `,` would corrupt the query string.
- **`labelStringsToMap` in `internal/cli` silently drops** malformed `--label` input, the CLI-side analogue of
  #241. Lower stakes (the user sees their own typo on the command line), but the same class of bug.
- The **web UI's duplicate-key validation** that #241 mentions, and the `web/e2e` Playwright suites. Backend-only
  by request; the server rule landing first is the right order, since it's what the client-side check would be
  mirroring.

## What shipped

All five steps, in `internal/argocd/wrapper.go`, `internal/argocd/argocdfakes/`, `internal/tangle/`
(`labels.go`, `handlers.go`), `internal/docs/docs.go`, `pkg/client/client.go` and the docs, plus
[ADR 0026](../../adrs/0026-reject-malformed-label-query-parameters.md). `task go:test`,
`go test -race`, `golangci-lint run` and the full `-tags=e2e` suite all pass against the live k3d/ArgoCD
stack.

**The bug was proven to fail at each layer before the fix, not just to pass after it.** Reverting the
one-word gate in `wrapper.go` fails exactly the intended rows and nothing else:

- unit — `TestArgoCDWrapper_ListApplicationsByLabelsSelector/excludes_only` and `…/excludes_only,_multiple`
- integration — `TestHandlersIntegration/exclude_only` and `…/exclude_only_multiple`
- e2e, against the live cluster — `TestE2E_ServerApplications/exclude_only` (got 2 applications, wanted 1)
  and `…/exclude_only_multiple` (got 2, wanted 0)

All four repro steps from the two issues were also run by hand against `tangle-server` on the live stack:
an exclude-only query now filters, and the three malformed shapes return `400` with the message naming the
segment and parameter, while `?labels=env:test&excludeLabels=env:prod` stays `200`.

**`FakeClient` grew query capture rather than only a call counter.** The plan called for a `LastQuery`
field; what shipped is `ListQueries` (every query, mutex-guarded because `ArgoCDWrapper` issues `List`
from a pond pool) behind two accessors — `LastListSelector()` returning the selector *and whether one was
sent at all*, and `ListCallCount()`. The "was one sent" half is what distinguishes #240's failure from a
selector that happens to match everything; a plain string accessor couldn't express it.

**`respondError` doesn't log.** The plan left it open whether the helper should fold in the error-level
log. It doesn't: the 500 path already logs with the ArgoCD instance name, and the 400 paths log with the
parameter name, so a logging helper would have had to take variadic context or lose it. Call sites log,
the helper writes the response.

**One shadowing fix the plan didn't anticipate.** `applicationsHandler` now declares `err` at the top for
`parseLabels`, so the closing `err := json.NewEncoder(w).Encode(response)` became `err =`.

**A `swagger_test.go` was added beyond what Step 3 described.** The plan suggested it as optional; it's
worth having, because `swagger.json` is gitignored and regenerated, so a dropped annotation would never
appear in a diff. It asserts the `400` is present and that both label parameters document `key:value`.

**`FakeWrapper.matchesLabels` gained a warning comment.** Its existing comment said it mirrored the
selector semantics, which is true of the result but misleading about the mechanism — a future test author
reaching for `FakeWrapper` to cover selector construction would get false confidence. The comment now
points at `newIntegrationTangle`.

Two things the plan listed and this deliberately did not do: nothing in `web/` changed, and the four items
under [Not in scope](#not-in-scope) are still open.
