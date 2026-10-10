# ==============================================================================
# FLEDGER DUNNING — Production Dockerfile
# Service: fledger-dunning (:8086)
# ==============================================================================

# --- Stage 1: Build ---
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/server ./cmd/server

# --- Stage 2: Runtime ---
FROM gcr.io/distroless/static-debian12:nonroot

ENV TZ=UTC
USER nonroot:nonroot
WORKDIR /app

COPY --from=builder /out/server /app/server

EXPOSE 8086
ENTRYPOINT ["/app/server"]
