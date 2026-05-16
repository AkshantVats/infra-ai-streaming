# infra-ai-streaming — 7-Day Build Plan
### One week. Working repo. Recruiter-ready. Cursor-powered.

---

## Why This Is Different From Existing Tools

Before writing a line: know your positioning.

| Tool | What it does | What it misses |
|---|---|---|
| Langfuse | Application traces, prompt management | Not built for 1M events/min, no multi-tenant cost isolation |
| OpenLLMetry | OTel extensions for LLM calls | SDK-first, can't handle infra-level cardinality |
| OpenLIT | OTel-native, ClickHouse backend | Close — but single-tenant, no Kafka transport layer |
| Helicone | Proxy-based logging | No streaming pipeline, no self-hosted scale path |

**Your gap:** A streaming-first, infra-native pipeline built for teams running LLM inference at scale — multi-tenant cost tracking, sub-100ms ingestion P99, Kafka-backed durability, ClickHouse analytics. Built by someone who ran 1.5T events/day at Agoda. That context is the moat.

---

## The 7-Day Target

By end of Day 7 you have:
- ✅ Working Docker Compose stack (one command to run everything)
- ✅ Rust ingestion engine accepting real events
- ✅ Go consumer writing to ClickHouse
- ✅ Grafana dashboard with 4 live panels
- ✅ k6 load test showing P99 numbers
- ✅ DESIGN.md, CHAOS.md, OBSERVABILITY.md written
- ✅ README with architecture diagram, benchmark table, getting started
- ✅ GitHub Actions CI badge (green)

That's the repo you share with every recruiter from Day 8 onward.

---

## Repo Structure (build in this exact layout)

```
infra-ai-streaming/
├── ingestion/                    # Rust — HTTP ingest hot path
│   ├── Cargo.toml
│   └── src/
│       ├── main.rs
│       ├── server.rs
│       ├── config.rs
│       ├── metrics.rs
│       ├── handlers/
│       │   └── ingest.rs
│       ├── kafka/
│       │   └── producer.rs
│       ├── wal/
│       │   └── writer.rs
│       └── rate_limit/
│           └── token_bucket.rs
├── consumer/                     # Go — Kafka → ClickHouse
│   ├── go.mod
│   ├── cmd/consumer/
│   │   └── main.go
│   └── internal/
│       ├── kafka/reader.go
│       ├── clickhouse/writer.go
│       ├── redis/overflow.go
│       ├── anomaly/detector.go
│       └── metrics/server.go
├── deploy/
│   ├── docker-compose.yml
│   ├── clickhouse/init.sql
│   ├── prometheus/prometheus.yml
│   ├── grafana/datasources.yml
│   └── otel/config.yml
├── dashboards/
│   └── grafana/ai-inference.json
├── load-test/
│   └── k6-script.js
├── chaos/
│   └── run_chaos.sh
├── .github/
│   └── workflows/ci.yml
├── DESIGN.md
├── CHAOS.md
├── BENCHMARKS.md
├── OBSERVABILITY.md
└── README.md
```

---

## Event Schema (canonical — everything must match this)

```json
{
  "events": [{
    "event_id": "uuid-v4 (generated server-side if omitted)",
    "tenant_id": "string — REQUIRED",
    "model_id": "string — e.g. gpt-4o, claude-sonnet-4-20250514",
    "timestamp_unix_ms": 1715000000000,
    "latency_ms": 342,
    "prefill_latency_ms": 45,
    "decode_latency_ms": 297,
    "prompt_tokens": 512,
    "completion_tokens": 128,
    "cost_usd": 0.00423,
    "status": "success | error | timeout",
    "error_code": null,
    "request_id": "caller trace ID"
  }]
}
```

---

# DAY 1 — Foundation: README + DESIGN.md + Repo Structure

**Goal:** Repo is live, design is documented, architecture is visible.  
**Time:** ~3 hours  
**Cursor sessions:** 2

No code today. The design doc written before code is what separates Staff from Senior in any hiring review.

---

### Session 1A — README.md

```
I'm building infra-ai-streaming — a production-grade AI inference observability pipeline.

My background: Staff Engineer, 7.5 years. Built a 1.5T events/day TSDB at Agoda (Rust + 
Ceph + Kafka). Geospatial tracking at Delivery Hero (5k events/sec). 7M IoT sensors at 
Walmart on Azure IoT Hub.

Positioning: existing tools (Langfuse, OpenLIT, Helicone) are application-layer tracing 
tools. This is an infrastructure-layer streaming pipeline — built for teams running LLM 
inference at scale who need multi-tenant cost isolation, Kafka-backed durability, and 
sub-100ms ingestion P99 at 1M events/min. Not another SDK wrapper.

Generate the complete README.md:

1. Header badges (placeholder shields.io — build status, license MIT, rust 1.77+)

2. One-line tagline: "Sub-100ms AI inference observability at 1M events/min. 
   Kafka-backed, ClickHouse-native, multi-tenant."

3. "Why this exists" (3 focused sentences): 
   - Prometheus breaks at the cardinality that model_id × tenant_id × version produces
   - Existing LLM tools are SDK wrappers — no streaming pipeline, no durability guarantees
   - No open tool tracks per-tenant cost_usd with multi-tenant isolation at Kafka throughput

4. Architecture section with this placeholder:
   <!-- architecture-diagram -->
   (I'll add Mermaid after)

5. Features (technical bullets, no fluff):
   - Rust Axum ingestion: batched HTTP, per-tenant token-bucket rate limiting via Redis
   - WAL-backed durability: fsync before Kafka produce, replay on crash
   - ClickHouse columnar store: partitioned by date, ordered by (tenant_id, model_id, timestamp)
   - Materialized view: per-tenant/model hourly cost rollup, zero-query overhead
   - Z-score anomaly detection: sliding window per model_id, publishes to ai_anomalies topic
   - Circuit breaker: Redis overflow buffer absorbs load during ClickHouse slowdowns
   - OpenTelemetry: distributed traces across Rust engine + Go consumer
   - Kubernetes-native: Helm charts, HPA on Kafka consumer lag

6. Tech stack table (Component | Technology | Why):
   HTTP ingestion | Rust + Axum | Zero GC pauses on hot path, async with tokio
   Event transport | Kafka / Redpanda | Durable, replayable, consumer-group semantics
   Stream processor | Go | Goroutine model fits concurrent batch flush logic
   Analytical store | ClickHouse | Columnar: 10x faster than Postgres for aggregations at this cardinality
   Rate limiting | Redis | Atomic Lua script for distributed token bucket
   Overflow buffer | Redis LIST | Zero-loss during ClickHouse outages
   Observability | Prometheus + Grafana | Pipeline observing itself
   Tracing | OpenTelemetry (OTLP) | Vendor-neutral, connects to any backend
   Deployment | Kubernetes + Helm | HPA on custom Kafka lag metric

7. Target metrics table:
   Throughput | Ingestion P99 | Cardinality | Durability | Multi-tenant
   1M events/min | < 100ms | Unlimited (columnar) | At-least-once (WAL) | Yes — per-tenant rate limits + cost tracking

8. Getting Started:
   Prerequisites: Docker + docker compose, Rust 1.77+, Go 1.22+
   
   git clone https://github.com/YOURUSERNAME/infra-ai-streaming
   cd infra-ai-streaming
   docker compose -f deploy/docker-compose.yml up -d
   # Wait ~15s for services to be healthy
   cargo run --manifest-path ingestion/Cargo.toml
   go run ./consumer/cmd/consumer/
   # Grafana: http://localhost:3000 (admin/admin)
   # Send test event:
   curl -X POST http://localhost:8080/ingest \
     -H "Content-Type: application/json" \
     -H "X-Tenant-ID: demo" \
     -d '{"events":[{"model_id":"gpt-4o","timestamp_unix_ms":TIMESTAMP,
          "latency_ms":342,"prompt_tokens":512,"completion_tokens":128,
          "cost_usd":0.00423,"status":"success"}]}'

9. Design Decisions (3 entries, 2-3 sentences each):
   - Rust for ingestion (GC pause reasoning)
   - ClickHouse over TimescaleDB (columnar aggregation reasoning)
   - AP over CP (WAL durability reasoning)

10. Roadmap:
    - [ ] Semantic cache layer (vector similarity for prompt deduplication)
    - [ ] Multi-region ClickHouse replication
    - [ ] eBPF-based zero-SDK inference tracing
    - [ ] Cost anomaly detection (budget burn rate alerts)
    - [ ] AI gateway integration (this repo as the observability backend)

Tone: written by the engineer who built it. No marketing language. 
Technical precision over prose.
```

