---
status: "proposed"
date: 2026-09-21
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Reconnect the ArgoCD gRPC client instead of holding one connection for the process lifetime

## Context and Problem Statement

Users occasionally see the web UI's `ErrorAlert` ("System error!") with this underneath:

```text
rpc error: code = Unavailable desc = connection error: desc = "transport: failed to write client preface: write unix @->/tmp/argocd-ZaFuBGlzuGsgrjYt.sock: use of closed network connection"
```

The socket in that message is ours, not ArgoCD's. `internal/argocd/client.go:54` sets `GRPCWeb: true`, and with that flag argo-cd's apiclient does not dial ArgoCD directly — it starts a local gRPC → gRPC-Web reverse proxy listening on a unix socket named `/tmp/argocd-<16 random chars>.sock` (`pkg/apiclient/grpcproxy.go:113-119`) and points the gRPC client at that socket. Every RPC goes client → unix socket → proxy → HTTPS gRPC-Web → ArgoCD.

The connection to that socket can never be re-established. The dial runs through argo-cd's `util/grpc.BlockingNewClient`, which dials once, eagerly, and then installs a `grpc.WithContextDialer` closed over that single `net.Conn`:

```go
rawConn, err := proxy.Dial(ctx, network, address)   // dialed once
customDialer := func(_ context.Context, _ string) (net.Conn, error) {
    return rawConn, nil                             // ...handed back forever
}
```

So when gRPC re-creates the transport, the "dialer" returns the same, now-closed file descriptor, and gRPC writes the HTTP/2 client preface into it — `grpc@v1.82.2/internal/transport/http2_client.go:433`. The `@->` (empty local address) and "use of closed network connection" both say the descriptor was closed by our own side; ArgoCD was never contacted. The usual trigger is gRPC's idle timeout, 30 minutes with no RPCs (`grpc@v1.82.2/dialoptions.go:729`), after which the `ClientConn` closes its transport; keepalive pings do not prevent it, since `PermitWithoutStream` is false and pings only flow while a stream is active. Any transport error or `GOAWAY` does the same.

`internal/tangle/server.go:99` creates one `ArgoCDClient` per configured ArgoCD at startup and holds it for the life of the process, so this is not a transient failure: once that connection dies, *every* subsequent request to that instance fails the same way until the pod restarts. It reads as intermittent only because there are several replicas, or because pods get restarted for unrelated reasons. `internal/argocd/client.go:57` compounds it by discarding the `io.Closer` that `NewApplicationClientOrDie` returns, so nothing ever stops the proxy's `grpc.Server`, ends its goroutine, or unlinks the socket file — harmless today at one-per-startup, but it makes any design that creates a second connection a leak.

This is still present on argo-cd `master`, and no upstream issue for it was found. [ADR 0015](0015-restrict-argo-cd-go-module-updates-to-major-only.md) disables minor and patch updates for `github.com/argoproj/argo-cd/v3` entirely, so even a fix released in v3.6 would not reach this repository until someone does a manual major bump. Whatever we decide has to work on the v3.5.3 internals we have.

## Decision Drivers

- The failure is permanent per process and silent until a user hits it. Nothing in `/metrics` or the logs distinguishes "ArgoCD is down" from "our connection to ArgoCD has been dead since 04:10".
- The fix has to live in tangle. The defect is in a vendored SDK whose update path is deliberately manual (ADR 0015), so "wait for upstream" is a multi-month bet, not a plan.
- We do not control the connection object. `NewApplicationClient()` hands back only an `io.Closer` and a service client, never the `*grpc.ClientConn`, so `GetState()`, `WithIdleTimeout`, and every other dial option are out of reach without reimplementing `newConn` ourselves.
- Whatever detects the dead transport must not mistake a sick ArgoCD for one. The proxy forwards ArgoCD's own `Grpc-Status` header verbatim (`pkg/apiclient/grpcproxy.go`, `executeRequest`), so `codes.Unavailable` is not exclusively a local signal.
- Recovery must be cheap under concurrency. `ListApplicationsByLabels` and `GetManifests` fan out over `pond` pools, so a dead transport surfaces as several simultaneous failures, not one.
- Per `AGENTS.md`, observability is a first-class requirement, and this is exactly the class of bug that argues for it: the only reason it took a user report to find is that nothing counted it.

## Considered Options

