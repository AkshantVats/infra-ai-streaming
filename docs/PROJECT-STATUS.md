# Project status (honest audit)

This document states what exists in the repository today versus what is still design or backlog. It is meant for operators and contributors, not marketing.

## Production-grade in tree today

- **LICENSE**: MIT, suitable for open source distribution.
- **DESIGN.md**: Architecture and data-plane decisions are documented at a level sufficient to implement the full stack.
- **CI** (`.github/workflows/ci.yml`): GitHub Actions runs `cargo test -p ingestion` on pushes and pull requests to `main`, with system packages needed for `rdkafka`'s cmake build.
- **Cargo.lock**: Committed for reproducible Rust builds of the ingestion crate.
- **Ingestion Rust library + binary**: Builds on toolchain **1.86** (`rust-toolchain.toml`). Unit tests cover configuration defaults, WAL behavior, rate limiting primitives, and related units under `ingestion/`.
- **Runnable `ingestion` binary** (`cargo build -p ingestion --bin ingestion` / `cargo run -p ingestion`): Axum HTTP server on `HTTP_PORT` (default **8080**) with `GET /health`, `GET /metrics`, and `POST /ingest`.
- **WAL + Kafka produce path**: Handler appends to a segment WAL (`WAL_DIR`, default `/tmp/wal`) before enqueue; a background task drives an **rdkafka** producer to `KAFKA_TOPIC` with DLQ on persistent failure; WAL entries are **mark_acked** after broker delivery. Startup **replays unacked** WAL entries into the channel.
- **Redis rate limiting**: Per-tenant token bucket on ingest (fail-open when Redis is unavailable, per design).
- **Go consumer** (`consumer/`): franz-go reader → ClickHouse **BatchWriter** (1000 events / 500ms), **circuit breaker**, **Redis overflow**, **DLQ** (`ai_inference_dlq`), Prometheus on **`:9091`**. Unit tests for JSON, breaker, row mapping.
- **Day 12 G-09**: z-score inference latency anomaly detection in the Go consumer, publishing to **`ai_anomalies`** and exposing **`anomalies_detected_total`** with a Grafana alert rule.
- **OBSERVABILITY.md**: Metrics catalog, SLO sketches, ClickHouse verification queries.
- **docs/ARCHITECTURE-AND-FLOWS.md**: Architecture, G-01..G-07 status, code walkthrough, lifecycle paths, dual-dashboard observability matrix, troubleshooting.
- **Local dependency stack**: Docker Compose runs **Redis**, **Redpanda**, **ClickHouse**, **Prometheus**, **Grafana**, plus one-shot **redpanda-init** (topics) and **clickhouse-init** (DDL). See [deploy/README.md](../deploy/README.md) for ports.
- **Helm / k3d (G-07):** Umbrella chart `deploy/helm/lensai/`, Dockerfiles, k3d config, consumer HPA on `kafka_consumer_lag_sum` via prometheus-adapter. Three-command path in [deploy/README.md](../deploy/README.md).

## OSS contributions (Day 11 — OSS-01)

- **Vector** [#25455](https://github.com/vectordotdev/vector/issues/25455) — memory enrichment counter `_total` suffix fix: [PR #25496](https://github.com/vectordotdev/vector/pull/25496) (open).

## Not production-complete yet (gaps)

- **Integration / load tests in CI**: CI runs **Rust unit tests** only; compose E2E and `go test ./consumer/...` are local (`scripts/smoke-e2e.sh`).
- **Partition key:** producer keys by `tenant_id` only; DESIGN target `hash(tenant_id:model_id)` not implemented.

## E2E status (Day 5–6)

| Step | Status |
|------|--------|
| `docker compose up` — 5 long-running services + 2 init jobs | Implemented |
| Topics `ai_inference_events`, `ai_inference_dlq` | `redpanda-init` |
| `curl /ingest` → 202 | Requires running ingestion binary |
| ClickHouse `infra_ai.inference_events` rows with `cost_usd` | Requires running consumer + ingest |
| Prometheus scrape ingestion `:8080` and consumer `:9091` | Compose + host binaries |
| Grafana **Local E2E** ops dashboard | Provisioned (`ai-inference-e2e-local`) |
| Grafana **Product SLOs** dashboard (G-05) | Provisioned (`ai-inference-product`) — 4 panels: ingest eps, CH P99 by model, cost/hour, `kafka_consumer_lag_events` |
| `kafka_consumer_lag_events` on consumer `:9091` | Implemented in Go reader |
| k3d + Helm full stack + lag HPA (G-07) | `deploy/helm/lensai/`, `scripts/smoke-k8s-e2e.sh` (local verify) |

## 3-step local demo (HTTP → Kafka → ClickHouse)

1. **Stack:** `cp deploy/.env.example deploy/.env && docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d` — Grafana http://localhost:3000 (`admin`/`admin`) → **Local E2E** (`/d/ai-inference-e2e-local`) or **Product SLOs** (`/d/ai-inference-product`).
2. **Pipeline:** Terminal A — `cd consumer && set -a && source ../deploy/.env && set +a && go run ./cmd/consumer`. Terminal B — `set -a && source deploy/.env && set +a && cargo run -p ingestion`.
3. **Proof:** `curl -X POST http://localhost:8080/ingest …` (see root `README.md`); ClickHouse — `SELECT count(), max(cost_usd) FROM infra_ai.inference_events`; Grafana consumer panels + Prometheus `up{job="consumer"}`.

Or: `./scripts/smoke-e2e.sh` (compose + tests; ingest + CH check if services are up).

## How to use this doc

- For **local development**, see [dev-macos.md](dev-macos.md) and [../deploy/docker-compose.yml](../deploy/docker-compose.yml). Compose services use `env_file: .env` under `deploy/` — copy [../deploy/.env.example](../deploy/.env.example) to `deploy/.env` first.
- For **observability**, see [../OBSERVABILITY.md](../OBSERVABILITY.md).
- For **architecture and observability mapping**, see [ARCHITECTURE-AND-FLOWS.md](ARCHITECTURE-AND-FLOWS.md).
- For **E2E flows, Grafana panel guide, and demo scenarios**, see [END-TO-END-FLOWS.md](END-TO-END-FLOWS.md) and `./scripts/demo-flows.sh`.
- For **contributing** (tests, compose, PR expectations), see [../CONTRIBUTING.md](../CONTRIBUTING.md).
