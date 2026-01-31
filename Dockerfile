# syntax=docker/dockerfile:1.7

# ============================================================================
# Stage: deps - Download and cache dependencies
# ============================================================================
FROM golang:1.25-alpine AS deps

WORKDIR /app

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

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

# Copy cached dependencies
COPY --from=deps /go/pkg/mod /go/pkg/mod

# Copy source
COPY . .

# Build binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${GIT_COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /app/service \
    ./cmd/service

# ============================================================================
# Stage: test - Run tests (optional, for CI)
# ============================================================================
FROM golang:1.25-alpine AS test

WORKDIR /app

COPY --from=deps /go/pkg/mod /go/pkg/mod
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

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/service", "healthcheck"]

ENTRYPOINT ["/service"]
