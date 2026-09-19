---
status: "accepted"
date: 2026-09-19
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Replace ingress-nginx with Traefik and Gateway API

## Context and Problem Statement

`integration/kubernetes/ingress-nginx` (chart `ingress-nginx` 4.12.1) fronts the local ArgoCD instance used for integration testing and local development. The upstream [ingress-nginx project retired](https://www.kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/) in March 2026: best-effort maintenance has ended, and there will be no further releases, bugfixes, or security patches. The Kubernetes project's official recommendation is to migrate to [Gateway API](https://gateway-api.sigs.k8s.io/), the successor to the `networking.k8s.io/Ingress` resource. We need a replacement ingress path for the k3d-based local/CI cluster (`k3d-config.yaml`, `tasks/k8s.yaml`).

## Decision Drivers

- ingress-nginx receives no further security patches; running it after retirement is a growing risk even for a local/CI-only cluster.
- `k3d-config.yaml` already disables k3d's bundled Traefik (`--disable=traefik`) purely to make room for ingress-nginx, so removing ingress-nginx also removes a whole Helm chart dependency (`integration/kubernetes/ingress-nginx`, its vendored `.tgz`, and its `helm repo add`/`dependency build` steps in `tasks/k8s.yaml`).
- AGENTS.md requires pinned versions; whatever replaces ingress-nginx should be as easy to keep current as the rest of the stack.
- Tangle's only ingress consumer is a single `Ingress` object (`integration/kubernetes/argocd/templates/ingress.yaml`) fronting `argocd-server`, so the blast radius of a routing-model change is small.

## Considered Options

- Traefik via classic `networking.k8s.io/Ingress`
- Traefik via Gateway API (`Gateway` + `HTTPRoute`)
- Another maintained Ingress controller (e.g. ingate, F5 NGINX Ingress Controller)

## Decision Outcome

Chosen option: "Traefik via Gateway API", because it follows Kubernetes' official long-term recommendation instead of trading one soon-to-be-legacy Ingress implementation for another, and it reuses Traefik, which k3d/k3s already bundles and which has stable (non-experimental as of Traefik v3) Gateway API support.

### Consequences

- Good, because it removes an entire vendored Helm chart (`integration/kubernetes/ingress-nginx`) in favor of re-enabling k3d's built-in Traefik.
- Good, because the routing model (Gateway API) matches where the ecosystem and Kubernetes docs are steering all Ingress users, so this doesn't need to be revisited again soon.
- Bad, because it introduces a new CRD set (Gateway API `standard` channel) that must be installed and kept in sync with the Traefik chart version, adding a moving part that classic Ingress wouldn't have needed.
- Bad, because `integration/kubernetes/argocd/templates/ingress.yaml` and any docs referencing it (`docs/configuring_argocd.md`, `docs/agents/internals.md`) must be rewritten for the `Gateway`/`HTTPRoute` model rather than a one-line `ingressClassName` swap.

## Pros and Cons of the Options

### Traefik via classic Ingress

Re-enable k3d's bundled Traefik, drop ingress-nginx, change `ingressClassName: nginx` to `ingressClassName: traefik`.

- Good, because it's the smallest possible change — no new CRDs, no new resource types.
- Good, because the `Ingress` API itself is not deprecated, only the ingress-nginx implementation.
- Bad, because it's a lateral move to another Ingress implementation instead of adopting the direction Kubernetes is pushing the ecosystem toward.

### Traefik via Gateway API

- Good, because it's the officially recommended long-term path and avoids doing this migration again in a couple of years.
- Good, because Traefik still ships as k3d's default, so no new controller image is introduced.
- Bad, because it requires installing and version-pinning the Gateway API CRDs separately, and rewriting the existing `Ingress` template as `Gateway` + `HTTPRoute`.

### Another maintained Ingress controller

e.g. the community `ingate` fork of ingress-nginx, or F5 NGINX Ingress Controller.

- Good, because annotations/behavior stay closest to today's nginx-based setup.
- Bad, because it adds a chart dependency where the Traefik options remove one (Traefik ships with k3d already).
- Bad, because it's still classic Ingress, so it doesn't move the project toward Gateway API.

## More Information

- [Ingress NGINX Retirement: What You Need to Know (Kubernetes blog)](https://www.kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/)
- [Traefik Kubernetes Gateway API provider docs](https://doc.traefik.io/traefik/reference/install-configuration/providers/kubernetes/kubernetes-gateway/)
- [Gateway API getting started guide](https://gateway-api.sigs.k8s.io/guides/getting-started/introduction/)
- Superseded by, if adopted later: none.
