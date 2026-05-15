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
- **Local dependency stack**: Docker Compose runs **Redis**, **Redpanda** (Kafka-compatible API on **9092**), and **ClickHouse** for end-to-end local dev—not a separate native Kafka install on the host.

## Not production-complete yet (gaps)

- **No Go consumer** as source in this repository: the consumer, ClickHouse batch writer, circuit breaker, and Redis overflow path are specified in design and diagrams only.
- **No Helm / Kubernetes charts** checked in; deployment artifacts here are local-dev oriented (Docker Compose, observability placeholders).
- **End-to-end pipeline incomplete**: events can be ingested and produced to Kafka/Redpanda locally, but nothing in-tree yet consumes into ClickHouse.
- **Integration / load tests**: CI and `./scripts/test-ingestion.sh` run **unit tests**; full HTTP→Kafka E2E requires Compose (Redis + Redpanda) and a running binary—documented in README, not gated in CI yet.

## How to use this doc

- For **local development**, see [dev-macos.md](dev-macos.md) and [../deploy/docker-compose.yml](../deploy/docker-compose.yml). Compose services use `env_file: .env` under `deploy/` — copy [../deploy/.env.example](../deploy/.env.example) to `deploy/.env` first.
- For **contributing** (tests, compose, PR expectations), see [../CONTRIBUTING.md](../CONTRIBUTING.md).
