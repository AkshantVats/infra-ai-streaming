# Show HN: LensAI — open-source AI inference observability (Rust + Kafka + ClickHouse)

## Title
Show HN: LensAI — open-source AI inference observability (Rust + Kafka + ClickHouse)

## Body

I spent four years building distributed systems at Agoda (1.5T events/day TSDB), Delivery Hero (5k geo-events/s rider tracking), and Wayfair (pricing analytics). When I moved to inference infrastructure, I hit the same problems in a new context: no good way to see *which* model answered which request, at what latency, at what token cost, with what error distribution — in real time.

LensAI is what I built to fix that.

---

**What it is**

LensAI is a production-grade AI inference observability pipeline. It ingests per-request telemetry from LLM API calls — model ID, prompt tokens, completion tokens, latency, error code — and makes it queryable in ClickHouse with sub-second Grafana dashboards.

The stack:

- **Rust ingestion service**: ~4k lines, async Axum HTTP, WAL-backed Kafka producer, Redis rate limiting per tenant
- **Redpanda (Kafka-compatible)**: `inference_events` topic, 3 partitions, 7-day retention
- **ClickHouse**: `inference_events` table with `MergeTree`, pre-aggregated daily/hourly rollup MVs
- **Grafana**: pre-built dashboards for latency percentiles, token burn, error rate by model
- **distributed-flagd**: etcd + gRPC streaming flag system for model A/B routing and canary rollouts
- **Kubernetes**: Helm chart for k3d/production deployment, CRD for FlagDefinition resources

---

**Why I built it**

The LLM API is a black box. `gpt-4o` costs ~5× what `gpt-4o-mini` costs. If you're routing 30% of traffic to the wrong model because a feature flag is misconfigured, you'll see it in your billing before you see it in your logs. LensAI makes model routing observable and controllable — you can see exactly what percentage of requests hit each model, drill to the p99 latency for that model on that tenant, and flip a flag to kill-switch a bad model within one etcd write.

---

**Key design decisions**

1. **WAL before Kafka**: The ingestion service writes to a local write-ahead log before producing to Kafka. On Kafka unavailability, requests still succeed and WAL replays on reconnect. Inference latency SLOs don't degrade because the observability pipeline is slow.

2. **ClickHouse over Postgres**: At 50k events/min sustained (tested with k6), ClickHouse `GROUP BY` aggregations over 7 days of data run in <200ms. Postgres at that scale requires aggressive partitioning and still struggles with arbitrary time-range queries.

3. **distributed-flagd instead of LaunchDarkly**: LaunchDarkly costs $200/month at our scale. etcd is already in the cluster. flagd gives us gRPC streaming flag updates, CRD-based GitOps for flag config, and an audit log — all with <1ms p99 flag evaluation latency because it's in-process cache + watch.

4. **Kafka message key = tenant_id**: All events from one tenant land on the same partition. ClickHouse consumers maintain per-partition offset tracking. This makes exactly-once delivery achievable without two-phase commit.

5. **Prometheus metrics on the ingestion side, not just logs**: Every request emits `ingestion_request_duration_seconds`, `ingestion_kafka_lag`, `ingestion_wal_depth`. You can alert on WAL depth > 10k before it becomes a delivery problem.

---

**Honest limitations**

- No multi-cluster support yet. The etcd cluster is single-region. Cross-region replication is on the roadmap but not implemented.
- The Kubernetes operator for FlagDefinition CRDs is a reconciliation loop, not a full operator framework. It doesn't handle CRD version upgrades automatically.
- k6 load tests top out at 50k events/min in our k3d setup. We haven't run this against a real Kubernetes cluster with dedicated ClickHouse nodes.
- DALL-E / Claude / Anthropic API instrumentation requires adding the LensAI SDK client. There's no auto-instrumentation proxy yet.

---

**Repo**

https://github.com/akshantvats/infra-ai-streaming

Quickstart: `make up` (requires Docker + k3d). Helm chart in `deploy/helm/lensai/`. Load test in `load-test/`.

Happy to answer questions about the ClickHouse schema design, the WAL implementation, or the flagd architecture.
