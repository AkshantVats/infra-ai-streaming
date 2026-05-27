#!/usr/bin/env bash
# Runnable demo scenarios for local E2E — pair with docs/END-TO-END-FLOWS.md and Grafana
# uid ai-inference-e2e-local (http://localhost:3000/d/ai-inference-e2e-local).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE=(docker compose --env-file deploy/.env -f deploy/docker-compose.yml)
ENV_FILE="deploy/.env"
INGEST_URL="${INGEST_URL:-http://localhost:8080}"
METRICS_CONSUMER="${METRICS_CONSUMER:-http://localhost:9091/metrics}"
METRICS_INGEST="${METRICS_INGEST:-http://localhost:8080/metrics}"
TENANT="${DEMO_TENANT:-demo}"

usage() {
  cat <<'EOF'
Usage: ./scripts/demo-flows.sh <command>

Prerequisites (typical):
  Terminal 1: docker compose up (Redis, Redpanda, ClickHouse, Prometheus, Grafana)
  Terminal 2: cd consumer && set -a && source ../deploy/.env && set +a && go run ./cmd/consumer
  Terminal 3: set -a && source deploy/.env && set +a && cargo run -p ingestion
  Grafana:  http://localhost:3000/d/ai-inference-e2e-local  (admin/admin)

Commands:
  stack-up          Start compose stack (copies deploy/.env if missing)
  stack-down        docker compose down
  happy-path        POST /ingest; expect CH rows + consumer metrics
  circuit-breaker   Stop ClickHouse; ingest; expect breaker open + overflow
  overflow-depth    Same as circuit-breaker; print redis_overflow_depth
  dlq-path          Stop CH; burst ingest; expect dlq_events_total increase
  invalid-json      Produce garbage to ai_inference_events (offset stuck)
  rate-limit        Hammer /ingest to trigger 429 (lower RATE_LIMIT_DEFAULT_RPS first)
  per-tenant-limit  Demo per-tenant limits (tenant-demo=5rps vs tenant-b=1000rps)
  fail-open         Stop Redis; show fail-open; restart Redis; limits resume
  burst             Send N parallel ingest requests (default N=50)
  validation-error  POST invalid batch (empty events) → 400 + validation metric
  timeout-event     POST event with status=timeout (still 202; CH stores status)
  recovery-hint     Print WAL metrics + restart instructions for replay path
  metrics-snapshot  Curl Prometheus-oriented metrics from both services
  help              This message

Env:
  DEMO_TENANT, INGEST_URL, BURST_N (default 50), RATE_LIMIT_HAMMER (default 200)
EOF
}

require_ingest() {
  if ! curl -sf "${INGEST_URL}/health" >/dev/null; then
    echo "ERROR: ingestion not reachable at ${INGEST_URL} (cargo run -p ingestion)" >&2
    exit 1
  fi
}

require_consumer_metrics() {
  if ! curl -sf "$METRICS_CONSUMER" | grep -q kafka_records_processed_total; then
    echo "ERROR: consumer metrics not on :9091 (go run ./cmd/consumer)" >&2
    exit 1
  fi
}

ingest_payload() {
  local tenant="${1:-$TENANT}"
  local model="${2:-gpt-4o}"
  local ts_ms
  ts_ms="$(python3 -c 'import time; print(int(time.time()*1000))')"
  cat <<EOF
{"events":[{"tenant_id":"${tenant}","model_id":"${model}","timestamp_unix_ms":${ts_ms},"latency_ms":342,"prompt_tokens":512,"completion_tokens":128,"cost_usd":0.00423,"status":"success"}]}
EOF
}

post_ingest() {
  local tenant="${1:-$TENANT}"
  curl -sS -o /dev/null -w "%{http_code}" -X POST "${INGEST_URL}/ingest" \
    -H 'Content-Type: application/json' \
    -H "X-Tenant-ID: ${tenant}" \
    -d "$(ingest_payload "$tenant")"
}

