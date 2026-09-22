# Internals

How the pieces fit together at runtime. For the technology choices and the reasoning behind them,
see [Architecture](../architecture.md)
([published](https://ivanklee86.github.io/tangle/architecture/)).

Tangle is a stateless fan-out proxy in front of N ArgoCD API servers. A single Go binary
([`tangle-server`](https://github.com/ivanklee86/tangle/blob/main/cmd/tangle-server/main.go)) serves
the REST API, an embedded Swagger UI, Prometheus metrics, and the pre-built static SPA. A second
binary ([`tangle-cli`](https://github.com/ivanklee86/tangle/blob/main/cmd/tangle-cli/main.go))
consumes that same API so pipelines and humans have feature parity.

## Topology

```mermaid
flowchart LR
    subgraph clients[Clients]
        browser[Browser]
        ci[CI/CD pipeline]
    end

    ci --> cli["tangle-cli<br/>(cobra + viper)"]
    cli --> sdk["pkg/client<br/>retries + backoff"]
    browser --> spa["SvelteKit SPA<br/>(adapter-static)"]

    subgraph server["tangle-server (single process)"]
        direction TB
        router["chi router<br/>httplog · cors · recoverer · timeout"]
        handlers["internal/tangle/handlers<br/>label parsing · sortOrder · JSON"]
        manifests["internal/tangle/manifests<br/>JSONToYAML → exec diff -uNar"]
        static["http.FileServer ./build"]
        ops["/metrics · /health · /swagger"]
        router --> handlers --> manifests
        router --> static
        router --> ops
    end

    sdk -- "GET /api/applications<br/>POST /api/argocd/{argocd}/applications/{name}/diffs" --> router
    spa -- "fetch PUBLIC_BASE_URL/api" --> router
    static -. "serves" .-> spa

    subgraph wrappers["internal/argocd — one IArgoCDWrapper per configured ArgoCD"]
        direction TB
        pool1["pond pool: list (10)"]
        pool2["pond pool: manifests (5)"]
        pool3["pond pool: hard-refresh (5)"]
    end

    handlers --> wrappers
    wrappers --> client["ArgoCDClient<br/>argo-cd apiclient, gRPC-Web + token<br/>redials on connection loss"]
    client --> argo1[(ArgoCD A<br/>api-server → repo-server)]
    client --> argo2[(ArgoCD B<br/>api-server → repo-server)]
```

## Request paths

- **`GET /api/applications?labels=k:v&excludeLabels=k:v`** — fans out over every configured ArgoCD,
  translating labels into a single Kubernetes selector (`k=v,k!=v`), and returns per-instance results
  ordered by `sortOrder`. Deep links back into each ArgoCD UI are synthesized from the instance
  address and scheme (`http`/`https`, from that instance's `plainText`/`insecure` config).
- **`POST /api/argocd/{argocd}/applications/{name}/diffs`** — submits a `refresh=hard` `Get` on the
  hard-refresh pool, then generates manifests for `liveRef` and `targetRef` concurrently on the
  manifests pool, converts each to YAML, and shells out to `diff -uNar` over two tempfiles.

Every pool is a [`alitto/pond`](https://github.com/alitto/pond) worker pool sized per-instance via
config; this is the throttle that keeps Tangle from overwhelming ArgoCD's `repo-server`. Pools are
registered as Prometheus `GaugeFunc`/`CounterFunc` collectors labeled `pool` + `argocd`
(`internal/argocd/metrics.go`), unless `DoNotInstrument` is set.

## Connections to ArgoCD

`ArgoCDClient` sets `GRPCWeb: true`, so argo-cd's apiclient never dials ArgoCD directly: it starts a
local gRPC → gRPC-Web reverse proxy on a unix socket (`/tmp/argocd-<random>.sock`) and points the gRPC
client at that. Every RPC goes client → socket → proxy → gRPC-Web → ArgoCD. gRPC-Web is hardcoded
rather than configurable because Tangle can't know whether a given ArgoCD sits behind an ingress that
carries native gRPC.

That connection can't re-establish itself — argo-cd's `BlockingNewClient` dials once and reuses the
same `net.Conn` for every subsequent "dial" — so the client owns recovery. Each `ArgoCDClient` holds
one connection behind a mutex; an RPC that fails with `codes.Unavailable`, or `codes.Canceled` while
its request context is still live, triggers one redial and one retry. Concurrent failures from a pool
fan-out collapse into a single redial via a generation counter, and the replaced connection is closed,
which is what stops the old proxy and unlinks its socket. `Close()` does the same at shutdown. Dials
are counted by `argocd_client_dials_total{argocd,reason,result}` and
`argocd_client_connection_generation{argocd}` — unlike the pool collectors, these are always
registered. See [ADR 0025](../adrs/0025-reconnect-the-argocd-grpc-client.md).

A client whose first dial fails is kept and connects lazily on first use, so an ArgoCD that's briefly
down at boot doesn't take the pod with it. A missing `authTokenEnvVar` is still fatal for that
instance: `internal/tangle/server.go` logs it and skips registering a wrapper for it.

## Configuration

[`koanf`](https://github.com/knadh/koanf) layers, last wins: struct defaults → YAML at
`TANGLE_CONFIG_PATH` → `TANGLE_`-prefixed env vars (`_` → `.`). ArgoCD auth tokens are never in the
config file — each instance names an env var (`authTokenEnvVar`) read at client construction.

Env var names are resolved against `TangleConfig`'s `koanf` tags, reflected into a key tree at
startup (`internal/tangle/envkeys.go`): a variable that names no key is dropped and recorded in
`IgnoredEnvVars`, which `New` logs once at `WARN`, and one that does is rewritten to the tag's
own spelling (`TANGLE_LISTWORKERS` → `listWorkers`) so it overrides the file's key instead of
landing beside it. Map keys under `argocds` pass through as written
(`TANGLE_ARGOCDS_TEST_ADDRESS` → `argocds.test.address`), so an ArgoCD instance name containing
`_` can't be targeted this way. The filter exists because Kubernetes injects
`<SERVICE>_PORT_<port>_<proto>_ADDR`-style vars for every Service in the pod's namespace, and a
Service named `tangle` collides with this prefix — see
[ADR 0023](../adrs/0023-resolve-env-var-overrides-against-the-config-schema.md).

The CLI uses [`cobra`](https://github.com/spf13/cobra)/[`viper`](https://github.com/spf13/viper)
with the same `TANGLE_` prefix.

## Build

Multi-stage [`Dockerfile`](https://github.com/ivanklee86/tangle/blob/main/Dockerfile): stage 1 runs
[`swagger generate spec`](https://goswagger.io/) (the result is `go:embed`ed and served at
`/swagger`) then `go build`; stage 2 runs `npm run build` against the
[SvelteKit](https://svelte.dev/docs/kit) app; stage 3 is a non-root Alpine image holding
`tangle-server`, `tangle-cli`, and `./build`. The server resolves static assets from `./build`
relative to its working directory, so the API and the SPA are always the same deploy.
