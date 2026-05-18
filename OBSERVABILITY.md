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
| `kafka_consumer_lag_events{topic,partition,group}` | Gauge | High watermark minus committed offset (events) per partition |

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

Two provisioned dashboards (see `dashboards/` for canonical JSON; mirrored under `deploy/grafana/provisioning/dashboards/`).

| Dashboard | UID | URL | Use when |
|-----------|-----|-----|----------|
| **AI Inference Observability — Local E2E** | `ai-inference-e2e-local` | http://localhost:3000/d/ai-inference-e2e-local | Proving pipeline health (scrape UP, breaker, overflow, DLQ, WAL) |
| **AI Inference — Product SLOs** | `ai-inference-product` | http://localhost:3000/d/ai-inference-product | Tenant throughput, inference P99 by model, cost/hour, consumer lag |

Credentials: `admin` / `admin` (from `deploy/.env`).

### Product SLO panels (G-05)

| Panel | Source | Query |
|-------|--------|-------|
| Ingest throughput by tenant | Prometheus | `sum(rate(batch_size_events_sum[1m])) by (tenant_id)` |
| P99 inference latency by model | ClickHouse | `quantile(0.99)(latency_ms)` on `infra_ai.inference_events` by `model_id` |
| Cost per hour by tenant | ClickHouse | `sum(cost_usd)` by `toStartOfHour(timestamp)`, `tenant_id` |
| Kafka consumer lag | Prometheus | `sum(kafka_consumer_lag_events) by (topic)` |

**Note:** Panel 1 tracks **ingest accept**; panels 2–3 read **ClickHouse** after the consumer has written. Temporary skew is normal when lag is high.

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

**Prometheus — consumer lag (events):**

```promql
sum(kafka_consumer_lag_events) by (topic)
max(kafka_consumer_lag_events) by (partition)
```

## SLO sketches (local dev)

| SLO | PromQL (illustrative) |
|-----|------------------------|
| Ingestion available | `up{job="ingestion"} == 1` |
| Consumer available | `up{job="consumer"} == 1` |
| Breaker rarely open | `avg_over_time(circuit_breaker_state{state="open"}[1h]) < 0.01` |
| DLQ quiet | `rate(dlq_events_total[5m]) == 0` |
| Consumer lag &lt; 50k events | `max(kafka_consumer_lag_events) < 50000` |

## Logs

Structured `log.Printf` lines with `level=` and `msg=`:

- `consumer_started`, `metrics_server_started`
- `clickhouse_batch_failed`, `overflow_push_failed`, `dlq_publish_failed`
- `record_failed`, `commit_failed`

Rust ingestion uses the same key=value style where applicable.
