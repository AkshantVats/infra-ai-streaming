# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Go consumer Kafka reader skeleton (stdout logging, JSON batch deserialize).
- Local Docker stack: Redis, Redpanda, ClickHouse, Prometheus, Grafana provisioning.
- Grafana dashboard **AI Inference Observability — Local E2E** (`deploy/grafana/provisioning/dashboards/ai-inference-e2e.json`).
- E2E smoke script and README quickstart for HTTP → Kafka → consumer stdout.

### Changed

- CI runs `go test ./...` in `consumer/` alongside Rust ingestion tests.

## [0.1.0-dev] — 2026-05-16

Early development preview: Rust ingestion library and binary (WAL, Kafka produce, rate limiting), design docs, and local compose stack. ClickHouse writer and production consumer metrics are not complete yet — see [docs/PROJECT-STATUS.md](docs/PROJECT-STATUS.md).
