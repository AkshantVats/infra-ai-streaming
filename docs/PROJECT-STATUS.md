# Project status (honest audit)

This document states what exists in the repository today versus what is still design or backlog. It is meant for operators and contributors, not marketing.

## Production-grade in tree today

- **LICENSE**: MIT, suitable for open source distribution.
- **DESIGN.md**: Architecture and data-plane decisions are documented at a level sufficient to implement the full stack.
- **CI** (`.github/workflows/ci.yml`): GitHub Actions runs `cargo test -p ingestion` on pushes and pull requests to `main`, with system packages needed for `rdkafka`’s cmake build.
- **Cargo.lock**: Committed for reproducible Rust builds of the ingestion crate.
- **Ingestion Rust library**: Builds on toolchain **1.86** (`rust-toolchain.toml`). Tests cover configuration defaults, WAL behavior, rate limiting primitives, and related units as implemented under `ingestion/`.

## Not production-complete yet (gaps)

- **No HTTP server wired as a runnable binary** in this repo: configuration and libraries anticipate Axum and an `/ingest` path, but there is no committed `main` / server binary that exposes HTTP in production form.
- **No Kafka producer integration** in a runnable ingestion path: broker settings exist in `Config`, but producer wiring and topic lifecycle are not the full Day-1 pipeline described in `DESIGN.md`.
- **No Go consumer** as source in this repository: the consumer, ClickHouse batch writer, circuit breaker, and Redis overflow path are specified in design and diagrams only.
- **No Helm / Kubernetes charts** checked in; deployment artifacts here are local-dev oriented (Docker Compose, observability placeholders).

## How to use this doc

- For **local development**, see [dev-macos.md](dev-macos.md) and [../deploy/docker-compose.yml](../deploy/docker-compose.yml).
- For **contributing** (tests, compose, PR expectations), see [../CONTRIBUTING.md](../CONTRIBUTING.md).
