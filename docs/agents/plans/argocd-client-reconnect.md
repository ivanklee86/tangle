# Reconnect the ArgoCD client instead of holding one un-redialable connection

Status: implemented · 2026-09-21

`ArgoCDClient` dials ArgoCD once at startup and keeps that `grpc.ClientConn` for the life of the process. The
argo-cd SDK builds that conn in a way that can never re-dial, so the first time gRPC re-creates the transport —
normally after 30 idle minutes — every subsequent request to that instance fails with:

```text
rpc error: code = Unavailable desc = connection error: desc = "transport: failed to write client preface:
write unix @->/tmp/argocd-ZaFuBGlzuGsgrjYt.sock: use of closed network connection"
```

This plan makes the client own its connection: detect the dead transport, redial, and release the old one.

## Why it happens

1. `internal/argocd/client.go:54` sets `GRPCWeb: true`. With that flag argo-cd's apiclient does not talk to
   ArgoCD directly — it starts a local gRPC → gRPC-Web reverse proxy listening on a unix socket named
   `/tmp/argocd-<16 random chars>.sock` (`pkg/apiclient/grpcproxy.go:113-119`) and points the gRPC client at
   that socket. That is where the path in the error comes from.
2. The dial goes through `util/grpc.BlockingNewClient`, which dials **once**, eagerly, and then installs a
   "dialer" closed over that single connection:

   ```go
   rawConn, err := proxy.Dial(ctx, network, address)   // dialed once
   customDialer := func(_ context.Context, _ string) (net.Conn, error) {
       return rawConn, nil                             // ...handed back forever
   }
   opts = append(opts, grpc.WithContextDialer(customDialer), ...)
   ```

   So the `grpc.ClientConn` can never create a second transport. When gRPC re-connects, the dialer returns the
   same, now-closed fd and gRPC writes the HTTP/2 client preface into it —
   `grpc@v1.82.2/internal/transport/http2_client.go:433`. The `@->` (empty local address) and "use of closed
   network connection" both say the fd was closed by our own side; ArgoCD was never contacted. This is still
   present on argo-cd `master`; no upstream issue found. Worth filing one, but tangle should not wait for it.
3. The usual trigger is gRPC's idle timeout: 30 minutes with no RPCs (`grpc@v1.82.2/dialoptions.go:729`) and the
   ClientConn closes its transport. Keepalive pings do not prevent it (`PermitWithoutStream` is false, so pings
   only flow while a stream is active). Any transport error or GOAWAY does the same.
4. `internal/tangle/server.go:99` builds one client per configured ArgoCD at startup and holds it forever, and
   `internal/argocd/client.go:57` throws away the `io.Closer` that `NewApplicationClientOrDie` returns. So the
   breakage is permanent for that process, and nothing ever stops the proxy `grpc.Server`, its goroutine, or
   unlinks the socket file.

Expect the symptom to look intermittent only because there are several replicas or because pods restart — a
single tangle-server process does not recover on its own.

## Decision

Recorded in [ADR 0025](../../adrs/0025-reconnect-the-argocd-grpc-client.md): reconnect on failure, with the
`io.Closer` owned rather than discarded. That ADR weighs the alternatives (a keep-warm ticker, a fresh client
per request, waiting for an upstream fix, reimplementing `newConn` in tangle) and records the four
sub-decisions this plan implements — classify on the gRPC status code rather than on the error string, defer
any rate limit on redialing, drop the `OrDie` constructors so dialing returns errors, and instrument the dial
path. It also notes that
[ADR 0015](../../adrs/0015-restrict-argo-cd-go-module-updates-to-major-only.md) holds argo-cd at manual major
bumps, so an upstream fix would not reach this repository for months.

## Workstream 1 — make connection setup return errors

Prerequisite for everything else: the current `NewClientOrDie` / `NewApplicationClientOrDie` pair calls
`log.Fatal` (logrus) on failure. That is survivable at startup and unacceptable on a reconnect path, where a
transient failure would kill the server.

- Swap to `apiclient.NewClient` and `argocdClient.NewApplicationClient()`, both of which return errors.
- Extract the whole thing into one `func (c *ArgoCDClient) dial() (*connection, error)`.
- Add `Name string` and `Logger *slog.Logger` to `ArgoCDClientOptions` (defaulting to `slog.Default()`), so the
  client can label metrics and log lines per instance. `internal/argocd` cannot import `internal/tangle` —
  `tangle` already imports `argocd` — so the logger has to be injected. Pass the config map key at
  `internal/tangle/server.go:99`.
