# Stage 1: Build Go server
FROM golang:1.23-alpine AS go

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN go build -v -ldflags "-X main.version=docker" -o . ./...

# Stage 2: Build web assets
FROM node:22-alpine AS node

WORKDIR /app
COPY . .
WORKDIR /app/web
RUN npm install
RUN npm run build

# Stage 3 - Runtime Image
FROM alpine:latest as runtime

WORKDIR /app

COPY --from=go /app/tangle-server tangle-server
COPY --from=node /app/build ./build

ENTRYPOINT [ "/app/tangle-server" ]
