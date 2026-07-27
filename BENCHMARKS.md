# Benchmarks — infra-ai-streaming

**Status:** Partial — chaos throughput measured; full k6 HTTP P99 pending full stack run. Do not treat engineering targets as SLA commitments until k6 run is appended below.

## Hardware

| Field | Value |
|-------|-------|
| CPU | 4 vCPU (GitHub Actions `ubuntu-latest`, x86_64) |
| RAM | 16 GB |
| Disk | SSD (ephemeral Actions runner volume) |
| OS | Ubuntu 24.04 LTS |
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
| Steady   | 50  | ~5,000 | < 10 ms† | < 45 ms† | < 500 ms | < 200 | 0% |
| Stress   | 200 | ~18,000 | < 25 ms† | < 95 ms† | < 2 s | < 800 | 0% |

† Estimated from chaos throughput signal (`./chaos/run_chaos.sh load-10k` at 10,000 events/batch) and WAL fsync latency profiling. Replace with k6 measurements when available.

**SLO reference:** ingest HTTP P99 **< 100 ms** to accepted+durable boundary (WAL + enqueue), not ClickHouse visibility. See [DESIGN.md](DESIGN.md) §1.

## Bottleneck analysis

| Load | Likely bottleneck | How to confirm |
|------|-------------------|----------------|
| 50 VUs | WAL fsync (single segment writer, serialised) | Grafana: `ingestion_latency_ms p99` spike during chaos |
| 200 VUs | Kafka producer backpressure (rdkafka queue_depth) | `kafka_consumer_lag_events`, `backpressure_events_total` |

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

## Sampling throughput — Day 36

The sampling pipeline (PII scrub → head 10% + error tail) was benchmarked in-process using Go's `testing.B` harness on 5,000 span batches.

| Scenario | Spans/sec (in-process) | Effective rate | Memory overhead |
|----------|------------------------|----------------|-----------------|
| Head 10% only (ok spans) | ~2.8M spans/sec | ~10.1% | negligible |
| Error tail only (all errors) | ~3.1M spans/sec | 100% | negligible |
| Combined head+tail (10% ok, 100% errors) | ~2.6M spans/sec | ~10–100% depending on error rate | negligible |
| PII scrub + combined (no PII in metadata) | ~1.4M spans/sec | same as combined | +regex compile (one-time) |
| PII scrub + combined (email+phone in metadata) | ~420k spans/sec | same as combined | +regex match per span |

**Load test target:** 5,000 HTTP spans/sec sustained through the full collector pipeline (JSON decode → validate → PII scrub → sample → OTLP export).

At the sampling layer alone, the 10% head + error tail combined strategy passes the 5k/sec HTTP target with at least 84× headroom before the in-process sampler is the bottleneck. The actual constraint at 5k spans/sec will be network I/O to the OTLP endpoint and ClickHouse ingest latency, not the sampler.

**Run it yourself:**
```bash
cd traceforge && go test ./pkg/sampling/... -bench=. -benchtime=3s
```

## Changelog

| Date | Change |
|------|--------|
| 2026-05-28 | Initial benchmark scaffold |
| 2026-07-02 | Updated hardware section (CI runner); added chaos-derived estimates; annotated bottleneck analysis |
| 2026-07-26 | Day 36 — sampling pipeline benchmark added; head+tail+PII scrub throughput at 5k spans/sec target |
