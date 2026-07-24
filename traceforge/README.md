# TraceForge — Agent Execution Trace Collector

Phase 2 of [Inferix](https://github.com/AkshantVats/infra-ai-streaming). LensAI observes inference calls. TraceForge traces how agents use them.

## What it solves

LensAI tells you that `gpt-4o` was called 1,200 times in the last hour with a P99 of 1.4s. It cannot tell you which agent step made that call, which tool was executing, or whether the step before it failed silently.

TraceForge collects **agent execution spans** — one span per tool call, one root span per agent run — and writes them into the same ClickHouse instance that backs LensAI.

## Schema

Every span carries: `trace_id`, `span_id`, `parent_span_id`, `tool_name`, `model`, `tokens`, `cost_usd`, `status`, `latency_ms`. See [DESIGN.md](./DESIGN.md) for the full schema and ClickHouse DDL.

## Quickstart

```bash
# Run schema tests
cd traceforge && go test ./pkg/schema/...
```

## Roadmap

| Day | Work |
|---|---|
| 30 (today) | DESIGN.md + Go span schema + OTel mapping |
| 31 | OTel Collector pipeline: OTLP receiver → ClickHouse exporter + Kafka producer |
| 32+ | Collector compose overlay on LensAI quickstart, waterfall UI |

## Part of the 150-day plan

See [akshant-150-day-plan](https://github.com/AkshantVats/akshant-150-day-plan) for context.
