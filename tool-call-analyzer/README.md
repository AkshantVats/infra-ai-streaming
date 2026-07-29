# tool-call-analyzer

Normalizes AI vendor tool call formats (OpenAI `tool_calls`, Anthropic `tool_use`, LangChain `AgentAction`) into a canonical `ToolCall` struct and emits to Kafka topic `tools.normalized.v1`.

Part of the [TraceForge](https://github.com/AkshantVats/infra-ai-streaming) observability stack.

## Quickstart

```bash
go test ./...
```

Expected output:
```
ok  github.com/AkshantVats/tool-call-analyzer/pkg/adapter  0.002s
ok  github.com/AkshantVats/tool-call-analyzer/pkg/types   0.003s
```

## Architecture

See [DESIGN.md](DESIGN.md) for the canonical `ToolCall` struct, `Adapter` interface contract, cost model, Kafka output schema, and decision log.

## Adapter roadmap

| Vendor | File | Status |
|--------|------|--------|
| OpenAI `tool_calls` | `pkg/adapter/openai.go` | Day 38 |
| Anthropic `tool_use` | `pkg/adapter/anthropic.go` | Day 38 |
| LangChain `AgentAction` | `pkg/adapter/langchain.go` | Day 38 |
| LlamaIndex `ToolOutput` | `pkg/adapter/llamaindex.go` | Day 40 |

## ToolCategory constants

| Constant | Value | Examples |
|----------|-------|----------|
| `CategoryHTTP` | `http` | search_web, get_weather, call_api |
| `CategoryDB` | `db` | sql_query, vector_search, redis_get |
| `CategoryCode` | `code` | run_python, bash_exec, code_interpreter |
| `CategoryFile` | `file` | read_file, write_file, fetch_s3_object |
| `CategoryAgent` | `agent` | call_subagent, delegate_task, run_llm_chain |
