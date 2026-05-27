# Data retention and backup

Operational notes for **ClickHouse**, **ingestion WAL**, and **Kafka/Redpanda** in this repo.
Production clusters need explicit policies; defaults below match the M1 / local charts.

## ClickHouse (`infra_ai.inference_events`)

- **Schema:** [`deploy/clickhouse/init.sql`](../deploy/clickhouse/init.sql) — `MergeTree` ordered by `(tenant_id, model_id, timestamp_unix_ms)`.
- **TTL:** Not enabled in the shipped DDL. For production, add a table TTL on `timestamp_unix_ms` (example: 90 days raw, longer for rollup MVs).
- **Backups:** No automated backup job in-tree. Recommended for production:
  - `clickhouse-backup` or object-storage snapshots of data volume.
  - Test **restore** quarterly; document RPO/RTO in your org runbook.
- **Local/k3d:** Ephemeral PVC or emptyDir — **data is disposable** after cluster delete.

Example TTL (not applied by default):

```sql
ALTER TABLE infra_ai.inference_events
  MODIFY TTL toDateTime(timestamp_unix_ms / 1000) + INTERVAL 90 DAY;
```

## Ingestion WAL (Rust)

- **Location:** `WAL_DIR` (default `/tmp/wal` locally; `/data/wal` in Helm with a PVC when enabled).
- **Retention:** Segments remain until **Kafka ack** (`mark_acked`); unacked entries replay on restart.
- **Disk:** Size grows with unacked volume during broker outages — monitor PVC usage in Kubernetes.
- **Backup:** WAL is a **short-term durability buffer**, not the system of record; Kafka/ClickHouse hold authoritative history after ack.

## Kafka / Redpanda topics

| Topic | Role | Retention (typical) |
|-------|------|---------------------|
| `ai_inference_events` | Primary stream | Broker default (often 7d local); tune `retention.ms` for compliance |
| `ai_inference_dlq` | Poison / failed batches | Longer retention (30d+) for investigation |
| `ai_anomalies` | Z-score anomaly events | Align with events topic or shorter if high volume |

Helm/Compose init scripts create topics; production should set explicit `retention.bytes` / `retention.ms` per tier.

## Redis

- **Rate-limit keys:** ephemeral (token bucket state).
- **Overflow LIST (`REDIS_OVERFLOW_KEY`):** drained when ClickHouse path is healthy; monitor `redis_overflow_depth` — persistent depth indicates sustained CH degradation.

## What to do before production

1. Define **legal/compliance** retention per tenant tier.
2. Enable ClickHouse TTL or tiered storage + backups.
3. Set Kafka retention ≥ max acceptable replay window for consumer rebuilds.
4. Document restore drill in [RUNBOOK.md](RUNBOOK.md).