---

### Session 1B — DESIGN.md

```
Write DESIGN.md for infra-ai-streaming.

Author: Staff Engineer. Built 1.5T events/day TSDB at Agoda (Rust + Ceph + Kafka).

This is a Staff-level engineering document. It shows tradeoff reasoning, not just 
descriptions. The reader is a hiring committee or senior engineer reviewing the repo.

Sections:

## 1. Problem & Goals
Why Prometheus fails here: label cardinality limit (~10M series), and 
model_id × tenant_id × version creates combinatorial explosion.
Why existing LLM tools fail: Langfuse, OpenLIT are application-layer; no streaming 
pipeline, no Kafka transport, no at-least-once durability guarantee.
Goals: 1M events/min, P99 < 100ms, unlimited cardinality, at-least-once delivery, 
multi-tenant cost isolation.

## 2. CAP Decision — AP over CP
We choose Availability over Consistency at the ingestion layer.
What we give up: possible duplicate events on crash recovery.
How we handle it: event_id as deduplication key; ClickHouse ReplacingMergeTree 
can be applied later if strong dedup is needed.
Why this is correct: a 503 to the calling AI application adds latency to the 
user-facing request — more costly than a potential duplicate metric.

## 3. Partition Strategy
Problem: naive partition key = model_id → hot partition (gpt-4o gets 10x traffic).
Solution: partition key = tenant_id (routes same tenant to same partition for ordering).
Why not model_id: causes hot partitions.
Why not random: loses per-tenant ordering guarantee needed for cost aggregation.
ClickHouse partition key: toYYYYMMDD(timestamp) — date-based for TTL efficiency.
ClickHouse sort key: (tenant_id, model_id, timestamp) — optimal for the query patterns.

## 4. Backpressure Design
Rust ingestion engine uses a bounded tokio::mpsc channel (capacity: 50k events).
Channel fills → try_send() returns Err → HTTP 503 with Retry-After: 1.
This means: we never accept more than we can durably buffer. 503 is honest; 
accepting events and silently dropping them is not.
Kafka drain task reads from channel and calls producer.produce() — decouples 
HTTP latency from Kafka produce latency.

## 5. Failure Mode Analysis

| Failure | Detection | Recovery | Data Loss |
|---|---|---|---|
| Kafka broker down | rdkafka delivery callback error | WAL buffers, replays on restart | None — WAL fsynced before produce |
| ClickHouse write timeout | batchInsert returns error | Circuit breaker → Redis overflow | None — overflow drains when CH recovers |
| Redis connection lost | PING fails at startup | Rate limiter fails open | None for data; rate limiting temporarily disabled |
| Ingestion engine OOM killed | Pod restart (K8s) | WAL replay on startup | None — WAL fsynced before channel send |
| Consumer crash | Kafka consumer group rebalance | Rejoin, read from last committed offset | None — at-least-once |

## 6. Scaling Strategy
Ingestion engine: stateless → horizontal scale freely. 
Redis holds rate limit state (distributed token bucket via Lua script).
WAL is local but flushed before response — no shared state needed.
HPA trigger: kafka_consumer_lag_current metric (not CPU — CPU doesn't reflect backlog).
ClickHouse scaling: columnar storage scales read throughput with replicas.

## 7. ClickHouse Schema Rationale
LowCardinality(String) for tenant_id and model_id: dictionary encoding reduces 
storage 10x for repeated values.
MergeTree ordered by (tenant_id, model_id, timestamp): optimal for the dominant 
query pattern — filter by tenant + model, range on time.
Materialized view for hourly cost rollup: zero-cost analytics — pre-aggregated 
at insert time, no scan needed for dashboard queries.
TTL 90 days: raw events expire, rollup view is permanent.

Write as a real engineering document. No bullet-point descriptions — explain 
the reasoning behind every decision as if defending it to a Principal Engineer.
```

**After both sessions:** Commit — `docs: initial README + DESIGN.md`

Also add the Mermaid architecture diagram to README. Use this prompt:

```
Generate a Mermaid flowchart LR diagram for infra-ai-streaming showing:

Left side: multiple "AI App" clients → POST /ingest (batched JSON)

Rust Ingestion Engine:
  validates schema → checks Redis rate limit → writes WAL (fsync) → 
  sends to kafka_tx channel → returns 202

Redis: two roles shown — "rate limit (token bucket)" and "overflow buffer"

Kafka/Redpanda: three topics — ai_inference_events, ai_inference_dlq, ai_anomalies

Go Consumer:
  reads from ai_inference_events → 
  BatchWriter (1000 events or 500ms) → 
  CircuitBreaker check →
  Primary path: ClickHouse batch insert
  Fallback path (CB open): Redis overflow
  Also: AnomalyDetector → publishes to ai_anomalies topic

ClickHouse → Grafana (reads for dashboards)
Prometheus ← scrapes both Rust engine (:9090/metrics) and Go consumer (:9091/metrics)

Keep it readable. Label every arrow with the action.
Output: complete Mermaid code block.
```

---

# DAY 2 — Rust Ingestion Engine (Part 1)

**Goal:** Cargo.toml + config + metrics + WAL writer  
**Time:** ~4 hours  
**Cursor sessions:** 4 (one per file)

These are the foundation files. Every other Rust file imports from these.

---

### Session 2A — Cargo.toml + config.rs

```
Initialize the Rust ingestion engine for infra-ai-streaming.

Create ingestion/Cargo.toml:

[package]
name = "ingestion"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = { version = "0.7", features = ["json", "http1", "tokio"] }
tower = "0.4"
tower-http = { version = "0.5", features = ["trace", "timeout", "limit"] }
tokio = { version = "1", features = ["full"] }
rdkafka = { version = "0.36", features = ["cmake-build"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
redis = { version = "0.25", features = ["tokio-comp", "script"] }
prometheus = { version = "0.13", features = ["process"] }
opentelemetry = "0.22"
opentelemetry-otlp = { version = "0.15", features = ["grpc-tonic"] }
opentelemetry_sdk = { version = "0.22", features = ["rt-tokio"] }
tracing = "0.1"
tracing-subscriber = { version = "0.3", features = ["env-filter", "json"] }
tracing-opentelemetry = "0.23"
uuid = { version = "1", features = ["v4"] }
anyhow = "1"
thiserror = "1"
bytes = "1"

Create ingestion/src/config.rs:

Config struct — all fields loaded from environment variables with defaults:
  kafka_brokers: String = "localhost:9092"        (KAFKA_BROKERS)
  kafka_topic: String = "ai_inference_events"     (KAFKA_TOPIC)
  kafka_dlq_topic: String = "ai_inference_dlq"    (KAFKA_DLQ_TOPIC)
  redis_url: String = "redis://localhost:6379"    (REDIS_URL)
  http_port: u16 = 8080                           (HTTP_PORT)
  wal_dir: String = "/tmp/wal"                    (WAL_DIR)
  rate_limit_default_rps: u32 = 10_000            (RATE_LIMIT_DEFAULT_RPS)
  rate_limit_burst_multiplier: f32 = 2.0          (RATE_LIMIT_BURST_MULTIPLIER)
  batch_channel_capacity: usize = 50_000          (BATCH_CHANNEL_CAPACITY)
  max_batch_size: usize = 1_000                   (MAX_BATCH_SIZE)
  max_event_age_ms: u64 = 3_600_000              (MAX_EVENT_AGE_MS)

impl Config {
  pub fn from_env() -> anyhow::Result<Self>
  // Use std::env::var with defaults. No external crate needed.
  // Return descriptive error if any parse fails (e.g. "HTTP_PORT must be a valid u16")
}

No unwrap(). Return anyhow::Result from from_env().
```

