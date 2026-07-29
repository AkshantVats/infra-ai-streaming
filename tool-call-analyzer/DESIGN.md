# tool-call-analyzer — Design Document

**Status**: Active · Day 37 scaffold  
**Owner**: AkshantVats  
**Kafka output topic**: `tools.normalized.v1`

---

## Problem

Every AI framework encodes a tool invocation differently:

| Vendor | Wire format | Key fields |
|--------|-------------|------------|
| OpenAI | `tool_calls[].function.{name,arguments}` | `id`, `type="function"`, `arguments` is a JSON string |
| Anthropic | `content[].{type="tool_use", name, input}` | `id`, `name`, `input` is a JSON object |
| LangChain | `AgentAction{tool, tool_input, log}` | no canonical ID; tool_input is free-form |
| LlamaIndex | `ToolOutput{tool_name, content, raw_output}` | no cost attribution |

Without normalization, every downstream consumer (ClickHouse, Grafana, cost alerting) must parse vendor-specific formats. Schema drift between OpenAI API versions has broken production pipelines in practice — `function_call` (pre-2023 singular) was replaced by `tool_calls` (array) with no deprecation window.

---

## Canonical ToolCall Struct

Defined in `pkg/types/tool_call.go`. All adapter implementations must populate every field marked **required**.

```go
type ToolCall struct {
    // Identity
    ID      string // required; vendor-provided or generated UUID
    TraceID string // required; W3C traceparent trace-id if available
    SpanID  string // required; W3C traceparent span-id if available

    // Tool identity
    Name     string       // required; e.g. "get_weather", "search_web"
    Vendor   string       // required; "openai" | "anthropic" | "langchain" | "llamaindex"
    Category ToolCategory // required; semantic type — see ToolCategory section

    // Invocation payload
    InputJSON  string // required; JSON-encoded arguments/input
    OutputJSON string // optional; JSON-encoded result (empty on error)

    // Timing
    StartedAtNs  int64 // required; Unix nanoseconds
    FinishedAtNs int64 // required; Unix nanoseconds
    DurationMs   int64 // computed: (FinishedAtNs - StartedAtNs) / 1e6

    // Cost
    Cost    CostEstimate // required; see CostEstimate
    Retries RetryMeta    // required; see RetryMeta

    // Status
    Status   string // required; "OK" | "ERROR" | "TIMEOUT" | "EMPTY_RESPONSE"
    ErrorMsg string // optional; populated on non-OK status
    HasError bool   // required; true when Status != "OK"

    // Source metadata
    ModelName    string // optional; LLM that produced this tool_use block
    AgentStep    int    // optional; step index in ReAct loop (0-indexed)
    FrameworkVer string // optional; e.g. "langchain-0.2.14"
}
```

---

## ToolCategory — Ontology Before Metrics

Without categories, `tool_call.latency` is a meaningless aggregate over HTTP searches (500ms), database queries (5ms), and code executions (8s). With categories, each type has its own latency budget and alert threshold.

```go
type ToolCategory string

const (
    CategoryHTTP  ToolCategory = "http"  // outbound HTTP: search, API, fetch
    CategoryDB    ToolCategory = "db"    // database: SQL, vector, Redis
    CategoryCode  ToolCategory = "code"  // code execution: Python, bash, sandbox
    CategoryFile  ToolCategory = "file"  // filesystem: read_file, S3, list_dir
    CategoryAgent ToolCategory = "agent" // sub-agent delegation: delegate, llm_chain
)
```

**Categorization rules** (evaluated in priority order):
1. Name contains `sql`, `query`, `db`, `vector`, `elastic`, `redis` → `CategoryDB`
2. Name contains `run`, `exec`, `python`, `bash`, `compile`, `code` → `CategoryCode`
3. Name contains `file`, `read`, `write`, `dir`, `s3`, `fs` → `CategoryFile`
4. Name contains `agent`, `delegate`, `llm`, `chain`, `subagent` → `CategoryAgent`
5. Default → `CategoryHTTP` (safe fallback; most tools are HTTP under the hood)

**Operational SLOs per category**:

| Category | P99 budget | Alert on |
|----------|-----------|----------|
| http | 5s | timeout, 4xx/5xx rate |
| db | 200ms | slow query, index miss |
| code | 30s | sandbox timeout, OOM |
| file | 2s | permission error, not-found |
| agent | 120s | depth limit, cumulative cost |

---

## CostEstimate and RetryMeta

