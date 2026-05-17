# Observability — infra-ai-streaming

Local E2E stack: **Prometheus** (`:9090`), **Grafana** (`:3000`), **ClickHouse** (`:8123`), plus host-run **ingestion** (`:8080/metrics`) and **consumer** (`:9091/metrics`).

## Metrics endpoints

| Service   | Port | Path      | Scrape job (Prometheus) |
|-----------|------|-----------|-------------------------|
| Ingestion | 8080 | `/metrics` | `ingestion` |
| Consumer  | 9091 | `/metrics` | `consumer` |

Prometheus runs in Docker and scrapes the host via `host.docker.internal` (see `deploy/prometheus/prometheus.yml`).

## Consumer metrics (Day 5)

| Metric | Type | Meaning |
|--------|------|---------|
| `kafka_records_processed_total` | Counter | Kafka records handed off after CH / overflow / DLQ |
| `clickhouse_write_errors_total` | Counter | Failed CH batch inserts (before overflow/DLQ) |
| `clickhouse_batch_size` | Histogram | Events per successful CH insert |
| `clickhouse_flush_duration_seconds` | Histogram | Wall time per CH flush attempt |
| `circuit_breaker_state{state}` | Gauge | `1` on active state (`closed`, `open`, `halfopen`) |
| `redis_overflow_depth` | Gauge | Redis LIST length (`REDIS_OVERFLOW_KEY`) |
| `dlq_events_total` | Counter | Events published to `ai_inference_dlq` |

## Circuit breaker

- **5** consecutive insert failures → **open** (skip CH, push to Redis overflow).
- **30s** after open → **half-open** (try CH again).
- **1** success in half-open → **closed**.
- Overflow **drain** every **5s** (up to **5000** events) when breaker is closed.

## Batch writer

- Flush when buffer reaches **1000** events **or** every **500ms**.
- **3** insert retries per batch; then per-event **DLQ** publish.
- Kafka offsets commit only after all events in the record are handed off.

## Grafana

- Dashboard: **AI Inference Observability — Local E2E** (`uid: ai-inference-e2e-local`)
- URL: http://localhost:3000/d/ai-inference-e2e-local (admin / admin by default)
- Panels: scrape UP, consumer batch/flush, circuit breaker, overflow depth, DLQ, ClickHouse row count (when datasource is healthy).

## Useful queries

**ClickHouse — recent rows with cost:**

```sql
SELECT tenant_id, model_id, cost_usd, timestamp
FROM infra_ai.inference_events
ORDER BY timestamp DESC
LIMIT 10;
```

**Prometheus — consumer scrape:**

```promql
up{job="consumer"}
```

**Prometheus — overflow depth:**

```promql
redis_overflow_depth
```

## SLO sketches (local dev)

| SLO | PromQL (illustrative) |
|-----|------------------------|
| Ingestion available | `up{job="ingestion"} == 1` |
| Consumer available | `up{job="consumer"} == 1` |
| Breaker rarely open | `avg_over_time(circuit_breaker_state{state="open"}[1h]) < 0.01` |
| DLQ quiet | `rate(dlq_events_total[5m]) == 0` |

## Logs

Structured `log.Printf` lines with `level=` and `msg=`:

- `consumer_started`, `metrics_server_started`
- `clickhouse_batch_failed`, `overflow_push_failed`, `dlq_publish_failed`
- `record_failed`, `commit_failed`

Rust ingestion uses the same key=value style where applicable.