---

### Session 2B — metrics.rs

```
Implement ingestion/src/metrics.rs for infra-ai-streaming.

Use the prometheus crate. All metrics are lazy_static globals.

Define exactly these metrics (names must match — Prometheus scrape configs depend on them):

INGESTION_LATENCY_MS: HistogramVec
  labels: ["tenant_id", "status"]
  buckets: [1.0, 5.0, 10.0, 25.0, 50.0, 100.0, 250.0, 500.0, 1000.0]
  help: "HTTP ingest handler latency in milliseconds"

KAFKA_PRODUCE_ERRORS_TOTAL: CounterVec
  labels: ["tenant_id", "error_type"]
  help: "Total Kafka produce errors by tenant and error type"

BATCH_SIZE_EVENTS: HistogramVec
  labels: ["tenant_id"]
  buckets: [1.0, 10.0, 50.0, 100.0, 250.0, 500.0, 1000.0]
  help: "Number of events per ingest batch"

WAL_SEGMENTS_PENDING: Gauge
  help: "Number of WAL segments with un-acked entries"

WAL_REPLAY_EVENTS_TOTAL: Counter
  help: "Total events replayed from WAL on startup"

RATE_LIMITED_REQUESTS_TOTAL: CounterVec
  labels: ["tenant_id"]
  help: "Requests rejected due to rate limiting"

BACKPRESSURE_EVENTS_TOTAL: Counter
  help: "Requests rejected due to full internal channel (backpressure)"

pub fn gather_metrics() -> String
  Returns prometheus::TextEncoder text for GET /metrics handler
```

---

### Session 2C — WAL writer

```
Implement ingestion/src/wal/writer.rs for infra-ai-streaming.

This is the durability guarantee: events are fsynced to disk BEFORE Kafka produce.
If the engine crashes, replay_unacked() returns everything that wasn't Kafka-confirmed.

WalEntry (serde Serialize + Deserialize):
  entry_id: u64
  batch_id: String
  events_json: String
  written_at_unix_ms: u64
  kafka_acked: bool

WalWriter:
  base_dir: PathBuf
  current_file: std::io::BufWriter<std::fs::File>  (open in append mode)
  current_segment_id: u64
  current_bytes: usize
  max_segment_bytes: usize  (default 64MB = 67_108_864)
  next_entry_id: std::sync::atomic::AtomicU64

Methods:

WalWriter::new(base_dir: &str) -> anyhow::Result<Self>
  Create base_dir if not exists
  Find highest segment_id (list files matching segment_*.wal), open for append
  If no segments exist, create segment_0000000001.wal
  Update WAL_SEGMENTS_PENDING gauge

append(&mut self, batch_id: &str, events_json: &str) -> anyhow::Result<u64>
  Increment and get entry_id from AtomicU64
  Serialize WalEntry to JSON, write as single line + '\n'
  CRITICAL: call self.current_file.flush() then get the underlying File and call sync_all()
  If current_bytes > max_segment_bytes: rotate to next segment
  Update current_bytes
  Return entry_id

mark_acked(&self, entry_id: u64) -> anyhow::Result<()>
  Write empty file at: {base_dir}/acks/{entry_id}.ack
  Create acks/ subdirectory if not exists

replay_unacked(&self) -> anyhow::Result<Vec<WalEntry>>
  Read all .wal files in base_dir in segment order
  Parse each newline-delimited JSON line as WalEntry
  Skip lines that parse to entries where {base_dir}/acks/{entry_id}.ack exists
  Return remaining entries (un-acked)
  For each returned entry: increment WAL_REPLAY_EVENTS_TOTAL counter
  Update WAL_SEGMENTS_PENDING gauge

File naming: segment_{id:010}.wal  (zero-padded for lexicographic sort)
fsync on every append — never skip this. It is the contract.
No unwrap(). Every I/O error is anyhow::Error with context.
```

---

### Session 2D — Rate limiter

```
Implement ingestion/src/rate_limit/token_bucket.rs for infra-ai-streaming.

Per-tenant rate limiting via atomic Redis Lua script.
Must work correctly across multiple engine instances (distributed token bucket).

RateLimiter:
  redis_client: redis::Client
  default_rps: u32
  burst_multiplier: f32

RateLimitResult:
  Allowed { remaining: u32 }
  Denied { retry_after_ms: u64 }

impl RateLimiter {
  pub fn new(redis_url: &str, default_rps: u32, burst_multiplier: f32) -> anyhow::Result<Self>
  
  pub async fn check_and_consume(&self, tenant_id: &str, cost: u32) -> anyhow::Result<RateLimitResult>
}

The Lua script (execute via redis::Script):

local key = "ratelimit:" .. KEYS[1]
local now = tonumber(ARGV[1])
local default_rps = tonumber(ARGV[2])
local burst_mult = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local capacity = math.floor(default_rps * burst_mult)

local data = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(data[1]) or capacity
local last_refill = tonumber(data[2]) or now

local elapsed_ms = now - last_refill
local refilled = (elapsed_ms / 1000.0) * default_rps
local new_tokens = math.min(capacity, tokens + refilled)

if new_tokens >= cost then
  redis.call("HMSET", key, "tokens", new_tokens - cost, "last_refill", now)
  redis.call("EXPIRE", key, 60)
  return {1, math.floor(new_tokens - cost)}
else
  local needed = cost - new_tokens
  local retry_ms = math.ceil((needed / default_rps) * 1000)
  return {0, retry_ms}
end

Pass KEYS=[tenant_id], ARGV=[now_ms, default_rps, burst_multiplier, cost]
Map: {1, remaining} → Allowed, {0, retry_ms} → Denied

IMPORTANT: On any Redis connection error → return Ok(Allowed{remaining: 0})
Fail open — never drop traffic because Redis is unavailable.
On Denied → increment RATE_LIMITED_REQUESTS_TOTAL{tenant_id=tenant_id}
```

**End of Day 2:** Commit — `feat(ingestion): config, metrics, WAL writer, rate limiter`

---

# DAY 3 — Rust Ingestion Engine (Part 2) + Kafka Producer

**Goal:** Kafka producer + ingest handler + server + main. Engine compiles and runs.  
**Time:** ~4 hours  
**Cursor sessions:** 4

---

### Session 3A — Kafka producer

