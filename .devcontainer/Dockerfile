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
ARG K3D_VERSION=v5.9.0 # github-releases/k3d-io/k3d
RUN curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=${K3D_VERSION} bash

COPY --from=rancher/kubectl:v1.32.5 /bin/kubectl /usr/local/bin/kubectl
COPY --from=alpine/helm:4.3.0 /usr/bin/helm /usr/local/bin
COPY --from=derailed/k9s:v0.50.18 /bin/k9s /usr/local/bin
COPY --from=quay.io/argoproj/argocd:v3.5.3 /usr/local/bin/argocd /usr/local/bin

# Playwright (for web/ e2e tests and vitest-browser-svelte component tests) —
# keep the version in sync with playwright/@playwright/test in web/package.json.
# Deliberately not `playwright install --with-deps` — that fails on this Debian
# base (ttf-ubuntu-font-family/ttf-unifont aren't in its apt sources). Without a
# font package installed, Chromium renders real content but all text is
# genuinely zero-height/invisible — looks exactly like a CSS bug.
ARG PLAYWRIGHT_VERSION=1.63.0 # npm/playwright
RUN npx --yes playwright@${PLAYWRIGHT_VERSION} install chromium chromium-headless-shell && \
    apt-get update && apt-get install -y --no-install-recommends \
        libglib2.0-0 libnss3 libnspr4 libdbus-1-3 libatk1.0-0 \
        libatk-bridge2.0-0 libatspi2.0-0 libx11-6 libxcomposite1 \
        libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 \
        libxkbcommon0 libasound2 libcups2 libpango-1.0-0 libcairo2 \
        fonts-liberation fonts-dejavu-core && \
    rm -rf /var/lib/apt/lists/*

USER vscode

# Python (kept for mkdocs local preview)
ENV UV_CACHE_DIR="/home/vscode/.uv_cache"
COPY --from=ghcr.io/astral-sh/uv:debian /usr/local/bin/uv /usr/local/bin/uv
COPY --from=ghcr.io/astral-sh/uv:debian /usr/local/bin/uvx /usr/local/bin/uvx
ARG PYTHON_VERSION=3.13
RUN uv python install ${PYTHON_VERSION}
ENV UV_LINK_MODE=copy

# Go extras not in the central go image
RUN go install github.com/air-verse/air@v1.67.4 && \
    go install github.com/go-swagger/go-swagger/cmd/swagger@v0.36.6
