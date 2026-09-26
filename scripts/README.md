# scripts/ - Operational Scripts

PowerShell scripts for local development of FMCG Wallet. All scripts are
**idempotent** (safe to re-run) and **AI-agent friendly** (clear output, exit
codes, `-Json` flag for structured parsing).

## Quick Reference

| Lifecycle | Script | Purpose |
|---|---|---|
| Setup (first time) | [setup-everything.ps1](setup-everything.ps1) | 7-phase orchestrator: preflight → postgres → env → migrations → RLS fix → seed → self-test |
| Reset (destructive) | [start-fresh.ps1](start-fresh.ps1) | Drop db + roles + recreate + re-migrate + re-seed |
| Validate | [verify-setup.ps1](verify-setup.ps1) | Read-only state checker (24 checks) |
| Operate | [run-stack.ps1](run-stack.ps1) / [stop-stack.ps1](stop-stack.ps1) | Start/stop API + web as background processes |
| Debug | [show-state.ps1](show-state.ps1) | Detailed runtime + DB state |
| Debug | [show-config.ps1](show-config.ps1) | Tool versions + env + PG config |
| DB access | [db-shell.ps1](db-shell.ps1) | Interactive psql with fmcg_wallet defaults |

## Cheat Sheet

```powershell
# First time only
.\scripts\setup-everything.ps1 -PostgresPassword "Desmone8327"

# Daily
.\scripts\run-stack.ps1
# ... open browser at http://localhost:3000 ...
.\scripts\stop-stack.ps1

# Debug / inspect
.\scripts\verify-setup.ps1          # 24 checks, PASS/WARN/FAIL
.\scripts\show-state.ps1            # runtime processes + DB row counts
.\scripts\show-config.ps1           # tool versions + env vars + PG config
.\scripts\db-shell.ps1              # psql to fmcg_wallet as fmcg user

# Reset
.\scripts\start-fresh.ps1           # prompts for confirmation
.\scripts\start-fresh.ps1 -Force    # skip prompt (CI / AI agent)

# AI-agent mode
.\scripts\setup-everything.ps1 -Json -PostgresPassword "X" | tee setup.log
.\scripts\verify-setup.ps1 -Json | Out-File state.json
.\scripts\show-state.ps1 -Json | Out-File state.json
```

## Exit Codes (for CI / AI agents)

All scripts use a consistent exit-code scheme:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Cancelled by user (start-fresh) / preflight missing tools |
| 2 | Setup phase failed |
| 64 | Fatal (not in repo root / missing tools) |

## Flags Quick Reference

| Script | Common flags |
|---|---|
| `setup-everything.ps1` | `-PostgresPassword X`, `-CheckOnly`, `-SkipPhase N`, `-Json`, `-ApiPort`, `-WebPort`, `-ApiOnly` |
| `start-fresh.ps1` | `-Force` (skip confirm), `-Json`, `-PostgresPassword X` |
| `verify-setup.ps1` | `-Json` |
| `show-state.ps1` | `-Json`, `-ApiPort`, `-WebPort` |
| `show-config.ps1` | `-Json` |
| `run-stack.ps1` | `-ApiPort`, `-WebPort`, `-ApiOnly`, `-WebOnly` |
| `stop-stack.ps1` | `-Force`, `-ApiPort`, `-WebPort` |
| `db-shell.ps1` | `-AsPostgres`, `-Query "SQL"`, `-File script.sql`, `-DbHost`, `-DbPort`, `-DbName` |

## What Each Script Does

### setup-everything.ps1
Master orchestrator (7 phases, all skip-able):
1. Preflight (Go, Node, psql, Postgres service, git)
2. Postgres setup (admin: pg_hba.conf edit, restart, create fmcg user + db)
3. .env setup (copy from .env.example, generate JWT secret)
4. Migrations (drop+recreate db, apply 18 schemas as postgres)
5. RLS workaround (disable RLS on refresh_tokens - dev only)
6. Seed data (call seed-local-dev-data.ps1)
7. Self-test (start API + curl /healthz, /v1/auth/login, stop)

### start-fresh.ps1
**DESTRUCTIVE** - wipes everything. Steps:
1. Drop fmcg_wallet db + app_admin role
2. Recreate db + grants
3. Run all 18 migrations (as postgres)
4. Disable RLS on refresh_tokens
5. Re-seed demo data

Prompts for confirmation unless `-Force`.

### verify-setup.ps1
Read-only state checker. Reports 24 individual checks (tools, PG, .env,
migrations, seed counts) with PASS/WARN/FAIL. Use as pre-flight before
running other scripts or as CI gate.

### show-state.ps1
Detailed runtime + DB inspect:
- Running processes on API/Web ports (PID, CPU, mem)
- /healthz + /readyz status
- DB size, latest migration, per-table row counts
- Unpublished outbox events
- RLS status
- Active queries
- Recent log tails (errors only)
- Last 5 records from audit_logs + transactions

### show-config.ps1
One-shot dump of:
- Tool versions (go, node, psql, curl, git)
- All .env contents (secrets masked as `ab***cd`)
- Postgres runtime config (listen_addresses, max_connections, etc.)
- DB size + table count
- Latest migration version
- Important paths (repo, logs)

### run-stack.ps1 / stop-stack.ps1
- `run-stack` starts API + web as background processes, redirects logs to
  `%TEMP%\fmcg-{api,web}.log`, verifies via `/healthz`
- `stop-stack` kills processes on the ports

### db-shell.ps1
Interactive psql wrapper:
- Loads `.env` to auto-set `PGPASSWORD`
- Default connects as `fmcg`; use `-AsPostgres` for superuser
- Prints table list on startup so you can start exploring immediately
- Non-interactive mode: `-Query "SQL"` or `-File script.sql`

## AI Agent Workflow

```powershell
# 1. Verify state
$state = powershell -NoProfile -File scripts/verify-setup.ps1 -Json | ConvertFrom-Json
if ($state.summary.fail -gt 0) {
    # 2. Run setup (idempotent, but may require admin UAC for postgres phase)
    powershell -NoProfile -File scripts/setup-everything.ps1 -PostgresPassword "X" -Json
}

# 3. Start the stack
powershell -NoProfile -File scripts/run-stack.ps1

# 4. Inspect runtime + DB state (for debugging)
powershell -NoProfile -File scripts/show-state.ps1 -Json | Out-File state.json

# 5. Reset if needed (destructive!)
powershell -NoProfile -File scripts/start-fresh.ps1 -Force

# 6. Stop when done
powershell -NoProfile -File scripts/stop-stack.ps1
```

## Known Limitations (dev-only)

| Limitation | Status |
|---|---|
| Postgres password hardcoded to `Desmone8327` | Pass via `-PostgresPassword X` (use secret store in prod) |
| `curl` may not be installed | Falls back to `Invoke-WebRequest` |

## Resolved in Sprint 27

| Was | Now |
|---|---|
| RLS on `refresh_tokens` blocked login (had to disable) | Migration `000019_refresh_tokens_rls_fix` recreates the policy to allow login flow when GUC is NULL |
| `app_admin` role creation failed on re-runs (no IF NOT EXISTS) | Migration `000020_app_admin_role_idempotent` wraps in DO block + EXCEPTION handling |
| `fmcg` user had to be created manually by setup script | Migration `000021_create_fmcg_user` creates user + db idempotently |
| Setup scripts manually disabled RLS + dropped app_admin | All workarounds removed — migrations handle it |

Production deploy no longer needs the manual RLS disable workaround.
