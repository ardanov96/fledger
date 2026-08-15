# FMCG Wallet Web Dashboard

**Sprint 20 — Frontend Dashboard MVP** (Portfolio Sprint 3 / Fase 8 frontend)

Lightweight web UI untuk consume FMCG Wallet backend API. Built sebagai static SPA tanpa framework dependency — vanilla JS + vanilla CSS + Node.js dev server.

## Why No Framework?

- **Portfolio demo, not production SPA** — needs to be readable + understandable by interviewer dalam <5 menit
- **Zero npm dependencies** — tidak ada `node_modules` 500MB, tidak ada supply chain risk
- **Minimal attack surface** — pure vanilla JS, tidak ada framework CVE
- **Production swap-ready** — untuk production-grade SPA, swap ke Next.js / Nuxt / SvelteKit (effort ~1 minggu)

## Quick Start

### 1. Install (no npm deps needed!)

```bash
# Zero dependencies — just Node.js 18+
node --version  # ensure 18+
```

### 2. Start backend (Fly.io demo atau local)

```bash
# Option A: Fly.io demo (recommended)
# https://fmcg-wallet-demo.fly.dev

# Option B: Local backend (Sprint 19 Docker)
docker compose up postgres redis
DATABASE_URL=... go run ./cmd/migrator up
API_BASE_URL= go run ./cmd/api
```

### 3. Start web dashboard

```bash
# Default: proxies to http://localhost:8080
node web/server.js

# Or with explicit backend URL
PORT=3000 API_BASE_URL=https://fmcg-wallet-demo.fly.dev node web/server.js
```

### 4. Open browser

```
http://localhost:3000
```

Login dengan demo credentials:
- `admin@demo.fmcg-wallet` / `demo123` (hq_admin)
- `sales@demo.fmcg-wallet` / `demo123` (sales_rep)

## Architecture

```
┌──────────────────────────────────────────┐
│ Browser (http://localhost:3000)          │
│   ↓ HTTP                                │
│   ↓ static SPA files + /v1/* proxy      │
│                                          │
│ web/server.js (Node.js, ~120 lines)     │
│   ├── static file server                 │
│   │     serves web/public/*              │
│   └── reverse proxy → /v1/*             │
│         forwards to API_BASE_URL          │
└──────────────────────────────────────────┘
                  ↓
┌──────────────────────────────────────────┐
│ Backend API (Fly.io demo or local)       │
│   - cmd/api/main.go (Go)                 │
│   - Postgres 16                          │
│   - 36 endpoints across 9 modules        │
└──────────────────────────────────────────┘
```

## File Structure

```
web/
├── package.json          # Project metadata (no deps)
├── server.js             # Node static + proxy server (~120 lines)
├── README.md             # This file
├── .gitignore            # node_modules, etc.
└── public/               # SPA files
    ├── index.html         # SPA shell (login + 9 views)
    ├── css/
    │   └── styles.css     # Vanilla CSS (no Tailwind)
    └── js/
        └── app.js         # All JS in 1 file (~450 lines)
                            # - API client
                            # - Auth flow
                            # - View renderers (9 views)
                            # - App orchestration
```

## Views Implemented (9 total)

| View | Backend endpoint | What it shows |
|---|---|---|
| 📊 **Dashboard** | `/v1/accounts`, `/v1/invoices` | Stats cards (account count, open invoices, paid invoices) |
| 🏦 **Accounts** | `GET /v1/accounts` | List accounts with balance + status |
| 💸 **Transfers** | `POST /v1/transfers` | New transfer form (with Idempotency-Key) + create |
| 📄 **Invoices** | `GET /v1/invoices` | Invoice list with status badges |
| 📅 **Aging** | `GET /v1/customers/{id}/aging` | 6 aging buckets (current, d_1_7, ..., d_90_plus) |
| 🔄 **Periods** | (placeholder) | Two-step approval workflow (use API for now) |
| 🔍 **Reconciler** | `POST /v1/reconciler/run`, `GET /v1/reconciler/runs` | Trigger run + recent runs table |
| 💱 **Currencies** | `GET /v1/currencies`, `POST /v1/currencies/convert` | Currency list + FX converter widget |
| 📋 **Audit** | `GET /v1/audit` | Recent audit entries |

## Demo Walkthrough (untuk Interview)

```
1. Open http://localhost:3000
2. Login with admin@demo.fmcg-wallet / demo123
3. Dashboard: see account/invoice stats
4. Click Accounts → see 2 demo accounts (Cash IDR 100M, Bank BCA IDR 500M)
5. Click Transfers → fill form → submit → see transaction ID in success message
6. Click Invoices → see sample invoice (INV-2026-001, IDR 25M)
7. Click Aging → see 6 aging buckets
8. Click Reconciler → enter period_id from "INV-2026-001" → run → see status
9. Click Currencies → see IDR, USD, JPY + try FX converter (USD 100 → IDR 1,575,000)
10. Click Audit → see audit trail of all operations
```

## Known Limitations (vs production SPA)

This is a **portfolio-quality demo**, not a production-grade SPA. Known limitations:

| Limitation | Production fix |
|---|---|
| Vanilla JS (no React/Vue) | Swap to Next.js + React |
| No client-side router (hash-based only) | Use Next.js file-based router |
| No build step (no TypeScript, no bundler) | Add Vite/Webpack + TypeScript |
| Tokens in `localStorage` (XSS-vulnerable) | Use httpOnly secure cookies |
| No code splitting | Use dynamic imports |
| No SSR (slow first paint) | Next.js SSR/SSG |
| No PWA / offline support | Use service worker |

## Environment Variables

| Var | Default | Description |
|---|---|---|
| `PORT` | `3000` | Web server port |
| `API_BASE_URL` | `http://localhost:8080` | Backend API URL (proxied for /v1/*) |

## Deployment

### Option A: Local-only (current setup)

```bash
node web/server.js  # serve on :3000, proxy to :8080
```

### Option B: Standalone with Caddy (recommended for VPS)

```bash
# /etc/caddy/Caddyfile
web.fmcg-wallet.example.com {
  reverse_proxy /v1/* localhost:8080
  reverse_proxy /healthz localhost:8080
  reverse_proxy /readyz localhost:8080
  root * /var/www/fmcg-wallet/web/public
  file_server
}
```

### Option C: Bundle into Go binary (embed)

For single-binary deployment, use `go:embed` to bundle `web/public/*` into the Go API binary. (Not implemented yet — deferred.)

## Future Work (post-Sprint 20)

1. **Migrate to Next.js** — when production-grade SPA is needed (real users, real traffic)
2. **Add OpenAPI codegen** — generate TypeScript types from `docs/openapi.json`
3. **Add Storybook** — for visual regression testing of components
4. **Add PWA manifest** — offline support + "install as app"
5. **Add i18n** — Indonesian + English (FMCG distributor serves both)

## Related Documentation

- [API Overview](docs/api/overview.md)
- [Auth API](docs/api/auth.md) — login, refresh, MFA
- [Transfers API](docs/api/transfers.md) — main use case
- [Architecture Overview](docs/architecture/overview.md)
- [Sprint 19: Fly.io Deployment](docs/runbooks/deployment-fly.md)