```
Implement ingestion/src/kafka/producer.rs for infra-ai-streaming.

ProduceMessage:
  batch_id: String
  partition_key: String   (use tenant_id — ordering per tenant)
  payload: bytes::Bytes
  wal_entry_id: u64

KafkaProducer:
  producer: rdkafka::producer::FutureProducer
  topic: String
  dlq_topic: String

impl KafkaProducer {

  pub fn new(brokers: &str, topic: &str, dlq_topic: &str) -> anyhow::Result<Self>
    ClientConfig:
      bootstrap.servers = brokers
      message.timeout.ms = 5000
      enable.idempotence = true
      acks = all
      compression.type = lz4
      batch.size = 1048576
      linger.ms = 5
      retries = 3

  pub async fn produce(
    &self,
    msg: ProduceMessage,
    wal: Arc<tokio::sync::Mutex<WalWriter>>
  ) -> anyhow::Result<()>
    
    Use FutureProducer.send() with timeout Duration::from_secs(5)
    On success (delivery confirmed by Kafka):
      wal.lock().await.mark_acked(msg.wal_entry_id)?
    On failure after rdkafka retries:
      Attempt to send to dlq_topic with same payload
      Increment KAFKA_PRODUCE_ERRORS_TOTAL{tenant_id from partition_key, "max_retries"}
      Log: tracing::error!(batch_id, error=?e, "Kafka produce failed, sent to DLQ")
      Return Err (caller should not 503 — WAL preserved the event)
}

The WAL ack happens ONLY after Kafka delivery confirmation.
This is the durability contract: WAL entry stays live until Kafka says it's safe.
```

---

### Session 3B — Ingest handler

```
Implement ingestion/src/handlers/ingest.rs for infra-ai-streaming.

InferenceEvent (serde Deserialize + Serialize):
  event_id: Option<String>           (generated server-side if None)
  tenant_id: String
  model_id: String
  timestamp_unix_ms: u64
  latency_ms: u32
  prefill_latency_ms: Option<u32>
  decode_latency_ms: Option<u32>
  prompt_tokens: u32
  completion_tokens: u32
  cost_usd: f64
  status: Option<String>             (default "success")
  error_code: Option<String>
  request_id: Option<String>

IngestRequest: { events: Vec<InferenceEvent> }
IngestResponse: { batch_id: String, event_count: usize, accepted_at_unix_ms: u64 }

AppState (Clone via Arc):
  config: Arc<Config>
  kafka_tx: tokio::sync::mpsc::Sender<ProduceMessage>
  wal_writer: Arc<tokio::sync::Mutex<WalWriter>>
  rate_limiter: Arc<RateLimiter>

Handler function: async fn handle_ingest(
  State(state): State<AppState>,
  headers: HeaderMap,
  Json(body): Json<IngestRequest>
) -> impl IntoResponse

Logic (in strict order):
  1. Record start = Instant::now()
  2. Extract X-Tenant-ID header → if missing: return (StatusCode::BAD_REQUEST, 
     Json(json!({"error": "missing_header", "header": "X-Tenant-ID"})))
  3. Validation:
     a. events.is_empty() → 400 {"error": "empty_batch"}
     b. events.len() > config.max_batch_size → 400 {"error": "batch_too_large", "max": N}
     c. For each event: latency_ms == 0 → 400 {"error": "invalid_latency", "event_id": "..."}
     d. For each event: cost_usd < 0.0 → 400 {"error": "invalid_cost"}
     e. For each event: timestamp is older than max_event_age_ms → 400 {"error": "event_too_old"}
  4. Assign event_id (UUID v4) to any event missing one
  5. Set status = "success" on any event where it's None
  6. rate_limiter.check_and_consume(&tenant_id, events.len() as u32).await?
     → Denied { retry_after_ms }: return 429 with 
       header Retry-After: {retry_after_ms / 1000 + 1}
       body: {"error": "rate_limit_exceeded", "retry_after_ms": N}
  7. Serialize events to JSON bytes (serde_json::to_vec)
  8. Generate batch_id = Uuid::new_v4().to_string()
  9. wal_writer.lock().await.append(&batch_id, &json_string)?
     → On error: return 503 {"error": "wal_failure"} — do NOT proceed
  10. kafka_tx.try_send(ProduceMessage { batch_id, partition_key: tenant_id, 
       payload: Bytes::from(json_bytes), wal_entry_id })
      → Err (channel full): 
        BACKPRESSURE_EVENTS_TOTAL.inc()
        return 503 with header Retry-After: 1, body {"error": "backpressure"}
  11. INGESTION_LATENCY_MS.with_label_values(&[&tenant_id, "success"])
        .observe(start.elapsed().as_millis() as f64)
  12. BATCH_SIZE_EVENTS.with_label_values(&[&tenant_id])
        .observe(events.len() as f64)
  13. Return 202 Json(IngestResponse { batch_id, event_count, accepted_at_unix_ms: now_ms })

Use #[tracing::instrument(skip(state, body))] on the handler.
No unwrap(). Every ? must have context via .context("...").
```

---

### Session 3C — Server + main

```
Implement ingestion/src/server.rs and ingestion/src/main.rs for infra-ai-streaming.

server.rs:

pub fn build_router(state: AppState) -> Router
  Routes:
    POST /ingest  → handle_ingest
    GET  /health  → async || (StatusCode::OK, Json(json!({"status":"ok"})))
    GET  /metrics → async || {
      ([(header::CONTENT_TYPE, "text/plain; version=0.0.4")], gather_metrics())
    }
  
  Tower middleware layers (applied in this order):
    ConcurrencyLimitLayer::new(state.config.max_concurrent_requests)  — needs config in scope
    TimeoutLayer::new(Duration::from_secs(30))
    TraceLayer::new_for_http()
  
  .with_state(state)

pub async fn serve(config: Arc<Config>, state: AppState) -> anyhow::Result<()>
  let addr = SocketAddr::from(([0, 0, 0, 0], config.http_port))
  let router = build_router(state)
  let listener = tokio::net::TcpListener::bind(addr).await?
  tracing::info!(port = config.http_port, "ingestion engine listening")
  
  axum::serve(listener, router)
    .with_graceful_shutdown(shutdown_signal())
    .await?
  Ok(())

async fn shutdown_signal()
  Wait for SIGTERM or SIGINT using tokio::signal

main.rs:

#[tokio::main]
async fn main() -> anyhow::Result<()>

Startup sequence (order is important):
  1. let config = Arc::new(Config::from_env()
       .context("failed to load config from environment")?);
  
  2. Init tracing:
     if std::env::var("LOG_FORMAT").unwrap_or_default() == "json" {
       tracing_subscriber::fmt().json().with_env_filter(...).init()
     } else {
       tracing_subscriber::fmt().pretty().with_env_filter(...).init()
     }
  
  3. Init OTel (skip if OTEL_EXPORTER_OTLP_ENDPOINT not set):
     if let Ok(endpoint) = std::env::var("OTEL_EXPORTER_OTLP_ENDPOINT") {
       // init opentelemetry-otlp tracer
     }
  
  4. Connect Redis + verify PING:
     let redis_client = redis::Client::open(config.redis_url.as_str())?
     let mut conn = redis_client.get_async_connection().await
       .context("Redis unreachable — is Redis running?")?
     redis::cmd("PING").query_async::<_, ()>(&mut conn).await
       .context("Redis PING failed")?
     tracing::info!("Redis connected")
  
  5. Init WAL:
     let mut wal = WalWriter::new(&config.wal_dir)?
     let unacked = wal.replay_unacked()?
     let replay_count = unacked.len()
     tracing::info!(replayed = replay_count, "WAL replay complete")
     let wal = Arc::new(tokio::sync::Mutex::new(wal))
  
  6. Init Kafka producer:
     let producer = Arc::new(KafkaProducer::new(
       &config.kafka_brokers, &config.kafka_topic, &config.kafka_dlq_topic)?
     )
  
  7. Create mpsc channel:
     let (kafka_tx, mut kafka_rx) = 
       tokio::sync::mpsc::channel::<ProduceMessage>(config.batch_channel_capacity)
  
  8. Re-enqueue WAL unacked entries:
     for entry in unacked {
       let msg = ProduceMessage { ... reconstruct from WalEntry ... }
       kafka_tx.send(msg).await.ok()  // best-effort on startup replay
     }
  
  9. Spawn Kafka drain task:
     let producer_clone = Arc::clone(&producer)
     let wal_clone = Arc::clone(&wal)
     tokio::spawn(async move {
       while let Some(msg) = kafka_rx.recv().await {
         if let Err(e) = producer_clone.produce(msg, Arc::clone(&wal_clone)).await {
           tracing::error!(error=?e, "Kafka produce error in drain task")
         }
       }
     })
  
  10. Init rate limiter:
      let rate_limiter = Arc::new(RateLimiter::new(
        &config.redis_url, config.rate_limit_default_rps, 
        config.rate_limit_burst_multiplier)?
      )
  
  11. Build AppState + serve:
      let state = AppState { config: Arc::clone(&config), kafka_tx, wal_writer: wal, rate_limiter }
      server::serve(config, state).await
```

