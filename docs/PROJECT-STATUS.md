# Project status (honest audit)

This document states what exists in the repository today versus what is still design or backlog. It is meant for operators and contributors, not marketing.

## Production-grade in tree today

- **LICENSE**: MIT, suitable for open source distribution.
- **DESIGN.md**: Architecture and data-plane decisions are documented at a level sufficient to implement the full stack.
- **CI** (`.github/workflows/ci.yml`): Rust fmt/clippy/test, Go test, Helm template, shellcheck, gitleaks on PRs and `main`.
- **E2E k3d workflow** (`.github/workflows/e2e-k3d-dispatch.yml`): weekly + manual full M1 E2E (not on every PR).
- **Cargo.lock**: Committed for reproducible Rust builds of the ingestion crate.
- **Ingestion Rust library + binary**: Builds on toolchain **1.86** (`rust-toolchain.toml`). Unit tests cover configuration defaults, WAL behavior, rate limiting primitives, and related units under `ingestion/`.
- **Runnable `ingestion` binary** (`cargo build -p ingestion --bin ingestion` / `cargo run -p ingestion`): Axum HTTP server on `HTTP_PORT` (default **8080**) with `GET /health` (version + git SHA + build time), `GET /metrics`, and `POST /ingest`.
- **WAL + Kafka produce path**: Handler appends to a segment WAL (`WAL_DIR`, default `/tmp/wal`) before enqueue; a background task drives an **rdkafka** producer to `KAFKA_TOPIC` with DLQ on persistent failure; WAL entries are **mark_acked** after broker delivery. Startup **replays unacked** WAL entries into the channel.
- **Redis rate limiting**: Per-tenant token bucket on ingest (fail-open when Redis is unavailable, per design).
- **Go consumer** (`consumer/`): franz-go reader → ClickHouse **BatchWriter** (1000 events / 500ms), **circuit breaker**, **Redis overflow**, **DLQ** (`ai_inference_dlq`), Prometheus on **`:9091`**. **Z-score latency anomaly detection** publishes to **`ai_anomalies`**. Unit tests for JSON, breaker, row mapping.
- **OBSERVABILITY.md**: Metrics catalog, SLO sketches, ClickHouse verification queries.
- **docs/SLOs.md**, **docs/DATA-RETENTION.md**, **docs/SECURITY-HARDENING.md**: production posture docs (honest placeholders where not implemented).
- **docs/ARCHITECTURE-AND-FLOWS.md**: Architecture, milestone status, code walkthrough, lifecycle paths, dual-dashboard observability matrix, troubleshooting.
- **Local dependency stack**: Docker Compose runs **Redis**, **Redpanda**, **ClickHouse**, **Prometheus**, **Grafana**, plus one-shot **redpanda-init** (topics) and **clickhouse-init** (DDL). See [deploy/README.md](../deploy/README.md) for ports.
- **Helm / k3d:** Umbrella chart `deploy/helm/lensai/`, Dockerfiles, k3d config, consumer HPA on `kafka_consumer_lag_sum` via prometheus-adapter. Three-command path in [deploy/README.md](../deploy/README.md).

## External OSS contributions

- **Vector** [#25455](https://github.com/vectordotdev/vector/issues/25455) — memory enrichment counter `_total` suffix fix: [PR #25496](https://github.com/vectordotdev/vector/pull/25496) (open).

## Not production-complete yet (gaps)

- **AuthN/AuthZ** on `/ingest` (API keys, OIDC gateway, mTLS).
- **Multi-region / HA** Kafka, ClickHouse, Redis topologies.
- **Automated ClickHouse backup/restore** in-tree.
- **Partition key:** producer keys by `tenant_id` only; DESIGN target `hash(tenant_id:model_id)` not implemented.

## E2E status

| Step | Status |
|------|--------|
| `docker compose up` — long-running services + init jobs | Implemented |
| Topics `ai_inference_events`, `ai_inference_dlq`, `ai_anomalies` | `redpanda-init` / Helm init |
| `curl /ingest` → 202 | Requires running ingestion binary |
| ClickHouse `infra_ai.inference_events` rows with `cost_usd` | Requires running consumer + ingest |
| Prometheus scrape ingestion `:8080` and consumer `:9091` | Compose + host binaries |
| Grafana **Local E2E** ops dashboard | Provisioned (`ai-inference-e2e-local`) |
| Grafana **Product SLOs** dashboard | Provisioned (`ai-inference-product`) |
| `kafka_consumer_lag_events` on consumer `:9091` | Implemented in Go reader |
| k3d + Helm full stack + lag HPA | `deploy/helm/lensai/`, `./scripts/e2e-k3d-full.sh` |

## 3-step local demo (HTTP → Kafka → ClickHouse)

1. **Stack:** `cp deploy/.env.example deploy/.env && docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d` — Grafana http://localhost:3000 (`admin`/`admin`) → **Local E2E** (`/d/ai-inference-e2e-local`) or **Product SLOs** (`/d/ai-inference-product`).
2. **Pipeline:** Terminal A — `cd consumer && set -a && source ../deploy/.env && set +a && go run ./cmd/consumer`. Terminal B — `set -a && source deploy/.env && set +a && cargo run -p ingestion`.
3. **Proof:** `curl -X POST http://localhost:8080/ingest …` (see root `README.md`); ClickHouse — `SELECT count(), max(cost_usd) FROM infra_ai.inference_events`; Grafana consumer panels + Prometheus `up{job="consumer"}`.

Or: `./scripts/smoke-e2e.sh` (compose + tests; ingest + CH check if services are up).

## How to use this doc

- For **local development**, see [dev-macos.md](dev-macos.md) and [../deploy/docker-compose.yml](../deploy/docker-compose.yml). Compose services use `env_file: .env` under `deploy/` — copy [../deploy/.env.example](../deploy/.env.example) to `deploy/.env` first.
- For **observability**, see [../OBSERVABILITY.md](../OBSERVABILITY.md) and [SLOs.md](SLOs.md).
- For **architecture and observability mapping**, see [ARCHITECTURE-AND-FLOWS.md](ARCHITECTURE-AND-FLOWS.md).
- For **E2E flows, Grafana panel guide, and demo scenarios**, see [END-TO-END-FLOWS.md](END-TO-END-FLOWS.md) and `./scripts/demo-flows.sh`.
- For **contributing** (tests, compose, PR expectations), see [../CONTRIBUTING.md](../CONTRIBUTING.md).