- Fix the ignored error at `internal/tangle/server.go:99` (`client, _ := argocd.NewArgoCDClient(...)`). Today a
  missing auth-token env var returns `nil, err`, the error is dropped, and the nil client is handed to
  `argocd.New`, so the first request nil-derefs into chi's `Recoverer` and 500s. Log the error and skip the
  instance instead of registering a wrapper that cannot work.

**Startup behaviour change to make deliberately:** with the `OrDie` variants gone, a dial failure at boot no
longer exits the process. Make `NewArgoCDClient` attempt the initial dial, log a warning if it fails, and leave
the client to connect lazily on first use. A missing auth token stays a hard error from the constructor — that
is a config mistake, not a transient one. Note that with `GRPCWeb: true` the initial dial only proves the local
proxy socket is up; it never contacts ArgoCD, so "connected at startup" was never a reachability check anyway.
The comment in `internal/argocd/client_e2e_test.go:29-32` claiming an eager handshake needs correcting.

## Workstream 2 — own the closer

Hold the connection and its closer together, so the client can release one:

```go
type connection struct {
    applications application.ApplicationServiceClient
    closer       io.Closer // closes the ClientConn *and* stops the gRPC-Web proxy
    generation   uint64
}

type ArgoCDClient struct {
    Options *ArgoCDClientOptions

    mu         sync.Mutex
    conn       *connection
    generation uint64
    closed     bool
}
```

`closer.Close()` runs argo's composed closer: it decrements `proxyUsersCount` to zero, calls `proxyServer.Stop()`
and closes the `grpc.ClientConn` (`pkg/apiclient/grpcproxy.go:198-223`, `apiclient.go:550-559`). Stopping the
server closes the unix listener, and Go unlinks the socket file it created — that is the observable proof the
leak is gone.

Add `Close() error` to `IArgoCDClient` and `IArgoCDWrapper` (the wrapper delegates), implement a no-op on
`argocdfakes.FakeClient`, and call it from the graceful-shutdown block in `internal/tangle/server.go:196-205`,
after `t.Server.Shutdown` returns so in-flight requests keep their connection. `Close` must be idempotent; calls
after it return a sentinel error rather than silently redialing.

## Workstream 3 — reconnect when the connection is lost

Go methods cannot take type parameters, so the retry lives in a package-level generic helper that all three RPC
methods funnel through:

```go
func callWithReconnect[T any](ctx context.Context, c *ArgoCDClient, method string,
    op func(application.ApplicationServiceClient) (T, error)) (T, error) {

    conn, err := c.connection() // dials lazily if there is none
    if err != nil {
        var zero T
        return zero, err
    }

    result, err := op(conn.applications)
    if !isConnectionLost(ctx, err) { // Unavailable, or Canceled with a live ctx
        return result, err
    }

    fresh, rerr := c.reconnect(conn) // no-ops if someone already replaced this generation
    if rerr != nil {
        var zero T
        return zero, errors.Join(err, rerr)
    }
    return op(fresh.applications)
}
```

Two properties that matter:

- **One retry, never a loop.** If the retry also fails, that error is what the caller gets.
- **Generation collapsing.** `reconnect` takes the connection the caller actually used and, under the mutex,
  redials only if `c.conn` is still that same pointer; otherwise it hands back whatever is current. This matters
  because `ListApplicationsByLabels` and `GetManifests` fan out over pond pools, so a dead transport surfaces as
  N simultaneous `Unavailable`s — without the check, N redials and N new proxies.

