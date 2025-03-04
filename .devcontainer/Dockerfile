# See here for image contents: https://github.com/microsoft/vscode-dev-containers/blob/main/containers/go/.devcontainer/Dockerfile
ARG VARIANT="1"
FROM mcr.microsoft.com/devcontainers/go:1-${VARIANT}

# Install dependencies.
# [Choice] Node.js version: none, lts/*, 16, 14, 12, 10
ARG NODE_VERSION="22"
RUN if [ "${NODE_VERSION}" != "none" ]; then su vscode -c "umask 0002 && . /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1"; fi

# Setup
ENV GOTOOLCHAIN=auto

ENV HOME="/home/vscode"
WORKDIR /home/vscode

# Install standard packages
RUN apt-get update && \
        # General purpose tools
    apt-get install -y \
        # Python
        libsasl2-dev libldap2-dev libssl-dev libsnmp-dev libffi-dev \
        libncurses-dev libsqlite3-dev libbz2-dev libreadline-dev liblzma-dev tzdata tk-dev graphviz;

USER vscode

# Install and configure Python
ENV UV_CACHE_DIR="/home/vscode/.uv_cache"
COPY --from=ghcr.io/astral-sh/uv:debian /usr/local/bin/uv /usr/local/bin
COPY --from=ghcr.io/astral-sh/uv:debian /usr/local/bin/uvx /usr/local/bin
ARG PYTHON_VERSION=3.13
RUN uv python install ${PYTHON_VERSION}
ENV UV_LINK_MODE=copy

# Install tools
# - Tools
RUN sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d
COPY --from=quay.io/argoproj/argocd /usr/local/bin/argocd /usr/local/bin

# - Golang
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
RUN go install github.com/jstemmer/go-junit-report/v2@latest
RUN go install github.com/hairyhenderson/gomplate/v4/cmd/gomplate@latest
RUN go install github.com/goreleaser/goreleaser/v2@latest
RUN go install github.com/air-verse/air@latest
RUN go install github.com/go-swagger/go-swagger/cmd/swagger@latest

# - Kubernetes
RUN curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

COPY --from=bitnami/kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin
COPY --from=alpine/helm /usr/bin/helm /usr/local/bin
COPY --from=derailed/k9s /bin/k9s /usr/local/bin

# Modify dotfile
RUN echo 'export PYENV_ROOT="$HOME/.pyenv"' >> ~/.bashrc
RUN echo 'export PYENV_ROOT="$HOME/.pyenv"' >> ~/.zshrc
RUN echo 'export PATH="$PYENV_ROOT/shims:$PYENV_ROOT/bin:$PATH"' >> ~/.bashrc
RUN echo 'export PATH="$PYENV_ROOT/shims:$PYENV_ROOT/bin:$PATH"' >> ~/.zshrc
RUN echo 'eval "$(pyenv virtualenv-init -)"' >> ~/.bashrc
RUN echo 'eval "$(pyenv virtualenv-init -)"' >> ~/.zshrc