**End of Day 3:** Commit — `feat(ingestion): Kafka producer, ingest handler, server — engine runnable`

**Verify:** `cargo build` in ingestion/ should compile cleanly.

---

# DAY 4 — Docker Compose Stack + ClickHouse Schema

**Goal:** One-command local stack. ClickHouse schema live. Verify end-to-end.  
**Time:** ~3 hours  
**Cursor sessions:** 2

---

### Session 4A — Docker Compose + ClickHouse schema

```
Create deploy/docker-compose.yml for infra-ai-streaming.

All services on a single bridge network: infra-ai-net

Services:

1. redpanda:
   image: redpandadata/redpanda:v23.3.5
   container_name: infra-ai-redpanda
   command: >
     redpanda start --smp 1 --memory 512M --reserve-memory 0M
     --overprovisioned --node-id 0
     --kafka-addr PLAINTEXT://0.0.0.0:9092
     --advertise-kafka-addr PLAINTEXT://localhost:9092
     --pandaproxy-addr 0.0.0.0:8082
   ports: ["9092:9092", "9644:9644", "8082:8082"]
   healthcheck:
     test: ["CMD", "rpk", "cluster", "health", "--watch=false"]
     interval: 5s, timeout: 3s, retries: 5

2. redpanda-init (creates topics):
   image: redpandadata/redpanda:v23.3.5
   depends_on: { redpanda: { condition: service_healthy } }
   entrypoint: ["/bin/bash", "-c"]
   command: >
     "rpk topic create ai_inference_events ai_inference_dlq ai_anomalies
      --partitions 8 --replicas 1 -X brokers=redpanda:9092"
   restart: "no"

3. clickhouse:
   image: clickhouse/clickhouse-server:24-alpine
   container_name: infra-ai-clickhouse
   ports: ["8123:8123", "9000:9000"]
   environment:
     CLICKHOUSE_DB: default
     CLICKHOUSE_USER: default
     CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: "1"
   volumes:
     - ./clickhouse/init.sql:/docker-entrypoint-initdb.d/init.sql
   healthcheck:
     test: ["CMD", "wget", "-q", "--spider", "http://localhost:8123/ping"]
     interval: 5s, timeout: 3s, retries: 10

4. redis:
   image: redis:7-alpine
   container_name: infra-ai-redis
   ports: ["6379:6379"]
   command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
   healthcheck:
     test: ["CMD", "redis-cli", "ping"]
     interval: 5s, retries: 5

5. prometheus:
   image: prom/prometheus:v2.51.0
   container_name: infra-ai-prometheus
   ports: ["9090:9090"]
   volumes:
     - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
   command:
     - --config.file=/etc/prometheus/prometheus.yml
     - --storage.tsdb.retention.time=7d

6. grafana:
   image: grafana/grafana:10.4.0
   container_name: infra-ai-grafana
   ports: ["3000:3000"]
   environment:
     GF_SECURITY_ADMIN_PASSWORD: admin
     GF_AUTH_ANONYMOUS_ENABLED: "true"
     GF_AUTH_ANONYMOUS_ORG_ROLE: Viewer
     GF_INSTALL_PLUGINS: grafana-clickhouse-datasource
   volumes:
     - ./grafana/datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
     - ../../dashboards/grafana:/var/lib/grafana/dashboards
   depends_on: [clickhouse, prometheus]

All services: restart: unless-stopped (except redpanda-init)

---

Also create these config files:

deploy/prometheus/prometheus.yml:
  global: { scrape_interval: 15s }
  scrape_configs:
    - job_name: ingestion-engine
      static_configs: [{targets: ["host.docker.internal:9090"]}]
      -- Note: metrics server runs on port 9090 separate from HTTP server on 8080
      -- Use host.docker.internal because engine runs outside Docker in dev
    - job_name: go-consumer
      static_configs: [{targets: ["host.docker.internal:9091"]}]

deploy/grafana/datasources.yml:
  apiVersion: 1
  datasources:
    - name: ClickHouse
      type: grafana-clickhouse-datasource
      url: http://clickhouse:8123
      jsonData: { defaultDatabase: default }
    - name: Prometheus
      type: prometheus
      url: http://prometheus:9090
      isDefault: true

---

Also create deploy/clickhouse/init.sql:

CREATE TABLE IF NOT EXISTS ai_inference_events
(
  event_id        UUID DEFAULT generateUUIDv4(),
  tenant_id       LowCardinality(String),
  model_id        LowCardinality(String),
  timestamp       DateTime64(3, 'UTC'),
  latency_ms      UInt32,
  prefill_ms      UInt32 DEFAULT 0,
  decode_ms       UInt32 DEFAULT 0,
  prompt_tokens   UInt32,
  completion_tokens UInt32,
  cost_usd        Float64,
  status          LowCardinality(String) DEFAULT 'success',
  error_code      Nullable(String),
  request_id      String DEFAULT '',
  ingested_at     DateTime64(3, 'UTC') DEFAULT now64()
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (tenant_id, model_id, timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS ai_cost_hourly
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMMDD(hour)
ORDER BY (tenant_id, model_id, hour)
AS SELECT
  tenant_id,
  model_id,
  toStartOfHour(timestamp) AS hour,
  sum(cost_usd)            AS total_cost_usd,
  sum(prompt_tokens)       AS total_prompt_tokens,
  sum(completion_tokens)   AS total_completion_tokens,
  count()                  AS request_count,
  quantile(0.99)(latency_ms) AS p99_latency_ms
FROM ai_inference_events
GROUP BY tenant_id, model_id, hour;
```

---

### Session 4B — Go consumer

