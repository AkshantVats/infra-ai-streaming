# OTel Semantic Convention Mapping — TraceForge

TraceForge ingests OpenTelemetry spans via the OTLP gRPC receiver and maps standard OTel/GenAI semconv attributes to `agent_spans` columns.

## GenAI Semantic Conventions (draft)

The [OpenTelemetry GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) define the following attributes for LLM operations:

| OTel Attribute | Type | TraceForge Column | Notes |
|---|---|---|---|
| `gen_ai.system` | string | `tool_name` | Mapped: `openai` → `llm_call`, `anthropic` → `llm_call` |
| `gen_ai.request.model` | string | `model` | Direct: `gpt-4o`, `claude-3-5-sonnet-20241022` |
| `gen_ai.usage.input_tokens` | int | — | Summed with output_tokens into `tokens` |
| `gen_ai.usage.output_tokens` | int | — | Summed with input_tokens into `tokens` |
| `gen_ai.usage.input_tokens` + `gen_ai.usage.output_tokens` | — | `tokens` | Computed sum |
| *(custom)* `gen_ai.usage.cost_usd` | float | `cost_usd` | Optional; if absent, computed from model rate card |
| `otel.status_code` | enum OK/ERROR | `status` | ERROR→`error`; OK→`ok`; see retry logic below |
| span duration | milliseconds | `latency_ms` | `(end_time - start_time).as_millis()` |

## Non-LLM Tool Spans

For tool calls that are not LLM calls (e.g. `web_search`, `code_interpreter`):

| OTel Attribute | TraceForge Column | Notes |
|---|---|---|
| `span.name` | `tool_name` | Must match ToolCategory taxonomy |
| `otel.status_code` | `status` | Same mapping as LLM spans |
| span duration | `latency_ms` | Same |
| *(all GenAI attrs absent)* | `model=""`, `tokens=0`, `cost_usd=0.0` | Zero values |

## Status Mapping

OTel `status_code` has only two values (OK, ERROR). The TraceForge `status` field has five. The mapping uses span events and attributes:

| Condition | TraceForge status |
|---|---|
| `otel.status_code == OK` | `ok` |
| `otel.status_code == ERROR` + `exception.type` contains `Timeout` | `timeout` |
| `otel.status_code == ERROR` + span event `retry_attempt` present | `retry` |
| `otel.status_code == ERROR` + span event `cancelled` present | `cancelled` |
| `otel.status_code == ERROR` (all other cases) | `error` |

## Trace Context Propagation

TraceForge uses W3C `traceparent` / `tracestate` headers for context propagation. The `trace_id` in `agent_spans` is the same 32-hex-char trace ID from W3C format, stored as a string (no binary encoding).

This means a LensAI `inference_events` row and its corresponding TraceForge `agent_spans` rows can be joined on `trace_id` directly — no transformation needed.

## Example: ReAct Loop

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01

agent_spans rows:
  trace_id=4bf92f3577b34da6a3ce929d0e0e4736
  ├── span_id=root_001  parent=NULL       tool=agent_run    status=ok  latency=4210ms
  ├── span_id=s001      parent=root_001   tool=web_search   status=ok  latency=820ms
  ├── span_id=s002      parent=root_001   tool=llm_call     status=ok  latency=1100ms  model=gpt-4o  tokens=980
  ├── span_id=s003      parent=root_001   tool=code_interp  status=error latency=340ms
  └── span_id=s004      parent=root_001   tool=llm_call     status=ok  latency=940ms   model=gpt-4o  tokens=620
```

The `s003` span shows Step 7 failing — with a span, the diagnosis takes seconds.
