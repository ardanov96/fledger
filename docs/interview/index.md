# Interview Prep — Q&A Index

**Audience:** Interviewer (technical) atau candidate (you) preparing for FMCG Wallet interview.

**Goal:** Quick-reference untuk 15 anticipated technical questions, organized by category, dengan codebase-grounded answers.

---

## 🎬 Demo (Start Here)

| File | Description |
|---|---|
| [Demo Script](demo-script.md) | 5-7 menit structured demo (`masalah → solusi → live demo → Q&A`) dengan cURL cheat sheet + talking points |

---

## 📂 Fintech (Domain-Specific)

| Q# | Question | Reference |
|---|---|---|
| 1 | Why double-entry vs single-entry? | [q-fintech.md#q1](q-fintech.md#q1-why-double-entry-accounting-vs-single-entry) |
| 2 | How does hash chain prevent tampering? | [q-fintech.md#q2](q-fintech.md#q2-how-does-hash-chain-prevent-tampering) |
| 3 | How do you handle FX rate volatility? | [q-fintech.md#q3](q-fintech.md#q3-how-do-you-handle-fx-rate-volatility) |
| 4 | Reconciliation — manual vs automated? | [q-fintech.md#q4](q-fintech.md#q4-reconciliation--manual-vs-automated) |
| 5 | Idempotency-Key TTL — how long is safe? | [q-fintech.md#q5](q-fintech.md#q5-idempotency-key-ttl--how-long-is-safe) |

---

## 🏗️ Distributed Systems (Architecture)

| Q# | Question | Reference |
|---|---|---|
| 1 | Why UUID-sorted lock ordering? | [q-distributed-systems.md#q1](q-distributed-systems.md#q1-why-uuid-sorted-lock-ordering) |
| 2 | RLS vs application-layer tenant filter — why both? | [q-distributed-systems.md#q2](q-distributed-systems.md#q2-rls-vs-application-layer-tenant-filter--why-both) |
| 3 | How do you handle clock skew? | [q-distributed-systems.md#q3](q-distributed-systems.md#q3-how-do-you-handle-clock-skew-for-distributed-systems) |
| 4 | What's your consistency model — strong or eventual? | [q-distributed-systems.md#q4](q-distributed-systems.md#q4-whats-your-consistency-model--strong-or-eventual) |
| 5 | How would you scale to 1M req/s? | [q-distributed-systems.md#q5](q-distributed-systems.md#q5-how-would-you-scale-to-1m-reqs) |

---

## 🔐 Security (Cross-Cutting)

| Q# | Question | Reference |
|---|---|---|
| 1 | JWT vs opaque refresh — why use both? | [q-security.md#q1](q-security.md#q1-jwt-vs-opaque-refresh--why-use-both) |
| 2 | MFA bypass scenarios? | [q-security.md#q2](q-security.md#q2-mfa-bypass-scenarios) |
| 3 | How do you prevent timing attacks on bcrypt? | [q-security.md#q3](q-security.md#q3-how-do-you-prevent-timing-attacks-on-bcrypt) |
| 4 | Rate limit false positives? | [q-security.md#q4](q-security.md#q4-rate-limit-false-positives) |
| 5 | RLS bypass scenarios (e.g., super_admin)? | [q-security.md#q5](q-security.md#q5-rls-bypass-scenarios-eg-super_admin) |

---

## 📚 Related Documentation

- [Demo Script](demo-script.md) — full 5-7 menit interview walkthrough
- [Architecture Overview](../architecture/overview.md) — system context
- [C4 Diagrams](../architecture/c4-diagrams.md) — static architecture views
- [Sequence Flows](../architecture/sequences.md) — critical user journeys
- [ADRs](../adr/index.md) — 6 architectural decision records
- [API Reference](../api/overview.md) — 36 endpoints
- [Runbooks](../runbooks/backup-restore.md) — operational SOPs
- [Sprint Log](../SPRINTS.md) — 19 sprint selesai

---

## 🎯 Quick-Prep Cheat Sheet

**If interviewer asks X, open Y first:**

| Interviewer topic | Open this file |
|---|---|
| "Walk me through a transfer" | [sequences.md#4](../architecture/sequences.md#4-double-entry-transfer-same-currency) |
| "How do you handle money correctness" | [q-fintech.md#q1](q-fintech.md#q1-why-double-entry-accounting-vs-single-entry) |
| "What about scaling?" | [q-distributed-systems.md#q5](q-distributed-systems.md#q5-how-would-you-scale-to-1m-reqs) |
| "Multi-tenant strategy?" | [q-distributed-systems.md#q2](q-distributed-systems.md#q2-rls-vs-application-layer-tenant-filter--why-both) |
| "Auth security?" | [q-security.md#q1](q-security.md#q1-jwt-vs-opaque-refresh--why-use-both) |
| "Why this stack?" | [Architecture Overview](../architecture/overview.md) + [ADR-0001](../adr/0001-go-as-backend-language.md) |
| "Defense in depth?" | [Architecture Overview — Concurrency model](../architecture/overview.md#concurrency-model) |

---

## 📋 One-Liner Talking Points

If you only have 30 seconds:

- **"Double-entry ledger + hash chain tamper detection + multi-tenant RLS + JWT + MFA + rate limit = production-grade fintech backend in 19 sprints."**
- **"Tier 1 defense-in-depth: every concern (audit, RLS, idempotency, rate limit) has minimum 2 layers."**
- **"Single Fly.io app multi-process via supervisord = $0/bulan operational cost, production-grade deployment pattern."**
- **"15 property tests cover 10,000+ random scenarios per CI run = robust invariants (FIFO, conservation, hash chain).**
- **"9 docs API ref + 4 runbooks + C4 + sequence diagrams = operational maturity, not just 'running code'."**

---

**Good luck! 🚀**
