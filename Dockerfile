# Stage 1: Build Go server
FROM golang:1.27-alpine AS go

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download -x
RUN go install github.com/go-swagger/go-swagger/cmd/swagger@v0.33.1
COPY . .
RUN swagger generate spec -o ./internal/tangle/swagger.json --scan-models
RUN go build -v -ldflags "-X main.version=docker" -o . ./...

# Stage 2: Build web assets
FROM node:24-alpine AS node

WORKDIR /app
COPY ./web .
RUN npm install
RUN npm run build

# Stage 3 - Runtime Image
FROM alpine:latest AS runtime

RUN addgroup -S tangle && adduser -S tangle -G tangle

WORKDIR /app

COPY --from=go --chown=tangle:tangle /app/tangle-server tangle-server
COPY --from=go --chown=tangle:tangle /app/tangle-cli /usr/bin/tangle-cli
COPY --from=node --chown=tangle:tangle /app/build ./build

USER tangle

ENTRYPOINT [ "/app/tangle-server" ]