metric_val() {
  local pattern="$1"
  curl -sf "$METRICS_CONSUMER" | grep -E "^${pattern}" | head -1 | awk '{print $2}'
}

cmd_stack_up() {
  if [[ ! -f "$ENV_FILE" ]]; then
    cp deploy/.env.example "$ENV_FILE"
  fi
  "${COMPOSE[@]}" up -d
  echo "Grafana: http://localhost:3000/d/ai-inference-e2e-local"
}

cmd_stack_down() {
  "${COMPOSE[@]}" down
}

cmd_happy_path() {
  require_ingest
  require_consumer_metrics
  code="$(post_ingest)"
  echo "POST /ingest → HTTP ${code} (expect 202)"
  sleep 3
  echo "--- consumer metrics ---"
  curl -sf "$METRICS_CONSUMER" | grep -E 'kafka_records_processed_total|clickhouse_batch_size|redis_overflow_depth' || true
  echo "--- ClickHouse (demo rows) ---"
  "${COMPOSE[@]}" exec -T clickhouse clickhouse-client --query \
    "SELECT count() AS n, max(cost_usd) FROM infra_ai.inference_events WHERE tenant_id='${TENANT}'" 2>/dev/null || \
    echo "(ClickHouse not reachable via compose)"
  echo ""
  echo "Grafana: Ingest request rate ↑ | Kafka handoff rate ↑ | ClickHouse table rows ↑"
}

cmd_circuit_breaker() {
  require_ingest
  require_consumer_metrics
  echo "Stopping ClickHouse container..."
  "${COMPOSE[@]}" stop clickhouse
  sleep 2
  for _ in $(seq 1 8); do
    post_ingest >/dev/null || true
    sleep 0.3
  done
  sleep 2
  open_val="$(metric_val 'circuit_breaker_state\{state="open"\}')"
  overflow="$(metric_val 'redis_overflow_depth')"
  echo "circuit_breaker_state{open} = ${open_val:-?} (expect 1)"
  echo "redis_overflow_depth = ${overflow:-?} (expect > 0 after sustained ingest)"
  echo ""
  echo "Grafana: Circuit breaker OPEN = red | Overflow depth / DLQ stat rises"
  echo "Restore: ${COMPOSE[*]} start clickhouse — overflow drains when breaker closes"
}

cmd_overflow_depth() {
  cmd_circuit_breaker
}

cmd_dlq_path() {
  require_ingest
  require_consumer_metrics
  dlq_before="$(metric_val 'dlq_events_total' || echo 0)"
  echo "dlq_events_total before = ${dlq_before}"
  echo "Stopping ClickHouse (insert failures before breaker saturates overflow)..."
  "${COMPOSE[@]}" stop clickhouse
  sleep 1
  for _ in $(seq 1 3); do
    post_ingest >/dev/null || true
    sleep 1
  done
  sleep 3
  dlq_after="$(metric_val 'dlq_events_total' || echo 0)"
  echo "dlq_events_total after = ${dlq_after}"
  echo ""
  echo "Grafana: increase(dlq_events_total[1h]) on Overflow depth / DLQ panel"
  echo "Note: first failures may land in overflow once breaker opens; DLQ appears after 3 insert retries per batch while breaker allows inserts."
  "${COMPOSE[@]}" start clickhouse 2>/dev/null || true
}

cmd_invalid_json() {
  require_consumer_metrics
  echo "Producing invalid JSON to ai_inference_events..."
  "${COMPOSE[@]}" exec -T redpanda rpk topic produce ai_inference_events -k "${TENANT}" <<< 'not-valid-json'
  sleep 2
  echo "kafka_records_processed_total should NOT increase for that offset (record_failed, no commit)."
  curl -sf "$METRICS_CONSUMER" | grep kafka_records_processed_total || true
  echo ""
  echo "Grafana: Kafka handoff flat; check consumer logs: record_failed"
}

