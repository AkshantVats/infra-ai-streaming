# DESIGN — infra-ai-streaming

Author perspective: distributed systems background (high-volume TSDB, streaming, IoT-scale ingestion). This document is the contract between intent and implementation: tradeoffs first, components second.

---

## 1. Problem and goals

**Why Prometheus is the wrong primary store here.** Prometheus indexes time series by label sets. LLM inference telemetry naturally carries dimensions such as `tenant_id`, `model_id`, `deployment`, `region`, and eventually `finetune_version`. The Cartesian product of those labels creates cardinality that exceeds practical `storage.tsdb.retention` and scrape cardinality budgets long before you reach “1M events/min” in a single control plane. Prometheus remains valuable for **pipeline** metrics (lag, batch latency, circuit state) where cardinality is bounded — not as the system of record for raw inference facts.

**Why common LLM observability tools are insufficient at this layer.** Tools in the Langfuse / OpenTelemetry-extension / proxy-logging space excel at developer workflows: traces, prompts, experiments. They are not optimized as a **durable, partitioned append log** with **consumer-group replay**, **hot-path isolation**, and **cost accounting** that must survive broker restarts and partial downstream outages without blocking the caller’s inference request for hundreds of milliseconds. This project targets the **infrastructure plane**: accept events cheaply, guarantee durability boundaries, move work asynchronously to ClickHouse.

**Goals (engineering targets).**

- Sustained **1M events/min** ingest with horizontal scaling of stateless Rust services.
- **P99 < 100 ms** from HTTP receipt to “durable enough to return success” (WAL fsync + enqueue to internal channel / Kafka produce acknowledgment policy — exact boundary documented per phase in `BENCHMARKS.md` once implemented).
- **Unbounded logical cardinality** for tenant/model dimensions in the analytical store (physical limits = disk + partitioning + pruning), not Prometheus series limits.
- **At-least-once** delivery to ClickHouse with explicit duplicate handling keys.
- **Multi-tenant isolation** for rate limiting and cost rollups (`tenant_id` mandatory on every event).

---

## 2. CAP — AP over CP at ingestion

The ingestion API chooses **availability** and **partition tolerance** over **strong consistency** with ClickHouse at the moment of the HTTP response.

**What we give up.** A successful response does not imply the event is queryable in ClickHouse. It implies the event is **durable in the streaming log path** (local WAL + Kafka produce per policy) and will become visible after consumer flush. On crash or duplicate delivery, **the same logical event may appear more than once** in ClickHouse unless deduplicated.

**Why duplicates are acceptable here.** Downstream analytics (cost, latency percentiles) are mostly **idempotent under duplication** if keyed by `event_id` or if ReplacingMergeTree / materialized dedup is applied. Worse than duplicates is **dropping** silently or **blocking** callers on a remote quorum while models are serving live traffic.

**How we bound harm.** Producers attach `event_id` (server-generated if omitted). Consumers write idempotent batches where possible; ClickHouse schema may evolve toward `ReplacingMergeTree(event_id)` if strong dedup becomes a product requirement. The alternative — CP ingestion that waits for ClickHouse commit — pushes tail latency and failure modes into the **user-facing inference path**, which is the wrong place to absorb analytical store slowness.

---

## 3. Partition strategy

**Kafka topic keys.** Naive partitioning on `model_id` creates **hot partitions** (everyone’s traffic piles onto `gpt-4o`). Random keys remove hot spots but **destroy per-tenant locality**, making ordered consumption and some rollups harder.

**Choice: partition by `tenant_id`.** All events for a tenant route to the same partition, preserving **per-tenant ordering** and spreading load across tenants. Large tenants still produce hot partitions; mitigation is **more partitions** and **multiple independent models** writing under sub-tenant IDs if needed (operational convention, not code magic).

**ClickHouse partitioning.** Partition by calendar date (`toYYYYMMDD(timestamp)`): efficient TTL, partition pruning on dashboard time ranges, and predictable maintenance windows.

**ClickHouse sorting key.** `(tenant_id, model_id, timestamp)` matches the dominant query: **filter tenant**, optionally **filter model**, **range on time**. LowCardinality on high-repeat string fields remains on the table for compression and scan speed once DDL lands.

---

## 4. Backpressure

The Rust service uses a **bounded channel** between HTTP handlers and the Kafka produce path. When the channel is full, `try_send` fails and the API returns **503** with **`Retry-After`** rather than accepting memory growth without bound. **Honest overload** beats silent loss.

A separate drain task reads from the channel and calls the Kafka producer so that **TCP backpressure from brokers** does not pin HTTP worker threads indefinitely. WAL persistence policy is **before** acknowledging durability to the client (implementation detail: fsync batching strategy documented in code comments once merged).

---

## 5. Failure modes

| Failure | Detection | Recovery | Data loss |
|---------|-----------|----------|-----------|
| Kafka broker unavailable | Producer error callbacks / metadata refresh failures | WAL retains not-yet-acknowledged records; retry with backoff | None for events already WAL-fsynced under chosen policy |
| ClickHouse insert timeout or 500 | Context deadline / driver error | Circuit breaker opens; **Redis LIST overflow** stores serialized batches | None while overflow has capacity; if Redis full, consumer blocks or DLQs per policy |
| Redis unavailable (rate limit) | Ping / command errors at ingest | **Fail open** on rate limit only (documented): prefer accepting traffic with degraded fairness | No event loss; fairness degraded |
| Redis unavailable (overflow) | Consumer write errors | Circuit stays open; events remain in Kafka until retry | None at consumer group level (lag grows) |
| Ingest OOM / kill | K8s restart | WAL replay on startup | None for WAL-fsynced events |
| Consumer crash | Group rebalance | Resume from last committed offset | **At-least-once** duplicates possible without dedup |

---

## 6. Scaling

- **Ingestion:** Stateless pods behind a load balancer; **Redis** holds distributed token-bucket state; **WAL is local** — scaling replicas increases aggregate WAL disk need; operations must size PVCs or instance store accordingly.
- **Consumers:** Scale on **Kafka lag**, not CPU alone. HPA custom metric: `kafka_consumer_lag` (exporter or client gauge).
- **ClickHouse:** Read replicas for Grafana; writes through batching consumer (and optional ch proxy layer if added later).

---

## 7. ClickHouse schema rationale (preview)

Raw fact table stores the canonical JSON fields (see README / `deploy/clickhouse/init.sql` when added). **LowCardinality(String)** for `tenant_id` and `model_id` where cardinality is high but repetition per partition is large — reduces storage and speeds scans. **Materialized views** compute hourly **cost**, **token totals**, and **latency quantile sketches** (or simple sums + sample tables) so dashboards do not scan raw facts for every panel. **TTL** on raw rows (e.g., 90 days) with long-lived rollups matches cost constraints for high-volume tenants.

---

## 8. Security and tenancy (non-goals for v0)

Authentication, mTLS, and per-tenant encryption keys are **not** solved in the first milestone beyond **header-based tenant identification** for development. Production hardening belongs in a threat model appendix once the data plane is live.

---

## 9. Open questions

- Exact Kafka **ACK** level (`acks=all` vs latency tradeoff) per environment.
- Whether **idempotency** is enforced only at consumer or also via ClickHouse engine choice.
- OTLP exporter backend (Jaeger, Tempo, vendor) for local compose vs prod.

This document should change as code lands; each significant behavioral change updates **§4–§7** and cross-links `OBSERVABILITY.md` / `CHAOS.md` when those files exist.
