---
status: "proposed"
date: 2026-09-23
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Retry ArgoCD calls whose response was lost in transit

## Context and Problem Statement

Users occasionally see a request fail with:

```text
rpc error: code = Unknown desc = unexpected EOF
```

It also failed CI on #242, where `TestE2E_ServerApplications` got it on four subtests while `cmd/tangle-cli`'s e2e tests were running against the same ArgoCD.

This is not the failure [ADR 0025](0025-reconnect-the-argocd-grpc-client.md) handles. That one is our local connection to argo-cd's gRPC-Web proxy dying, which arrives as `codes.Unavailable` and is fixed by redialing. Here the local connection is fine. The response was lost further out.

With `GRPCWeb: true`, every RPC goes client → unix socket → argo-cd's in-process proxy → HTTP POST → ArgoCD. The proxy reads the HTTP response as a sequence of gRPC-Web frames and stops at the trailer frame. When the body ends before the trailer, it turns the short read into `io.ErrUnexpectedEOF` (`pkg/apiclient/grpcproxy.go`, `startGRPCProxy`), which reaches the caller as `codes.Unknown`.

We isolated where the body gets cut off by running 80 concurrent `List` calls, 2,000 at a time, against the local k3d stack over three paths:

| Path | Failed calls |
| --- | --- |
| Port-forward straight to `argocd-server` | 0 of 4,000 |
| Port-forward to Traefik, which routes to `argocd-server` | 17 of 4,000 |
| `localhost:8080`: k3d load balancer, then Traefik | 4 of 4,000 |

A raw HTTP probe showed the failing responses were `200 OK`, HTTP/1.1, chunked, and ended about 6 KB into the body with no terminating chunk. The reverse proxy in front of ArgoCD closed the response partway through. Ingresses and load balancers do this under load, during rollouts, and when an idle timeout races a request, so any real deployment behind one will see it at some rate.

Nothing retried it:

- argo-cd's `grpc_retry` interceptor retries only `Unavailable` and `ResourceExhausted` (go-grpc-middleware's `DefaultRetriableCodes`).
- ADR 0025's reconnect retries only `Unavailable` and `Canceled`, and deliberately not `Unknown`.
- argo-cd's `HttpRetryMax` option wraps `http.Client.Do`, which has already succeeded by the time the body is cut short. On TLS connections argo-cd also replaces the retrying transport with its own, so the option does nothing there.

Two related findings in argo-cd v3.5.3 that we can't change without reimplementing its client:

- After every response the proxy calls `httpClient.CloseIdleConnections()`. For plaintext connections that client is a bare `&http.Client{}`, which uses `http.DefaultTransport`, so every RPC empties the process-wide keep-alive pool. That makes each RPC more likely to open a fresh connection through the ingress, but it didn't cause the truncation: the direct path, which goes through the same code, never failed.
- The proxy reads frames up to the trailer and then closes the body, so connections are rarely reused anyway.

## Decision Drivers

- The failure is transient and uncorrelated. In testing, each one succeeded on the first retry.
- Every RPC Tangle makes is a read (`List`, `Get`, `GetManifests`), so repeating one is safe.
- The failure arrives as `codes.Unknown`, the same code ArgoCD uses for its own errors, so it can't be told apart by code.
- A retry must not keep a caller waiting after it has gone away, and must not multiply load during a real outage.
- Per `AGENTS.md`, retries have to be visible, so a rising rate is noticed rather than hidden.

## Considered Options

