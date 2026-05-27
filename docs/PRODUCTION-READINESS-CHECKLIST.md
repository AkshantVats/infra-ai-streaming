# Production Readiness Checklist

This checklist is intended for OSS/prod-readiness review. It is not a substitute for tests or real production validation, but it makes the repo’s operational posture explicit.

## Scope

Applies to the full stack described in `README.md`:
Rust ingestion (`ingestion/`), Go consumer (`consumer/`), Kafka-compatible broker (Redpanda), ClickHouse, Redis, and the local Docker/Kubernetes deployment paths.

## Reliability (must-have operational guarantees)

### Ingestion edge

- **Durability on ingest acknowledgment**
  - Ingestion returns success only after WAL append + fsync, and after enqueuing to the bounded producer channel.
  - Replay exists for un-acked WAL entries on restart.

- **Backpressure and honest overload behavior**
  - Bounded internal queue with deterministic HTTP overload response (`503` + `Retry-After`).
  - No unbounded in-memory growth on the hot path.

### Consumer / storage

- **Consumer handoff semantics**
  - Consumer offsets advance only after ClickHouse insert / overflow drain / DLQ handoff for the relevant record.
  - Poison/deserialize failures do not silently commit offsets.

### Failure-mode observability

- Each failure mode emits at least one Prometheus metric and/or a stable log key.
- Dashboards exist for “healthy”, “degraded”, and “failure recovery” situations.

## Scaling & performance

- **Load shedding is explicit**: overload is a fast failure (no “slow death” via queues).
- **Consumer batching is bounded**: batch size + flush interval are configurable and surfaced via metrics.
- **Kafka partitions are intentional**: partition count is selected for the expected write/read concurrency.

## Security & secrets posture

- **Vulnerability reporting is documented**: `SECURITY.md`.
- **No committed secrets**: `.env` is documented but not committed; example files are safe.
- **Secrets scanning exists**: repository runs a lightweight secret scan in CI.
- **Least-privilege defaults are explicit**: dev defaults are safe; production hardening is documented before real deployments.

## Observability (production expectations)

- **Metrics**: scrape endpoints exist for both ingestion and consumer; naming is stable and matches dashboards.
- **Dashboards**: “Product SLOs” and “Local E2E” dashboards exist and are provisioned.
- **Grafana queries are stable**: dashboards do not rely on untracked local state.

## Ops & incident response

- **Evergreen docs**: runtime docs avoid sprint-style noise; operational steps use stable terminology.
- **Runbooks exist for degraded scenarios**:
  - `docs/RUNBOOK.md` provides symptom → checks → actions.
  - `CHAOS.md` provides reproducible fault injection and expected metric signals.

## Build / test / release gates

- **Test matrix is runnable locally**
  - Rust ingestion: `cargo test -p ingestion`
  - Go consumer: `cd consumer && go test ./...`
  - Helm: `helm template … -f deploy/helm/lensai/values-m1.yaml`
  - Shell scripts: `shellcheck` (preferred) or `bash -n`

- **CI enforces the matrix**
  - Rust: `cargo fmt --check`, `cargo clippy -D warnings`, `cargo test -p ingestion`
  - Go: gofmt check + `go test ./...` in `consumer/`
  - Shell: `shellcheck` + `bash -n`
  - Helm: `helm template` with `values-m1.yaml`

- **End-to-end verification is automated (M1-first)**
  - `HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh` — uses `values-m1.yaml` only.
  - Checklist: [`docs/E2E-CHECKLIST.md`](E2E-CHECKLIST.md).
  - Proof log is appended to `docs/E2E-PROOF-K3D.md`.

## Status

When running the full verification matrix, record the results into `docs/E2E-PROOF-K3D.md` (and keep the proof close to the commands).

## Explicit “not implemented yet” (be honest)

These are the most common “production expectations” that are intentionally **not** implemented in this repo today.

- **AuthN/AuthZ**: no first-class authentication model for `/ingest` (mTLS/OIDC/API keys) and no tenant auth boundary enforcement.
- **Encryption**: no at-rest encryption guidance for WAL/PVCs and no end-to-end TLS topology documented.
- **Backups / restore**: no ClickHouse backup/restore runbook and no retention policy automation.
- **Multi-cluster / HA**: no multi-AZ Kafka/ClickHouse/Redis topology and no tested disaster recovery path.
- **SLO-based alerting**: dashboards exist, but alerting policies/escalation are not codified.
- **E2E in CI**: full k3d E2E is designed for laptops; CI runs unit + template gates only by default.

## Next steps (prioritized)

1. Add a documented auth model for ingestion (API keys or OIDC gateway) and enforce per-tenant authorization.
2. Add ClickHouse retention + backup strategy (and test restore).
3. Add a scheduled, time-bounded nightly E2E job (k3d) behind `workflow_dispatch` / cron.

