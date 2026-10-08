# Contributing to FMCG Wallet

> Solo project untuk portfolio, jadi "contributor" di sini = diri sendiri plus interviewer yang penasaran. Tapi prinsip-prinsip ini akan di-mirror kalau project ini di-open-source someday.

## 🎯 Mission

Membangun **production-grade financial backend** yang defensible di interview dan siap deploy ke production real. Bukan MVP, bukan prototype — proper engineering.

## 🛠️ Development Workflow

### Daily Routine

1. **Pull latest** — `git pull origin main`
2. **Pick a task** — dari milestone current (lihat `docs/interview/` atau project board)
3. **Create branch** — `git checkout -b feat/<short-name>` atau `fix/<short-name>`
4. **Code + test** — tulis test dulu kalau bisa (TDD-ish)
5. **Verify locally** — `make verify` (fmt + vet + lint + test + coverage)
6. **Commit** — conventional commits (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`)
7. **Push & open PR** — kalau di GitHub

### Weekly Routine

- **Senin pagi** (30 min): review minggu lalu, set target minggu ini
- **Rabu malam** (15 min): mid-week check, adjust kalau off-track
- **Jumat sore** (30 min): demo kecil / self-review, tulis ADR kalau ada keputusan baru
- **Minggu** (15 min): retro — apa yang bisa diperbaiki

---

## ✅ Definition of Done (DoD)

Task dianggap selesai kalau SEMUA ini terpenuhi:

- [ ] Kode ditulis & diformat (`make fmt`)
- [ ] Lolos `make lint` (golangci-lint strict, no warnings)
- [ ] Lolos `make vet`
- [ ] Ada unit test untuk logic baru (coverage min 80% di package terkait)
- [ ] Ada integration test kalau涉及 DB / external service
- [ ] OpenAPI doc di-update (kalau endpoint baru)
- [ ] ADR ditulis untuk keputusan arsitektur baru
- [ ] Commit message conventional (`feat:`, `fix:`, dll)
- [ ] CI green (lint + test + security + build)
- [ ] Self-review pass (kayak jadi reviewer orang lain)
- [ ] Update `docs/interview/` kalau ada Q&A baru yang muncul

---

## 📐 Coding Standards

### Style

- **gofmt** — non-negotiable, di-apply via `make fmt`
- **Effective Go** — [https://go.dev/doc/effective_go](https://go.dev/doc/effective_go)
- **Go Code Review Comments** — [github.com/golang/go/wiki/CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)
- **Function length** — max 50 statements (enforced by `funlen` linter)
- **Cyclomatic complexity** — max 15 (enforced by `gocyclo`)

### Naming

- **Packages** — lowercase, single word, no underscores (`money`, `ledger`, `httpx`)
- **Types** — PascalCase, exported if public (`Money`, `AccountService`)
- **Functions** — PascalCase exported, camelCase unexported
- **Variables** — camelCase, avoid single-letter unless in tight scope (`i`, `j` di loop OK)
- **Constants** — PascalCase, or SCREAMING_SNAKE_CASE untuk enum-like
- **Errors** — prefix `Err` (`ErrInsufficientBalance`)
- **Interfaces** — single-method interface = method name + `er` (e.g. `Reader`)

### Errors

```go
// ✅ Good — typed, wrapped, contextual
if err != nil {
    return fmt.Errorf("load account %s: %w", id, ledger.ErrAccountNotFound)
}

// ❌ Bad — bare error, no context
if err != nil {
    return err
}
```

Selalu gunakan sentinel errors dari `internal/platform/errors` atau `internal/domain/ledger`. Custom error inline hanya untuk kasus yang tidak ter-cover.

### Logging

```go
// ✅ Good — structured, dengan context
log.Info("transfer completed",
    "transaction_id", txID,
    "from", fromID,
    "to", toID,
    "amount", amount.String(),
)

// ❌ Bad — string formatting, tidak searchable
log.Printf("Transfer %s from %s to %s done", txID, fromID, toID)
```

### Testing

- **Test files** — `*_test.go` di package yang sama
- **Naming** — `TestXxx` untuk unit, `TestXxx_Integration` untuk integration
- **Table-driven** — dipakai untuk test dengan multiple cases
- **Subtests** — `t.Run("case name", ...)` untuk grouping
- **Parallel** — `t.Parallel()` di awal test kalau independent
- **Race** — selalu jalankan `go test -race` (default di `make test`)

```go
func TestTransfer(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name    string
        input   TransferInput
        wantErr error
    }{
        {"happy path", validInput, nil},
        {"insufficient balance", poorInput, ledger.ErrInsufficientBalance},
    }
    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // ... test body
        })
    }
}
```

### SQL

- **snake_case** untuk nama tabel & kolom (`cached_balance`, `idempotency_key`)
- **Plural** untuk nama tabel (`accounts`, `entries`, `transactions`)
- **Singular** untuk nama kolom (`status`, `amount`, `created_at`)
- **UUID** untuk primary key (`gen_random_uuid()` dari pgcrypto)
- **TIMESTAMPTZ** untuk semua waktu (bukan TIMESTAMP)
- **BIGINT** untuk money (minor units)
- **TEXT** untuk string panjang, **VARCHAR(N)** hanya kalau ada batasan eksplisit

### Migrations

- **Format** — `<version>_<description>.up.sql` dan `.down.sql`
- **Always reversible** — setiap up harus punya down
- **Backward-compatible first** — expand-then-contract pattern untuk schema change besar
- **Tested** — apply + rollback di staging dulu
- **No data loss** — `DROP COLUMN` setelah deprecation period

### Git

- **Conventional commits** — `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`, `perf:`
- **Imperative mood** — "add" bukan "added", "fix" bukan "fixed"
- **Lowercase** subject line
- **No period** at end of subject
- **72 chars** max subject line
- **Body** wrap at 100 chars, explain WHAT and WHY (not HOW)

```
feat(ledger): add trial balance reconciler

