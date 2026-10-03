# Alerting Deployment Runbook (Sprint 60)

**Goal:** Deploy the Sprint 60 Alertmanager config to production with real Slack + PagerDuty + email receivers. Every secret comes from an environment variable — never hardcoded.

## Required secrets

| Env var | Source | Used by |
|---|---|---|
| `SLACK_WEBHOOK_OPS_URL` | Slack → Apps → Incoming Webhooks → #fmcg-ops-alerts | warning alerts |
| `SLACK_WEBHOOK_CRITICAL_URL` | Slack → #fmcg-alerts-critical | critical alerts |
| `PAGERDUTY_INTEGRATION_KEY` | PagerDuty → Service Directory → Integrations → Events API v2 | critical page-oncall |
| `ALERT_EMAIL_RECIPIENTS` | ops@fmcg-wallet.example.com | info email summary |
| `ALERT_SMTP_HOST` | ops@fmcg-wallet.example.com:587 | email relay |
| `ALERT_SMTP_FROM` | alertmanager@fmcg-wallet.example.com | email sender |

## Setup steps (Fly.io deployment)

### 1. Slack webhooks

```bash
# Visit https://api.slack.com/messaging/webhooks
# Create incoming webhook for #fmcg-ops-alerts → copy URL
# Repeat for #fmcg-alerts-critical
fly secrets set SLACK_WEBHOOK_OPS_URL="https://hooks.slack.com/services/T0/B0/XXXX"
fly secrets set SLACK_WEBHOOK_CRITICAL_URL="https://hooks.slack.com/services/T1/B1/YYYY"
```

### 2. PagerDuty integration

```bash
# PagerDuty → Service Directory → FMCG Wallet service → Integrations
# → Add an integration → Events API v2 → name "fmcg-wallet-prod" → copy integration key
fly secrets set PAGERDUTY_INTEGRATION_KEY="abcd1234..."
```

### 3. Email relay (optional)

```bash
fly secrets set ALERT_SMTP_HOST="smtp.sendgrid.net:587"
fly secrets set ALERT_SMTP_FROM="alertmanager@fmcg-wallet.example.com"
# SendGrid API key configured elsewhere via env (SMTP_PASSWORD)
fly secrets set ALERT_EMAIL_RECIPIENTS="oncall@fmcg-wallet.example.com"
```

### 4. Restart Alertmanager

```bash
fly deploy  # re-runs docker-compose / Dockerfile; Alertmanager picks up secrets via env interpolation

# Or if running on Kubernetes:
kubectl rollout restart deployment/alertmanager
```

## Verification

After deploy, force a critical alert to verify the pipeline:

```bash
# Option 1: Stop the API container
fly ssh console --app fmcg-wallet -C "supervisorctl stop api"

# Wait 2 min → FMCGApiDown fires → should appear in:
#   - PagerDuty incidents (check PagerDuty dashboard)
#   - Slack #fmcg-alerts-critical (should mention @oncall)
#   - Alertmanager UI (port 9093)

# Restart the API
fly ssh console --app fmcg-wallet -C "supervisorctl start api"

# Verify alert resolves within 5m
```

## Severity routing reference

| Severity | Receiver | Channel | Response time |
|---|---|---|---|
| `critical` | `oncall-pagerduty` | PagerDuty → phone call | Page within 5 min |
| `critical` | `oncall-slack` | Slack #fmcg-alerts-critical | (parallel) |
| `warning` | `ops-slack` | Slack #fmcg-ops-alerts | Investigate within 1h |
| `info` | `info-email` | Daily digest email | Next business day |
| (default) | `dev-null` | (no notifications — local dev) | n/a |

## Inhibition rules

Sprint 60 adds three inhibition patterns to suppress noise:

1. **`FMCGApiDown` suppresses `severity=warning`** — if API is down, individual latency/error alerts are noise
2. **`FMCDbPoolExhausted` suppresses `severity=warning, alertname=~FMCGApi.*`** — DB exhaustion → API slowness, but the DB alert is the actionable one
3. **`FMCOutboxPublisherCritical` suppresses `severity=warning, area=worker`** — critical lag implies warning lag

These reduce alert fatigue during known-incident scenarios.

## Slack message template

The `slack-default` template renders alerts as:

> 🔥 FMCGApiDown
> *FMCG Wallet API is DOWN*
> Prometheus can't reach the API for 2m. Page oncall.
> 📖 Runbook: https://github.com/ardanov96/fledger/blob/main/docs/runbooks/alerts.md#fmcg-api-down
> 📊 Dashboard: http://grafana.example.com/d/fmcg-system-health

The `title_link` points to the runbook URL embedded in the alert annotation. Click → ops/oncall sees the runbook immediately, no Slack context switching.

## PagerDuty payload

```json
{
  "routing_key": "abcd1234...",
  "event_action": "trigger",
  "dedup_key": "FMCGApiDown",
  "payload": {
    "summary": "FMCGApiDown: FMCG Wallet API is DOWN",
    "severity": "critical",
    "source": "fmcg-wallet-prod",
    "custom_details": {
      "firing": "FMCGApiDown",
      "severity": "critical",
      "dashboard": "http://grafana.../d/fmcg-system-health",
      "runbook": "https://github.com/.../alerts.md#fmcg-api-down"
    }
  }
}
```

PagerDuty auto-creates an incident. dedup_key = alertname means repeat fires within PagerDuty's dedup window (30m) are grouped into one incident — no alert storm.

## Operational checks (quarterly)

```bash
# 1. Are all receivers firing correctly?
curl -X POST http://alertmanager:9093/api/v1/receivers/test \
  -H 'Content-Type: application/json' \
  -d '{"receivers": ["oncall-pagerduty", "oncall-slack", "ops-slack", "info-email"]}'

# 2. Are alert rules loaded?
curl -s http://prometheus:9090/api/v1/rules | jq '.data.groups[] | {name, rules: [.rules[].name]}'

# 3. Any firing alerts right now?
curl -s http://alertmanager:9093/api/v2/alerts | jq '.[] | {name, state, severity: .labels.severity}'
```

## Rollback plan

If Sprint 60 config breaks alerting in production:

```bash
# Revert to Sprint 59 stub config:
git revert <sprint-60-commit> -- deployments/alertmanager/alertmanager.yml

# Or force dev-null receiver only:
fly secrets unset SLACK_WEBHOOK_OPS_URL SLACK_WEBHOOK_CRITICAL_URL PAGERDUTY_INTEGRATION_KEY

# Restart alertmanager
fly deploy
```

## Sprint 60 follow-ups

- Sprint 60.1: per-tenant rate-limit alerts (Sprint 22)
- Sprint 60.2: fraud flag spike alerts (Sprint 31)
- Sprint 60.3: cost-budget alerts (GCP/AWS spend)

---

## References

- Sprint 60 commit: `9a1c4e7`
- `deployments/alertmanager/alertmanager.yml`: production config
- `deployments/prometheus/alerts.yml`: alert rules
- `docs/runbooks/alerts.md`: per-alert triage
- PagerDuty Events API v2: https://developer.pagerduty.com/docs/events-api-v2/overview/
- Slack incoming webhooks: https://api.slack.com/messaging/webhooks