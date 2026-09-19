ARG NODE_VERSION=24
FROM node:${NODE_VERSION} AS node

FROM ghcr.io/ivanklee86/devcontainer/go:1.27

USER root

# Node (for web/ SvelteKit build)
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/npm
RUN ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

# Kubernetes / ArgoCD tooling (tangle-specific, not centralized)
ARG K3D_VERSION=v5.9.0
RUN curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=${K3D_VERSION} bash

COPY --from=rancher/kubectl:v1.32.5 /bin/kubectl /usr/local/bin/kubectl
COPY --from=alpine/helm /usr/bin/helm /usr/local/bin
COPY --from=derailed/k9s /bin/k9s /usr/local/bin
COPY --from=quay.io/argoproj/argocd:v3.5.3 /usr/local/bin/argocd /usr/local/bin

USER vscode

# Python (kept for mkdocs local preview)
ENV UV_CACHE_DIR="/home/vscode/.uv_cache"
COPY --from=ghcr.io/astral-sh/uv:debian /usr/local/bin/uv /usr/local/bin/uv
COPY --from=ghcr.io/astral-sh/uv:debian /usr/local/bin/uvx /usr/local/bin/uvx
ARG PYTHON_VERSION=3.13
RUN uv python install ${PYTHON_VERSION}
ENV UV_LINK_MODE=copy

# Go extras not in the central go image
RUN go install github.com/air-verse/air@latest && \
    go install github.com/go-swagger/go-swagger/cmd/swagger@latest
