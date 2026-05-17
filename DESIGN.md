# infra-ai-streaming — Architecture Design Document

[![Build](https://img.shields.io/badge/build-pending-lightgrey.svg)](.github/workflows/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-0.1.0--pre-orange.svg)](#)

| Field | Value |
|-------|-------|
| **Author** | Akshant Sharma |
| **Status** | Draft — Active Development (ingestion HTTP + WAL + Kafka path implemented in `ingestion/`) |
| **Last Updated** | May 2026 |
| **Version** | 0.1.0 |

**Document purpose.** This file records **architectural decisions and tradeoffs** for infra-ai-streaming—not only what components exist, but **why** they exist. Contributors and reviewers should be able to trace every major choice (CAP boundary, partition keys, backpressure, failure semantics) to a stated rationale. When implementation diverges, either update this document or treat the divergence as a bug.

---

## Table of contents

1. [Problem and goals](#1-problem-and-goals)
2. [CAP theorem decision — AP over CP for ingestion](#2-cap-theorem-decision--ap-over-cp-for-ingestion)
3. [Kafka partition strategy — avoiding hot partitions](#3-kafka-partition-strategy--avoiding-hot-partitions)
4. [Backpressure design — channel-based in Rust](#4-backpressure-design--channel-based-in-rust)
5. [Failure modes and recovery](#5-failure-modes-and-recovery)
6. [Horizontal scaling strategy](#6-horizontal-scaling-strategy)
7. [Consistency model for analytics queries](#7-consistency-model-for-analytics-queries)

---

## 1. Problem and goals

**Why standard observability breaks for LLM inference at scale.** Application metrics stacks (Prometheus-style) assume **bounded label cardinality**. Inference telemetry multiplies dimensions—`model_id`, `tenant_id`, `error_type`, deployment, region—into a space that is manageable for **sampling or tracing products**, but painful as **dense time series** at millions of events per minute. Separately, **dollar cost per request** (`cost_usd`, token counts) is a **finance and capacity** signal, not something most APM SKDs treat as a first-class, queryable fact stream. Multi-tenant SaaS additionally needs **isolation**: rate limits, cost rollups, and noisy-neighbor containment **per tenant**, not only a global service graph.

**Goals.**

- **Throughput:** sustain **1M inference events/minute** at the ingestion boundary with horizontal scale-out.
- **Latency SLA:** **P99 under 100 ms** server-side for the **ingest acknowledgment path** (durable under the policy in §2 and §4—not “visible in ClickHouse”).
- **Cardinality:** support **10k+** distinct combinations of high-cardinality dimensions (e.g. `model_id` × `tenant_id` × error class) in the **analytical store** without Prometheus-style series explosion.
- **Durability:** **at-least-once** from accepted ingest through Kafka to ClickHouse under stated failure modes; duplicates are an explicit data-plane concern (keys, `FINAL`, dedup engines).

**Non-goals (v0 / product boundary).**

- **No storage of raw prompt or completion text** in the hot pipeline—privacy, retention cost, and compliance explode; use a separate consent-scoped system if needed.
- **No model evaluation / offline scoring** (golden sets, LLM-as-judge pipelines)—this stack is **metering and reliability telemetry**, not an ML experimentation platform.
- **No replacement for Langfuse-style trace UX**—we may emit OTel, but the core primitive here is **append-only inference events**, not nested span exploration as the primary UI.

---

## 2. CAP theorem decision — AP over CP for ingestion

**Ingestion chooses Availability + Partition Tolerance over strong Consistency** with respect to the analytical store at HTTP response time.

**Rationale.** A **blocked or slow** ingest path sits on the **critical path of production inference** (or its adjacent gateway). Analytics consumers and dashboards can tolerate **eventual consistency**; a user-facing timeout or cascading retry storm is harder to tolerate than a duplicate row or a second of replication lag. We therefore **do not** wait for ClickHouse commit visibility before returning success.

**Where we tighten consistency.** For **dashboard reads** backed by ClickHouse, we target **ReplicatedMergeTree** (or successor family) with **synchronous replication** on the query-relevant replicas where we need **read-your-writes** within a bounded lag—still not CP at the HTTP edge, but **stronger read semantics** in the warehouse tier than at ingest.

**Concrete failure scenario.** If **Kafka is partitioned** or brokers are unreachable, the ingestion service continues to **accept** traffic **as long as local resources allow**: events are **WAL-persisted** first (see §4), then retried toward Kafka. We prefer **backpressure (429)** or **honest degradation** over silently dropping accepted work.

---

## 3. Kafka partition strategy — avoiding hot partitions

**Naive approach:** partition only by `model_id`. **Wrong:** a popular model (e.g. GPT-4 class) concentrates traffic on **one hot partition**, limiting parallelism and skewing retention.

**Our approach:** partition key = **`hash(tenant_id + ":" + model_id) % num_partitions`** (stable string hash; exact function documented with code). **Why include `model_id`:** spreads a **single tenant’s** high-volume model mix across partitions instead of pinning all tenant traffic to one broker partition. **Why tenant is in the key:** preserves **tenant-scoped locality** in expectation—related traffic co-locates enough for sane consumer batching without gifting the whole partition to one global model.

**Partition count:** start at **32** partitions for the primary topic; **double** (64, 128, …) when sustained produce rate or consumer lag per partition exceeds targets. **Halving** partitions is avoided in production: it requires **expensive re-keying** and risks violating ordering assumptions during migration.

**Consumer groups:** **one consumer group per downstream concern**—e.g. **ClickHouse writer**, **anomaly detector**, **future alerting fan-out**—so each can scale and commit offsets independently without coupling lag profiles.

---

## 4. Backpressure design — channel-based in Rust

**Shape:** HTTP handler → **bounded** async channel (**default capacity 10,000 events**, not unbounded RAM) → dedicated task(s) → Kafka producer.

**Overload behavior:** when the channel is full, return **HTTP 429** with **`Retry-After`**—do **not** block the runtime indefinitely, and do **not** drop silently after acceptance.

**Ordering with durability:** **WAL append + fsync completes before a successful enqueue** to the bounded channel for batches we promise to accept under the ingest contract. If the channel rejects, the caller may still retry; WAL replay distinguishes **un-acked** work on restart. (Exact “accepted vs durable” HTTP mapping is specified alongside implementation.)

**Why not rely on Kafka’s producer queue alone:** client internal buffers can grow **without explicit application bounds**, hiding memory pressure until OOM. The **Tokio `mpsc`** channel gives a **hard cap** integrated with async cancellation and backpressure signals.

**Tokio `mpsc` vs crossbeam:** handlers run in **async** context; **`tokio::sync::mpsc`** integrates with the runtime **without blocking threads** on `recv`. Crossbeam bounded channels are excellent for **sync** bridges; here the hot path is **async end-to-end**, so Tokio is the default.

---

## 5. Failure modes and recovery

| # | Scenario | What happens | Recovery | Data loss guarantee |
|---|----------|----------------|----------|---------------------|
| 1 | **Kafka broker dies mid-ingest** | Producer errors; accepted batches may sit in **WAL** and memory buffers | Retry with backoff; WAL **replay** on restart until Kafka ACKs | **No loss** for WAL-fsynced entries under the chosen produce-ack policy |
| 2 | **ClickHouse write timeout** (slow query / disk pressure) | Go consumer batch insert fails; **circuit breaker** opens; overflow path may buffer in **Redis** | Breaker half-open retries; drain overflow to ClickHouse when healthy | **No loss** while Kafka retains offsets and overflow capacity holds; otherwise **lag**, not silent drop |
| 3 | **Redis connection lost** (rate limit path) | Rate limiting **fails open** (documented): accept with degraded fairness; log / metric the miss | Reconnect Redis; token buckets refill on next success | **No intentional drop** of events solely because Redis is down |
| 4 | **Ingestion engine pod OOM-killed** (Kubernetes) | Process terminates; in-flight work may be partial | New pod starts; **WAL replay** resubmits un-acked entries | **At-least-once**; duplicates possible without idempotent consumer |
| 5 | **Network partition: Go consumer ↔ ClickHouse** | Inserts time out or reset; breaker may open; consumer **stops committing** forward offsets until policy says otherwise | Heal network; retry batches from Kafka; drain overflow | **No loss** from committed offsets; uncommitted work **replays** from Kafka (duplicates possible) |

---

## 6. Horizontal scaling strategy

- **Rust ingestion:** **stateless** behind a load balancer; **shared-nothing**; **each replica has its own WAL** (size PVCs / instance store per pod). Scale out for QPS; Redis coordinates **distributed** rate limits.
- **Go consumer:** add instances in the **same consumer group** for a partition set; Kafka **rebalances** partitions; scale on **consumer lag**, not CPU alone.
- **ClickHouse:** **distributed** tables across shards; **shard key = `tenant_id`** for **data locality** on tenant-heavy dashboards (implementation detail: distributed DDL in `deploy/clickhouse/` when checked in).
- **Redis:** a **single primary** (with replica for read scaling if needed) is acceptable for **token buckets and short TTL keys** at the target rates; if limits become Redis-bound, **Redis Cluster** is the migration path without changing the API contract.
- **Kubernetes HPA:** scale **ingestion** on **CPU** plus a **custom metric** (e.g. Kafka **producer queue depth** or channel saturation proxy); scale **consumers** primarily on **Kafka consumer lag**.

---

## 7. Consistency model for analytics queries

**Warehouse reality:** ClickHouse replication is **eventually consistent** within a small **replication lag** window (typically sub-second under healthy clusters).

**Query semantics:** dashboard queries that must collapse duplicates use **`FINAL`** (or equivalent deduping patterns) on **ReplacingMergeTree**-style tables—**tradeoff:** higher read cost for **correct single-row** semantics per logical key.

**Pre-aggregation:** **materialized views** roll up raw facts by **`(tenant_id, model_id, hour)`** for cost and latency panels—dashboards hit the **MV** for default views, not full raw scans.

**Why not Kafka Streams / Flink here:** this product’s first milestone is **durable capture + warehouse analytics + operable SRE metrics**. A stream processor cluster adds **operational surface** (state stores, checkpoints, version upgrades) disproportionate to v0 needs; **Go batch consumer + ClickHouse MVs** keeps the blast radius smaller while we prove throughput and cost correctness.

---

## Appendix — Implementation notes (Day 3)

The `ingestion` binary implements §2–§4 at a first milestone:

- **HTTP:** `POST /ingest` returns **202 Accepted** after WAL append + successful enqueue to a bounded `tokio::sync::mpsc` channel; channel full → **503** with `Retry-After`.
- **WAL:** Segment files under `WAL_DIR`; `replay_unacked` on startup; `mark_acked` after Kafka delivery confirmation.
- **Kafka:** **rdkafka** producer to `KAFKA_TOPIC`; failed batches after retries go to `KAFKA_DLQ_TOPIC`. Local dev uses **Redpanda** in Docker Compose (`127.0.0.1:9092`)—Kafka-compatible API, not a host-native Kafka install.
- **Rate limit:** Redis token bucket per `tenant_id` (and `X-Tenant-ID` header); fail-open on Redis errors per §5.

**Still out of scope in-tree:** Helm charts, anomaly detector, OTLP wiring.

## Appendix — Day 4 milestone (Go consumer skeleton)

- **Consumer group (local dev):** `ai-inference-consumer-dev` (`KAFKA_GROUP_ID` in `deploy/.env.example`). Production name TBD (`ai-inference-consumer-v1`).
- **Kafka message envelope:** each record is JSON `{"events":[<InferenceEvent>, ...]}` — matches `ingestion/src/kafka/producer.rs`.
- **Partition key gap:** producer keys by **`tenant_id` only** today; DESIGN §3 target `hash(tenant_id:model_id)` is **not** implemented yet (tracked as open item).

## Appendix — Day 5 milestone (ClickHouse writer)

- **BatchWriter:** flush at **1000** events or **500ms**; maps columns to `deploy/clickhouse/init.sql` (`infra_ai.inference_events`).
- **Circuit breaker:** **5** failures → open; **30s** → half-open; **1** success → closed. When open, batches go to **Redis LIST** overflow (`REDIS_OVERFLOW_KEY`).
- **Overflow drain:** every **5s**, pop up to **5000** events when breaker is closed.
- **DLQ:** **3** insert retries per batch, then per-event publish to `ai_inference_dlq`.
- **Offsets:** commit only after CH insert, overflow push, or DLQ handoff for all events in the Kafka record.
- **Metrics:** consumer HTTP **`METRICS_PORT` (default 9091)** — `clickhouse_*`, `circuit_breaker_state`, `redis_overflow_depth`, `dlq_events_total`. See [OBSERVABILITY.md](OBSERVABILITY.md).

## Appendix — Open items

- **Partition key:** migrate producer from `tenant_id` to `hash(tenant_id:model_id)` without breaking replay.
- Exact **Kafka `acks`** default per environment (latency vs durability).
- Whether **HTTP 429** vs **503** is exposed for all overload classes (backpressure currently uses **503**; document here if that changes).
- OTLP backend choice for local vs production.

This appendix is **not** part of the seven core sections; it tracks decisions still in flight.