A rate limit on redialing is deliberately *not* here — see [Follow-ups](#follow-ups-deferred). Note that
generation collapsing does not substitute for one: it bounds concurrent redials sharing a connection, but once
a redial succeeds the new connection is current, so the next request's failure matches generations again and
dials again. Generation bounds the storm in space, a throttle would bound it in time. We are choosing to find
out whether the time dimension needs bounding before writing the code for it.

Latency budget worth knowing: argo already installs `grpc_retry.UnaryClientInterceptor` with `WithMax(3)` and a
1s linear backoff (`pkg/apiclient/apiclient.go:524-530`), and go-grpc-middleware retries `Unavailable` by
default. So a call against a dead transport already burns ~2-3s before we see it, and our retry adds a dial plus
one more attempt. Both sit under chi's `middleware.Timeout(config.Timeout)`.

All three RPCs are reads and safe to repeat. `Get` with `Refresh: "hard"` has a side effect but is idempotent —
it queues another hard refresh, which is what the caller wanted anyway.

In-flight RPCs on the old connection are aborted when we close it. We only close a connection we have already
seen fail, and any sibling RPC that gets caught lands in its own `callWithReconnect`, sees `Unavailable`, finds
the generation already advanced, and retries on the new connection. Accept this rather than adding a grace
period.

## Workstream 4 — observability

Per `AGENTS.md`, this path needs to be visible without attaching a debugger — the whole point is that the
failure was previously silent until a user saw a red box.

- `internal/argocd/metrics.go`: a package-level `prometheus.CounterVec`
  `argocd_client_dials_total{argocd,reason="initial|reconnect",result="success|failure"}`, registered once in a
  `var`/`init` block rather than per-client (the existing `instrument*` helpers register const-labelled
  collectors per pool and would panic on a second client with the same name). Note in the ADR that this one is
  not gated by `DoNotInstrument`, unlike the pool collectors — a single process-wide vec is safe to register in
  tests.
- A gauge `argocd_client_connection_generation{argocd}` makes "this instance is flapping" legible on a dashboard
  for the cost of one line.
- `slog` at `Warn` when a reconnect starts (with `argocd`, `method`, and the triggering error) and at `Info`
  when it succeeds; `Error` when the redial itself fails. These are the lines that turn a user report of the
  pasted error into a one-grep diagnosis.

## Workstream 5 — tests

Test cases first, per `AGENTS.md`. The connection machinery becomes unit-testable by making the dial injectable:
a `dialFn func() (*connection, error)` field on `ArgoCDClient`, defaulting to the real `dial`. Every case below
asks "what promise to callers breaks if this fails?"

Unit (`internal/argocd/client_test.go`, fake dial, no ArgoCD):

1. A successful call dials once and does not reconnect — the common path pays nothing.
2. `Unavailable`, then success: the caller gets the result, the dial count is 2, the reconnect counter moved.
3. `Unavailable` twice: the caller gets an error, the dial count is exactly 2 — no retry loop.
4. A non-`Unavailable` error (`NotFound`, `Unknown`) is returned untouched with no redial — upstream ArgoCD
   errors must not be mistaken for transport failures.
5. A cancelled/expired context does not trigger a redial — shutdown must not spawn connections.
6. N concurrent callers all seeing `Unavailable` produce exactly one redial (generation collapsing).
7. A failing redial surfaces both the original and the dial error, rather than swallowing either.
8. Reconnect closes the previous closer exactly once (leak check).
9. `Close` closes the underlying closer, is safe to call twice, and calls after it fail rather than redial.
10. A client whose initial dial failed still serves once a later dial succeeds (lazy connect).

E2E (`internal/argocd/client_e2e_test.go`, live ArgoCD via `task services:cicd`):

1. **The reported bug.** With a working client, close the underlying transport out from under it — a test-only
    helper that closes the closer while leaving `c.conn` in place, which is exactly the idle-timeout state — then
    assert the next `List` succeeds. If this test fails, the bug is back.
2. `Close` on a live client removes its `/tmp/argocd-*.sock` (snapshot the matching paths before and after).
    This is the only direct evidence that workstream 2 actually releases the proxy.

Also confirm the existing `TestNewArgoCDClient` "invalid options" case still passes unchanged: a missing token
env var stays a constructor error.

## Docs

- ADR 0025 as above.
- `docs/agents/internals.md`: the `ArgoCDClient` box in the mermaid diagram and a short paragraph on the
  reconnect behaviour and the `/tmp/argocd-*.sock` proxy, which is currently unexplained there.
- Update this file with what actually shipped, matching the convention of the other plans in this directory.

## Risks and open questions

- **`Unavailable` is not exclusively a local signal.** The proxy forwards ArgoCD's own `Grpc-Status` header
  verbatim (`pkg/apiclient/grpcproxy.go`, `executeRequest`), so an argocd-server rollout or an ingress returning
  grpc-status 14 also arrives as `Unavailable` and will cost us one pointless redial. A transport-level failure
  (`httpClient.Do` error, non-200 response) arrives as `Unknown` instead, so the common upstream-outage case does
  not trigger us. We could tighten the match on the error string ("client preface", "use of closed network
  connection"), but that is brittle against grpc-go wording changes. Shipping without a rate limit means the
  worst case is one wasted dial per failing request for the duration of an ArgoCD outage; the dial counter is
  what tells us whether that is a real cost — see [Follow-ups](#follow-ups-deferred).
- **We cannot inspect connectivity state.** `NewApplicationClient()` returns only a closer and a service client,
  never the `*grpc.ClientConn`, so `GetState()` is unavailable as a discriminator without reimplementing
  `newConn` ourselves. Not worth it.
- **Upstream.** File an argo-cd issue for the `BlockingNewClient` dialer reuse. If it is fixed, workstream 3's
  retry becomes belt-and-braces rather than load-bearing, and we keep it.

## Follow-ups (deferred)

Neither of these is in scope for this plan. Both are cheap to add later, and both want evidence first — which
is the point of shipping workstream 4 alongside the fix rather than after it.

**Rate-limit redialing.** `Unavailable` is a heuristic for "the local transport died", so an ArgoCD outage that
surfaces as grpc-status 14 makes every incoming request dial a new connection — a new unix socket, proxy
`grpc.Server`, and goroutine — retry, fail anyway, and tear it all down. A `lastDial` timestamp plus a minimum
interval would cap that; it is roughly five lines. It is deferred because nothing leaks (workstream 2 closes
the old connection), a unix-socket dial with an in-process server is cheap, and the worst case is bounded at
one wasted dial per failing request. It also has a real cost of its own: a genuine transport death landing
inside the interval would be refused a recovery it could have had. **Trigger:** a visible spike in
`argocd_client_dials_total{reason="reconnect"}` during an ArgoCD restart or outage. Pick the interval from what
that data shows, rather than guessing a constant now.

**Keep-warm ticker.** A background RPC per instance on an interval shorter than gRPC's 30-minute idle timeout
would stop the dominant trigger from ever firing. Once reconnect works this only saves the first post-idle
request the ~3s that argo's own `grpc_retry` burns, and it spends a real ArgoCD RPC per instance per interval
forever. **Trigger:** that first-request latency actually showing up in practice — a slow diffs page after a
quiet period, not a hypothetical.

## What shipped

All five workstreams, in `internal/argocd/client.go`, `internal/argocd/metrics.go`,
`internal/argocd/wrapper.go` and `internal/tangle/server.go`, plus the fakes and both test files.
`task go:test` and `go test -race ./internal/argocd/` pass; the e2e cases ran against the live stack from
`task services`.

**Corrected during implementation: the retry needs two status codes, not one.** The plan classified on
`codes.Unavailable` alone, and argued that a sibling RPC aborted by another goroutine's reconnect would see
`Unavailable` and retry on the new generation. That is wrong. An RPC against a closed `ClientConn` returns
grpc-go's `ErrClientConnClosing`, which is `codes.Canceled` (`grpc@v1.82.2/clientconn.go:72`) — so with the
plan as written, the in-flight RPC would have failed outright and the documented mitigation would not have
held. `isConnectionLost` therefore accepts `Unavailable` *or* `Canceled`, and the `ctx.Err()` guard is what
separates `ErrClientConnClosing` from a genuinely cancelled caller. Both codes are covered by
`TestArgoCDClient_ReconnectsWhenTheConnectionIsLost`, and
`TestArgoCDClient_DoesNotReconnectForACancelledCaller` pins the guard.

**The e2e regression test triggers the reconnect directly** rather than through a simulated `Unavailable`.
The production trigger is gRPC's 30-minute idle timeout, and the resulting error — a client preface written
to a closed socket — can't be provoked from outside the SDK: closing the connection ourselves produces
`Canceled`, not `Unavailable`. Classification is unit-tested against stubs;
`TestE2E_ArgoCDClient_ReconnectsAfterConnectionLoss` proves the half stubs can't, that a replacement
connection dialed against a live ArgoCD serves requests and that the previous proxy socket is gone
afterwards. The socket assertions (`assert.NoFileExists` against a `/tmp/argocd-*.sock` set difference) are
the direct evidence for workstream 2.

**`FakeWrapper` needed `Close` too.** `argocdfakes` has a fake wrapper as well as a fake client, which the
plan's inventory missed; both implement the new method as a no-op.

Two things the plan called for and this deliberately did not do: there is no rate limit on redialing and no
keep-warm ticker. Both stay in [Follow-ups](#follow-ups-deferred), gated on
`argocd_client_dials_total{reason="reconnect"}`.
