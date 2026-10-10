# ==============================================================================
# FLEDGER FLEET — Production Dockerfile
# Service: fledger-fleet (:8082)
# ==============================================================================

# --- Stage 1: Build ---
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./
COPY web/ ./web/

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/api ./cmd/api

# --- Stage 2: Runtime ---
FROM gcr.io/distroless/static-debian12:nonroot

ENV TZ=UTC
USER nonroot:nonroot
WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /app/web /app/web

EXPOSE 8082
ENTRYPOINT ["/app/api"]