```
Create the complete Go consumer for infra-ai-streaming.

go.mod:
  module github.com/YOURUSERNAME/infra-ai-streaming/consumer
  go 1.22
  
  require:
    github.com/twmb/franz-go v1.16.1
    github.com/ClickHouse/clickhouse-go/v2 v2.23.2
    github.com/redis/go-redis/v9 v9.5.1
    github.com/prometheus/client_golang v1.19.1
    go.uber.org/zap v1.27.0

--- consumer/internal/metrics/server.go ---

Define Prometheus metrics (prometheus/client_golang):
  KafkaConsumerLag: GaugeVec (labels: partition)
  KafkaRecordsProcessed: Counter
  KafkaDeserErrors: Counter
  ClickhouseWriteErrors: Counter
  ClickhouseBatchSize: HistogramVec (labels: status)
     buckets: [10, 50, 100, 250, 500, 1000, 5000]
  CircuitBreakerState: GaugeVec (labels: state)  
     // call with state="closed"/"open"/"halfopen", value=1.0 for active, 0.0 for inactive
  RedisOverflowDepth: Gauge
  AnomaliesDetected: CounterVec (labels: tenant_id, model_id)
  DLQEvents: Counter

func StartMetricsServer(port int, logger *zap.Logger)
  http.Handle("/metrics", promhttp.Handler())
  go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

--- consumer/internal/clickhouse/writer.go ---

InferenceEvent struct (matches JSON schema — use json tags):
  EventID, TenantID, ModelID, Status, ErrorCode, RequestID string
  TimestampUnixMs uint64
  LatencyMs, PrefillMs, DecodeMs, PromptTokens, CompletionTokens uint32
  CostUsd float64

CircuitBreaker (implement inline):
  states: Closed=0, Open=1, HalfOpen=2
  failureCount int, lastOpenTime time.Time
  threshold: 5 failures → Open
  resetTimeout: 30 seconds → HalfOpen
  halfOpenSuccess: 1 success → Closed
  Method: State() int, RecordSuccess(), RecordFailure()
  Update CircuitBreakerState metric on every state change

BatchWriter struct:
  conn driver.Conn
  batchSize int  (default 1000)
  flushInterval time.Duration  (default 500ms)
  buf []InferenceEvent
  mu sync.Mutex
  cb *CircuitBreaker
  overflow OverflowBuffer
  metrics *Metrics
  logger *zap.Logger

Methods:
  NewBatchWriter(dsn string, overflow OverflowBuffer, m *Metrics, log *zap.Logger) (*BatchWriter, error)
  Add(event InferenceEvent)  — lock, append, if len >= batchSize → go flush()
  Start(ctx context.Context) — ticker every flushInterval, flush if buf non-empty
                              — every 5s: drain up to 5000 events from overflow if CB closed
  flush() — drain buf under lock FIRST, then release lock, THEN do I/O
           — if CB open: push to overflow, return
           — else: batchInsert; on error: cb.RecordFailure(), push to overflow
                   on success: cb.RecordSuccess()

batchInsert(events []InferenceEvent) error:
  batch, err := conn.PrepareBatch(ctx, `INSERT INTO ai_inference_events 
    (event_id, tenant_id, model_id, timestamp, latency_ms, prefill_ms, decode_ms,
     prompt_tokens, completion_tokens, cost_usd, status, error_code, request_id) VALUES`)
  for _, e := range events:
    batch.Append(e.EventID, e.TenantID, e.ModelID, 
      time.UnixMilli(int64(e.TimestampUnixMs)).UTC(),
      e.LatencyMs, e.PrefillMs, e.DecodeMs, e.PromptTokens, e.CompletionTokens,
      e.CostUsd, e.Status, e.ErrorCode, e.RequestID)
  return batch.Send()

--- consumer/internal/redis/overflow.go ---

type OverflowBuffer interface {
  Push(events []InferenceEvent) error
  PopN(n int) ([]InferenceEvent, error)
  Depth() (int64, error)
}

RedisOverflow struct { client *redis.Client; key string }

Push: LPUSH key json(event) for each event, update RedisOverflowDepth gauge
PopN: LMPOP 1 key RIGHT COUNT n (or RPOP key for older Redis), deserialize
Depth: LLEN key, update gauge, return count

--- consumer/internal/anomaly/detector.go ---

SlidingWindow: ring buffer []float64, size int, head int, count int
  Add(v float64), Mean() float64, StdDev() float64 (Welford's algorithm)

AnomalyDetector:
  windows map[string]*SlidingWindow  (key: "tenantID:modelID")
  mu sync.RWMutex
  threshold float64 (default 3.0), windowSize int (100), minSamples int (20)
  producer KafkaProducer interface
  metrics *Metrics, logger *zap.Logger

Observe(tenantID, modelID string, latencyMs float64) — get or create window,
  add value, if count >= minSamples: compute z-score, if abs > threshold:
  emit to ai_anomalies Kafka topic + increment AnomaliesDetected counter

--- consumer/internal/kafka/reader.go ---

Config: group "ai-inference-consumer-v1", topic ai_inference_events
  session.timeout.ms 30000, heartbeat.interval.ms 3000

Run(ctx context.Context):
  Poll loop using franz-go kgo.Client.PollFetches(ctx)
  For each record:
    Deserialize JSON payload as struct with events: []InferenceEvent
    For each event: batchWriter.Add(event) + anomalyDetector.Observe(...)
    Increment KafkaRecordsProcessed
    On deserialization error: DLQEvents.Inc(), log and skip (don't crash)
  After processing all records in a fetch: commit offsets

--- consumer/cmd/consumer/main.go ---

Startup:
  1. Config from env (same var names as ingestion engine, plus CLICKHOUSE_DSN, METRICS_PORT=9091)
  2. Init zap logger
  3. StartMetricsServer(9091, logger)
  4. Connect ClickHouse + ping
  5. Connect Redis + ping
  6. Init RedisOverflow
  7. Init BatchWriter, start it in goroutine
  8. Init AnomalyDetector
  9. Init Kafka reader
  10. reader.Run(ctx)
  11. SIGTERM/SIGINT: cancel ctx, flush BatchWriter, close Kafka client

All I/O with context.Context. Use zap for all logging — no fmt.Println.
```

**End of Day 4:** Commit — `feat: docker-compose stack, ClickHouse schema, Go consumer — pipeline runnable`

**Verify:** `docker compose -f deploy/docker-compose.yml up -d` starts all services healthy.

---

# DAY 5 — Grafana Dashboard + Load Test + OBSERVABILITY.md

**Goal:** Live dashboard. Real benchmark numbers. Self-observability documented.  
**Time:** ~3 hours  
**Cursor sessions:** 3

---

### Session 5A — Grafana dashboard

```
Create dashboards/grafana/ai-inference.json for infra-ai-streaming.

A valid Grafana 10.x dashboard JSON (importable via Grafana UI → Dashboards → Import).

Dashboard settings:
  title: "AI Inference Observability"
  uid: "ai-inference-v1"
  refresh: "10s"
  time: { from: "now-1h", to: "now" }
  timezone: "browser"

4 panels:

Panel 1 — "Events/sec by Tenant" (top-left)
  type: timeseries
  datasource: Prometheus
  query: sum(rate(kafka_records_processed_total[1m])) by (tenant_id)
  legend: {{tenant_id}}
  unit: short
  gridPos: { x:0, y:0, w:12, h:8 }

Panel 2 — "P99 Latency by Model (ms)" (top-right)
  type: timeseries
  datasource: Prometheus
  query: histogram_quantile(0.99, sum(rate(ingestion_latency_ms_bucket[5m])) by (le, tenant_id))
  unit: ms
  thresholds: warning=200ms, critical=500ms
  gridPos: { x:12, y:0, w:12, h:8 }

Panel 3 — "Hourly Cost (USD) by Tenant" (bottom-left)
  type: barchart
  datasource: ClickHouse
  rawSQL: |
    SELECT toStartOfHour(hour) as time, tenant_id, sum(total_cost_usd) as cost
    FROM ai_cost_hourly
    WHERE hour >= now() - INTERVAL 24 HOUR
    GROUP BY time, tenant_id ORDER BY time
  unit: currencyUSD
  gridPos: { x:0, y:8, w:12, h:8 }

Panel 4 — "Kafka Consumer Lag" (bottom-right)
  type: timeseries
  datasource: Prometheus
  query: kafka_consumer_lag_current
  unit: short
  thresholds: warning=10000, critical=50000
  gridPos: { x:12, y:8, w:12, h:8 }

Output: complete, valid Grafana dashboard JSON.
```

