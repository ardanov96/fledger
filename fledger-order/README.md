# Fledger Order — B2B Order Management System & Hard Credit Gate

> **Service**: `fledger-order`
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)
> **Port**: `:8085`
> **Status**: ✅ **Production-Ready (Sprint 1–5 selesai, 100% test pass)**
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router + CORS, embedded PWA, no npm

---

## 🎯 Tanggung Jawab Layanan

1. **Katalog Produk FMCG & Multi-Tier Pricing** — Mengelola SKU dan harga bertingkat (`GROSIR`, `SEMI_GROSIR`, `RETAIL`, `STAR_OUTLET`) dengan volume discount otomatis.
2. **Alokasi Stok Gudang (*Stock Reservation*)** — Mengunci stok fisik (`reserved_qty`) saat pesanan dibuat; tolak jika `available_qty < requested_qty`.
3. **Hard Credit Gate (Anti-Credit Blindness)** — Pre-flight check ke **Fledger Core** (`:8081`) dengan 2 aturan ketat:
   - **Aturan 1 (Plafon)**: `Outstanding AR + new_order <= credit_limit`
   - **Aturan 2 (Aging)**: Tolak jika ada faktur jatuh tempo `> 30 hari`
   - Gagal → `CREDIT_BLOCKED`; hanya bisa dibuka via **Managerial Override** ber-PIN.
4. **Dispatching Bridge ke Fledger Fleet** — Begitu order `APPROVED`, enqueue event `order.dispatched_fleet` ke outbox → background worker panggil **Fledger Fleet** (`:8082`) `POST /v1/fleet/delivery-orders` dengan total tonase.
5. **PWA B2B Ordering Portal** — Embedded Vanilla JS catalog + keranjang + Credit Gate interaktif (zero npm).

---

## 📅 Sprint Progress

| Sprint | Status | Highlight |
|---|---|---|
| 1 — Pondasi DB & Health | ✅ | 8 tabel + `order_schema_migrations`; `/healthz` + `/readyz` (DB ping) |
| 2 — Katalog + Tier + Stok | ✅ | Volume discount (10 dus @ 115k vs 1 dus @ 120k) + atomic `reserved_qty` lock |
| 3 — Order + Hard Credit Gate | ✅ | 2-rule check ke Core, override PIN, audit trail |
| 4 — Fleet Dispatch Outbox | ✅ | Transaksional outbox + Idempotency-Key + exponential backoff |
| 5 — PWA + E2E | ✅ | Embedded portal + `e2e-flow.ps1` 8/8 steps PASS |

---

## 🤖 Panduan untuk AI Coding Agent / Developer

Baca dokumen di folder `docs/`:
- 📄 [AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md) — Blueprint eksekusi.
- 📄 [ROADMAP-ORDER.md](docs/ROADMAP-ORDER.md) — Rincian 5 sprint + DoD.
- 📄 [DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql) — DDL lengkap.
- 📄 [API-SPECIFICATION.md](docs/API-SPECIFICATION.md) — Kontrak REST.
- 📄 [HARD-CREDIT-GATE-AND-FLEET-BRIDGE-GUIDE.md](docs/HARD-CREDIT-GATE-AND-FLEET-BRIDGE-GUIDE.md) — Logika gate + dispatch.

---

## ⚙️ Konfigurasi `.env`

```ini
APP_ENV=development
PORT=8085
DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_order?sslmode=disable
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
FLEDGER_FLEET_URL=http://localhost:8082
JWT_SECRET=super-secret-fledger-order-jwt-key-minimum-32-chars!
MANAGER_OVERRIDE_PIN=992144
OUTBOX_POLL_INTERVAL=3s
OUTBOX_MAX_ATTEMPTS=10
```

---

## 🚀 Quick Start

```powershell
# 1. Setup database
psql -U fmcg -d fledger_order -f src/migrations/000001_init_order.sql

# 2. Run unit + integration tests
.\src\scripts\run-tests.ps1

# 3. Jalankan API server
.\src\scripts\start-api.cmd

# 4. (terminal lain) E2E smoke test
.\src\scripts\e2e-flow.ps1

# 5. Buka PWA
#    http://localhost:8085/
```

