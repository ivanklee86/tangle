# Kubernetes stack upgrade

Status: proposed · 2026-09-19

Upgrade the local/CI Kubernetes stack used by `task services` / `task k8s:*` (`k3d-config.yaml`, `integration/kubernetes/**`, `.devcontainer/Dockerfile`, `.github/workflows/ci.yaml`): ArgoCD, ingress-nginx → Traefik + Gateway API (see [ADR 0001](../../adrs/0001-replace-ingress-nginx-with-traefik-and-gateway-api.md)), and k3d.

Do these as a single PR, landing the workstreams in the order below so each one is validated against a working cluster before moving to the next.

## 1. Upgrade ArgoCD

**Current → target**

| | Current | Target |
|---|---|---|
| `argo-cd` Helm chart (`integration/kubernetes/argocd/Chart.yaml`) | 8.5.0 | 10.9.2 |
| ArgoCD app version (chart's `appVersion`) | v3.1.5 | v3.5.3 |
| `github.com/argoproj/argo-cd/v3` (`go.mod`) | v3.1.7 | v3.5.3 |

This is a same-major-version bump (3.1 → 3.5), not the 2.x→3.x jump the version-number jump (8.5.0→10.9.2) suggests — the vendored chart already deploys ArgoCD v3.1.5, matching the v3 Go SDK already in `go.mod`. Risk is lower than a major bump, but still read each minor's upgrade notes (`argo-cd.readthedocs.io/en/stable/operator-manual/upgrading/3.X-3.Y/` for X.Y = 1.2, 2.3, 3.4, 4.5) for anything affecting `configs.cm`/`configs.rbac` in `integration/kubernetes/argocd/values.yaml` or the RBAC policy CI relies on (`tasks/argocd.yaml`'s `automation`/`automationProd` accounts).

**Steps**

1. Bump `version: 8.5.0` → `10.9.2` in `integration/kubernetes/argocd/Chart.yaml`.
2. `cd integration/kubernetes/argocd && helm repo update && helm dependency update` to refresh `Chart.lock` and re-vendor `charts/argo-cd-10.9.2.tgz` (remove the old `.tgz`).
3. `helm template` the chart (`task k8s:helm:template` from that dir) and diff the rendered manifests against the current 8.5.0 output to catch value schema changes early, particularly around `configs.cm`/`configs.rbac` and `server.insecure`.
4. Bump `github.com/argoproj/argo-cd/v3` to `v3.5.3` in `go.mod`/`go.sum` (`go get github.com/argoproj/argo-cd/v3@v3.5.3 && go mod tidy`); build and run `internal/argocd` tests — this package talks to the ArgoCD gRPC/REST API directly, so a server/client version mismatch is the main regression risk here.
5. Bump `quay.io/argoproj/argocd` in `.devcontainer/Dockerfile` (source of the local `argocd` CLI) and the CI `argocd` CLI install step in `.github/workflows/ci.yaml` to match — pin both instead of the CI step's current `.../releases/latest` (AGENTS.md: versions should always be pinned).
6. `task services` end-to-end: cluster up, ArgoCD bootstrap, `argocd:healthcheck`, `argocd:token`, then `task go:test-ci` / `task services:cicd` to exercise `internal/argocd` and the diff/manifest endpoints against the real server.
7. Update `docs/configuring_argocd.md` if any RBAC/account config surface changed.

**Rollback**: revert the chart version, `Chart.lock`, vendored `.tgz`, and `go.mod`/`go.sum` changes; this workstream touches nothing outside `integration/kubernetes/argocd`, `.devcontainer`, `go.mod`/`go.sum`, and CI.

## 2. Replace ingress-nginx with Traefik + Gateway API

See [ADR 0001](../../adrs/0001-replace-ingress-nginx-with-traefik-and-gateway-api.md) for the decision. Target versions: Traefik Helm chart `41.6.0` (Traefik v3.7.12), Gateway API `v1.6.1` standard-channel CRDs.

**Steps**