- **Reconnect on failure** (chosen) — the client owns its connection, retries an RPC whose failure says the connection is gone once against a freshly dialed connection, and closes the old one.
- **Keep-warm ticker** — a background RPC per instance every few minutes, so the `ClientConn` never goes idle.
- **A fresh client per request** — dial in the handler, `defer Close()`.
- **Wait for an upstream fix** — file the issue against argo-cd and carry the breakage until a major bump picks it up.
- **Reimplement `newConn` in tangle** — copy argo-cd's connection setup with a real dialer, bypassing `BlockingNewClient`.

## Decision Outcome

Chosen: **reconnect on failure**, with the `io.Closer` owned rather than discarded.

`ArgoCDClient` holds a `connection{applications, closer, generation}` behind a mutex. All three RPC methods funnel through one generic helper that runs the call, and — if the failure says the connection itself is gone and the request context is still live — redials once and runs it again. `reconnect` takes the connection the caller actually used and redials only if that is still the current one, so N concurrent failures from a pool fan-out collapse into a single dial. Old connections are closed, which stops the proxy server and unlinks its socket. The client gains a `Close()`, surfaced on `IArgoCDClient` and `IArgoCDWrapper` and called from the graceful-shutdown block in `internal/tangle/server.go`.

Four sub-decisions are worth recording explicitly:

**Classify on the gRPC status code, not on the error string.** Matching "client preface" or "use of closed network connection" would pinpoint the local failure, but it binds us to grpc-go's wording. We accept that an ArgoCD rollout returning grpc-status 14 will cost one wasted redial. The common upstream-outage cases — `httpClient.Do` failing, a non-200 response — arrive as `codes.Unknown` and never trigger us.

Two codes count, not one. `codes.Unavailable` is the idle-timeout case: gRPC tried to re-create the transport and failed. `codes.Canceled` **with a live request context** is grpc-go's `ErrClientConnClosing` (`grpc@v1.82.2/clientconn.go:72`) — an RPC that was in flight when its connection was closed underneath it. An earlier draft of this ADR claimed such an RPC would see `Unavailable` and retry on the new generation; that was wrong, and checking the constant during implementation is what caught it. Without `Canceled`, the documented mitigation for the in-flight hazard below does not actually hold: a sibling RPC caught by another goroutine's reconnect would fail outright instead of retrying. A caller-cancelled request produces the same code, which is what the `ctx.Err()` guard separates.

**Ship without a rate limit on redialing, and let the metrics decide.** An earlier draft paired the retry with a `minReconnectInterval` constant, so that an ArgoCD outage arriving as `Unavailable` could not make every incoming request dial and discard a connection. It is deferred rather than adopted: generation collapsing already bounds concurrent redials, workstream 2 means a discarded connection leaks nothing, a unix-socket dial with an in-process server is cheap, and the unbounded worst case is still only one wasted dial per failing request. It also carries a cost of its own — a genuine transport death landing inside the interval would be refused a recovery it could have had. Rather than guess an interval, we ship the dial counter in the same change and add the limit if `argocd_client_dials_total{reason="reconnect"}` spikes during an ArgoCD restart. See the plan's follow-ups section.

**Dialing must return errors, so `NewClientOrDie`/`NewApplicationClientOrDie` go.** Both call `log.Fatal`; on a reconnect path a transient failure would kill the server. As a consequence, a dial failure at boot no longer exits the process: `NewArgoCDClient` logs a warning and connects lazily on first use. This is a deliberate improvement — note that with `GRPCWeb: true` the startup dial only ever proved the local proxy socket was up, never that ArgoCD was reachable, so nothing is being given up. A missing auth-token environment variable stays a hard constructor error, because that is a config mistake rather than a transient one.

**Instrument the path.** A process-wide `argocd_client_dials_total{argocd,reason,result}` counter and an `argocd_client_connection_generation{argocd}` gauge, plus `slog` lines at reconnect. Unlike the pool collectors in `internal/argocd/metrics.go`, these are not gated by `DoNotInstrument`: a single package-level vec is registered once per process and is safe under tests, where the per-pool const-labelled collectors would panic on re-registration.

### Consequences

