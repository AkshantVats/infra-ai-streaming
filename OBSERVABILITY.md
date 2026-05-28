# Observability — infra-ai-streaming

Local E2E stack: **Prometheus** (`:9090`), **Grafana** (`:3000`), **ClickHouse** (`:8123`), plus host-run **ingestion** (`:8080/metrics`) and **consumer** (`:9091/metrics`).

**Canonical mapping** (metrics ↔ dashboards ↔ source files): [docs/ARCHITECTURE-AND-FLOWS.md](docs/ARCHITECTURE-AND-FLOWS.md#6-observability-matrix). Runnable scenarios: [docs/END-TO-END-FLOWS.md](docs/END-TO-END-FLOWS.md), `./scripts/demo-flows.sh`.

## Metrics endpoints

| Service   | Port | Path      | Scrape job (Prometheus) |
|-----------|------|-----------|-------------------------|
| Ingestion | 8080 | `/metrics` | `ingestion` |
| Consumer  | 9091 | `/metrics` | `consumer` |

Prometheus runs in Docker and scrapes the host via `host.docker.internal` (see `deploy/prometheus/prometheus.yml`).

## Ingestion metrics

| Metric | Type | Meaning |
|--------|------|---------|
| `ingestion_latency_ms{tenant_id,status}` | Histogram | HTTP handler latency (success path) |
| `batch_size_events{tenant_id}` | Histogram | Events per accepted batch |
| `rate_limited_requests_total{tenant_id}` | Counter | HTTP 429 rate-limit denials |
| `backpressure_events_total` | Counter | HTTP 503 when internal channel full |
| `ingestion_validation_errors_total{error}` | Counter | HTTP 400 validation (`empty_batch`, `invalid_cost`, …) |
| `redis_rate_limit_degraded_total` | Counter | Rate limit checks that fell back to fail-open (Redis unavailable) |
| `kafka_produce_errors_total{tenant_id,error_type}` | Counter | Produce exhausted retries (DLQ attempted) |
| `wal_segments_pending` | Gauge | WAL segments with unacked entries |
| `wal_replay_events_total` | Counter | Events replayed from WAL on startup |

## Consumer metrics

| Metric | Type | Meaning |
|--------|------|---------|
| `kafka_records_processed_total` | Counter | Kafka records handed off after CH / overflow / DLQ |
| `kafka_deserialization_errors_total` | Counter | Bad JSON on topic; offset **not** committed |
| `kafka_record_handoff_errors_total` | Counter | `Accept` failed; offset **not** committed |
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

### Product SLO panels

| Panel | Source | Query |
|-------|--------|-------|
| Ingest throughput by tenant | Prometheus | `sum(rate(batch_size_events_sum[1m])) by (tenant_id)` |
| P99 inference latency by model | ClickHouse | `quantile(0.99)(latency_ms)` on `infra_ai.inference_events` by `model_id` |
| Cost per hour by tenant | ClickHouse | `sum(cost_usd)` by `toStartOfHour(timestamp)`, `tenant_id` |
| Kafka consumer lag | Prometheus | `sum(kafka_consumer_lag_events) by (topic)` |

**Note:** Panel 1 tracks **ingest accept**; panels 2–3 read **ClickHouse** after the consumer has written. Temporary skew is normal when lag is high.

## Rate limiting (per-tenant)

When `TENANT_LIMITS_PATH` is set, each tenant resolves its own `max_events_per_sec` / `burst_multiplier` from the JSON file. Unknown tenants fall back to the `default` entry (or env-level `RATE_LIMIT_DEFAULT_RPS`).

| What to check | How |
|---------------|-----|
| Denied requests by tenant | `rate(rate_limited_requests_total[5m])` grouped by `tenant_id` |
| Fail-open events (Redis down) | `rate(redis_rate_limit_degraded_total[5m])` — should be 0 normally |
| Per-tenant config in use | Check `TENANT_LIMITS_PATH` env var; file schema in `deploy/tenant-limits.example.json` |

Failure behavior and local reproduction: [CHAOS.md](CHAOS.md) (scenario 3 — Redis lost).

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

## Troubleshooting

See [docs/ARCHITECTURE-AND-FLOWS.md#8-troubleshooting](docs/ARCHITECTURE-AND-FLOWS.md#8-troubleshooting) for scrape failures, stuck partitions, empty Grafana CH panels, and breaker/overflow behavior.

## Logs

Structured `log.Printf` lines with `level=` and `msg=`:

- `consumer_started`, `metrics_server_started`
- `clickhouse_batch_failed`, `overflow_push_failed`, `dlq_publish_failed`
- `record_failed`, `commit_failed`

Rust ingestion uses the same key=value style where applicable.