- **Retry on the known transport-failure messages, in `ArgoCDClient`** (chosen)
- **Retry every `codes.Unknown`**
- **Retry in the HTTP handlers, or leave it to callers such as `tangle-cli --retries`**
- **Configure argo-cd's `HttpRetryMax`**
- **Serialize or throttle calls to ArgoCD** (what #242 tried first, with `go test -p 1`)

## Decision Outcome

Chosen: **retry on the known transport-failure messages, in `ArgoCDClient`**.

`callWithReconnect` becomes one loop with two independent recoveries:

- A lost connection (ADR 0025) still redials once.
- A transient transport failure is retried on the same connection up to `maxTransientRetries` (2) times. The backoff is 100 ms × attempt, with jitter of up to half.

A call can use both recoveries, for example a truncated response followed by a dead connection. A caller whose context ends during a backoff gets its answer immediately, with no further attempt.

A failure counts as transient when it is `codes.Unknown` and its message is, or ends with, one of these:

- `unexpected EOF`: the body ended before the trailer.
- `: EOF`: `http.Client.Do` found the connection closed before any response arrived, reported as `Post "<url>": EOF`.
- `connection reset by peer`.

Each retry is counted in `argocd_client_retries_total{argocd,method,reason}`, where `reason` is `transient` or `reconnect`. Each retry logs a warning, and a call that succeeds after retrying logs at info.

### Consequences

- Good, because a truncated response no longer fails the user's request. After the change, the same stress test succeeded on 8,000 of 8,000 calls through Traefik. The parallel Go e2e suite, which failed about 1 run in 4 before, passed 8 of 8, so the `-p 1` workaround is reverted.
- Good, because the retry rate per instance is on `/metrics`, which gives an operator a direct measure of how often their ingress is cutting responses off.
- Bad, because this matches error text, which ADR 0025 chose not to do. The code carries no signal here, so text is all there is. The match is anchored to the whole message or its suffix, and the texts come from Go's standard library (`io.ErrUnexpectedEOF`, `net/http`'s `url.Error` wording, and the `ECONNRESET` string), which are stable.
- Bad, because an ArgoCD error whose own message happens to end in one of those texts will be retried twice before it's returned. An example is a repo-server Git fetch that failed with `EOF`. The cost is bounded at two extra calls and at most about 300 ms, and the final error is unchanged.
- Bad, because a retried `GetManifests` repeats the hard-refresh `Get` that precedes it. ADR 0025 already accepts that cost for reconnects, for the same reason.
- Neutral, because the argo-cd behaviours above (emptying the default transport's idle pool, no retry on `Unknown`) remain. If a future argo-cd major version retries these itself, this becomes a second layer and stays harmless.

## Pros and Cons of the Options

### Retry on the known transport-failure messages, in `ArgoCDClient` (chosen)

- Good, because it covers every caller: the Applications and Diffs handlers, the manifest fan-out, and so `tangle-cli` and the web UI.
- Good, because it sits next to ADR 0025's reconnect, where the knowledge of how argo-cd's proxy fails already lives.
- Bad, because it depends on error text.

### Retry every `codes.Unknown`

- Good, because it needs no text matching.
- Bad, because `codes.Unknown` is also what ArgoCD returns for real failures, such as a manifest generation error or a bad selector. Retrying those would triple their cost and delay every such error for nothing.

### Retry in the HTTP handlers, or leave it to callers

- Good, because a whole request can be retried.
- Bad, because the handlers fan out over many applications, so one lost response would redo all of them.
- Bad, because the web UI has no retry, and `tangle-cli --retries` only helps CLI users who remember to pass it.

### Configure argo-cd's `HttpRetryMax`

- Good, because it's one option.
- Bad, because it wraps `http.Client.Do`, which has already returned successfully when the body is cut short, so it wouldn't catch the observed failure.
- Bad, because on TLS connections argo-cd overwrites the retrying transport with its own, silently disabling the option.

### Serialize or throttle calls to ArgoCD

- Good, because less concurrency means fewer truncations.
- Bad, because it only lowers the rate. The failure happened even with one client and 20 concurrent calls.
- Bad, because it slows every fan-out down for everyone to avoid a rare failure that a retry absorbs in about 100 ms.

## More Information

- [ADR 0025](0025-reconnect-the-argocd-grpc-client.md): the reconnect this extends.
- Evidence, at the pinned `github.com/argoproj/argo-cd/v3 v3.5.3`:
  - the frame loop and `io.ErrUnexpectedEOF` in `pkg/apiclient/grpcproxy.go` (`startGRPCProxy`), and the `CloseIdleConnections()` call after each response in the same function;
  - the bare `&http.Client{}` for plaintext and the transport override for TLS in `pkg/apiclient/apiclient.go` (`NewClient`);
  - the `grpc_retry` options in `newConn` in the same file.
- The stress harness was a temporary e2e test running concurrent `List` calls from 1–10 `ArgoCDClient`s at a time; it isn't kept in the repository.
