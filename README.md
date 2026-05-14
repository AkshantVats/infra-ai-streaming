# infra-ai-streaming

**Sub-100ms AI inference observability at 1M events/min — Kafka-backed, ClickHouse-native, multi-tenant.**

[![Build](https://img.shields.io/badge/build-pending-lightgrey.svg)](.github/workflows/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.1.0--pre-orange.svg)](#)

Production-grade streaming pipeline for LLM inference events: ingest, durably buffer, stream-process, and query at cardinality and throughput where application-layer tracing tools and Prometheus-style metrics backends stop being viable.

---

## Why this exists

Prometheus (and similar pull-based metrics stores) breaks when `model_id × tenant_id × deployment` explodes series cardinality; LLM inference telemetry routinely crosses that threshold. Application-layer LLM tools optimize for traces and prompt UX, not for a replayable event bus, bounded hot-path latency, or per-tenant cost accounting at Kafka-scale throughput. There is no widely adopted open stack that combines multi-tenant `cost_usd` isolation, durable streaming, and columnar analytics purpose-built for inference-shaped payloads — this project is that stack.

---

## Architecture overview

Ingestion is **AP-oriented**: accept and durably record quickly (WAL + Kafka), then make data **eventually consistent** in ClickHouse. The Go consumer batches writes, protects ClickHouse with a circuit breaker, and spills to Redis when the analytical path is degraded. Prometheus scrapes both services; Grafana reads ClickHouse for product dashboards.

→ [Read the full design document](DESIGN.md)

<!-- architecture-diagram-start -->

```mermaid
flowchart LR
  subgraph clients["AI application clients"]
    C1["App"]
    C2["App"]
    C3["App"]
  end

  subgraph rust["Rust ingestion engine (Axum)"]
    R["Receive batches\nvalidate schema\nWAL append\nproduce to Kafka"]
  end

  subgraph redis["Redis"]
    RL["Rate limiting\ntoken bucket / tenant"]
    OV["Overflow buffer\nwhen ClickHouse path is unhealthy"]
  end

  subgraph kafka["Kafka / Redpanda"]
    TEV[["topic: ai_inference_events"]]
    TDLQ[["topic: ai_inference_dlq"]]
  end

  subgraph go["Go consumer"]
    G["Consume\nbatch: 1000 events or 500ms\ncircuit breaker"]
  end

  subgraph ch["ClickHouse"]
    CH[("Columnar store\ntime-series aggregations")]
  end

  subgraph viz["Dashboards"]
    GF["Grafana\n(datasource: ClickHouse)"]
  end

  subgraph prom["Pipeline metrics"]
    PR["Prometheus\n(scrape)"]
  end

  C1 & C2 & C3 -->|"HTTP POST /ingest"| R
  R <-->|"allow / deny (per tenant)"| RL
  R -->|"produce accepted batches"| TEV
  TEV -->|"consumer group read"| G
  G -->|"batch INSERT"| CH
  G -.->|"circuit open: enqueue"| OV
  G -.->|"dead-letter poison / failed batches"| TDLQ
  OV -.->|"drain when CH healthy"| G
  CH -->|"queries / panels"| GF
  R -->|":9090 /metrics"| PR
  G -->|":9091 /metrics"| PR
```

<!-- architecture-diagram-end -->

*(Diagram: left-to-right flow from clients through ingestion, Kafka, consumer, ClickHouse, and observability sidecars.)*

---

## Features

- **Rust (Axum) HTTP ingestion**: batched JSON ingest, schema validation, bounded in-memory backpressure before honest `503`/`Retry-After`.
- **WAL before Kafka produce**: local durability so a crash after ACK boundaries can be reconciled with replay semantics (at-least-once).
- **Kafka / Redpanda**: `ai_inference_events` as primary stream; `ai_inference_dlq` for poison or persistently failing batches.
- **Go stream consumer**: concurrent read, **1000 events or 500ms** flush policy, ClickHouse batch writer, **circuit breaker** with **Redis LIST overflow** when inserts fail or time out.
- **Redis**: distributed **token-bucket rate limit per `tenant_id`** on ingest; **overflow buffer** when ClickHouse is slow or unavailable.
- **ClickHouse**: MergeTree-style storage, high-cardinality dimensions (`tenant_id`, `model_id`), time-range scans optimized for rollups and dashboards.
- **Self-observability**: Prometheus scrapes Rust and Go (`/metrics`) — ingestion latency histograms, Kafka consumer lag, DLQ depth, circuit-breaker state.
- **Grafana**: starter dashboards wired to ClickHouse (to ship with repo).
- **OpenTelemetry**: OTLP export across Rust and Go (planned wiring in compose).
- **Kubernetes / Helm**: stateless ingest scales horizontally; HPA hooks on **Kafka lag**, not CPU alone (planned charts).

---

## Tech stack

| Component | Technology | Why |
|-----------|------------|-----|
| HTTP ingestion | Rust + Axum + Tokio | Predictable hot-path latency; no GC pauses on the accept/validate/encode path. |
| Event transport | Apache Kafka / Redpanda | Durable log, partition scaling, consumer groups, replay for backfills and failures. |
| Stream processor | Go | Straightforward batching, flush timers, and concurrent I/O for consumer → ClickHouse. |
| Analytical store | ClickHouse | Columnar engine tuned for high-cardinality, time-range aggregation, and rollup MVs at this event shape. |
| Rate limiting | Redis + Lua | Atomic token bucket across many ingest replicas. |
| Degraded-path buffer | Redis (LIST) | Cheap spillover when ClickHouse rejects or times out; drain when healthy. |
| Metrics | Prometheus | First-class histograms, gauges for lag/DLQ, standard scrape model. |
| Dashboards | Grafana | SQL to ClickHouse for cost/latency/token panels. |
| Tracing | OpenTelemetry (OTLP) | Vendor-neutral propagation across services. |
| Deployment | Kubernetes + Helm | Pod autoscaling; lag-driven HPA on consumers. |

---

## Target metrics (design goals)

| Throughput | Ingestion P99 | Storage model | Cardinality support |
|------------|---------------|---------------|---------------------|
| **1M events/min** (horizontal scale) | **< 100 ms** server-side to accepted+durable boundary (WAL + enqueue/produce path) | Columnar MergeTree family; raw TTL + rollups TBD in `DESIGN.md` | **High** — `tenant_id`, `model_id`, status dimensions without Prometheus series explosion |

*Numbers are engineering targets validated under load tests (k6) as the implementation lands; not benchmarks yet.*

---

## Getting started

> **Status:** Ingestion **Rust library** (config, Prometheus metrics, WAL writer, Redis rate limiter) builds and tests locally. HTTP `/ingest`, Kafka producer, and Docker Compose stack are **Day 3+**.

**Prerequisites:** Rust **1.86+** (see [`rust-toolchain.toml`](rust-toolchain.toml); `icu` / `idna` deps require it with current resolver), **cmake** (for `rdkafka` via `cmake-build`), Docker + Go when you run the full stack.

```bash
git clone https://github.com/YOURUSERNAME/infra-ai-streaming.git
cd infra-ai-streaming
# macOS: brew install cmake   (required for rdkafka native build)
./scripts/test-ingestion.sh
# or: cargo test -p ingestion
# Full stack (when available):
# docker compose -f deploy/docker-compose.yml up -d
# cargo run -p ingestion -- …
# go run ./consumer/cmd/consumer/
```

Example ingest (schema will match `DESIGN.md` / `deploy/clickhouse/init.sql` once checked in):

```bash
curl -sS -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d '{"events":[{"tenant_id":"demo","model_id":"gpt-4o","timestamp_unix_ms":1715000000000,"latency_ms":342,"prompt_tokens":512,"completion_tokens":128,"cost_usd":0.00423,"status":"success"}]}'
```

---

## Design decisions

### (a) Rust for ingestion

The ingest path is latency-sensitive and allocation-heavy under batch decoding. A GC’d runtime can pause at the wrong quantile and inflate P99 under churn; Rust + Tokio keeps the accept → validate → serialize → handoff path bounded and explicit. The hard work (compression, TLS, Kafka client backpressure) still happens — but on threads and budgets you control.

### (b) ClickHouse over TimescaleDB

This workload is **append-mostly analytical fact tables** with **wide, repetitive dimensions** and **sub-second dashboard queries** over billions of rows. ClickHouse’s columnar encoding and vectorized execution match aggregate-heavy panels (cost, tokens, latency percentiles by tenant/model) better than row-oriented Postgres hypertables at the same hardware envelope. Timescale remains excellent for many metrics workloads; here the dominant access pattern is OLAP-shaped.

### (c) AP over CP at the ingestion boundary

Ingest optimizes **availability** and **honest overload behavior**: prefer accepting work into a durable log (WAL + Kafka) over synchronously waiting for global consistency with the analytical store. We explicitly trade **immediate cross-system consistency** for **bounded client latency** and **replayability**. Duplicates are handled as a data-plane concern (`event_id`, idempotent consumers, optional ReplacingMergeTree) rather than blocking callers on ClickHouse write quorum.

---

## Roadmap

1. **Semantic cache layer** — embedding-backed prompt deduplication API (optional sidecar) to cut duplicate spend.
2. **Multi-region ClickHouse** — replication and read fanout for geo-distributed tenants.
3. **eBPF / host-level probes** — zero-SDK capture path for inference calls where HTTP ingest is not possible.
4. **Cost anomaly detection** — budget burn-rate alerts per tenant (Z-score / EWMA on hourly rollups).
5. **AI gateway integration** — treat this repo as the observability backend behind Envoy/APISIX-style gateways.

---

## Repository layout (planned)

```
infra-ai-streaming/
├── ingestion/          # Rust — Axum, WAL, Kafka producer, rate limit client
├── consumer/           # Go — Kafka reader, ClickHouse writer, Redis overflow
├── deploy/             # docker-compose, Prometheus, Grafana, ClickHouse init
├── dashboards/         # Grafana JSON exports
├── load-test/          # k6 scripts
├── chaos/              # scripted failure injections
└── docs/               # DESIGN.md, OBSERVABILITY.md, CHAOS.md, BENCHMARKS.md
```

---

## License

[MIT](LICENSE).

---

## Acknowledgements

Built as an open, infrastructure-native alternative to SDK-only LLM observability stacks — optimized for **durability**, **cardinality**, and **per-tenant cost** at streaming scale.
