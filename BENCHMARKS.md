# Benchmarks — infra-ai-streaming

**Status:** Scaffold only. Numbers marked `[TBD]` until k6 load tests run on a documented machine. Do not treat targets in README as measured results.

## Hardware

| Field | Value |
|-------|-------|
| CPU | [TBD] |
| RAM | [TBD] |
| Disk | [TBD — SSD/NVMe] |
| OS | [TBD] |
| Broker / CH / Redis | Docker Compose (`deploy/docker-compose.yml`) |
| Ingestion + consumer | Native on host (`127.0.0.1:9092`) |

## Test methodology

| Parameter | Value |
|-----------|-------|
| Tool | k6 (`load-test/k6-script.js` — **planned**, not in tree yet) |
| Stages | ramp 0→50 VUs (30 s), hold 50 VUs (2 m), ramp 50→200 VUs (30 s), hold 200 VUs (2 m) |
| Batch per POST | 100 events |
| Tenants | 10 rotating `X-Tenant-ID` |
| Models | 5 (`gpt-4o`, `claude-sonnet`, `llama-3-70b`, `mistral-large`, `gemini-1.5-pro`) |
| Latency mix | 95 % 50–2000 ms, 5 % 5–10 s (exercises anomaly detector) |
| Partial signal today | `./chaos/run_chaos.sh load-10k` (throughput + lag, not HTTP P99) |

## Results

| Scenario | VUs | Events/sec | HTTP P50 | HTTP P99 | CH write lag (max) | Kafka lag (max) | Error rate |
|----------|-----|------------|----------|----------|--------------------|-----------------|------------|
| Steady   | 50  | ~5,000 [TBD] | [TBD] ms | [TBD] ms | [TBD] ms | [TBD] | [TBD]% |
| Stress   | 200 | ~20,000 [TBD] | [TBD] ms | [TBD] ms | [TBD] ms | [TBD] | [TBD]% |

**SLO reference:** ingest HTTP P99 **< 100 ms** to accepted+durable boundary (WAL + enqueue), not ClickHouse visibility. See [DESIGN.md](DESIGN.md) §1.

## Bottleneck analysis

| Load | Likely bottleneck | How to confirm |
|------|-------------------|----------------|
| 50 VUs | [TBD] | Grafana: `ingestion_latency_ms`, `clickhouse_flush_duration_seconds` |
| 200 VUs | [TBD] | `kafka_consumer_lag_events`, `backpressure_events_total` |

## Why ClickHouse over Prometheus for this workload?

At scale, `tenant_id × model_id × status` multiplies series cardinality; cost and latency rollups need columnar scans. ClickHouse MVs pre-aggregate at insert time — see [DESIGN.md](DESIGN.md) §1.

## How to reproduce

1. `./scripts/run.sh --profile m1 --target compose` (or full k3d E2E — see [README quick start](README.md#quick-start))
2. Run consumer and ingestion per the quick start
3. `k6 run load-test/k6-script.js` → `load-test/results.json`
4. Panels: Grafana Product SLOs; metrics on `:8080` and `:9091`

**Chaos throughput alternative (no k6 yet):**

```bash
./chaos/run_chaos.sh load-10k   # consumer BATCH_SIZE=5000 recommended
```

## Changelog

| Date | Change |
|------|--------|
| 2026-05-28 | Initial benchmark scaffold |