1. Install Gateway API standard-channel CRDs as a pinned step before Traefik in `tasks/k8s.yaml`'s `bootstrap` chain, e.g. `kubectl apply --server-side -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.6.1/standard-install.yaml`. Pin the URL to `v1.6.1`, not a moving branch ref.
2. In `k3d-config.yaml`, remove the `--disable=traefik` extra arg so k3d's bundled Traefik runs. Confirm the bundled Traefik version is acceptable, or pin an explicit Traefik version via a Helm-chart-based install instead of relying on whatever k3s vendors, so the version is controlled the same way ArgoCD's is — the Helm-chart route (next step) makes this moot if we install Traefik via `integration/kubernetes/traefik` rather than trusting k3d's bundled binary.
3. Add `integration/kubernetes/traefik/` (mirroring the existing `argocd`/`ingress-nginx` chart wrapper layout): `Chart.yaml` pinning `traefik` 41.6.0 from `https://traefik.github.io/charts`, and `values.yaml` enabling the `kubernetesGateway` provider (`providers.kubernetesGateway.enabled: true`) and a `GatewayClass` named `traefik`.
4. Delete `integration/kubernetes/ingress-nginx/` entirely (`Chart.yaml`, `values.yaml`, `Chart.lock`, vendored `charts/ingress-nginx-4.12.1.tgz`).
5. In `integration/kubernetes/argocd/templates/`, replace `ingress.yaml`'s `Ingress` with a `Gateway` (referencing the `traefik` `GatewayClass`) and an `HTTPRoute` routing to `argocd-server:80`, preserving the current `ssl-redirect: false`/insecure behavior needed for `argocd login --insecure --grpc-web` in `tasks/argocd.yaml`. Note ArgoCD's server is gRPC-Web, so the `HTTPRoute` needs to pass through both HTTP and the gRPC-Web content type — verify against Traefik's Gateway API docs rather than assuming parity with the old `Ingress` annotations.
6. Update `tasks/k8s.yaml`'s `bootstrap` task: replace `bootstrap:ingress-nginx` with a `bootstrap:traefik` task (CRDs → `helm repo add traefik` → `helm dependency build` → `helm install`), keeping it before `bootstrap:application` since ArgoCD's `Gateway`/`HTTPRoute` depend on the `GatewayClass` existing.
7. Update `tasks/argocd.yaml`'s `healthcheck` task: the `app.kubernetes.io/name=ingress-nginx` pod-readiness wait becomes a wait on the Traefik pod label instead.
8. Update docs: `docs/architecture.md` (tool list mentions k3d/ingress-nginx implicitly via the stack), `docs/agents/internals.md` if it references ingress-nginx, and `docs/configuring_argocd.md` if the external access instructions change.
9. Validate locally: `task k8s:cluster:delete && task services` end-to-end, confirm `https://localhost:8080` (per `k3d-config.yaml`'s `8080:443` port mapping) reaches `argocd-server` through the new `Gateway`, and that `task argocd:login` / `task services:cicd` still pass.
10. Confirm CI (`.github/workflows/ci.yaml`'s `go` job, which runs `task services:cicd`) is green — this is the workstream most likely to break CI, since it changes how the ArgoCD endpoint becomes reachable.

**Rollback**: revert `k3d-config.yaml`, restore `integration/kubernetes/ingress-nginx/` and the old `ingress.yaml` from git history, and drop `integration/kubernetes/traefik/` and the Gateway API CRD install step.

## 3. Update k3d to latest

**Current state**: `.devcontainer/Dockerfile`'s `K3D_VERSION=v5.9.0` is already the latest k3d release (confirmed via GitHub releases, published 2026-06-02) — no devcontainer change needed unless a newer release ships before this work starts (recheck `k3d-io/k3d` releases at implementation time).

**Actual gap**: `.github/workflows/ci.yaml`'s `Install k3d` step installs via `curl ... k3d-io/k3d/main/install.sh | bash` with no `TAG` pin, so CI silently floats to whatever is latest at run time — inconsistent with the devcontainer's pinned install and with AGENTS.md's "versions should always be pinned" rule.

**Steps**

1. Re-check `k3d-io/k3d` latest release tag at implementation time; bump `.devcontainer/Dockerfile`'s `K3D_VERSION` if a newer one has shipped.
2. Pin CI's install to the same version: `TAG=${K3D_VERSION} bash` in `.github/workflows/ci.yaml`, ideally reading the version from one place (e.g. a repo variable or a value grepped out of the Dockerfile) so the devcontainer and CI can't drift again.
3. Re-run `task devcontainer` (builds the devcontainer image) and CI to confirm both still bring up a working cluster.

**Rollback**: trivial single-line revert in `.github/workflows/ci.yaml` (and the Dockerfile ARG, if changed).

## Sequencing notes

- Do ArgoCD (§1) and k3d (§3) first — they're independent, low-risk, and give a green baseline to build on within the PR.
- Do the ingress replacement (§2) last, since it's the only workstream that changes the cluster's networking model and is most likely to need a few iterations to get the `HTTPRoute` right for ArgoCD's gRPC-Web server.
- Commit each workstream separately within the branch (even though they ship as one PR) so a regression is still easy to bisect to ArgoCD, ingress, or k3d specifically, and run the full `task services` / CI validation again after the ingress change to confirm nothing from §1 or §3 regressed.