- Good, because a tangle-server process recovers by itself from a dead transport, which is the reported bug. A user-visible 500 becomes one logged warning and a slightly slower request.
- Good, because the proxy `grpc.Server`, its goroutine, and `/tmp/argocd-*.sock` are now released — on reconnect and at shutdown.
- Good, because "how often are we redialing ArgoCD?" becomes a dashboard question instead of an investigation.
- Good, because a briefly unreachable ArgoCD at boot no longer takes the pod down with it.
- Bad, because the first request after a dead transport pays for the failure before it recovers. argo-cd already installs `grpc_retry.UnaryClientInterceptor` with `WithMax(3)` and a 1s linear backoff (`pkg/apiclient/apiclient.go:524-530`), and go-grpc-middleware retries `Unavailable` by default, so ~2-3s is spent before we even see the error; our dial and retry come on top. All of it sits under chi's `middleware.Timeout(config.Timeout)`.
- Bad, because retrying on `Unavailable` occasionally redials when ArgoCD, not the socket, was the problem. The one-retry cap bounds it per request; nothing yet bounds it across a sustained outage, which is the deferred rate limit above.
- Bad, because closing a connection aborts anything still in flight on it. We only close one we have already seen fail, and a sibling RPC caught this way sees `codes.Canceled` and retries on the new generation, but it is a real ordering hazard rather than an impossible one.
- Neutral, because this is a workaround for an SDK defect. If argo-cd fixes `BlockingNewClient`, the retry becomes belt-and-braces and stays.

## Pros and Cons of the Options

### Reconnect on failure (chosen)

- Good, because it addresses the actual defect — the connection cannot be re-established — rather than one of its triggers.
- Good, because it recovers from any cause of transport death, not just the idle timeout.
- Good, because the steady-state cost is zero: a healthy call takes one extra error check.
- Bad, because it adds concurrency-sensitive state (mutex, generation counter) to a struct that had none.
- Bad, because classification on `codes.Unavailable` is a heuristic, for the reasons above.

### Keep-warm ticker

- Good, because it is a dozen lines and needs no new state.
- Good, because it removes the dominant trigger — the 30-minute idle timeout — outright.
- Bad, because it is not a fix. Any other transport death (`GOAWAY`, a transport error, a proxy hiccup) still bricks the client permanently, and the symptom would get rarer and therefore harder to diagnose.
- Bad, because it spends a real ArgoCD RPC per instance per interval forever to work around a client bug.

### A fresh client per request

- Good, because it is obviously correct and stateless.
- Good, because it makes the discarded-closer fix mandatory rather than optional.
- Bad, because every request pays a dial plus a new gRPC-Web proxy `grpc.Server`, listener, goroutine, and socket file — the fan-out in `GetManifests` would create several per diff.
- Bad, because it trades a rare permanent failure for a permanent latency and file-descriptor cost.

### Wait for an upstream fix

- Good, because it puts the fix where the defect is, and everyone using `--grpc-web` benefits.
- Bad, because ADR 0015 holds argo-cd at manual major bumps, so the fix would not arrive here for months.
- Bad, because it leaves users staring at a stack-trace-shaped error in the meantime.
- Worth doing anyway, in parallel — it just cannot be the plan.

### Reimplement `newConn` in tangle

- Good, because a real dialer would make gRPC's own reconnect machinery work as designed, and would let us set `WithIdleTimeout` and inspect connectivity state.
- Bad, because it means vendoring a copy of argo-cd's proxy setup, auth interceptors, retry options, and TLS handling, then keeping it in sync across major bumps by hand.
- Bad, because it is a much larger surface to get wrong than a retry, for a failure mode we can already handle.

## More Information

- [Implementation plan](../agents/plans/argocd-client-reconnect.md) — workstreams, code sketches, the test-case list, and the open questions.
- Evidence, all at the pinned versions in `go.mod` (`github.com/argoproj/argo-cd/v3 v3.5.3`, `google.golang.org/grpc v1.82.2`): the socket name at `pkg/apiclient/grpcproxy.go:113-119`, the dialer reuse in `util/grpc/grpc.go`'s `BlockingNewClient`, the preface write at `internal/transport/http2_client.go:433`, the 30-minute idle default at `dialoptions.go:729`, and the closer chain at `pkg/apiclient/grpcproxy.go:198-223` and `pkg/apiclient/apiclient.go:550-559`.
- Upstream: [`util/grpc/grpc.go` on master](https://github.com/argoproj/argo-cd/blob/master/util/grpc/grpc.go) still reuses `rawConn`; the gRPC-Web proxy design dates to [PR #1077](https://github.com/argoproj/argo-cd/pull/1077).
- Related: [ADR 0015](0015-restrict-argo-cd-go-module-updates-to-major-only.md) (why an upstream fix would not reach us), [ADR 0023](0023-resolve-env-var-overrides-against-the-config-schema.md) (the same "a variable nobody set became config with no trace" lesson about failures that must be visible).
