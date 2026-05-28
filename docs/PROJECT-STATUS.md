# Project status

Honest snapshot of what exists in the repository today versus planned work. For operators and contributors — not marketing copy.

## Shipped today

| Area | Status | Notes |
|------|--------|-------|
| **Rust ingestion** | ✅ | Axum `/ingest`, WAL + fsync, Kafka producer, Redis rate limits (fail-open), Prometheus |
| **Go consumer** | ✅ | Batch writer (1000 / 500ms), circuit breaker, Redis overflow, DLQ, lag metrics, z-score anomaly topic |
| **Local stack** | ✅ | Docker Compose: Redis, Redpanda, ClickHouse, Prometheus, Grafana |
| **Kubernetes** | ✅ | Helm chart `deploy/helm/lensai/`, k3d path, consumer HPA on Kafka lag |
| **Observability** | ✅ | Product SLO + Local E2E Grafana dashboards, `OBSERVABILITY.md`, `docs/SLOs.md` |
| **CI** | ✅ | Rust fmt/clippy/test, Go test, Helm template, shellcheck, gitleaks |
| **E2E** | ✅ | `./scripts/run.sh --profile m1` — k3d deploy, smoke, chaos |
| **Docs** | ✅ | Architecture, runbook, chaos, security hardening, data retention |

## Roadmap (not yet production-complete)

| Item | Priority | Notes |
|------|----------|-------|
| AuthN/AuthZ on `/ingest` | High | API keys, gateway OIDC, or mTLS |
| Partition key `hash(tenant_id:model_id)` | Medium | Today keys by `tenant_id` only |
| Multi-region HA | Medium | Kafka, ClickHouse, Redis topologies |
| ClickHouse backup/restore automation | Medium | Documented in `docs/DATA-RETENTION.md`; no in-tree tooling |
| OTLP tracing wired end-to-end | Low | Env stubs exist; not fully wired in compose |
| Load benchmarks in CI | Low | k6 scripts planned; numbers not gated in CI |

## External contributions

- **Vector** [#25455](https://github.com/vectordotdev/vector/issues/25455) — memory enrichment counter fix: [PR #25496](https://github.com/vectordotdev/vector/pull/25496) (open).

## E2E verification

| Step | How to verify |
|------|----------------|
| Unit tests | `cargo test -p ingestion` · `cd consumer && go test ./...` |
| Compose stack | `./scripts/run.sh --profile m1 --target compose` |
| Full k3d E2E | `./scripts/run.sh --profile m1` |
| Manual checklist | [`docs/E2E-CHECKLIST.md`](E2E-CHECKLIST.md) |

## Quick demo (Compose + host binaries)

1. `./scripts/run.sh --profile m1 --target compose`
2. Run consumer and ingestion with `deploy/compose/values-m1.env` sourced (see README).
3. `curl` `/ingest` → check ClickHouse `infra_ai.inference_events` and Grafana dashboards.

Or: `./scripts/smoke-e2e.sh` after Compose is up.

## Related docs

- [ARCHITECTURE.md](ARCHITECTURE.md) — stable component overview
- [ARCHITECTURE-AND-FLOWS.md](ARCHITECTURE-AND-FLOWS.md) — lifecycles, code walkthrough, troubleshooting
- [END-TO-END-FLOWS.md](END-TO-END-FLOWS.md) — demo scenarios and Grafana panel guide
- [deploy/README.md](../deploy/README.md) — deploy profiles and ports
- [CONTRIBUTING.md](../CONTRIBUTING.md) — PR expectations