---

### Session 5B — k6 load test

```
Create load-test/k6-script.js for infra-ai-streaming.

import http from 'k6/http'
import { check, sleep } from 'k6'
import { Rate, Trend } from 'k6/metrics'

const errorRate = new Rate('errors')
const ingestionLatency = new Trend('ingestion_latency', true)

export const options = {
  stages: [
    { duration: '30s', target: 50 },    // ramp: ~5k events/sec
    { duration: '2m',  target: 50 },    // hold: ~5k events/sec
    { duration: '30s', target: 200 },   // ramp: ~20k events/sec
    { duration: '2m',  target: 200 },   // hold: ~20k events/sec
    { duration: '30s', target: 0 },     // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(99)<100'],   // P99 under 100ms — this is the target
    http_req_failed: ['rate<0.01'],
    errors: ['rate<0.01'],
  },
}

const TENANTS = ['alpha','beta','gamma','delta','epsilon','zeta','eta','theta','iota','kappa']
const MODELS = ['gpt-4o','claude-sonnet-4-20250514','llama-3-70b','mistral-large-2','gemini-1.5-pro']
const BASE_URL = __ENV.INGEST_URL || 'http://localhost:8080'

function randomInt(min, max) { return Math.floor(Math.random() * (max - min + 1)) + min }
function randomFrom(arr) { return arr[Math.floor(Math.random() * arr.length)] }

export default function () {
  const tenant = randomFrom(TENANTS)
  const batchSize = 100
  const events = []

  for (let i = 0; i < batchSize; i++) {
    const promptTokens = randomInt(100, 2000)
    const completionTokens = randomInt(50, 500)
    const isSpike = Math.random() < 0.05  // 5% latency spikes
    events.push({
      tenant_id: tenant,
      model_id: randomFrom(MODELS),
      timestamp_unix_ms: Date.now() - randomInt(0, 5000),
      latency_ms: isSpike ? randomInt(5000, 10000) : randomInt(50, 2000),
      prefill_latency_ms: randomInt(20, 200),
      decode_latency_ms: randomInt(30, 1800),
      prompt_tokens: promptTokens,
      completion_tokens: completionTokens,
      cost_usd: parseFloat(((promptTokens * 0.000005) + (completionTokens * 0.000015)).toFixed(6)),
      status: Math.random() < 0.98 ? 'success' : 'error',
    })
  }

  const start = Date.now()
  const res = http.post(
    `${BASE_URL}/ingest`,
    JSON.stringify({ events }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-Tenant-ID': tenant,
      },
      timeout: '10s',
    }
  )

  ingestionLatency.add(Date.now() - start)
  errorRate.add(res.status >= 400 && res.status !== 429)
  check(res, {
    'accepted (202)': r => r.status === 202,
    'rate limited (429)': r => r.status === 429,
    'not server error': r => r.status < 500,
  })

  sleep(0.1)  // 100ms between iterations per VU → ~10 batches/sec per VU × 100 events = 1000 events/sec per VU
}

export function handleSummary(data) {
  return {
    'load-test/results.json': JSON.stringify(data, null, 2),
    stdout: textSummary(data, { indent: '  ', enableColors: true }),
  }
}

// After running: k6 run load-test/k6-script.js
// Update BENCHMARKS.md with the p99 value from http_req_duration
```

---

### Session 5C — OBSERVABILITY.md + CHAOS.md

```
Write two documents for infra-ai-streaming:

=== OBSERVABILITY.md ===

Title: "The Pipeline Observing Itself"

Intro paragraph: explain why the observability pipeline is itself fully observable —
"eating our own dog food" means you can detect if the pipeline is falling behind 
before your customers notice.

Then document every metric with:
  ### metric_name
  Type: Histogram | Counter | Gauge | CounterVec
  Source: ingestion engine (:9090) | go consumer (:9091)
  What it measures: [one sentence]
  Alert on: [PromQL threshold suggestion]

Ingestion engine metrics:
  ingestion_latency_ms{tenant_id, status}
  kafka_produce_errors_total{tenant_id, error_type}
  batch_size_events{tenant_id}
  wal_segments_pending
  wal_replay_events_total
  rate_limited_requests_total{tenant_id}
  backpressure_events_total

Consumer metrics:
  kafka_consumer_lag_current{partition}
  kafka_records_processed_total
  kafka_deserialization_errors_total
  clickhouse_write_errors_total
  clickhouse_batch_size{status}
  circuit_breaker_state{state}
  redis_overflow_depth
  anomalies_detected_total{tenant_id, model_id}
  dlq_events_total

SLO section:
  SLO 1: Ingestion P99 < 100ms (5-min windows)
    PromQL: histogram_quantile(0.99, rate(ingestion_latency_ms_bucket[5m])) < 100
  SLO 2: Data loss rate < 0.01%
    PromQL: rate(dlq_events_total[1h]) / rate(kafka_records_processed_total[1h]) < 0.0001
  SLO 3: Consumer lag < 50k events
    PromQL: max(kafka_consumer_lag_current) < 50000
  SLO 4: Circuit breaker Open < 1% of time per hour
    PromQL: avg_over_time(circuit_breaker_state{state="open"}[1h]) < 0.01

=== CHAOS.md ===

Write a table-based chaos engineering document.

For each of 5 failure scenarios, provide:
  Failure name
  How to trigger (exact docker command)
  Expected system behavior
  Recovery mechanism  
  Data loss guarantee
  Which chaos/run_chaos.sh test validates it

Scenarios: 
  1. Kafka broker death (docker compose stop redpanda)
  2. ClickHouse slow writes (SET max_threads=1)
  3. Redis connection lost (docker compose stop redis)
  4. Ingestion engine OOM (docker compose kill -s KILL ingestion)
  5. Consumer crash (docker compose kill -s KILL consumer)

Write it as if you're a Staff Engineer documenting runbook behavior,
not a tutorial author explaining what chaos engineering is.
```

**End of Day 5:** Commit — `feat: Grafana dashboard, k6 load test, OBSERVABILITY.md, CHAOS.md`

---

# DAY 6 — CI Pipeline + BENCHMARKS.md + Final Polish

**Goal:** Green CI badge. Benchmark numbers filled in. Repo is recruiter-ready.  
**Time:** ~3 hours  
**Cursor sessions:** 2

---

### Session 6A — GitHub Actions CI

