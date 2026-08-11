# =============================================================================
# FMCG Wallet — Multi-stage Dockerfile
# Produces a small distroless image (~20MB) with all binaries.
# =============================================================================

# --- Stage 1: Build ---
FROM golang:1.23-alpine AS builder

# Build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Caching layer
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Source
COPY . .

# Build all binaries with reproducible flags
# -trimpath removes absolute paths from binary
# -ldflags -s -w strips debug info
ARG VERSION=dev
ARG BUILD_TIME
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags="-s -w" \
    -o /out/migrator ./cmd/migrator

# --- Stage 2: Runtime ---
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/runut/fmcg-wallet"
LABEL org.opencontainers.image.title="fmcg-wallet"
LABEL org.opencontainers.image.description="FMCG/F&B Hybrid Wallet — production-grade backend"
LABEL org.opencontainers.image.licenses="MIT"

# Use UTC timezone by default (override with TZ env)
ENV TZ=UTC

# Distroless runs as nonroot (uid 65532)
USER nonroot:nonroot
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /out/api /app/api
COPY --from=builder /out/worker /app/worker
COPY --from=builder /out/migrator /app/migrator

# Default command runs the API; override for worker/migrator
ENTRYPOINT ["/app/api"]

# Health check via HTTP probe (k8s/Docker HEALTHCHECK)
# Distroless lacks curl/wget; healthcheck handled by orchestrator
EXPOSE 8080
