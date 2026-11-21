---
status: "accepted"
date: 2026-11-21
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Pin the k3s version of the k3d test cluster

## Context and Problem Statement

`k3d-config.yaml` doesn't set `image`, so the cluster runs whatever k3s version the pinned k3d binary defaults to. For k3d v5.9.0 that is `v1.35.5-k3s1`. The Kubernetes version under test is therefore not pinned, contrary to AGENTS.md's "Versions should always be pinned" rule. It also changes silently whenever Renovate bumps `K3D_VERSION`, bundled into an unrelated k3d PR.

argonap hit the consequence when it moved to k3s 1.37. k3s 1.37 bundles a `gateway-api-crd` add-on, installed by its helm-controller. That add-on races the pinned Gateway API CRDs `task k8s:bootstrap:traefik` applies with `kubectl apply --server-side`. The apply then fails with a field-manager conflict against `helm`. tangle would hit the same failure the first time a k3d bump moved its default k3s to 1.37.

## Decision Drivers

- The Kubernetes version under test should be explicit and reviewable.
- `tasks/k8s.yaml` should stay the single source of the Gateway API CRDs ([ADR 0001](0001-replace-ingress-nginx-with-traefik-and-gateway-api.md)).
- k3s should be bumped by Renovate like every other pin ([ADR 0013](0013-renovate-weekly-grouped-updates.md)).

## Considered Options

- Pin `image: rancher/k3s:<version>` in `k3d-config.yaml`, disable the `gateway-api-crd` add-on, and track the image with a Renovate regex manager (chosen).
- Keep inheriting k3d's default and deal with the add-on when a k3d bump brings in 1.37.
- Drop the pinned Gateway API CRDs and rely on k3s's bundled add-on.

## Decision Outcome

Chosen option: pin `rancher/k3s:v1.37.2-k3s1`, matching argonap, and pass `--disable=gateway-api-crd` to the server. A regex custom manager in `renovate.json` tracks the image under the `k8s` group. Its versioning template understands the `-k3sN` build suffix, so `v1.37.2-k3s2` is seen as newer than `v1.37.2-k3s1`.

### Consequences

- Good, because the Kubernetes version under test is visible in the repo and changes only in its own reviewed (or automerged, gated) PR.
- Good, because the Gateway API CRD version stays under the pinned `tasks/k8s.yaml` URL, not whatever a k3s release bundles.
- Bad, because this moves the test cluster from 1.35 to 1.37 in one step. CI on this PR is the check that nothing in the stack depends on the older version.
- Neutral, because k3d bumps no longer change the cluster version, so a k3d upgrade and a k3s upgrade now arrive as separate updates.

## Pros and Cons of the Options

### Keep k3d's default

- Good, because there is nothing to maintain.
- Bad, because the version under test is implicit, and the 1.37 add-on conflict would surface as an unexplained failure in a k3d bump PR.

### Use k3s's bundled Gateway API CRDs

- Good, because there is one fewer manual apply in `bootstrap:traefik`.
- Bad, because the CRD version would then be whatever k3s bundles. It could drift from what the Traefik chart and argocd's `HTTPRoute` expect, and it would no longer be pinned independently.
