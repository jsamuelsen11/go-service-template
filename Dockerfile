# syntax=docker/dockerfile:1.7

# ============================================================================
# Stage: build - Compile the application
# ============================================================================
FROM golang:1.25-alpine AS build

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /app

# Copy go.mod first for better layer caching
COPY go.mod go.sum* ./

# Download dependencies (cached by Docker layer)
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source
COPY . .

# Build binary with cache mounts for faster rebuilds
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/service \
    ./cmd/service

# ============================================================================
# Stage: test - Run tests (optional, for CI)
# ============================================================================
FROM golang:1.25-alpine AS test

WORKDIR /app

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test -race -v ./...

# ============================================================================
# Stage: production - Minimal runtime image
# ============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS production

# OCI Image Spec labels
LABEL org.opencontainers.image.title="go-service-template" \
      org.opencontainers.image.description="Enterprise-grade Go backend service template" \
      org.opencontainers.image.url="https://github.com/jsamuelsen/go-service-template" \
      org.opencontainers.image.source="https://github.com/jsamuelsen/go-service-template" \
      org.opencontainers.image.vendor="jsamuelsen" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.base.name="gcr.io/distroless/static-debian12:nonroot"

# Build-time labels
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}"

# Copy binary
COPY --link --from=build /app/service /service

# Copy config files
COPY --link --from=build /app/configs /configs

# Non-root user (uid 65532)
USER nonroot:nonroot

EXPOSE 8080

# Note: HEALTHCHECK removed - let orchestrator (K8s) handle health probes
# The service exposes /-/live and /-/ready endpoints for liveness/readiness

ENTRYPOINT ["/service"]
