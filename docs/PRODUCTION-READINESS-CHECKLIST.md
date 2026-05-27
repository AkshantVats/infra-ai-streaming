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
- **No committed secrets**: `.env` is documented but not committed; example files are safe; `.gitignore` covers credentials patterns.
- **Secrets scanning exists**: repository runs gitleaks in CI.
- **Production hardening documented**: `docs/SECURITY-HARDENING.md` (TLS, K8s secrets, no `.env` in prod).
- **Least-privilege defaults are explicit**: dev defaults are safe; production hardening is documented before real deployments.

## Observability (production expectations)

- **Metrics**: scrape endpoints exist for both ingestion and consumer; naming is stable and matches dashboards.
- **Build metadata**: `/health` exposes `version`, `git_sha`, `build_time` on ingestion and consumer.
- **Dashboards**: “Product SLOs” and “Local E2E” dashboards exist and are provisioned.
- **SLO doc**: `docs/SLOs.md` with latency, availability, lag targets and PromQL sketches.
- **Grafana queries are stable**: dashboards do not rely on untracked local state.

## Ops & incident response

- **Evergreen docs**: runtime docs avoid sprint-style noise; operational steps use stable terminology.
- **Runbooks exist for degraded scenarios**:
  - `docs/RUNBOOK.md` provides symptom → checks → actions.
  - `docs/DATA-RETENTION.md` covers ClickHouse TTL, WAL PVC, Kafka retention.
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
  - Secrets: gitleaks

- **End-to-end verification**
  - Local: `HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh` — uses `values-m1.yaml`.
  - CI (optional): `.github/workflows/e2e-k3d-dispatch.yml` — weekly cron + `workflow_dispatch`, 30m timeout.
  - Checklist: [`docs/E2E-CHECKLIST.md`](E2E-CHECKLIST.md).
  - Proof log: `docs/E2E-PROOF-K3D.md`.

- **Release process**: `RELEASE.md`, `CHANGELOG.md`, build metadata documented.

## Status

When running the full verification matrix, record the results into `docs/E2E-PROOF-K3D.md` (and keep the proof close to the commands).

## Explicit “not implemented yet” (be honest)

These are common production expectations that are **not** fully implemented in this repo today.

- **AuthN/AuthZ**: no first-class authentication model for `/ingest` (mTLS/OIDC/API keys) and no tenant auth boundary enforcement.
- **Encryption in production**: TLS termination and at-rest encryption guidance exist in `docs/SECURITY-HARDENING.md` but are not automated in charts.
- **Backups / restore**: retention documented in `docs/DATA-RETENTION.md`; no automated backup job or tested restore path in-tree.
- **Multi-cluster / HA**: no multi-AZ Kafka/ClickHouse/Redis topology and no tested disaster recovery path.
- **SLO-based alerting**: dashboards and `docs/SLOs.md` exist; Prometheus alert rules / escalation are not fully codified.
- **Production Kubernetes**: M1/k3d values only; no hardened prod `values-prod.yaml` or pen test.

## Next steps (prioritized)

1. Add a documented auth model for ingestion (API keys or OIDC gateway) and enforce per-tenant authorization.
2. Implement ClickHouse backup + restore drill; enable TTL in DDL for production profiles.
3. Codify Prometheus alert rules from `docs/SLOs.md` and wire to on-call.