```
Create .github/workflows/ci.yml for infra-ai-streaming.

Triggers: push to main, pull_request to main

Jobs:

1. rust-checks:
   runs-on: ubuntu-latest
   steps:
     - uses: actions/checkout@v4
     - uses: dtolnay/rust-toolchain@stable
         with: { components: "clippy, rustfmt" }
     - uses: Swatinem/rust-cache@v2  (cache Cargo deps)
     - name: Build
       run: cargo build --manifest-path ingestion/Cargo.toml
     - name: Test
       run: cargo test --manifest-path ingestion/Cargo.toml
     - name: Clippy (deny warnings)
       run: cargo clippy --manifest-path ingestion/Cargo.toml -- -D warnings
     - name: Format check
       run: cargo fmt --manifest-path ingestion/Cargo.toml -- --check

2. go-checks:
   runs-on: ubuntu-latest
   steps:
     - uses: actions/checkout@v4
     - uses: actions/setup-go@v5
         with: { go-version: "1.22" }
     - name: Build
       working-directory: consumer
       run: go build ./...
     - name: Test
       working-directory: consumer
       run: go test ./...
     - name: Vet
       working-directory: consumer
       run: go vet ./...

3. docker-compose-validate:
   runs-on: ubuntu-latest
   steps:
     - uses: actions/checkout@v4
     - name: Validate compose file
       run: docker compose -f deploy/docker-compose.yml config --quiet

After adding this file, update README.md badges:
[![CI](https://github.com/YOURUSERNAME/infra-ai-streaming/actions/workflows/ci.yml/badge.svg)](...)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Rust 1.77+](https://img.shields.io/badge/rust-1.77%2B-orange.svg)](https://rustup.rs/)
[![Go 1.22+](https://img.shields.io/badge/go-1.22%2B-blue.svg)](https://go.dev/)

Also add a LICENSE file (MIT) to the repo root.
```

---

### Session 6B — BENCHMARKS.md template

```
Create BENCHMARKS.md for infra-ai-streaming.

I'll fill in the actual numbers after running k6, but generate the complete 
structure with placeholder [TBD] values.

Sections:

## Hardware
Document where tests were run. Template:
  CPU: [your CPU]
  RAM: [your RAM]  
  Disk: [SSD/NVMe]
  OS: [your OS]
  Redpanda, ClickHouse, Redis: Docker on localhost
  Ingestion engine: native (outside Docker)

## Test Methodology
  k6 stages: ramp 0→50 VUs (30s), hold 50 VUs (2m), ramp 50→200 VUs (30s), hold 200 VUs (2m)
  Batch size per request: 100 events
  Tenants: 10 rotating
  Models: 5 (gpt-4o, claude-sonnet, llama-3-70b, mistral-large, gemini-1.5-pro)
  Latency distribution: 95% normal (50-2000ms), 5% spike (5-10s)
  Run command: k6 run load-test/k6-script.js

## Results

| Scenario | VUs | Events/sec | HTTP P50 | HTTP P99 | CH Write Lag | Kafka Lag (max) | Error Rate |
|---|---|---|---|---|---|---|---|
| 50 VUs (5k events/sec) | 50 | ~5,000 | [TBD] ms | [TBD] ms | [TBD] ms | [TBD] | [TBD]% |
| 200 VUs (20k events/sec) | 200 | ~20,000 | [TBD] ms | [TBD] ms | [TBD] ms | [TBD] | [TBD]% |

> Note: P99 target is < 100ms. Update [TBD] values by running k6 and checking 
> the results.json output + Grafana screenshots.

## Bottleneck Analysis
(Fill after running tests)
At 50 VUs: bottleneck is likely [TBD — Kafka produce latency / ClickHouse insert / Redis rate check]
At 200 VUs: bottleneck is likely [TBD]

## Comparison: Why Not Prometheus?
At this cardinality (10 tenants × 5 models = 50 series minimum, but real deployments 
have 100s of tenants and 10s of model versions):
- Prometheus series limit: ~10M series total (not per-metric)
- Our query pattern (avg cost per tenant-model per hour) requires full scan in Prometheus
- ClickHouse materialized view: pre-aggregated at insert time, sub-second regardless of volume

## How to Reproduce
Prerequisites: k6 installed (https://k6.io/docs/get-started/installation/)
Stack running: docker compose -f deploy/docker-compose.yml up -d
Engine running: cargo run --manifest-path ingestion/Cargo.toml
Consumer running: go run ./consumer/cmd/consumer/

Run: k6 run load-test/k6-script.js
Results: load-test/results.json + Grafana at http://localhost:3000
```

**End of Day 6:** Commit — `chore: CI pipeline, BENCHMARKS.md, MIT license — v0.1.0`

---

# DAY 7 — Run Everything + Fill Numbers + GitHub Polish

**Goal:** Actually run the full stack. Fill benchmark numbers. Screenshot Grafana. Repo is done.  
**Time:** ~4 hours  
**No Cursor today — this is execution day**

### Checklist:

```
□ docker compose -f deploy/docker-compose.yml up -d
  → Verify all services healthy: docker compose ps

□ cargo run --manifest-path ingestion/Cargo.toml &
  → Verify startup logs: "ingestion engine listening", "Redis connected"

□ go run ./consumer/cmd/consumer/ &
  → Verify startup logs: "ClickHouse connected", "Kafka consumer started"

□ Send 1 test event manually (curl command from README Getting Started)
  → Verify 202 response
  → Verify event appears in ClickHouse:
    docker exec infra-ai-clickhouse clickhouse-client \
      --query "SELECT count() FROM ai_inference_events"

□ Run load test:
  k6 run load-test/k6-script.js
  → Note P99 from output
  → Open Grafana (localhost:3000), take screenshot of all 4 panels under load

□ Fill BENCHMARKS.md:
  → Replace all [TBD] values with real numbers from k6 output

□ Add Grafana screenshot to repo:
  mkdir -p docs/
  → Copy screenshot to docs/grafana-dashboard.png
  → Update README Architecture section to include the screenshot:
    ![Dashboard under load](docs/grafana-dashboard.png)

□ Run chaos test 1 (Kafka death) if time permits:
  bash chaos/run_chaos.sh  (or just the first test manually)
  → Update CHAOS.md with actual test result

□ Final GitHub checks:
  → All 4 repos pinned to GitHub profile
  → repo has topics: distributed-systems, rust, go, kafka, clickhouse, 
    ai-infrastructure, observability, kubernetes, opentelemetry
  → README renders correctly on GitHub (check Mermaid diagram renders)
  → CI workflow passes (check the Actions tab)
  → Star count target: get 3 friends/colleagues to star it on Day 7

□ Final commit: "docs: benchmark results, Grafana screenshot, v0.1.0 release"
□ Create GitHub Release: v0.1.0 with a release note summarizing what's built
```

---

## Cursor Session Template

Use this at the start of every session:

```
Project: infra-ai-streaming
Production-grade AI inference observability pipeline.
Stack: Rust (Axum ingestion) + Go (Kafka→ClickHouse consumer) + Redpanda + Redis.
Author: Staff Engineer, 7.5 years. Built 1.5T events/day TSDB at Agoda.

Today: [paste the specific session description from above]

Code requirements — non-negotiable:
- No unwrap() anywhere in non-test code
- anyhow::Result (Rust) / error returns (Go) throughout  
- tracing::instrument on every function doing I/O (Rust)
- zap structured logging — no fmt.Println (Go)
- Prometheus metric names must match the spec exactly
- Full implementation — no TODOs, no stubs, no placeholders
- All channels/goroutines handle shutdown via context cancellation
```

---

## What You Have After Day 7

| Artifact | Status |
|---|---|
| README.md with Mermaid diagram + benchmark table | ✅ |
| DESIGN.md — CAP, partition, backpressure, failure modes | ✅ |
| CHAOS.md + OBSERVABILITY.md | ✅ |
| BENCHMARKS.md with real numbers | ✅ |
| Rust ingestion engine — compiles, handles real load | ✅ |
| Go consumer — ClickHouse writes, circuit breaker, anomaly detection | ✅ |
| Docker Compose — one command to run everything | ✅ |
| Grafana dashboard — 4 live panels with screenshot | ✅ |
| k6 load test — reproducible benchmark | ✅ |
| GitHub Actions CI — green badge | ✅ |
| GitHub Release v0.1.0 | ✅ |

This is what you link to from your resume, LinkedIn, and every recruiter conversation from Day 8.
