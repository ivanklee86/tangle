# Go major dependency migration: httplog v3, koanf env v2, drop fiber

Status: implemented · 2026-09-20

Renovate opened [PR #201](https://github.com/ivanklee86/tangle/pull/201) proposing three major-version
bumps: `github.com/go-chi/httplog/v2` → `v3`, `github.com/gofiber/fiber/v2` → `v3`, and
`github.com/knadh/koanf/providers/env` `v1` → `v2`. Renovate's automated PR only added the new
module versions as extra `require` lines alongside the old ones — it did not (and could not) migrate
any code, so its green CI is misleading: nothing in the tree imports the new packages yet. This plan
does the actual migration, researched against each package's real v3.5.0 (httplog) / v2.0.1 (koanf
env) source in the module cache, plus a directly-verified finding that fiber is entirely unused.

## Scope and findings

| Dependency | Used at | Verdict |
| --- | --- | --- |
| `github.com/gofiber/fiber/v2` | one call site: `cmd/tangle-server/main.go` imported `github.com/gofiber/fiber/v2/log` purely for `log.Fatalf` — the initial `grep -rn "gofiber/fiber" --include="*.go" .` research pass missed it (matched the parent `gofiber/fiber` string but the grep was scoped wrong), and `go build ./...` with the require line removed is what actually caught it (`no required module provides package .../fiber/v2/log`) | **Remove**, don't migrate. The fiber *web framework* itself (routing, middleware, `fiber.App`) is unused — the project's HTTP layer runs entirely on `go-chi/chi` (`internal/tangle/server.go`). The one real usage was fiber's tiny standalone `log` subpackage, swapped for the standard library's `log.Fatalf` (identical signature) in `cmd/tangle-server/main.go`. No v2→v3 migration needed — drop the `require` line, fix the one import, `go mod tidy`. |
| `github.com/go-chi/httplog/v2` | `internal/tangle/server.go` (`Tangle.Log` field, `New()`'s logger setup, `Start()`'s error logging) | **Migrate** — actively used, and v3 changed enough of the API that this needs real code changes (below). |
| `github.com/knadh/koanf/providers/env` | `internal/tangle/loader.go`'s `LoadConfig()` | **Migrate** — actively used; v2's `Provider()` signature changed. |

## 1. Drop unused `gofiber/fiber/v2`

**Steps**

1. Remove `github.com/gofiber/fiber/v2 v2.52.15` from `go.mod`'s direct `require` block.
2. `go build ./...` — this is what surfaces the one real usage (`cmd/tangle-server/main.go`'s
   `github.com/gofiber/fiber/v2/log` import), since `grep` alone missed it. Replace that import with
   the standard library's `"log"` (its `Fatalf(format string, v ...any)` is a drop-in match).
3. `go mod tidy` to drop fiber's now-unreferenced indirect dependencies from `go.sum`.
4. `go build ./...` again to confirm the module graph is clean.

**Rollback**: revert `go.mod`/`go.sum`.

## 2. Migrate `go-chi/httplog` v2 → v3

**Why this isn't a drop-in bump**: v3 rewrote the package around the standard library's `log/slog`
directly instead of wrapping it in its own `httplog.Logger` type. Concretely, comparing
`go-chi/httplog/v2@v2.1.1` against `go-chi/httplog/v3@v3.5.0`'s source:

- `httplog.NewLogger(name, Options)` (constructs and returns a `*httplog.Logger`) is **gone**. The
  caller now builds a plain `*slog.Logger` itself (e.g. `slog.New(slog.NewJSONHandler(...))`) and
  passes it directly to `RequestLogger`.
- The `httplog.Logger` type is gone — `Tangle.Log`'s type must become `*slog.Logger`. Its `Info`/
  `Error` methods have the same `(msg string, args ...any)` signature as `slog.Logger`, so every
  existing `t.Log.Info(...)`/`t.Log.Error(...)` call site in `internal/tangle/handlers.go` and
  `internal/tangle/server.go` keeps working unchanged.
- `httplog.RequestLogger` is now `func RequestLogger(logger *slog.Logger, o *Options) func(http.Handler) http.Handler`
  — it takes the `*slog.Logger` and a separate `*Options`, instead of a single pre-built `*Logger`.
- `httplog.ErrAttr(err)` is **gone**. Use the new exported `httplog.ErrorKey` constant (`= "error"`)
  instead: `t.Log.Error("HTTP server error.", httplog.ErrorKey, err)`, or `slog.Any(httplog.ErrorKey, err)`
  if building an `slog.Attr` directly.
