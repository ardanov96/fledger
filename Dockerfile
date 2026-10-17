# ==============================================================================
# FLEDGER OS — Master All-in-One Cloud Dockerfile (Rootless & Zero-Card Edition)
# Runs 6 Go microservices + Gateway + PostgreSQL + Redis + NATS in a single image
# Optimized for Hugging Face Spaces (16GB RAM / 0 Cards), Koyeb, and Render
# ==============================================================================

# --- Stage 1: Build All Go Binaries ---
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy entire repository (utilizing go.work)
COPY . .

# Build NATS server
RUN CGO_ENABLED=0 go install github.com/nats-io/nats-server/v2@v2.10.22

# Build all 6 microservices + gateway
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/core ./fledger-core/cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/fleet ./fledger-fleet/src/cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/pay ./fledger-pay/src/cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/force ./fledger-force/src/cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/order ./fledger-order/src/cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/dunning ./fledger-dunning/cmd/server && \
    go build -trimpath -ldflags="-s -w" -o /out/gateway ./deployments/gateway

# --- Stage 2: Production Runtime ---
FROM alpine:3.20

# Install runtime dependencies (PostgreSQL 16, Redis 7, bash, curl)
RUN apk add --no-cache \
    bash \
    curl \
    redis \
    postgresql16 \
    postgresql16-contrib \
    tzdata \
    ca-certificates

ENV TZ=Asia/Jakarta
WORKDIR /app

# Copy binaries
COPY --from=builder /go/bin/nats-server /usr/local/bin/nats-server
COPY --from=builder /out/ /app/bin/

# Copy static web assets
COPY fledger-core/web/public /app/fledger-core/web/public
COPY fledger-core/internal/auth/rbac/policies /app/fledger-core/internal/auth/rbac/policies
COPY fledger-fleet/web /app/fledger-fleet/web
COPY fledger-pay/web /app/fledger-pay/web
COPY fledger-force/web /app/fledger-force/web
COPY fledger-order/web /app/fledger-order/web
COPY deployments/gateway /app/deployments/gateway

# Copy SQL Migrations
COPY fledger-core/migrations /app/migrations/core
COPY fledger-fleet/src/migrations /app/migrations/fleet
COPY fledger-pay/src/migrations /app/migrations/pay
COPY fledger-force/src/migrations /app/migrations/force
COPY fledger-order/src/migrations /app/migrations/order
COPY fledger-dunning/docs/DATABASE-SCHEMA.sql /app/migrations/dunning/000001_init.sql

# Copy Entrypoint & Seeder
COPY deployments/docker/all-in-one/entrypoint.sh /app/entrypoint.sh
COPY deployments/docker/all-in-one/seed.sh /app/seed.sh
RUN chmod +x /app/entrypoint.sh /app/seed.sh /app/bin/*

# Set up standard non-root user (UID 1000) for Hugging Face Spaces compatibility
RUN adduser -D -u 1000 user && \
    mkdir -p /home/user/data /home/user/run && \
    chown -R user:user /home/user /app

USER user
ENV HOME=/home/user

# Default port 7860 (Hugging Face) / 10000 (Render) / 8000
ENV PORT=7860
EXPOSE 7860 10000 8000 80

ENTRYPOINT ["/app/entrypoint.sh"]
