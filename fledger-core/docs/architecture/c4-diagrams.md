# C4 Architecture Diagrams

This page presents the system architecture using the [C4 model](https://c4model.com/) (Context, Container, Component, Code). See [Architecture Overview](overview.md) for the high-level narrative.

---

## Level 1 — System Context

Who uses the system and how does it fit into the wider world.

```mermaid
flowchart TB
    subgraph Users
        FinanceHQ["💼 Finance HQ Operator<br/>(hq_admin)"]
        FieldRep["🚚 Field Sales Rep<br/>(sales_rep, mobile)"]
        Auditor["🔍 Auditor<br/>(read-only)"]
    end

    subgraph FMCG_Wallet["📦 FMCG Wallet System"]
        Wallet["Production-grade<br/>hybrid wallet backend<br/>(Go + PostgreSQL)"]
    end

    External["🏦 External Systems<br/>(planned: payment gateway,<br/>bank API, FX provider)"]

    FinanceHQ -->|"HTTPS + JWT"| Wallet
    FieldRep   -->|"HTTPS + JWT<br/>(mobile-optimized)"| Wallet
    Auditor    -->|"HTTPS + JWT"| Wallet
    Wallet     -.->|"webhooks (planned)"| External
```

**Key points:**
- Single bounded context: FMCG Wallet serves one Indonesian FMCG distributor's finance operations
- Three actor types: HQ (full access), field (scoped to own routes), auditor (read-only)
- External integrations (payment gateway, FX provider) are planned but not yet implemented (deferred to post-MVP)

---

## Level 2 — Container Diagram

The deployable units inside FMCG Wallet.

```mermaid
flowchart TB
    subgraph Client
        MobileApp["📱 Mobile App<br/>(sales_rep)<br/>Future"]
        WebConsole["🌐 Web Console<br/>(hq_admin)<br/>Future"]
    end

    subgraph FlyApp["☁️ Fly.io App: fmcg-wallet-demo"]
        subgraph Container["🐳 Container (multi-process)"]
            API["🚀 API Server<br/>Go 1.23 binary<br/>:8080"]
            Migrate["🔄 Migrator<br/>Go binary<br/>(one-shot on boot)"]
            Worker["⏰ Reconciler Worker<br/>ticker 1h<br/>(background)"]
        end

        DB[("🐘 PostgreSQL 16<br/>+ RLS policies<br/>+ 14 migrations")]
    end

    subgraph Observability["📊 Observability"]
        Prom["📈 Prometheus<br/>metrics scrape"]
        Loki["📜 Loki + Promtail<br/>log aggregation"]
        Tempo["🔭 Tempo<br/>trace storage"]
        Grafana["📊 Grafana<br/>dashboards"]
    end

    MobileApp -.->|"HTTPS + JWT"| API
    WebConsole -.->|"HTTPS + JWT"| API

    API --> DB
    Migrate --> DB
    Worker --> API

    API -->|"logs/metrics/traces"| Prom
    API -->|"structured JSON"| Loki
    API -->|"OTLP/W3C traceparent"| Tempo
    Prom --> Grafana
    Loki --> Grafana
    Tempo --> Grafana
```

**Key points:**
- **Single container, multi-process** via `supervisord` (postgres + migrator + api + worker)
- API + worker share codebase (worker is built from `cmd/worker/`)
- DB is local to the container in demo (production would use managed Postgres)
- Observability stack is provisioned via `docker-compose.yml` (TIG stack)

---

## Level 3 — Component Diagram

What's inside the API binary.

```mermaid
flowchart TB
    subgraph Edge["Edge / Transport"]
        Router["HTTP Router<br/>(chi)"]
        MW["Middleware Chain<br/>RequestID → Trace → Auth<br/>→ RBAC → RateLimit<br/>→ Audit"]
    end

    subgraph Handlers["Handler Layer (HTTP)"]
        HAuth["AuthHandler<br/>(5 endpoints)"]
        HAccount["AccountHandler<br/>(4 endpoints)"]
        HTransfer["TransferHandler<br/>(1 endpoint)"]
        HInvoice["InvoiceHandler<br/>(6 endpoints)"]
        HPeriod["PeriodHandler<br/>(6 endpoints)"]
        HRec["ReconcilerHandler<br/>(4 endpoints)"]
        HColl["CollectionHandler<br/>(11 endpoints)"]
        HCurr["CurrencyHandler<br/>(9 endpoints)"]
        HAudit["AuditHandler<br/>(1 endpoint)"]
    end

    subgraph Services["Use Case Layer"]
        SAuth["AuthService"]
        SAcc["AccountService"]
        STr["TransferService"]
        SInv["InvoiceService"]
        SPer["PeriodService"]
        SRec["ReconcilerService"]
        SColl["CollectionService"]
        SCurr["CurrencyService"]
        SHash["HashChainVerifier"]
    end

    subgraph Domain["Domain Layer (interface-only)"]
        DAcc["Account / Entry /<br/>Transaction"]
        DInv["Invoice / Payment /<br/>CreditLimit"]
        DPer["Period / CloseRequest /<br/>Snapshot"]
        DRec["ReconcilerRun /<br/>AccountResult"]
        DColl["Route / Stop / Event /<br/>Settlement"]
        DCurr["Currency / FxRate"]
        DAuth["RefreshToken /<br/>UserCredentials"]
    end

    subgraph Infra["Infrastructure"]
        Repo["Postgres Repositories<br/>(pgx pool)"]
        TxAdapter["7x TxAdapter<br/>(auto-bind GUC)"]
        Crypto["Bcrypt + TOTP<br/>+ SHA-256 helpers"]
        JWT["JWT Signer/Verifier"]
    end

    Router --> MW
    MW --> Handlers
    Handlers --> Services
    Services --> Domain
    Services --> Repo
    Services --> Crypto
    Services --> JWT
    Repo --> TxAdapter
    TxAdapter --> DB[("Postgres")]
```

**Key points:**
- **Clean-lite architecture**: domain layer has zero infrastructure imports
- 9 cohesive modules, each with: domain types + repository interface + service + handler
- Handlers do thin DTO mapping only (validation in `dto.go`, business logic in services)
- Services depend on domain interfaces, not concrete repos (DI-friendly for testing)
- 7 TxAdapter wrappers bridge pgx.Tx ↔ each domain's Tx interface (auto-bind tenant context)

---

## Module dependency graph

```mermaid
flowchart TB
    Platform["internal/platform<br/>(money, errors, httpx,<br/>config, logger)"]
    Domain["internal/domain<br/>(9 modules, pure types + interfaces)"]
    Usecase["internal/usecase<br/>(services, orchestration)"]
    Handler["internal/handler<br/>(HTTP routes, DTOs)"]
    Middleware["internal/middleware<br/>(auth, RBAC, audit, trace,<br/>rate-limit, tenant)"]
    Repo["internal/repository/postgres<br/>(pgx adapters)"]
    Platform --> Domain
    Domain --> Usecase
    Platform --> Usecase
    Middleware --> Usecase
    Repo --> Domain
    Repo --> Platform
    Handler --> Usecase
    Handler --> Middleware
    Handler --> Platform
```

**Dependency direction:** Platform → Domain → Usecase → Handler. Repo implements Domain interfaces. No upward dependencies.

---

## Key abstractions

| Layer | Pattern | Example |
|---|---|---|
| Domain | Interface-only repository | `AccountRepository.GetByID(ctx, id) (Account, error)` |
| Tx | Per-domain interface | `ledger.Tx`, `invoice.Tx`, `period.Tx` (each wraps pgx.Tx) |
| Use case | Service with deps struct | `TransferServiceDeps{Accounts, Transactions, Entries, DB}` |
| Handler | Service interface (subset) | `TransferAPI interface { Transfer(...) }` |
| Adapter | Pattern: `usecase.X` ↔ `handler.Y` | `transferAPIAdapter`, `currencyAPIAdapter` |
| Middleware | Composable chain | `r.Use(RequireAuth, TenantContext, RequirePermission)` |

---

## Reference

- [Architecture Overview](overview.md) — narrative + module list
- [Sequence Flows](sequences.md) — critical user journeys (transfer, login, period close, FX)
- [ADRs](../adr/index.md) — key decisions (Go, sqlc, double-entry, locking, multi-currency, RLS)
- [C4 model](https://c4model.com/) — Simon Brown's notation