- `Options` changed shape:
  - `LogLevel` → `Level` (same `slog.Level` type).
  - `Concise: true` is gone from `Options` — concise-ness now lives on the `Schema` via
    `httplog.SchemaECS.Concise(bool)` (or `SchemaOTEL`/`SchemaGCP`), assigned to the new `Schema *Schema`
    field. Pick `SchemaECS` (closest to v2's default field names) unless there's a reason to prefer
    OTEL/GCP naming.
  - `Tags map[string]string` is gone — v3 has no built-in "static tags" concept. Replicate the
    `"version"`/`"env"` tags this codebase currently sets by attaching them to the base `slog.Logger`
    itself with `.With(slog.String("version", version), slog.String("env", config.Env))` before passing
    it to `RequestLogger`.
  - `QuietDownRoutes []string` + `QuietDownPeriod` (rate-limited noisy-route suppression) is gone —
    v3's closest equivalent is the `Skip func(req *http.Request, respStatus int) bool` predicate, but
    `Skip` is a static per-request decision, not the old time-windowed "quiet down after N requests"
    behavior. This is a real behavior change, not just an API rename: decide whether `Skip` returning
    `true` for `/`, `/metrics`, `/swagger`, `/health` unconditionally (dropping the "quiet down" gating
    entirely, logging none of them ever) is acceptable, or whether that's a regression worth flagging
    for a follow-up (e.g. logging them at `slog.LevelDebug` via a custom `Skip` that checks the
    configured level) — read `options.go`'s `Skip` doc comment and `middleware.go` before deciding.
  - `MessageFieldName` (renamed the request-log message field) is gone from `Options` — v3 gets this
    from `Schema.ReplaceAttr`; check whether `"message"` is already `SchemaECS`'s default before
    assuming a behavior change.
  - `RequestHeaders: false` → replaced by `LogRequestHeaders []string` (explicit list, default
    `["Content-Type", "Origin"]`, empty list to log none).

**Steps**

1. `go get github.com/go-chi/httplog/v3@v3.5.0`, remove the old `github.com/go-chi/httplog/v2` require,
   `go mod tidy`.
2. In `internal/tangle/server.go`: change the `import` from `.../httplog/v2` to `.../httplog/v3`,
   change `Tangle.Log`'s type from `*httplog.Logger` to `*slog.Logger`, and rewrite the `New()` logger
   setup per the mapping above — build the base `*slog.Logger` with `.With(...)` for the `version`/`env`
   tags, construct `&httplog.Options{Level: ..., Schema: httplog.SchemaECS, ...}` separately, and call
   `httplog.RequestLogger(logger, &options)`.
3. Replace the two `httplog.ErrAttr(err)` call sites in `Start()` with `httplog.ErrorKey, err` (as
   variadic args) or `slog.Any(httplog.ErrorKey, err)`.
4. Decide and implement the `QuietDownRoutes`/`QuietDownPeriod` → `Skip` replacement per the bullet
   above.
5. `go build ./...`, `go vet ./...`.
6. Manually hit the running server (`task services` up, or just `go run ./cmd/tangle-server` if that's
   enough locally) and diff actual log output against what v2 produced for a few requests — field
   names, the `version`/`env` tags, and error logging — before trusting this is behavior-equivalent.
   This package sits directly in `docs/adrs`' "tracing and observability should be first-class citizens"
   territory (AGENTS.md), so log-shape regressions matter here more than in most dependency bumps.
7. Check whether anything downstream parses these logs by field name (dashboards, log pipelines) —
   grep `docs/` and any deployment manifests for `httplog`/log-schema references before assuming no
   consumer cares about field renames.

**Rollback**: revert `go.mod`/`go.sum` and `internal/tangle/server.go`.

## 3. Migrate `koanf/providers/env` v1 → v2

**API change**, comparing `providers/env@v1.1.0` against `providers/env/v2@v2.0.1`'s source:

- v1: `env.Provider(prefix, delim string, cb func(s string) string) *Env` — positional prefix and
  delimiter, callback takes only the key and returns only the transformed key (the value passes
  through unchanged).
- v2: `env.Provider(delim string, o Opt) *Env` — delimiter is now the only positional argument; prefix
  moves into `Opt.Prefix`, and the callback moves into `Opt.TransformFunc func(k, v string) (string, any)`,
  which now also receives (and can transform) the *value*, not just the key.

Current code (`internal/tangle/loader.go`):

```go
err = config.Load(env.Provider(EnvVarPrefix, ".", func(s string) string {
    return strings.ReplaceAll(strings.ToLower(
        strings.TrimPrefix(s, EnvVarPrefix)), "_", ".")
}), nil)
```

Becomes:

```go
err = config.Load(env.Provider(".", env.Opt{
    Prefix: EnvVarPrefix,
    TransformFunc: func(k, v string) (string, any) {
        key := strings.ReplaceAll(strings.ToLower(
            strings.TrimPrefix(k, EnvVarPrefix)), "_", ".")
        return key, v
    },
}), nil)
```

**Steps**

1. `go get github.com/knadh/koanf/providers/env/v2@v2.0.1`, remove the old
   `github.com/knadh/koanf/providers/env` require, `go mod tidy`.
2. Update the `import` in `internal/tangle/loader.go` from `.../providers/env` to
   `.../providers/env/v2`, and rewrite the `env.Provider(...)` call per the mapping above.
3. `go build ./...`, `go vet ./...`.
4. Run `internal/tangle`'s loader tests (`go test ./internal/tangle/...`) — check whether they already
   cover env-var override behavior; if not, this is a good place to add a case per this repo's testing
   philosophy (observable behavior from a caller's perspective: "given `TANGLE_FOO_BAR=x` set, does the
   loaded config have `Foo.Bar == "x"`?").
5. Manually verify one real env-var override end to end (e.g. `TANGLE_PORT=9999` against `task services`)
   since this is config-loading code that's easy to silently break in a way tests might not catch if the
   existing test coverage is thin.

**Rollback**: revert `go.mod`/`go.sum` and `internal/tangle/loader.go`.

## Sequencing and validation

- Do these three independently — they touch disjoint files (`go.mod` for all; `server.go` for httplog;
  `loader.go` for koanf env) and none depends on another. Dropping fiber (§1) is zero-risk and can land
  first/separately at any time.
- After all three: `go build ./...`, `go vet ./...`, `gofmt -l .`, `golangci-lint run`, then the full
  `go test ./...` plus the live-cluster suite (`task services:cicd`) per this repo's existing dependency-
  bump bar — the httplog change in particular changes real HTTP middleware behavior, not just types.
- `task docker:build` to confirm the production image still builds.
- Close [PR #201](https://github.com/ivanklee86/tangle/pull/201) once this plan's changes are merged —
  its diff (bare `require` additions) doesn't represent real migrated code and shouldn't be merged as-is.

## Outcome

Implemented 2026-09-20. `go build ./...`, `go vet ./...`, `gofmt -l .`, `golangci-lint run` (0 issues),
and `go test ./...` (all packages, `-count=1`) all pass; `go.mod`/`go.sum` are tidy. The httplog v3
request-logging pipeline was manually verified by running the built server and hitting `/health`,
`/metrics`, and `/api/applications`: JSON output carries the `version`/`env` tags via `.With(...)`,
`/health` and `/metrics` are correctly silent (the `Skip` replacement for `QuietDownRoutes` works), and
the 500 from `/api/applications` produced a structured `error.message` log line through `RequestLogger`.
The koanf env v2 migration is covered by a new `TestConfig/Environment_variable_overrides_file_config`
case in `internal/tangle/loader_test.go` using `t.Setenv`.

**Two pre-existing, unrelated bugs surfaced by this change** (both fixed here, not by the plan's original
scope — this go.mod/go.sum edit is what exposed them):

1. `pkg/client/client_test.go`'s `TestGetApplications` and its siblings (`TestGetApplicationsWithRetries`,
   `TestGetDiffs`, `TestGetDiffsWithRetries`) hardcoded `localhost:8081` with nothing anywhere in the repo
   ever starting a listener there — no `TestMain`, no `httptest.Server`, and the CI `go` job (unlike the
   separate `e2e` job) never runs `task services:cicd`. They "passed" in CI purely because
   `actions/setup-go@v5`'s `cache: true` persists Go's test result cache keyed off `go.sum`, and `go.sum`
   hadn't changed in a long time — so `go test` kept replaying a stale cached "ok" instead of actually
   re-executing. This `go.sum` edit changed the cache key, forced a real run, and the true
   connection-refused/nil-`resp`-panic behavior surfaced for the first time in PR #211's CI
   (`pkg/client/client_test.go:253` derefs `resp.Results` without checking `err`). Confirmed by diffing
   this same job's log against `main`'s last green run at the same commit both branches shared. Fixed by
   giving `pkg/client/client_test.go` its own `newFakeTangleServer` helper (a third copy of the same
   helper `cmd/tangle-cli/main_test.go` and `internal/cli/tanglecli_test.go` already use — `tangle.New()`
   run in-process behind `httptest.NewServer`, fake ArgoCD wrappers), and adding the missing
   `assert.NoError(t, err)` before dereferencing `resp`.
2. `internal/tangle/handlers.go`'s `applicationManifestsHandler` didn't `return` after writing the
   error response for a `GetManifests` failure, so it fell through and nil-dereferenced
   `generatedManifests.LiveManifests` (nil on error) on every single manifest-generation error. This had
   been silently recovered by chi's `Recoverer` on every occurrence without failing a test, because the
   error response was already fully written to the client *before* the panic — Recoverer sees the status
   already set and skips writing a second response, so the caller always got the correct output despite
   the server panicking (and logging a full stack trace) on the backend every time. Fixed by adding the
   missing `return`; covered by a new `TestDiffsError` case in `internal/tangle/handlers_test.go` that
   asserts `assert.NotPanics` around the handler call.

`task docker:build` and the live-cluster `task services:cicd`/`e2e` suite were not run as part of this
pass (the `go` job's unit/integration suite no longer needs a live cluster at all after the fixes above)
— worth confirming once more before this ships, since the `e2e` job is a separate, always-run gate.
