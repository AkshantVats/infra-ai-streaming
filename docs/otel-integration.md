# OpenTelemetry Collector Integration (OSS-03)

<!-- SPDX-License-Identifier: MIT -->

This document covers how to route OpenTelemetry traces from any instrumented
application into the LensAI inference observability pipeline.

## Why OTel alongside eBPF?

The `ebpf-llm-tracer` captures *every* TLS socket operation — it does not
require application changes. The OTel path complements it for two scenarios:

1. **Managed cloud inference** (AWS Bedrock, Azure OpenAI) where TLS terminates
   inside a cloud VPC that eBPF cannot reach from your cluster.
2. **SDK-first teams** that already instrument with OpenTelemetry and want to
   send spans directly, bypassing the eBPF tracer entirely.

Both paths write the same `inference_events` schema in ClickHouse, so queries
and dashboards work identically regardless of capture method.

## Schema mapping

The collector's `transform/lensai_schema` processor translates
[OTel Gen-AI semantic conventions][semconv] to LensAI column names:

| OTel attribute | LensAI column | Notes |
|---|---|---|
| `gen_ai.request.model` | `model_id` | Model the app requested |
| `gen_ai.response.model` | `resolved_model_id` | Model that actually ran (post-flagd evaluation) |
| `gen_ai.usage.input_tokens` | `input_tokens` | Prompt tokens |
| `gen_ai.usage.output_tokens` | `output_tokens` | Completion tokens |
| `gen_ai.system` | `provider` | `openai` / `anthropic` / `bedrock` |
| span duration | `latency_ms` | Derived from `end_time - start_time` |

`resolved_model_id` is the field that makes cost attribution accurate during
canary rollouts. If your SDK does not emit `gen_ai.response.model`, the
infra-ai-streaming ingest service falls back to `gen_ai.request.model`.

[semconv]: https://opentelemetry.io/docs/specs/semconv/gen-ai/

## Quickstart

### 1. Clone and configure

```bash
git clone https://github.com/AkshantVats/infra-ai-streaming
cd infra-ai-streaming

# Set the ingest endpoint (default targets local compose stack)
export LENSAI_INGEST_ENDPOINT=http://lensai-ingest:4318
```

### 2. Start the collector

```bash
# Standalone (OTel collector only)
docker compose -f deploy/otel-collector/docker-compose.yml up -d

# Or alongside the full LensAI stack (from repo root)
docker compose \
  -f docker-compose.yml \
  -f deploy/otel-collector/docker-compose.yml \
  up -d
```

### 3. Instrument your application

Add the OTel SDK and the Gen-AI instrumentation for your framework:

```python
# Python example — openai + opentelemetry-instrumentation-openai
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.instrumentation.openai import OpenAIInstrumentor

provider = TracerProvider()
provider.add_span_processor(
    BatchSpanProcessor(
        OTLPSpanExporter(endpoint="http://localhost:4317", insecure=True)
    )
)
trace.set_tracer_provider(provider)
OpenAIInstrumentor().instrument()

# All subsequent openai.chat.completions.create() calls are traced automatically.
```

For Go, Node.js, and Java see the [OTel instrumentation libraries][otel-libs].

[otel-libs]: https://opentelemetry.io/ecosystem/registry/?language=all&component=instrumentation

### 4. Verify a span reaches ClickHouse

```bash
# Make one inference call in your app, then:
scripts/smoke.sh --check-otel
# Exits 0 when at least one row with source='otlp' appears in inference_events.
```

## Collector health

| Endpoint | What it shows |
|---|---|
| `http://localhost:13133/` | Health check (200 = healthy) |
| `http://localhost:8888/metrics` | Prometheus metrics for spans received/sent/dropped |

Key metrics to watch:

- `otelcol_receiver_accepted_spans` — spans arriving from apps
- `otelcol_exporter_sent_spans` — spans forwarded to LensAI ingest
- `otelcol_exporter_queue_size` — WAL depth (rises if ingest is down)

## Durability

The collector uses a `file_storage` extension backed by a Docker volume
(`otel_queue`). If `lensai-ingest` is unreachable, spans queue on disk and
drain automatically when it recovers. No spans are silently dropped — the
collector retries with exponential back-off up to 5 minutes.

This mirrors the WAL-before-Kafka guarantee in the infra-ai-streaming ingest
service itself: two independent write-ahead durability layers before Redpanda.

## Configuration reference

See `deploy/otel-collector/config.yaml` for the full annotated configuration.
All environment variables:

| Variable | Default | Description |
|---|---|---|
| `LENSAI_INGEST_ENDPOINT` | `http://lensai-ingest:4318` | OTLP HTTP endpoint of infra-ai-streaming |

## Limitations

- The OTel path does not capture calls that skip SDK instrumentation. Use
  the eBPF tracer in parallel for ground-truth billing attribution.
- `resolved_model_id` is only populated if the SDK emits `gen_ai.response.model`.
  Many SDKs omit this field. The infra-ai-streaming ingest service falls back
  to `model_id` and logs a warning when it is missing.
