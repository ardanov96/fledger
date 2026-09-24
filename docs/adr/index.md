# Architecture Decision Records (ADRs)

> ADRs mendokumentasikan keputusan arsitektur yang signifikan — apa yang dipilih, apa alternatifnya, dan kenapa.
>
> **Format**: setiap ADR mengikuti template [Michael Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) dengan tambahan Sprint attribution & status lifecycle.

---

## 📋 ADR Index

| # | Title | Status | Sprint | Tags |
|---|---|---|---|---|
| [0001](0001-go-as-backend-language.md) | Go as backend language | ✅ Accepted | 1 | language, backend, hiring |
| [0002](0002-sqlc-over-orm.md) | sqlc over ORM | ✅ Accepted (not yet adopted) | 1 (planned) | data-access, sql-first |
| [0003](0003-double-entry-ledger.md) | Double-entry ledger with immutable entries | ✅ Accepted | 2-4 | accounting, ledger, immutability |
| [0004](0004-locking-strategy.md) | Locking strategy (SELECT FOR UPDATE + deterministic ordering) | ✅ Accepted | 7 | concurrency, postgres, locking |
| [0005](0005-multi-currency-strategy.md) | Multi-currency strategy (FX rate snapshot) | ✅ Accepted | 12 | currency, fx, accounting |
| [0006](0006-tenant-rls-strategy.md) | Tenant Row-Level Security strategy | ✅ Accepted | 15 | multi-tenancy, rls, security |
| [0007](0007-app-admin-rls-bypass.md) | `app_admin` role for RLS bypass | ✅ Accepted | 15 follow-up / 22A.4 | rls, ops, security |
| [0008](0008-sprint-22b-hardening-roadmap.md) | Sprint 22B hardening items & roadmap | ✅ Accepted | 22B | roadmap, hardening, sprint-planning |

---

## 🎯 Cara Pakai ADRs

**Untuk contributor baru:**
1. Baca ADR sebelum implement fitur yang menyentuh keputusan di bawah
2. Kalau merasa ada keputusan yang salah, **jangan langsung refactor** — buka ADR baru (propose) atau ADR-000X superseded
3. Update status field kalau keputusan berubah (Accepted → Deprecated → Superseded by ADR-NNNN)

**Untuk interview prep:**
- [q-distributed-systems.md](../interview/q-distributed-systems.md#q2-rls-vs-application-layer-tenant-filter--why-both) — grounded di ADR-0006
- [q-fintech.md](../interview/q-fintech.md#q1-why-double-entry-accounting-vs-single-entry) — grounded di ADR-0003
- [q-security.md](../interview/q-security.md#q5-rls-bypass-scenarios-eg-super_admin) — grounded di ADR-0007

**Sprint attribution** — setiap ADR terkait dengan sprint yang memperkenalkan keputusan tersebut. Lihat [Sprint Log](../SPRINTS.md) untuk timeline lengkap.

---

## 📐 ADR Lifecycle

| Status | Meaning |
|---|---|
| **Proposed** | Diskusi terbuka, belum final |
| **Accepted** | Keputusan final, implement aktif |
| **Deprecated** | Tidak relevan lagi tapi belum di-supersede |
| **Superseded** | Digantikan ADR lain (lihat ADR referensi) |
| **Rejected** | Diusulkan tapi ditolak (dokumentasi untuk historical record) |

---

## 🔮 Future ADR Candidates

Berikut topik yang **layak** jadi ADR tapi belum ditulis:

| Topik | Rationale | Related Sprint |
|---|---|---|
| Int64 minor units money type | `internal/platform/money/money.go` punya design decisions worth documenting | Sprint 2-4 (foundation) |
| Hash chain for tamper detection | Lebih detail dari overview di ADR-0003 | Sprint 8 |
| Period close accounting cycle | Lebih detail dari mention di ADR-0003 | Sprint 9 |
| Idempotency-Key TTL & storage | Stripe-style pattern dengan trade-offs | Sprint 6 |
| Background worker ticker pattern | Reconciler ticker vs job queue trade-off | Sprint 10 |
| Single Fly.io multi-process deployment | Alpine + supervisord trade-offs vs multi-container | Sprint 19 |
| W3C traceparent vs OpenTelemetry | Custom impl vs vendor SDK | Sprint 18 (decision deferred to Sprint 23.4) |

**Kapan menulis ADR baru:**
- Keputusan arsitektur yang **tidak reversible** tanpa effort besar
- Pilihan antara **≥2 alternatif** dengan trade-offs non-obvious
- Keputusan yang **bertahan** untuk >1 sprint

**Kapan TIDAK menulis ADR:**
- Detail implementasi minor (naming convention, dst — sudah di [CONTRIBUTING.md](../../CONTRIBUTING.md))
- Perpustakaan/tooling choice yang reversible (bisa di-replace dengan effort kecil)
- Keputusan yang sudah di-cover ADR lain (jangan duplikat)

---

## 📚 Related Documentation

- [Architecture Overview](../architecture/overview.md) — high-level system context
- [C4 Diagrams](../architecture/c4-diagrams.md) — static architecture views
- [Sequence Flows](../architecture/sequences.md) — critical user journeys
- [Sprint Log](../SPRINTS.md) — delivery timeline dengan ADR attribution
- [Domain Glossary](../domain/glossary.md) — business & technical terms

---

**Last updated:** Sprint 23 — Tech Debt Foundation (2026-09-20)