Adds background job that runs hourly to verify
SUM(debit) == SUM(credit) for the entire ledger.
Alerts if imbalance detected.

Refs: Fase 1B in roadmap
```

---

## 🧪 Testing Strategy

### Pyramid

```
        /\
       /  \      E2E (few, slow, high-fidelity)
      /----\
     /      \    Integration (moderate, real deps via testcontainers)
    /--------\
   /          \  Unit (many, fast, no deps)
  /------------\
```

### Coverage Targets

- **Unit**: 80% minimum (enforced by CI)
- **Integration**: 70% (DB & external services)
- **E2E**: smoke tests for critical paths only

### Test Categories

| Type | Tool | When |
|---|---|---|
| Unit | `go test` + `testify` | All pure logic |
| Integration | `testcontainers-go` | DB queries, migrations |
| Property-based | `gopter` | Invariants (Fase 7A) |
| Chaos | `toxiproxy` via testcontainers | Resilience (Fase 7B) |
| Load | `k6` | Performance (Fase 7C) |
| Mutation | `go-mutesting` | Test strength (Fase 7D) |
| Security | `govulncheck`, `gosec`, `OWASP ZAP` | Vuln scanning |

---

## 🔒 Security

- **Never commit secrets** — `.env` di-`.gitignore`, gunakan GitHub Secrets untuk CI
- **PII encryption** — kolom sensitif di-encrypt dengan pgcrypto
- **Audit log** — semua perubahan data penting di-log ke `audit_logs`
- **Rate limit** — aktif di semua endpoint (Redis token bucket)
- **JWT secret** — min 32 chars, di-rotate quarterly
- **HTTPS only** — di production, HSTS enabled

---

## 📦 Release Process

### Versioning

- **Semantic versioning** — `MAJOR.MINOR.PATCH`
- **MAJOR** — breaking API change
- **MINOR** — new feature, backward-compatible
- **PATCH** — bug fix, backward-compatible

### Cadence

- Tidak ada release schedule tetap — release saat milestone selesai
- Tag di main setelah merge
- Release notes di GitHub Releases
- Docker image di-push ke GHCR otomatis via CI

---

## 🆘 When Stuck

1. **Cek ADR** — `docs/adr/` mungkin sudah jawab pertanyaanmu
2. **Cek glossary** — `docs/domain/glossary.md` untuk istilah bisnis
3. **Cek roadmap** — `roadmap-production-grade.md` di parent dir untuk konteks fase
4. **Cek interview notes** — `docs/interview/` untuk pertanyaan yang sering muncul
5. **Cek tests** — test cases sering jadi specification hidup
6. **Cek CI** — kalau merah, baca log step-by-step
7. **Cek git history** — `git log --oneline` di file terkait

Kalau masih stuck lebih dari 30 menit, **tulis pertanyaanmu di `docs/questions.md`** dan move on. Besok pagi, jawab sendiri dengan kepala dingin.

---

## 📚 Referensi

- [Effective Go](https://go.dev/doc/effective_go)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Google Go Style Guide](https://google.github.io/styleguide/go/)
- [PostgreSQL Docs](https://www.postgresql.org/docs/)
- [Domain-Driven Design (Eric Evans)](https://www.domainlanguage.com/ddd/)
- [Designing Data-Intensive Applications](https://dataintensive.net/)
