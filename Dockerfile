# syntax=docker/dockerfile:1.10

ARG ALPINE_VERSION=3.23
ARG NODE_VERSION=24.12.0
ARG GO_VERSION=1.26.2

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine${ALPINE_VERSION} AS frontend-builder
WORKDIR /app
RUN corepack enable
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend /app
RUN pnpm exec vite build --outDir /app/dist

# Build app
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder
WORKDIR /app
ARG TARGETOS
ARG TARGETARCH
RUN apk add --no-cache ca-certificates
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN rm -rf /app/internal/router/dist
COPY --from=frontend-builder /app/dist /app/internal/router/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/rest-geoip .

# dev docker image
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS dev
RUN go install github.com/air-verse/air@v1.63.6
EXPOSE 1323
WORKDIR /app

# Main docker image
FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/rest-geoip /usr/bin/rest-geoip
ENV GOIP_PROGRAM__RELEASE_MODE=true
EXPOSE 1323
CMD ["/usr/bin/rest-geoip"]
