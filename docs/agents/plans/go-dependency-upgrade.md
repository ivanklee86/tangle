# Go toolchain and dependency upgrade

Status: proposed · 2026-09-19

Bump the Go toolchain and Tangle's direct Go dependencies to their latest versions. This is separate from [Kubernetes stack upgrade](kubernetes-stack-upgrade.md), which already moved `go.mod`'s `go` directive to `1.26.3` as a side effect of the ArgoCD SDK bump (argo-cd v3.5.3's own dependency graph requires Go 1.26.1+) — this plan finishes that line of work.

## Scope

**In scope**: the Go toolchain, and the ~24 packages in `go.mod`'s first (non-`// indirect`) `require` block — these are the modules Tangle actually imports and is responsible for keeping current.

**Out of scope**: the ~600 `// indirect` entries in the second `require` block. Almost all of them are transitively pulled in by `github.com/argoproj/argo-cd/v3`, which dominates the module graph (its own dependency tree includes hundreds of `cloud.google.com/go/*` and `k8s.io/*` packages pinned to versions argo-cd itself is built and tested against). Running `go get -u ./...` or `go get -u all` would bump those independently of what argo-cd actually declares, which risks producing a combination argo-cd was never built against — and would fight the `replace` directives [Kubernetes stack upgrade](kubernetes-stack-upgrade.md) just added for `github.com/argoproj/argo-cd/gitops-engine` and the `k8s.io/*` staging modules. Leave indirect versions to whatever `go mod tidy` naturally resolves from the direct requires below; only touch one by hand if it's carrying a known CVE.

## Current → target

| | Current | Target |
|---|---|---|
| Go toolchain (`go` directive in `go.mod`) | 1.26.3 | 1.27.x (latest patch at implementation time) |
| `github.com/alitto/pond/v2` | 2.5.0 | 2.7.1 |
| `github.com/go-chi/chi/v5` | 5.2.3 | 5.3.2 |
| `github.com/gofiber/fiber/v2` | 2.52.9 | 2.52.15 |
| `github.com/jedib0t/go-pretty/v6` | 6.6.8 | 6.8.3 |
| `github.com/knadh/koanf/v2` | 2.3.0 | 2.3.6 |
| `github.com/knadh/koanf/maps` | 0.1.2 | 0.1.3 |
| `github.com/knadh/koanf/parsers/yaml` | 1.1.0 | 1.1.1 |
| `github.com/knadh/koanf/providers/file` | 1.2.0 | 1.2.1 |
| `github.com/knadh/koanf/providers/structs` | 1.0.0 | 1.0.1 |
| `github.com/prometheus/client_golang` | 1.23.2 | 1.24.1 |
| `github.com/stretchr/testify` | 1.11.1 | 1.12.1 |

The rest of the direct requires (`argo-cd/v3`, `acarl005/stripansi`, `flowchartsman/swaggerui`, `go-chi/cors`, `go-chi/httplog/v2`, `google/uuid`, `hellofresh/health-go/v5`, `joho/godotenv`, `knadh/koanf/providers/env`, `spf13/cobra`, `spf13/pflag`, `spf13/viper`, `yarlson/chiprom`, `sigs.k8s.io/yaml`) were already at latest as of 2026-09-19 — re-check with `go list -u -m all` at implementation time since this will drift.

## Steps

1. Re-run `go list -u -m all` at implementation time to catch anything that moved since 2026-09-19 (both the toolchain and the table above).
2. Bump the Go toolchain: update `go.mod`'s `go` directive, `Dockerfile`'s `FROM golang:1.26-alpine` → `1.27-alpine`, and both `go-version` entries in `.github/workflows/ci.yaml` (currently `1.26` / `1.26.x`) to `1.27` / `1.27.x`. The devcontainer's base image (`ghcr.io/ivanklee86/devcontainer/go:1.27`) is already ahead of this, so no devcontainer change needed.
3. Bump each direct dependency in the table individually with `go get <module>@<version>` rather than a blanket `-u`, so a bad bump is easy to bisect; run `go mod tidy` after each and skim the resulting `go.sum`/indirect-require diff for anything unexpected pulled in.
4. `go build ./...`, `go vet ./...`, `gofmt -l .`, `golangci-lint run` — same bar as any dependency bump.
5. `go test ./...` for the packages that don't need a live cluster (`internal/argocd`, `internal/tangle`, `pkg/client`, `internal/cli`, `cmd/tangle-cli` need `task services` up first per [Kubernetes stack upgrade](kubernetes-stack-upgrade.md)'s validation steps) — run the full live-cluster suite too, since `gofiber/fiber`, `go-chi/chi`, and `koanf` bumps could plausibly change HTTP routing or config-parsing behavior even across minor versions.
6. `task docker:build` to confirm the production image still builds against the bumped toolchain and deps.
7. Skim each bumped package's changelog between old and new version for breaking changes before merging — these are all minor/patch bumps per semver, but `fiber/v2`, `chi/v5`, and `koanf/v2` are exactly the packages Tangle's HTTP/config surface depends on most directly.

**Rollback**: revert `go.mod`/`go.sum`, `Dockerfile`, and the CI `go-version` lines; no other files are touched.
