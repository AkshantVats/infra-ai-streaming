# Deploy (local dependencies)

[`docker-compose.yml`](docker-compose.yml) runs the full local stack:

| Service | Host ports | Purpose |
|---------|------------|---------|
| **redis** | 6379 | Rate limiting (ingestion) |
| **redpanda** | 9092 (Kafka API), 9644 (admin) | Event bus |
| **redpanda-init** | — (one-shot) | Creates `ai_inference_events`, `ai_inference_dlq` |
| **clickhouse** | 8123 (HTTP), 9000 (native) | Analytical store |
| **clickhouse-init** | — (one-shot) | Applies `clickhouse/init.sql` |
| **prometheus** | 9090 | Scrapes ingestion `/metrics` on host `:8080` |
| **grafana** | 3000 | Dashboards (default `admin` / `admin`) |

## Quick start

```bash
cp .env.example .env
docker compose --env-file .env -f docker-compose.yml up -d
```

From the repository root (same effect, explicit paths):

```bash
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

Wait until long-running services are healthy (~2 minutes on first pull):

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps
```

Verify topics after `redpanda-init` completes:

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml run --rm redpanda \
  rpk topic list --brokers redpanda:9092
```

## End-to-end order (Day 4)

1. Compose up (this file).
2. Terminal A: `go run ./consumer/cmd/consumer` (from repo root; see `consumer/README.md`).
3. Terminal B: `cargo run -p ingestion`.
4. Terminal C: `curl` to `POST http://localhost:8080/ingest` (see root `README.md`).

Or run [`../scripts/smoke-e2e.sh`](../scripts/smoke-e2e.sh) for a compose + ingest smoke check.

## Prometheus scrape

Ingestion exposes Prometheus metrics on **`HTTP_PORT` (default 8080)** at `/metrics`, not a separate `:9090` listener on the Rust binary. Prometheus in compose scrapes `host.docker.internal:8080` (Linux: `extra_hosts: host-gateway` on the prometheus service).

## Resource note

ClickHouse + Grafana together need roughly **8 GB** Docker RAM for comfortable local dev. For consumer-only testing, you can start Redis + Redpanda only:

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d redis redpanda redpanda-init
```

Environment variable names for the ingestion **binary** match [`deploy/.env.example`](.env.example) and [`ingestion/src/config.rs`](../ingestion/src/config.rs). Do not commit `.env` (gitignored).
