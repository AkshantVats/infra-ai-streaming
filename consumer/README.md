# Go consumer

Kafka consumer for `ai_inference_events`. Deserializes the Rust producer batch envelope `{"events":[...]}` and writes to **ClickHouse** via a batched writer with circuit breaker, Redis overflow, and DLQ.

## Run (local)

```bash
cp ../deploy/.env.example ../deploy/.env   # if needed
set -a && source ../deploy/.env && set +a
export KAFKA_BROKERS=127.0.0.1:9092
export CLICKHOUSE_DSN=clickhouse://127.0.0.1:9000/infra_ai
export REDIS_URL=redis://127.0.0.1:6379
go run ./cmd/consumer
```

Metrics: http://localhost:9091/metrics

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `KAFKA_BROKERS` | `127.0.0.1:9092` | Broker list |
| `KAFKA_TOPIC` | `ai_inference_events` | Consume topic |
| `KAFKA_GROUP_ID` | `ai-inference-consumer-dev` | Consumer group |
| `KAFKA_DLQ_TOPIC` | `ai_inference_dlq` | DLQ topic |
| `CLICKHOUSE_DSN` | `clickhouse://127.0.0.1:9000/infra_ai` | Native protocol DSN |
| `REDIS_URL` | `redis://127.0.0.1:6379` | Overflow buffer |
| `REDIS_OVERFLOW_KEY` | `ai_inference:overflow` | LIST key |
| `METRICS_PORT` | `9091` | Prometheus scrape port |
| `BATCH_SIZE` | `1000` | Max events per flush |
| `FLUSH_INTERVAL` | `500ms` | Max time between flushes |
| `CB_FAILURE_THRESHOLD` | `5` | Failures before breaker opens |
| `CB_RESET_TIMEOUT` | `30s` | Open → half-open delay |
| `CLICKHOUSE_INSERT_RETRIES` | `3` | Retries before DLQ |

## Tests

```bash
go test ./...
```
