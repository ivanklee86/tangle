---
status: "accepted"
date: 2026-10-24
decision-makers: ["Ivan Lee"]
consulted: []
informed: []
---

# Pull the live stack's argocd-redis image from Docker Hub

## Context and Problem Statement

The amd64 e2e job failed intermittently with the repo-server unable to reach Redis:

```text
rpc error: code = Unknown desc = failed to list refs: dial tcp 10.43.155.210:6379: connect: connection refused
```

It failed on `main` (run 39696797436) and on both runs of Renovate's k8s PR #261, so the argo-cd chart bump was not the cause. #263 made `task argocd:healthcheck` wait on every ArgoCD workload and dump cluster state when e2e fails. The dump showed `argocd-redis` stuck in `ImagePullBackOff`, with this pull error:

```text
Failed to pull image "ecr-public.aws.com/docker/library/redis:8.6.4-alpine": ... unexpected status from GET request to https://ecr-public.aws.com/v2/docker/library/redis/manifests/sha256:...: 429 Too Many Requests
```

The argo-cd chart's default Redis image lives on ECR Public. ECR Public throttles anonymous pulls per source IP, and GitHub-hosted runners share their egress IPs, so whether a run gets the image depends on what other tenants on that IP have been pulling. Every other image in the stack (quay.io, ghcr.io) pulled normally in the same run.

## Decision Drivers

- e2e should fail only for reasons in this repository.
- The image under test should match what the chart ships, and stay in step with chart bumps without manual edits.
- Versions stay pinned.

## Considered Options

- Override `redis.image.repository` to `docker.io/library/redis` (chosen).
- Pre-pull the image on the runner and `k3d image import` it into the cluster.
- Authenticate to ECR Public in CI.
- Leave it and re-run failed jobs.

## Decision Outcome

Chosen option: override `argo-cd.redis.image.repository` to `docker.io/library/redis` in `integration/kubernetes/argocd/values.yaml`. `docker/library/redis` on ECR Public is a mirror of the Docker Official Image, so the image content is the same. GitHub-hosted runners are not subject to Docker Hub's anonymous pull rate limit. The tag is still taken from the chart, so the pinned version moves with the chart version Renovate already tracks.

### Consequences

- Good, because the live stack no longer depends on ECR Public's anonymous quota, in CI or locally.
- Good, because there is no new CI step, secret, or second place to keep the Redis tag in sync.
- Bad, because the live stack's Redis now comes from a different registry than a default argo-cd install. The image is the same, but a chart release that moves Redis to an image Docker Hub doesn't carry would break the stack until this override is updated.
- Neutral, because Docker Hub has its own limits for pulls from outside GitHub-hosted runners. Local and devcontainer pulls of a single image are well within them.

## Pros and Cons of the Options

### Override `redis.image.repository` to Docker Hub (chosen)

- Good, because it is a one-line values change that applies to CI and local runs alike.
- Good, because the tag stays chart-owned.
- Bad, because it relies on Docker Hub keeping the same tags as the ECR mirror, which holds for official images.

### Pre-pull and `k3d image import`

- Good, because the cluster never pulls Redis itself.
- Bad, because the runner's own pull from ECR Public hits the same 429, so this only moves the problem unless the source registry changes too.
- Bad, because the image reference would have to be read out of the chart or kept in sync by hand.

### Authenticate to ECR Public in CI

- Good, because it keeps the chart's default registry.
- Bad, because it needs AWS credentials in CI for a public image, and fork PRs would not have them.

### Re-run failed jobs

- Bad, because it keeps failing `main` and Renovate PRs at random and hides real e2e failures among the flakes.

## More Information

- The `Dump cluster state on failure` step in `.github/workflows/ci.yaml` is what made this visible. Keep it.
- If a future chart moves Redis to an image without a Docker Hub equivalent, revisit this ADR rather than dropping the override silently.
