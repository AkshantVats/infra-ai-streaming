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
- **Go consumer skeleton** (`consumer/`): franz-go reader on `ai_inference_events`, deserializes `{"events":[...]}`, logs each event to stdout. Unit tests for JSON deserialize.
- **Local dependency stack**: Docker Compose runs **Redis**, **Redpanda**, **ClickHouse**, **Prometheus**, **Grafana**, plus one-shot **redpanda-init** (topics) and **clickhouse-init** (DDL). See [deploy/README.md](../deploy/README.md) for ports.

## Not production-complete yet (gaps)

- **ClickHouse writer from Go**: batch insert, circuit breaker, Redis overflow, DLQ consumer logic — **Day 5**.
- **Go consumer metrics HTTP server** (`:9091`): deferred to Day 5+.
- **No Helm / Kubernetes charts** checked in; deployment artifacts here are local-dev oriented.
- **End-to-end warehouse path incomplete**: events reach Kafka and consumer stdout; `infra_ai.inference_events` row count stays **0** until the writer lands.
- **Integration / load tests**: CI runs **Rust unit tests** only; compose E2E and `go test ./consumer/...` are local (`scripts/smoke-e2e.sh`).

## E2E status (Day 4)

| Step | Status |
|------|--------|
| `docker compose up` — 5 long-running services + 2 init jobs | Implemented |
| Topics `ai_inference_events`, `ai_inference_dlq` | `redpanda-init` |
| `curl /ingest` → 202 | Requires running ingestion binary |
| Go consumer stdout with `cost_usd` | Requires running consumer + ingest |
| Prometheus scrape ingestion `:8080/metrics` | Compose + running ingestion |

## How to use this doc

- For **local development**, see [dev-macos.md](dev-macos.md) and [../deploy/docker-compose.yml](../deploy/docker-compose.yml). Compose services use `env_file: .env` under `deploy/` — copy [../deploy/.env.example](../deploy/.env.example) to `deploy/.env` first.
- For **contributing** (tests, compose, PR expectations), see [../CONTRIBUTING.md](../CONTRIBUTING.md).
