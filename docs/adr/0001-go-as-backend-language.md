# ADR-0001: Go as the Backend Language

**Status:** Accepted
**Date:** 2026-08-10
**Deciders:** Project lead
**Tags:** language, backend, hiring

## Context

The project is a financial backend (ledger, receivables, reconciliation) for the FMCG/F&B domain. We need a language that:

1. **Matches the job description** — the role we're applying for explicitly requires Go.
2. **Performs well under concurrent load** — many outlets/sales reps will write to the same HQ accounts simultaneously.
3. **Has a strong ecosystem for financial/payments work** — Postgres drivers, JWT, observability, etc.
4. **Compiles to a single static binary** — easy to deploy to a cheap VPS.
5. **Has built-in concurrency primitives** — goroutines, channels, context — for the WebSocket/realtime components.

## Options Considered

| Option | Pros | Cons |
|---|---|---|
| **Go 1.23+** ✅ | Single static binary, excellent concurrency, strong stdlib, hiring alignment, great observability ecosystem | Verbose for some patterns, generics are limited |
| **TypeScript/Node.js** | Frontend team is already on this stack, faster iteration | Weaker typing for financial code, GC pauses can affect tail latency, less common for financial backends |
| **Rust** | Best-in-class performance, memory safety | Slower development velocity, smaller ecosystem for fintech-specific libraries, hiring pool smaller |
| **Java/Kotlin** | Mature ecosystem (Spring, etc.) | Heavier runtime, slower startup, more infra needed |

## Decision

**Use Go 1.23+** for the backend.

## Rationale

- **Hiring alignment**: The role explicitly requires Go experience. Building the portfolio in another language is wasted effort.
- **Concurrency**: Go's goroutines and the standard `sync` package make writing race-free code with `SELECT ... FOR UPDATE` patterns straightforward. Race detector (`go test -race`) catches issues at test time.
- **Static binary**: A single ~20MB distroless image is easy to deploy, version, and roll back.
- **Observability ecosystem**: OpenTelemetry, Prometheus, slog, pgx — all first-class in Go.
- **Compile-time safety**: Strong typing for money types, IDs, and request/response structs catches bugs at build time.

## Consequences

### Positive
- Direct skill demonstration for the target role.
- Fast test runs and CI cycles.
- Excellent tooling: `go vet`, `golangci-lint`, `gofmt` all built-in.
- Memory profile and `pprof` integration for performance work.

### Negative
- More verbose than TypeScript for some patterns (e.g. error handling).
- ORM ecosystem is weaker than Java/Python (mitigated by using `sqlc`).
- Smaller pool of contributors if we ever open-source.

## Implementation Notes

- **Linting**: `golangci-lint` with strict config (see `.golangci.yml`).
- **Go version**: Minimum 1.23; project built and tested with 1.23+.
- **Style**: Standard Go project layout (`cmd/`, `internal/`, `pkg/`).
- **Concurrency safety**: All shared state must be safe under `-race`. Document any intentional non-thread-safe APIs.

## Follow-ups

- ADR-0002: Why sqlc over GORM
- ADR-0003: Why double-entry ledger
- ADR-0004: Why SELECT FOR UPDATE for balance updates
- ADR-0010: Why int64 minor units for money

## References

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [100 Go Mistakes and How to Avoid Them](https://100go.co/)