---

## 📁 Struktur Folder

```text
fledger-order/
├── docs/                                 <-- Specs, schema, roadmap
├── src/
│   ├── cmd/
│   │   ├── api/                          <-- HTTP server
│   │   └── migrator/                     <-- SQL migrator CLI
│   ├── internal/
│   │   ├── auth/jwt/                     <-- HS256 sign/verify
│   │   ├── config/                       <-- env loader
│   │   ├── domain/                       <-- product, pricing, inventory, order, settlement, audit, outbox
│   │   ├── handler/                      <-- REST handlers
│   │   ├── integration/coreclient/       <-- Fledger Core HTTP client (AR + aging)
│   │   ├── integration/fleetclient/      <-- Fledger Fleet HTTP client (delivery orders)
│   │   ├── middleware/                   <-- RequireAuth
│   │   ├── platform/                     <-- errors, httpx, log, credit (Hard Credit Gate engine)
│   │   ├── repository/postgres/          <-- pgx-based repos
│   │   ├── usecase/                      <-- business services + outbox worker
│   │   └── webui/                        <-- embedded static PWA
│   ├── migrations/
│   │   └── 000001_init_order.sql
│   ├── go.mod / go.sum
│   └── scripts/                          <-- helper scripts (cmd/ps1)
├── web/                                  <-- source for PWA
│   ├── index.html
│   ├── style.css
│   ├── order-client.js
│   └── app.js
├── .env.example
└── README.md
```

---

## 🧪 Pengujian

| Test | Lokasi | Status |
|---|---|---|
| Hard Credit Gate: `IsOrderExpired` helper | `internal/platform/credit/evaluator_test.go` | ✅ PASS |
| Happy path: create order → credit PASS → dispatch to Fleet | `internal/integration/integration_test.go` | ✅ PASS |
| Credit Gate: overdue blocks | same | ✅ PASS |
| Credit Gate: limit exceeded blocks | same | ✅ PASS |
| Override: valid PIN unlocks | same | ✅ PASS |
| Override: bad PIN rejected (401) | same | ✅ PASS |
| Stock reservation: insufficient stock rejected | same | ✅ PASS |
| Order number format `ORD-YYYYMM-XXXXX` | same | ✅ PASS |
| E2E (live HTTP) | `scripts/e2e-flow.ps1` | ✅ PASS |

```
==> internal/platform/credit  [OK]
==> internal/integration      [OK]
```

---

## 🛡 Guardrails Terpenuhi

- ✅ **Integritas uang**: semua nominal `BIGINT` minor units (Rp integer, tanpa float).
- ✅ **Stock reservation atomic**: `UPDATE ... WHERE available >= qty` — no overselling.
- ✅ **Hard Credit Gate 2-rule**: plafond + aging > 30d.
- ✅ **Managerial override**: PIN-protected, audit-trail recorded.
- ✅ **Outbox transaksional**: order header + outbox event written together.
- ✅ **Idempotency-Key**: `Idempotency-Key: <order_id>` ke Fleet → no double DO.
- ✅ **CORS terbuka**: `cors.AllowAllOrigins` mounted at root.
- ✅ **Zero secret hardcoding**: all keys from env.

---

## 🔌 Integrasi dengan Fledger Core & Fleet

### Ke Fledger Core (`:8081`) — Hard Credit Gate

```
GET /v1/customers/{id}/ar-summary
GET /v1/customers/{id}/credit-limit
```

### Ke Fledger Fleet (`:8082`) — Dispatching Bridge

```
POST /v1/fleet/delivery-orders
Idempotency-Key: <order_id>

{
  "do_number": "DO-202610-00812",
  "customer_name": "Toko Sumber Rezeki",
  "destination_address": "Jl. Raya Daan Mogot No. 45",
  "total_nominal": 4000000,
  "total_weight_kg": 320,
  "items": [
    {"sku_id": "SKU-OIL-001", "name": "...", "quantity": 10, "unit_price": 120000}
  ]
}
```