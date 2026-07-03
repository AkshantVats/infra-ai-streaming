# TraceForge — Agent Execution Trace Collector

> Phase 2 of Inferix. LensAI observes inference calls. TraceForge traces *how agents use them*.

## Problem

LensAI tells you that model `gpt-4o` was called 1,200 times in the last hour and that P99 latency was 1.4s. It cannot tell you which agent step made that call, what tool was executing, whether it was a retry, or whether the step that came before it had already failed silently. That gap is what Step 7 at Delivery Hero taught me: the worst outages are the ones between services you already monitor.

TraceForge closes that gap by collecting agent execution spans — one span per tool call, one root span per agent run — and writing them into the same ClickHouse cluster that backs LensAI.

---

## Agent Execution Graph Model

An agent run is a **directed acyclic graph of spans**. Each node is a span; each edge is a parent→child relationship encoded in `parent_span_id`.

```
root span (agent_run)
├── span: tool_call → web_search
│     └── span: llm_call → gpt-4o (summarise results)
├── span: tool_call → code_interpreter
│     ├── span: llm_call → gpt-4o (generate code)
│     └── span: tool_call → file_write
└── span: llm_call → gpt-4o (final answer)
```

A ReAct loop maps to this graph as a **saga**: each tool call is a compensatable step with its own status and latency. Memory reads/writes are spans too (status=`memory_read` / `memory_write`). The graph is traversable in ClickHouse via recursive CTEs on `(trace_id, span_id, parent_span_id)`.

---

## Span Schema

Every span written to `agent_spans` has these columns:

| Column | Type | Description |
|---|---|---|
| `trace_id` | `String` | UUID v4. One per agent run. All spans in a run share this value. |
| `span_id` | `String` | UUID v4. Unique per span. |
| `parent_span_id` | `Nullable(String)` | NULL on the root span; `span_id` of the caller otherwise. |
| `tool_name` | `LowCardinality(String)` | Logical tool identifier (e.g. `web_search`, `code_interpreter`, `file_write`, `llm_call`). |
| `model` | `LowCardinality(String)` | Model identifier used in this span (e.g. `gpt-4o`, `claude-3-5-sonnet`). Empty string for non-LLM spans. |
| `tokens` | `UInt32` | Total tokens consumed (prompt + completion) for LLM spans. 0 for non-LLM spans. |
| `cost_usd` | `Float64` | Computed cost in USD. 0.0 for non-LLM spans. |
| `status` | `LowCardinality(String)` | One of: `ok`, `error`, `retry`, `timeout`, `cancelled`. |
| `latency_ms` | `UInt32` | Wall-clock duration from span start to span end in milliseconds. |
| `ts` | `DateTime64(3)` | Span start timestamp (millisecond precision, UTC). |
| `agent_id` | `LowCardinality(String)` | Agent type / class identifier (e.g. `research_agent`, `code_agent`). |
| `tenant_id` | `LowCardinality(String)` | Tenant or workspace identifier for multi-tenant deployments. |
| `error_message` | `Nullable(String)` | Error string when `status = error`. NULL otherwise. |
| `metadata` | `String` | JSON blob for span-specific attributes (prompt hash, tool input fingerprint, etc.). |

### ClickHouse DDL

```sql
CREATE TABLE agent_spans
(
    trace_id       String,
    span_id        String,
    parent_span_id Nullable(String),
    tool_name      LowCardinality(String),
    model          LowCardinality(String),
    tokens         UInt32,
    cost_usd       Float64,
    status         LowCardinality(String),
    latency_ms     UInt32,
    ts             DateTime64(3),
    agent_id       LowCardinality(String),
    tenant_id      LowCardinality(String),
    error_message  Nullable(String),
    metadata       String DEFAULT '{}'
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(ts)
ORDER BY (tenant_id, trace_id, span_id)
TTL ts + INTERVAL 90 DAY;
```

---

## Tool Taxonomy

Tools are grouped into four categories. The `tool_name` field uses snake_case values from this taxonomy.

| Category | Tool names |
|---|---|
| `retrieval` | `web_search`, `vector_search`, `document_read`, `db_query` |
| `execution` | `code_interpreter`, `bash_exec`, `api_call`, `http_request` |
| `memory` | `memory_read`, `memory_write`, `context_window_read` |
| `generation` | `llm_call`, `image_gen`, `embedding_gen` |

The `llm_call` tool is the only one that populates `model`, `tokens`, and `cost_usd`. All other tools leave those fields at their zero values.

---

## OTel Attribute Mapping

See `docs/otel-mapping.md` for the full semconv mapping. The four-to-one relationship between OTel span attributes and `agent_spans` columns is:

| OTel attribute | `agent_spans` column |
|---|---|
| `trace_id` | `trace_id` |
| `span_id` | `span_id` |
| `parent_span_id` | `parent_span_id` |
| `gen_ai.request.model` | `model` |
| `gen_ai.usage.input_tokens + output_tokens` | `tokens` |
| `gen_ai.usage.cost` | `cost_usd` |
| `otel.status_code` (OK/ERROR) + custom | `status` |
| span duration (end - start) | `latency_ms` |
| `gen_ai.system` → mapped to taxonomy | `tool_name` |

---

## Collector Architecture

```
agent SDK / OTel SDK
        │
        ▼
  OTLP gRPC receiver  (port 4317)
        │
        ▼
  batch processor (512 spans / 5s timeout)
        │
     ┌──┴──────────────────┐
     ▼                     ▼
ClickHouse exporter    Kafka producer
(agent_spans table)    (agent.spans.v1 topic)
```

The dual-sink design is intentional. ClickHouse is the query layer; Kafka is the replay and streaming-join layer. Day 31 implements the OTel Collector pipeline configuration.

---

## Connection to LensAI

LensAI writes to `inference_events` (one row per model API call). TraceForge writes to `agent_spans` (one row per tool call, with `llm_call` spans linking back to inference events via `trace_id`). The join:

```sql
SELECT
    s.trace_id,
    s.agent_id,
    s.latency_ms                  AS agent_step_ms,
    e.latency_ms                  AS model_latency_ms,
    s.cost_usd
FROM agent_spans s
JOIN inference_events e
  ON s.trace_id = e.trace_id
 WHERE s.tool_name = 'llm_call'
   AND s.ts > now() - INTERVAL 1 HOUR;
```

This is the query that would have found Step 7 in under 10 seconds.
