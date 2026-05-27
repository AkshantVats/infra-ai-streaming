# Architecture — infra-ai-streaming (evergreen)

This is a short, stable reference for what the system **is**, what it **guarantees**, and where to look for deeper details.

For the full walkthrough (diagrams, event lifecycle, troubleshooting matrix), see `docs/ARCHITECTURE-AND-FLOWS.md`.

---

## Components

- **`ingestion/` (Rust)**: HTTP edge (`/ingest`) with per-tenant validation + rate limiting, durability via WAL, and Kafka producer.
- **Redpanda (Kafka-compatible broker)**: durable event log for `ai_inference_events` (+ DLQ topic).
- **`consumer/` (Go)**: Kafka reader with explicit handoff semantics and a ClickHouse batch writer (breaker + overflow + DLQ).
- **ClickHouse**: analytics storage (`infra_ai.inference_events`).
- **Redis**: rate limiting (ingestion) and overflow buffer (consumer) during ClickHouse outages.
- **Prometheus/Grafana**: metrics + dashboards.

---

## Data flow (high level)

1. Client `POST /ingest` → ingestion validates payload and enforces rate limits.
2. Ingestion appends to WAL (fsync) → produces to Kafka → WAL is acked on delivery success.
3. Consumer reads Kafka → batches inserts to ClickHouse.
4. If ClickHouse is unhealthy, consumer breaker opens and it hands off to Redis overflow (or DLQ) and **only then** commits offsets.

---

## Operational posture (what we optimize for)

- **Availability at ingest edge**: fail-open rate limit on Redis errors; explicit overload responses (`503` with `Retry-After`) instead of unbounded queues.
- **Durability on ack**: ingest success is only returned after WAL fsync and safe enqueue.
- **At-least-once delivery**: WAL replay and Kafka retries can cause duplicates; dedupe is a downstream concern.
- **Honest backpressure**: bounded channels, fast failure under overload.
- **Failure-mode observability**: every “expected failure mode” should emit a metric and/or stable log key.

---

## Where to look

- **Runbook**: `docs/RUNBOOK.md`
- **E2E (k3d, M1-safe)**: `./scripts/e2e-k3d-full.sh` + `docs/E2E-PROOF-K3D.md`
- **Ops & metrics catalog**: `OBSERVABILITY.md`
- **Deployment**: `deploy/README.md`, `deploy/helm/lensai/`