cmd_rate_limit() {
  require_ingest
  local hammer="${RATE_LIMIT_HAMMER:-200}"
  local ok=0 denied=0
  echo "Sending ${hammer} rapid POSTs (expect some HTTP 429 if RATE_LIMIT_DEFAULT_RPS is low)..."
  for _ in $(seq 1 "$hammer"); do
    code="$(post_ingest "rate-limit-test")"
    if [[ "$code" == "202" ]]; then ok=$((ok + 1)); elif [[ "$code" == "429" ]]; then denied=$((denied + 1)); fi
  done
  echo "202=${ok} 429=${denied}"
  echo ""
  echo "Grafana: Errors & rejections → rate_limited_requests_total"
  echo "Tip: restart ingestion with RATE_LIMIT_DEFAULT_RPS=10 for easier 429s"
}

cmd_burst() {
  require_ingest
  require_consumer_metrics
  local n="${BURST_N:-50}"
  echo "Burst: ${n} parallel ingests..."
  for _ in $(seq 1 "$n"); do post_ingest & done
  wait
  sleep 3
  curl -sf "$METRICS_CONSUMER" | grep -E 'kafka_records_processed_total|clickhouse_flush_duration' || true
  echo ""
  echo "Grafana: Kafka handoff rate spike | flush latency p99 may wiggle"
}

cmd_validation_error() {
  require_ingest
  code="$(curl -sS -o /dev/null -w "%{http_code}" -X POST "${INGEST_URL}/ingest" \
    -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" \
    -d '{"events":[]}')"
  echo "POST empty batch → HTTP ${code} (expect 400)"
  curl -sf "$METRICS_INGEST" 2>/dev/null | grep ingestion_validation_errors_total || true
  echo ""
  echo "Grafana (e2e): Errors & rejections — add query rate(ingestion_validation_errors_total[5m]) if needed"
}

