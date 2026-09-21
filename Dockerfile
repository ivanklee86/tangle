# Stage 1: Build Go server
#
# Pinned to $BUILDPLATFORM (the runner's own architecture), not the target's.
# This stage both cross-compiles the server and *executes* a tool it just
# built (`swagger`, below), so it has to run natively. Nothing in this repo
# uses cgo, so GOOS/GOARCH cross-compilation is exact rather than a best
# effort.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS go

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download -x
RUN go install github.com/go-swagger/go-swagger/cmd/swagger@v0.36.6
COPY . .
RUN swagger generate spec -o ./internal/tangle/swagger.json --scan-models

# Declared here rather than at the top of the stage on purpose: buildx
# invalidates cache from the point an ARG is *used*, so keeping these below
# the dependency download, tool install, and swagger generation lets both
# platform legs share those layers verbatim instead of building them twice.
ARG TARGETOS
ARG TARGETARCH

# GOOS/GOARCH are set on this one command, deliberately not as a stage-wide
# ENV — the `go install` above has to produce a binary this stage can run,
# and a stage-wide GOARCH would cross-compile `swagger` and then fail to
# execute it. CGO_ENABLED=0 is belt-and-braces: there's no cgo to disable,
# but it makes a future cgo dependency fail here rather than silently
# producing a dynamically-linked binary alpine can't run.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -v -ldflags "-X main.version=docker" -o . ./...

# Stage 2: Build web assets
#
# Also $BUILDPLATFORM: `npm run build` emits static assets that the runtime
# image serves, never runs, so the output is identical whichever architecture
# produced it. Emulating this stage would cost minutes for a byte-identical
# result.
FROM --platform=$BUILDPLATFORM node:24-alpine AS node

WORKDIR /app
COPY ./web .
RUN npm install
RUN npm run build

# Stage 3 - Runtime Image
#
# No --platform pin: this one resolves per target, which is the whole point.
# It is also the only stage with a RUN that executes on the target platform
# (the adduser below), so it's the only reason a *cross*-build needs QEMU —
# see the `Set up QEMU` steps in ci.yaml's `image` job and release.yaml. A
# native arm64 build (ci.yaml's `e2e` arm64 leg) needs none.
FROM alpine:latest AS runtime

RUN addgroup -S tangle && adduser -S tangle -G tangle

WORKDIR /app

COPY --from=go --chown=tangle:tangle /app/tangle-server tangle-server
COPY --from=go --chown=tangle:tangle /app/tangle-cli /usr/bin/tangle-cli
COPY --from=node --chown=tangle:tangle /app/build ./build

USER tangle

ENTRYPOINT [ "/app/tangle-server" ]