```go
type CostEstimate struct {
    InputTokens  int     // prompt token count from usage block
    OutputTokens int     // completion token count from usage block
    ModelName    string  // e.g. "gpt-4o", "claude-sonnet-4-6"
    CostUSD      float64 // computed by EstimateCost()
}

type RetryMeta struct {
    RetryCount     int     // 0 = first attempt succeeded; N = N retries before success
    AttemptCostUSD float64 // cost of a single attempt
    TotalCostUSD   float64 // AttemptCostUSD * (RetryCount + 1)
    LastErrorMsg   string  // error message from last failed attempt
    RetryReason    string  // "timeout" | "rate_limit" | "server_error" | "empty_response"
}
```

**Cost computation** (from `EstimateCost` in `pkg/types/tool_call.go`):

```go
var perMillionTokens = map[string][2]float64{
    "gpt-4o":                    {2.50, 10.00},
    "gpt-4o-mini":               {0.15, 0.60},
    "gpt-4-turbo":               {10.00, 30.00},
    "claude-opus-4-8":           {15.00, 75.00},
    "claude-sonnet-4-6":         {3.00, 15.00},
    "claude-haiku-4-5-20251001": {0.80, 4.00},
}

func EstimateCost(inputTokens, outputTokens int, modelName string) float64 {
    prices, ok := perMillionTokens[modelName]
    if !ok {
        return 0.0 // unknown model — caller logs warning
    }
    return float64(inputTokens)/1_000_000*prices[0] + float64(outputTokens)/1_000_000*prices[1]
}
```

---

## Adapter Interface

Defined in `pkg/adapter/adapter.go`. Every vendor gets one implementation.

```go
type Adapter interface {
    Vendor() string                            // "openai", "anthropic", "langchain", etc.
    Parse(raw []byte) (types.ToolCall, error)  // normalize vendor JSON → ToolCall
    CanHandle(raw []byte) bool                 // auto-detection probe
}

var (
    ErrNilInput      = errors.New("tool-call-analyzer: nil input to adapter")
    ErrUnknownFormat = errors.New("tool-call-analyzer: unrecognized tool call format")
    ErrMissingField  = errors.New("tool-call-analyzer: required field missing in payload")
)
```

**OpenAI adapter contract** (Day 38):
- Input: `{"id":"call_abc","type":"function","function":{"name":"search_web","arguments":"{\"query\":\"golang\"}"}}`
- Note: `arguments` is a JSON-encoded **string**, not an object. Must be parsed twice.
- Backward compat: detect legacy `function_call` (pre-2023 singular) and normalize to same output.

**Anthropic adapter contract** (Day 38):
- Input: `{"type":"tool_use","id":"toolu_abc","name":"search_web","input":{"query":"golang"}}`
- Note: `input` is already a parsed object. No double-decode required.
- Requires `type == "tool_use"` to pass `CanHandle`.

---

## Kafka Output Schema

Topic: `tools.normalized.v1`  
Partition key: `trace_id` (trace-ordered consumption; enables waterfall reconstruction)  
Format: JSON (Avro schema planned for Day 40)

```json
{
  "schema_version": "1.0",
  "id": "tcall-abc123",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "name": "search_web",
  "vendor": "openai",
  "category": "http",
  "input_json": "{\"query\": \"current weather in Berlin\"}",
  "output_json": "{\"temperature\": 18, \"condition\": \"cloudy\"}",
  "started_at_ns": 1752652800000000000,
  "finished_at_ns": 1752652801234000000,
  "duration_ms": 1234,
  "cost": {
    "input_tokens": 512,
    "output_tokens": 64,
    "model_name": "gpt-4o",
    "cost_usd": 0.001920
  },
  "retries": {
    "retry_count": 1,
    "attempt_cost_usd": 0.001920,
    "total_cost_usd": 0.003840,
    "last_error_msg": "rate_limit_exceeded",
    "retry_reason": "rate_limit"
  },
  "status": "OK",
  "error_msg": "",
  "has_error": false,
  "model_name": "gpt-4o",
  "agent_step": 2,
  "framework_ver": "openai-1.50.2"
}
```

---

## Decision Log

| Decision | Chosen | Rejected | Reason |
|----------|--------|----------|--------|
| Output format | JSON | Avro | Avro requires schema registry setup; JSON unblocks Day 38 adapters without infra |
| Category field | typed string constant | free-form string | Free-form strings cause unbounded Grafana label cardinality |
| Cost model | hardcoded price table | external API lookup | External API adds latency and network dependency to the hot path |
| Retry attribution | `attemptCost × (retries+1)` | cumulative from usage | Usage blocks do not report per-retry costs; multiply from attempt cost |
| Partition key | `trace_id` | tool name | Trace ordering allows waterfall reconstruction in ClickHouse |
| Category default | `http` | `unknown` | Most tools are HTTP; `unknown` in a label creates an opaque dashboard bucket |
