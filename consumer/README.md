# Go consumer (Day 4 skeleton)

Kafka consumer for `ai_inference_events`. Deserializes the Rust producer batch envelope `{"events":[...]}` and logs each `InferenceEvent` to stdout. **ClickHouse writes, circuit breaker, and Redis overflow ship Day 5.**

## Prerequisites

- Go **1.22+**
- Redpanda/Kafka reachable at `KAFKA_BROKERS` (default `127.0.0.1:9092` with Compose)
- Topics created by `deploy/redpanda/init-topics.sh` via `redpanda-init`

## Run

From the repository root:

```bash
set -a && source deploy/.env && set +a
export KAFKA_BROKERS=127.0.0.1:9092
go run ./consumer/cmd/consumer
```

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `KAFKA_BROKERS` | `127.0.0.1:9092` | Comma-separated broker list |
| `KAFKA_TOPIC` | `ai_inference_events` | Topic to consume |
| `KAFKA_GROUP_ID` | `ai-inference-consumer-dev` | Consumer group; **new** groups join at the log end. Change the group id (or reset offsets) to replay history |
| `LOG_LEVEL` | `info` | Reserved for structured logging (stdout uses `log` today) |

## Tests

```bash
go test ./consumer/...
```
