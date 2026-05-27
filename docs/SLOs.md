# Service level objectives (SLOs)

Honest targets for the **local / M1 k3d** stack and the design goals for production.
Adjust numbers when you have sustained load-test evidence.

## Availability

| Objective | Target (design) | Measurement |
|-----------|-----------------|-------------|
| Ingestion HTTP availability | **99.5%** monthly (single-region, non-HA dev stack) | `sum(rate(http_requests_total{path="/ingest",status!~"5.."}[5m])) / sum(rate(http_requests_total{path="/ingest"}[5m]))` on ingestion `:8080/metrics` |
| Consumer process up | **99.5%** | `up{job="consumer"}` or `up{namespace="lensai",pod=~".*consumer.*"}` |
| End-to-end “event accepted → row in ClickHouse” | **99%** (excludes client retries) | Compare `ingestion_batches_accepted_total` growth to `clickhouse_rows_inserted_total` (lag bounded by batch window + lag SLO below) |

**Error budget note:** Planned maintenance (Helm upgrades, k3d recycle) should be excluded from customer-facing SLOs in production; document maintenance windows in the runbook.

## Latency (ingestion hot path)

| Objective | Target | Measurement |
|-----------|--------|-------------|
| Ingest **server-side** P99 (validate + WAL + enqueue) | **< 100 ms** at moderate load | `histogram_quantile(0.99, sum(rate(ingestion_request_duration_seconds_bucket[5m])) by (le))` |
| Ingest P50 | **< 20 ms** (laptop / M1 values) | Same histogram, `quantile=0.50` |

Overload responses (`503` + `Retry-After`) are **correct behavior**, not SLO violations.

## Consumer lag & freshness

| Objective | Target | Measurement |
|-----------|--------|-------------|
| Kafka consumer lag (events) | **< 10k events** steady-state; **< 60k** during burst | `kafka_consumer_lag_events` on consumer `:9091/metrics` |
| Time from ingest to ClickHouse row (freshness) | **< 2 min** P99 under normal load | Approximate: lag events / ingest rate, or compare max(`timestamp_unix_ms`) in CH vs wall clock |

## Data durability (not latency)

| Objective | Target | Measurement |
|-----------|--------|-------------|
| Accepted batch durable before HTTP 202 | **100%** (WAL fsync + channel enqueue) | WAL replay count on restart; `wal_unacked_entries` gauge |
| At-least-once to Kafka | **100%** of accepted batches eventually produced or DLQ | `kafka_produce_errors_total`, DLQ depth |

## Dashboards

- **Product SLOs:** Grafana `ai-inference-product` — tenant throughput, P99 by model, cost/hour, lag.
- **Ops / E2E:** Grafana `ai-inference-e2e-local` — breaker, overflow, DLQ, WAL.

## Alerting (placeholder)

Production should wire Prometheus alert rules (not fully codified in-repo yet):

- `kafka_consumer_lag_events > 50000` for 10m → page on-call.
- `circuit_breaker_state == 2` (open) for 5m → warn.
- `up{job="ingestion"} == 0` for 2m → page.

See [RUNBOOK.md](RUNBOOK.md) for response steps and [PRODUCTION-READINESS-CHECKLIST.md](PRODUCTION-READINESS-CHECKLIST.md) for gaps.
