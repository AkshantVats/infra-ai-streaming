# Deploy (local dependencies)

[`docker-compose.yml`](docker-compose.yml) runs **Redis**, **Redpanda** (Kafka-compatible API on host port **9092**), and **ClickHouse** (**8123** HTTP, **9000** native) for local development.

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

Environment variable names for the future ingestion **binary** match [`deploy/.env.example`](.env.example) and [`ingestion/src/config.rs`](../ingestion/src/config.rs). Do not commit `.env` (gitignored).

`grafana/` and `prometheus/` at repo root are placeholders for future scrape configs and dashboard exports.
