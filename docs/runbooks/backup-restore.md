# Backup & Restore Runbook

**Goal:** Define RPO/RTO targets + tested procedures for backing up and restoring the FMCG Wallet database.

---

## Targets

| Metric | Target (Demo) | Target (Production) |
|---|---|---|
| **RPO** (Recovery Point Objective) | 24 hours | 1 hour |
| **RTO** (Recovery Time Objective) | 4 hours | 15 minutes |
| **Backup frequency** | Manual on-demand | Every 1 hour (continuous WAL archiving) |
| **Retention** | 7 days (free Fly snapshots) | 90 days (S3 lifecycle policy) |
| **Verification** | Manual | Weekly automated restore drill |

**Current status (Sprint 19 demo):** Fly.io persistent volume snapshots (`fly volumes snapshots create`). Free tier retains snapshots for 7 days.

---

## Fly.io Demo Backup Procedure

### Create manual snapshot

```bash
fly volumes snapshots create pg_data --app fmcg-wallet-demo
```

Returns snapshot ID. Snapshots are stored in Fly's infrastructure (NOT in your container).

### List snapshots

```bash
fly volumes snapshots list pg_data --app fmcg-wallet-demo
```

### View snapshot details

```bash
fly volumes snapshots show <snapshot-id> --app fmcg-wallet-demo
```

---

## Restore Procedure (Fly.io)

⚠️ **Destructive**: replaces volume content. All post-snapshot changes will be lost.

```bash
# 1. Stop the app (releases volume lock)
fly scale count 0 --app fmcg-wallet-demo

# 2. Wait for machine to fully stop
fly status --app fmcg-wallet-demo

# 3. Restore snapshot to volume
fly volumes snapshots restore <snapshot-id> --app fmcg-wallet-demo

# 4. Restart the app (fresh machine reads restored volume)
fly scale count 1 --app fmcg-wallet-demo

# 5. Verify
curl https://fmcg-wallet-demo.fly.dev/healthz
curl https://fmcg-wallet-demo.fly.dev/readyz
```

Recovery time: ~3-5 minutes.

---

## Production Backup (PostgreSQL `pg_dump`)

For VPS / managed Postgres deployments. Run **nightly** via cron or systemd timer.

### Full logical backup (compressed)

```bash
# As postgres user
pg_dump -Fc -d fmcg_wallet -f /var/backups/fmcg-$(date +%Y%m%d-%H%M%S).dump

# Options:
#   -Fc = custom compressed format (smaller than plain SQL)
#   -d = database name
#   -f = output file
```

### Continuous WAL archiving (PITR — Point In Time Recovery)

For 1-hour RPO, enable WAL archiving in `postgresql.conf`:

```ini
wal_level = replica
archive_mode = on
archive_command = 'cp %p /var/backups/wal/%f'
archive_timeout = 60
```

Then take periodic base backups:

```bash
pg_basebackup -D /var/backups/base-$(date +%Y%m%d) -Ft -z -P
```

This enables point-in-time recovery to any second within the last 7 days (assuming you retain 7 days of WAL).

---

## Restore Procedure (Production)

### From logical backup (`pg_dump`)

```bash
# 1. Stop API server
systemctl stop fmcg-wallet-api

# 2. Drop and recreate database (DESTRUCTIVE)
dropdb fmcg_wallet --force
createdb fmcg_wallet

# 3. Restore
pg_restore -d fmcg_wallet -Fc /var/backups/fmcg-20260815-020000.dump

# 4. Run migrations (in case backup is from older migration)
go run ./cmd/migrator up

# 5. Restart API
systemctl start fmcg-wallet-api

# 6. Verify
curl https://api.example.com/healthz
curl https://api.example.com/readyz
```

### From PITR (base backup + WAL)

```bash
# 1. Stop API
systemctl stop fmcg-wallet-api

# 2. Restore base backup
systemctl stop postgresql
rm -rf /var/lib/postgresql/data/*
tar -xzf /var/backups/base-20260815.tar.gz -C /var/lib/postgresql/data

# 3. Create recovery.signal
touch /var/lib/postgresql/data/recovery.signal

# 4. Configure recovery target (in postgresql.conf)
echo "restore_command = 'cp /var/backups/wal/%f %p'" >> /var/lib/postgresql/data/postgresql.conf
echo "recovery_target_time = '2026-08-15 14:30:00'" >> /var/lib/postgresql/data/postgresql.conf
echo "recovery_target_action = 'pause'" >> /var/lib/postgresql/data/postgresql.conf

# 5. Start Postgres (will replay WAL until target time)
systemctl start postgresql

# 6. Verify recovery, then promote
sudo -u postgres psql -c "SELECT pg_wal_replay_pause();"
# Inspect data...
sudo -u postgres psql -c "SELECT pg_wal_replay_resume();"
# When ready:
sudo -u postgres psql -c "SELECT pg_promote();"

# 7. Remove recovery.signal (production now)
rm /var/lib/postgresql/data/recovery.signal

# 8. Restart API
systemctl start fmcg-wallet-api
```

---

## Backup Verification (Drill)

**Critical:** backups are useless if you can't restore from them. Test quarterly.

### Quarterly restore drill

```bash
# 1. Spin up isolated test environment (separate VM / Fly app)
# 2. Restore latest backup into test DB
pg_restore -d fmcg_wallet_test -Fc /var/backups/fmcg-latest.dump

# 3. Run integration tests
TEST_DATABASE_URL=postgres://... go test -tags=integration ./internal/usecase/...

# 4. Verify:
#    - All 5 integration scenarios pass
#    - Hash chain verification succeeds (no tampering during backup)
#    - RLS policies enforced (Tenant A can't see Tenant B)
#    - Reconciler finds zero imbalance
# 5. Document results in runbook log
```

### Automation with healthchecks (planned)

```yaml
# Pseudo-code for cron + health check
nightly_backup.sh:
  - pg_dump → /var/backups/daily.dump
  - SHA256 dump → /var/backups/daily.dump.sha256
  - rclone copy → s3://fmcg-backups/

weekly_drill.sh:
  - Spin up test VM
  - Restore latest dump
  - Run integration tests
  - If tests fail → page on-call
```

---

## Disaster scenarios

| Scenario | Recovery procedure | RTO |
|---|---|---|
| Single row corruption | Identify row, fix from last good backup OR write adjustment journal | <1h |
| Table dropped accidentally | PITR to point before drop, export missing rows | <2h |
| Database corruption (disk fail) | Restore from snapshot + replay WAL | <4h |
| Region-wide outage (Fly.io) | Deploy to backup region, restore from S3 backup | <8h |
| Ransomware / malicious deletion | Restore from offline backup (NOT connected to prod) | <24h |

---

## Backup checklist (monthly)

- [ ] Last nightly backup timestamp verified (not stuck)
- [ ] Backup size reasonable (delta <10% from previous day)
- [ ] Snapshot count within Fly.io limit
- [ ] Restore drill completed successfully (quarterly)
- [ ] Off-site backup verified (S3 / external storage)
- [ ] Backup encryption verified (if compliance required)

---

## Related Documentation

- [Deployment (Fly.io)](deployment-fly.md) — demo deployment
- [Architecture Overview](../architecture/overview.md) — Postgres role + RLS
- [ADR-0003: Double-entry ledger](../adr/0003-double-entry-ledger.md) — immutability principles
- [Sprint 19: Fly.io Deployment](../../SPRINTS.md#sprint-19--deployment-flyio-fase-3b--2026-08-15)