cmd_timeout_event() {
  require_ingest
  require_consumer_metrics
  ts_ms="$(python3 -c 'import time; print(int(time.time()*1000))')"
  curl -sS -X POST "${INGEST_URL}/ingest" \
    -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" \
    -d "{\"events\":[{\"tenant_id\":\"${TENANT}\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts_ms},\"latency_ms\":9000,\"prompt_tokens\":1,\"completion_tokens\":1,\"cost_usd\":0.001,\"status\":\"timeout\"}]}"
  sleep 3
  echo "--- ClickHouse status column ---"
  "${COMPOSE[@]}" exec -T clickhouse clickhouse-client --query \
    "SELECT status, latency_ms FROM infra_ai.inference_events WHERE tenant_id='${TENANT}' ORDER BY timestamp DESC LIMIT 3" 2>/dev/null || true
  echo ""
  echo "Grafana (product): P99 by model includes timeout latency_ms after flush"
}

cmd_recovery_hint() {
  echo "WAL recovery path (ingestion restart replays unacked WAL → Kafka):"
  curl -sf "$METRICS_INGEST" 2>/dev/null | grep -E 'wal_segments_pending|wal_replay_events_total' || echo "(ingestion not up)"
  echo ""
  echo "Steps: stop ingestion (Ctrl+C), optionally: docker compose stop redpanda (simulate broker outage),"
  echo "  ingest while down (WAL grows), start redpanda + restart ingestion — watch wal_replay_events_total spike."
}

cmd_metrics_snapshot() {
  echo "=== ingestion ==="
  curl -sf "$METRICS_INGEST" 2>/dev/null | grep -E 'ingestion_latency|wal_|rate_limited|backpressure|validation|kafka_produce' | head -25 || echo "(ingestion not up)"
  echo "=== consumer ==="
  curl -sf "$METRICS_CONSUMER" 2>/dev/null | grep -E 'kafka_records|kafka_deserialization|kafka_record_handoff|clickhouse_|circuit_breaker|redis_overflow|dlq_|kafka_consumer_lag' || echo "(consumer not up)"
}

cmd_per_tenant_limit() {
  require_ingest
  echo "Per-tenant rate limit demo (requires TENANT_LIMITS_PATH=deploy/tenant-limits.example.json)"
  echo "tenant-demo is capped at 5 rps (burst=10 tokens). Sending 20 rapid POSTs..."
  local ok=0 denied=0
  for _ in $(seq 1 20); do
    code="$(post_ingest "tenant-demo")"
    if [[ "$code" == "202" ]]; then ok=$((ok + 1)); elif [[ "$code" == "429" ]]; then denied=$((denied + 1)); fi
  done
  echo "tenant-demo: 202=${ok} 429=${denied}"
  echo ""
  echo "Now sending 20 as tenant-b (default or high limit)..."
  ok=0; denied=0
  for _ in $(seq 1 20); do
    code="$(post_ingest "tenant-b")"
    if [[ "$code" == "202" ]]; then ok=$((ok + 1)); elif [[ "$code" == "429" ]]; then denied=$((denied + 1)); fi
  done
  echo "tenant-b:    202=${ok} 429=${denied}"
  echo ""
  echo "Grafana: Errors & rejections → rate_limited_requests_total by tenant_id"
  echo "Punch line: tenant-demo is rate-limited while tenant-b is unbothered."
}

cmd_fail_open() {
  require_ingest
  echo "=== Scene 1: Rate limits enforced (Redis up) ==="
  local ok=0 denied=0
  for _ in $(seq 1 15); do
    code="$(post_ingest "tenant-demo")"
    if [[ "$code" == "202" ]]; then ok=$((ok + 1)); elif [[ "$code" == "429" ]]; then denied=$((denied + 1)); fi
  done
  echo "tenant-demo: 202=${ok} 429=${denied} (expect some 429s)"
  echo ""
  echo "=== Scene 2: Stopping Redis — fail-open ==="
  "${COMPOSE[@]}" stop redis
  sleep 1
  ok=0; denied=0
  for _ in $(seq 1 15); do
    code="$(post_ingest "tenant-demo")"
    if [[ "$code" == "202" ]]; then ok=$((ok + 1)); elif [[ "$code" == "429" ]]; then denied=$((denied + 1)); fi
  done
  echo "tenant-demo: 202=${ok} 429=${denied} (expect all 202 — fail-open)"
  echo "Check ingestion logs: 'redis unavailable; rate limit fail-open'"
  echo ""
  echo "=== Scene 3: Starting Redis — limits resume ==="
  "${COMPOSE[@]}" start redis
  sleep 2
  ok=0; denied=0
  for _ in $(seq 1 15); do
    code="$(post_ingest "tenant-demo")"
    if [[ "$code" == "202" ]]; then ok=$((ok + 1)); elif [[ "$code" == "429" ]]; then denied=$((denied + 1)); fi
  done
  echo "tenant-demo: 202=${ok} 429=${denied} (expect 429s again)"
  echo ""
  echo "Grafana: redis_rate_limit_degraded_total spiked during Scene 2"
  echo "Punch line: Redis up → limits enforced. Redis down → fail-open. Redis back → limits resume."
}

case "${1:-help}" in
  stack-up) cmd_stack_up ;;
  stack-down) cmd_stack_down ;;
  happy-path) cmd_happy_path ;;
  circuit-breaker) cmd_circuit_breaker ;;
  overflow-depth) cmd_overflow_depth ;;
  dlq-path) cmd_dlq_path ;;
  invalid-json) cmd_invalid_json ;;
  rate-limit) cmd_rate_limit ;;
  per-tenant-limit) cmd_per_tenant_limit ;;
  fail-open) cmd_fail_open ;;
  burst) cmd_burst ;;
  validation-error) cmd_validation_error ;;
  timeout-event) cmd_timeout_event ;;
  recovery-hint) cmd_recovery_hint ;;
  metrics-snapshot) cmd_metrics_snapshot ;;
  help|-h|--help) usage ;;
  *)
    echo "Unknown command: ${1:-}" >&2
    usage
    exit 1
    ;;
esac
