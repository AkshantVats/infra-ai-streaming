# CHAOS.md — Failure Scenario Runbook

**infra-ai-streaming** failure modes, expected behavior, recovery, and local reproduction steps.

Each scenario mirrors [DESIGN.md §5](DESIGN.md#5-failure-modes-and-recovery). Use this document to **verify** the pipeline's behavior under degradation — not to discover it after an incident.

Related: [OBSERVABILITY.md](OBSERVABILITY.md) (metric catalog), [docs/END-TO-END-FLOWS.md](docs/END-TO-END-FLOWS.md) (runnable scenarios), `./scripts/demo-flows.sh`.

---

## Prerequisites

```bash
# Terminal 1 — infrastructure
cd infra-ai-streaming
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d

# Terminal 2 — Go consumer
cd consumer && set -a && source ../deploy/.env && set +a
go run ./cmd/consumer

# Terminal 3 — Rust ingestion (with per-tenant limits for demo)
set -a && source deploy/.env && set +a
export TENANT_LIMITS_PATH=deploy/tenant-limits.example.json
cargo run -p ingestion
```

Grafana: http://localhost:3000/d/ai-inference-e2e-local (admin / admin).

---

## Scenario 1 — Kafka broker dies mid-ingest

| Field | Detail |
|-------|--------|
| **Trigger** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml stop redpanda` |
| **Expected behavior** | Kafka producer errors in ingestion logs. Events that were already WAL-fsynced stay durable. New `POST /ingest` calls may still return 202 (WAL accepts), but the drain task logs produce failures. `kafka_produce_errors_total` rises. No silent drop of fsynced WAL entries. |
| **Recovery** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml start redpanda` — wait for broker readiness (~5s). Restart ingestion binary: `cargo run -p ingestion`. WAL replay resubmits unacked entries (`wal_replay_events_total` spikes). |
| **Data loss** | **None** for entries that reached WAL fsync before the outage. In-flight entries in the mpsc channel that were never produced are replayed from WAL on restart. |
| **Fairness note** | All tenants are affected equally — Kafka is shared infrastructure. Per-tenant rate limits remain enforced (Redis is independent). |

### Metrics to watch

- `wal_segments_pending` — grows while Kafka is down
- `kafka_produce_errors_total` — increments on each failed produce
- `wal_replay_events_total` — spikes on restart

### Demo

```bash
# After happy-path traffic is flowing:
docker compose --env-file deploy/.env -f deploy/docker-compose.yml stop redpanda
# POST a few events (they WAL-accept, but produce fails):
curl -sS -X POST http://localhost:8080/ingest \
  -H 'Content-Type: application/json' -H 'X-Tenant-ID: demo' \
  -d '{"events":[{"tenant_id":"demo","model_id":"gpt-4o","timestamp_unix_ms":'$(python3 -c 'import time;print(int(time.time()*1000))')' ,"latency_ms":100,"prompt_tokens":10,"completion_tokens":5,"cost_usd":0.01,"status":"success"}]}'
# Check ingestion logs for produce errors, then restore:
docker compose --env-file deploy/.env -f deploy/docker-compose.yml start redpanda
# Restart ingestion — watch wal_replay_events_total
```

---

## Scenario 2 — ClickHouse write timeout

| Field | Detail |
|-------|--------|
| **Trigger** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml stop clickhouse` |
| **Expected behavior** | Consumer batch inserts fail. After 5 consecutive failures, the circuit breaker opens (`circuit_breaker_state{state="open"} == 1`). Subsequent batches are pushed to the Redis overflow LIST (`redis_overflow_depth` rises). Kafka offsets still commit after overflow push — no replay storm. Consumer lag grows. |
| **Recovery** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml start clickhouse`. After 30s the breaker enters half-open; one successful insert closes it. The drain loop (every 5s, up to 5000 events) flushes overflow back to ClickHouse. |
| **Data loss** | **None** while Kafka retains offsets and Redis overflow capacity holds. If overflow exceeds Redis memory, events go to DLQ (`dlq_events_total`). |
| **Fairness note** | All tenants share the overflow buffer. A noisy tenant's overflow volume may delay other tenants' drain. |

### Metrics to watch

- `circuit_breaker_state{state="open"}` — 1 when breaker is open
- `redis_overflow_depth` — LIST length
- `clickhouse_write_errors_total` — insert failures
- `kafka_consumer_lag_events` — lag grows during outage

### Demo

```bash
./scripts/demo-flows.sh circuit-breaker
# Or manually:
docker compose --env-file deploy/.env -f deploy/docker-compose.yml stop clickhouse
# Send events; watch breaker open + overflow rise in Grafana
# Restore:
docker compose --env-file deploy/.env -f deploy/docker-compose.yml start clickhouse
# Watch breaker close + overflow drain
```

---

## Scenario 3 — Redis lost (rate limit path)

| Field | Detail |
|-------|--------|
| **Trigger** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml stop redis` |
| **Expected behavior** | **Fail-open**: all `POST /ingest` requests are accepted regardless of rate limits. Ingestion logs emit `redis unavailable; rate limit fail-open`. The metric `redis_rate_limit_degraded_total` increments on each bypassed check. `rate_limited_requests_total` goes silent (no denials possible). Per-tenant fairness is **lost** — a noisy tenant can consume unbounded capacity. |
| **Recovery** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml start redis`. Next `POST /ingest` reconnects automatically. Token buckets refill from zero (buckets had TTL 60s, so old keys expired). Rate limiting resumes immediately — no restart required. |
| **Data loss** | **None** — events are never dropped because Redis is down. The tradeoff is **availability over fairness**. |
| **Fairness note** | This is a **product decision**, not a bug. Fail-closed would block all tenants when Redis is unavailable, trading availability for strict quota enforcement. DESIGN.md §5 row 3 documents this tradeoff. If you had fail-closed, a Redis blip would become a DoS for every tenant. |

### Metrics to watch

- `redis_rate_limit_degraded_total` — increments on each fail-open check
- `rate_limited_requests_total` — goes flat (no denials)
- Ingestion logs: `redis unavailable; rate limit fail-open`

### Demo (the four-scene story)

```bash
# Scene 1 — happy path with per-tenant limits active
export TENANT_LIMITS_PATH=deploy/tenant-limits.example.json
# tenant-demo is capped at 5 rps in the example config
for i in $(seq 1 20); do
  curl -sS -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' -H 'X-Tenant-ID: tenant-demo' \
    -d '{"events":[{"tenant_id":"tenant-demo","model_id":"gpt-4o","timestamp_unix_ms":'$(python3 -c 'import time;print(int(time.time()*1000))')' ,"latency_ms":100,"prompt_tokens":10,"completion_tokens":5,"cost_usd":0.01,"status":"success"}]}'
done
# Expect: first ~10 are 202 (burst=5*2=10), rest are 429

# Scene 2 — stop Redis
docker compose --env-file deploy/.env -f deploy/docker-compose.yml stop redis

# Scene 3 — all requests accepted (fail-open)
for i in $(seq 1 20); do
  curl -sS -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' -H 'X-Tenant-ID: tenant-demo' \
    -d '{"events":[{"tenant_id":"tenant-demo","model_id":"gpt-4o","timestamp_unix_ms":'$(python3 -c 'import time;print(int(time.time()*1000))')' ,"latency_ms":100,"prompt_tokens":10,"completion_tokens":5,"cost_usd":0.01,"status":"success"}]}'
done
# Expect: all 202

# Scene 4 — Redis returns, limits resume
docker compose --env-file deploy/.env -f deploy/docker-compose.yml start redis
sleep 2
for i in $(seq 1 20); do
  curl -sS -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' -H 'X-Tenant-ID: tenant-demo' \
    -d '{"events":[{"tenant_id":"tenant-demo","model_id":"gpt-4o","timestamp_unix_ms":'$(python3 -c 'import time;print(int(time.time()*1000))')' ,"latency_ms":100,"prompt_tokens":10,"completion_tokens":5,"cost_usd":0.01,"status":"success"}]}'
done
# Expect: 429s return after burst exhausted
```

---

## Scenario 4 — Ingestion OOM / pod kill

| Field | Detail |
|-------|--------|
| **Trigger** | `kill -9 <ingestion_pid>` (or pod eviction in k8s) |
| **Expected behavior** | Process terminates immediately. In-flight events in the mpsc channel that were produced to Kafka but not yet acked in WAL may be replayed as duplicates on restart. Events that were WAL-fsynced but not yet produced are replayed faithfully. The consumer is unaffected — it keeps draining Kafka. |
| **Recovery** | Restart ingestion: `cargo run -p ingestion`. WAL `replay_unacked()` fires at startup. `wal_replay_events_total` shows how many entries were resubmitted. |
| **Data loss** | **None** for WAL-fsynced entries. **At-least-once** semantics: duplicates are possible for entries that were produced but not acked before kill. Idempotent consumers or ReplacingMergeTree handle dedup at the warehouse layer. |
| **Fairness note** | Pod kill affects all tenants equally. Rate limit state in Redis is unaffected (TTL-based keys persist). |

### Metrics to watch

- `wal_replay_events_total` — spikes on restart
- `wal_segments_pending` — elevated before restart, drains after

### Demo

```bash
# Find the ingestion process PID
PID=$(pgrep -f 'target.*ingestion' | head -1)
# Kill it hard
kill -9 $PID
# Restart
set -a && source deploy/.env && set +a
cargo run -p ingestion
# Watch wal_replay_events_total in Grafana or:
curl -sf http://localhost:8080/metrics | grep wal_replay
```

---

## Scenario 5 — Network partition (consumer <-> ClickHouse)

| Field | Detail |
|-------|--------|
| **Trigger** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml pause clickhouse` (pauses without stopping — simulates network hang) |
| **Expected behavior** | Consumer inserts time out (connection hangs, no TCP RST). After timeout, `clickhouse_write_errors_total` increments. After 5 failures, breaker opens. Overflow buffering begins. Kafka offsets are **not** committed forward for records that haven't been handed off. Consumer lag grows. Unlike a clean stop (scenario 2), a pause may cause longer timeouts before the breaker opens. |
| **Recovery** | `docker compose --env-file deploy/.env -f deploy/docker-compose.yml unpause clickhouse`. Breaker half-open retry succeeds; overflow drains. If the partition lasted long enough for the consumer to build significant lag, catch-up may take minutes. |
| **Data loss** | **None** from committed offsets. Uncommitted records replay from Kafka (duplicates possible). |
| **Fairness note** | Same as scenario 2 — overflow is shared across tenants. |

### Metrics to watch

- `circuit_breaker_state{state="open"}` — 1 after timeout cascade
- `kafka_consumer_lag_events` — grows during partition
- `clickhouse_flush_duration_seconds` — p99 spikes to timeout values
- `redis_overflow_depth` — grows while breaker is open

### Demo

```bash
# Pause (not stop) ClickHouse to simulate network hang
docker compose --env-file deploy/.env -f deploy/docker-compose.yml pause clickhouse
# Send traffic; watch consumer logs for timeout + breaker open
for i in $(seq 1 10); do
  curl -sS -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' -H 'X-Tenant-ID: demo' \
    -d '{"events":[{"tenant_id":"demo","model_id":"gpt-4o","timestamp_unix_ms":'$(python3 -c 'import time;print(int(time.time()*1000))')' ,"latency_ms":100,"prompt_tokens":10,"completion_tokens":5,"cost_usd":0.01,"status":"success"}]}'
  sleep 1
done
# Unpause to recover
docker compose --env-file deploy/.env -f deploy/docker-compose.yml unpause clickhouse
# Watch breaker close + overflow drain
```

---

## Summary matrix

| # | Scenario | Ingest HTTP | Rate limits | Consumer | Data loss | Metric signal |
|---|----------|------------|-------------|----------|-----------|---------------|
| 1 | Kafka down | 202 (WAL), produce fails | Enforced (Redis independent) | Stalled (no records) | None (WAL replay) | `kafka_produce_errors_total`, `wal_segments_pending` |
| 2 | CH timeout | 202 | Enforced | Breaker open, overflow | None (overflow + DLQ) | `circuit_breaker_state`, `redis_overflow_depth` |
| 3 | Redis lost | 202 (fail-open, no limits) | **Bypassed** | Normal | None (availability wins) | `redis_rate_limit_degraded_total` |
| 4 | Ingest OOM | Down | Enforced (Redis persists) | Normal | None (WAL replay, at-least-once) | `wal_replay_events_total` |
| 5 | Network partition | 202 | Enforced | Breaker open, lag grows | None (offsets replay) | `kafka_consumer_lag_events`, `circuit_breaker_state` |

---

## Design philosophy

**Fail-open is a product decision, not a bug.** When Redis is down (scenario 3), the pipeline accepts all traffic at the cost of fairness. The alternative — fail-closed — would reject all tenants during a Redis outage, turning an infrastructure blip into a customer-visible DoS. We choose availability and document the tradeoff.

**At-least-once, not exactly-once.** Scenarios 1, 4, and 5 can produce duplicate events. Deduplication is a warehouse-layer concern (`event_id`, ReplacingMergeTree) — not enforced at the streaming boundary. This keeps the hot path simple and bounded.

**Every failure has a metric.** No scenario is silent. Each failure mode emits at least one Prometheus counter or gauge that Grafana can alert on. If you can't see a failure in a dashboard, that's a bug in observability — file it.
