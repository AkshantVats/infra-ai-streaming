# Production Readiness Checklist

This checklist is intended for OSS/prod-readiness review. It is not a substitute for tests or real production validation, but it makes the repo’s operational posture explicit.

## Scope

Applies to the full stack described in `README.md`:
Rust ingestion (`ingestion/`), Go consumer (`consumer/`), Kafka-compatible broker (Redpanda), ClickHouse, Redis, and the local Docker/Kubernetes deployment paths.

## Must-have operational guarantees

1. **Durability on ingest acknowledgment**
   - Ingestion returns success only after WAL append + fsync, and after enqueuing to the bounded producer channel.
   - Replay exists for un-acked WAL entries on restart.

2. **Backpressure and honest overload behavior**
   - Bounded internal queue with deterministic HTTP overload response (`503` + `Retry-After`).
   - No unbounded in-memory growth on the hot path.

3. **Consumer handoff semantics**
   - Consumer offsets advance only after ClickHouse insert / overflow drain / DLQ handoff for the relevant record.
   - Poison/deserialize failures do not silently commit offsets.

4. **Failure-mode observability**
   - Each failure mode emits at least one Prometheus metric or clearly named log key.
   - Dashboards exist for “healthy”, “degraded”, and “failure recovery” situations.

## Repo hygiene (OSS readiness)

5. **Evergreen docs**
   - Runtime docs avoid sprint-style noise; operational steps use stable terminology.
   - Architecture docs reference components and data flows without sprint noise.

6. **Runbooks exist for degraded scenarios**
   - `CHAOS.md` provides reproducible commands and expected metric signals.
   - `docs/ARCHITECTURE-AND-FLOWS.md#Troubleshooting` links the common failure checks.

## Build/test/release gates

7. **Test matrix is runnable locally**
   - Rust ingestion: `cargo test -p ingestion`
   - Go consumer: `cd consumer && go test ./...`
   - Shell scripts: `bash -n` over relevant scripts

8. **Kubernetes templating gate (M1-first)**
   - Helm chart renders with **`values-m1.yaml`** without template errors.
   - Do **not** use default `values.yaml` or `values-k3d.yaml` for laptop k3d runs (OOM / long Helm waits).

9. **End-to-end verification is automated (M1)**
   - `HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh` — compose down first; uses `values-m1.yaml` only.
   - Checklist: [`docs/E2E-CHECKLIST.md`](E2E-CHECKLIST.md).
   - Proof log is appended to `docs/E2E-PROOF-K3D.md`.

## Security & secrets posture

10. **No committed secrets**
   - `.env` is documented but not committed; example files are safe.

11. **Least-privilege defaults**
   - ClickHouse and credentials are scoped to local dev defaults; production hardening should be documented before real deployments.

## Observability requirements (product-grade expectations)

12. **Metrics**
   - Scrape endpoints exist for both ingestion and consumer.
   - Metrics naming is stable and matches dashboard queries.

13. **Dashboards**
   - “Product SLOs” and “Local E2E” dashboards exist and are provisioned.

14. **Grafana queries are stable**
   - Panels reference canonical metric names / queries; dashboards should not rely on untracked local state.

## Status

When running the full verification matrix, record the results into `docs/E2E-PROOF-K3D.md` (and keep the proof close to the commands).

## Known gaps (prioritized)

1. **CI coverage parity**
   - Integration/E2E (Compose + consumer) are currently local/scheduled scripts; if the repo targets “serious production”, wire key E2E checks into CI.

2. **Production hardening items**
   - Add production-grade authN/authZ guidance (ingest endpoint access, tenant isolation model, and rate-limit/fairness policy under Redis outages).

