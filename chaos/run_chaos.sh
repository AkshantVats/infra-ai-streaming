#!/usr/bin/env bash
# chaos/run_chaos.sh — Automated chaos scenarios for infra-ai-streaming
#
# Usage:
#   ./chaos/run_chaos.sh kill-redpanda    # Scenario 1: broker crash + WAL replay
#   ./chaos/run_chaos.sh throttle-clickhouse  # Scenario 2: CH pause + circuit breaker
#   ./chaos/run_chaos.sh load-10k         # Scenario 3: sustained 10k events/sec
#   ./chaos/run_chaos.sh all              # Run all three sequentially
#
# Prerequisites:
#   - Docker Compose stack running (deploy/docker-compose.yml)
#   - Go consumer running:  cd consumer && go run ./cmd/consumer
#   - Rust ingestion running: cargo run -p ingestion
#   (kill-redpanda will restart ingestion itself for WAL replay)
#
# Environment overrides:
#   INGEST_URL         (default: http://localhost:8080)
#   METRICS_INGEST     (default: http://localhost:8080/metrics)
#   METRICS_CONSUMER   (default: http://localhost:9091/metrics)
#   LOAD_DURATION_SEC  (default: 30 — for load-10k)
#   REDPANDA_DOWN_SEC  (default: 30 — broker outage window)
#   CH_PAUSE_SEC       (default: 30 — ClickHouse pause window)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE=(docker compose --env-file deploy/.env -f deploy/docker-compose.yml)
INGEST_URL="${INGEST_URL:-http://localhost:8080}"
METRICS_INGEST="${METRICS_INGEST:-http://localhost:8080/metrics}"
METRICS_CONSUMER="${METRICS_CONSUMER:-http://localhost:9091/metrics}"
LOAD_DURATION_SEC="${LOAD_DURATION_SEC:-30}"
REDPANDA_DOWN_SEC="${REDPANDA_DOWN_SEC:-30}"
CH_PAUSE_SEC="${CH_PAUSE_SEC:-30}"
TENANT="chaos-test"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ─────────────────────────────────────────────────────────────────────────────
# Helper functions
# ─────────────────────────────────────────────────────────────────────────────

log()  { echo -e "${CYAN}[chaos]${NC} $*"; }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
separator() { echo -e "\n${BOLD}═══════════════════════════════════════════════════════════════${NC}"; }

ts_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

wait_for_healthy() {
  local service="$1"
  local timeout="${2:-60}"
  local elapsed=0
  log "Waiting for ${service} to become healthy (timeout ${timeout}s)..."
  while (( elapsed < timeout )); do
    local health
    health="$("${COMPOSE[@]}" ps --format json 2>/dev/null | python3 -c "
import json, sys
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    o = json.loads(line)
    svc = o.get('Service') or o.get('Name','')
    if '${service}' in svc:
        print(o.get('Health','unknown'))
        break
" 2>/dev/null || echo "unknown")"
    if [[ "$health" == "healthy" ]]; then
      pass "${service} healthy after ${elapsed}s"
      return 0
    fi
    sleep 3
    elapsed=$((elapsed + 3))
  done
  fail "${service} not healthy after ${timeout}s"
  return 1
}

count_events_in_ch() {
  local tenant="${1:-$TENANT}"
  "${COMPOSE[@]}" exec -T clickhouse clickhouse-client --query \
    "SELECT count() FROM infra_ai.inference_events WHERE tenant_id='${tenant}'" 2>/dev/null || echo "0"
}

count_dlq() {
  "${COMPOSE[@]}" exec -T redpanda rpk topic consume ai_inference_dlq \
    --offset start --num 999999 --format '%v\n' 2>/dev/null | wc -l | tr -d ' '
}

metric_val() {
  local url="$1"
  local pattern="$2"
  curl -sf "$url" 2>/dev/null | grep -E "^${pattern}" | head -1 | awk '{print $2}' || echo ""
}

require_service() {
  local name="$1" url="$2"
  if ! curl -sf "$url" >/dev/null 2>&1; then
    fail "${name} not reachable at ${url}"
    exit 1
  fi
}

make_event_payload() {
  local n="${1:-1}" tenant="${2:-$TENANT}"
  local events=""
  for i in $(seq 1 "$n"); do
    local ts; ts="$(ts_ms)"
    [[ -n "$events" ]] && events="${events},"
    events="${events}{\"tenant_id\":\"${tenant}\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts},\"latency_ms\":$((RANDOM % 500 + 50)),\"prompt_tokens\":$((RANDOM % 1000 + 10)),\"completion_tokens\":$((RANDOM % 500 + 5)),\"cost_usd\":0.00$((RANDOM % 999)),\"status\":\"success\"}"
  done
  echo "{\"events\":[${events}]}"
}

start_ingest_load() {
  local rps="$1" duration="$2" events_per_req="${3:-100}"
  local reqs_per_sec=$((rps / events_per_req))
  local total_sent=0

  log "Load generator: ${rps} events/s (${reqs_per_sec} reqs/s × ${events_per_req} events/req) for ${duration}s"

  local payload
  payload="$(make_event_payload "$events_per_req" "$TENANT")"

  for sec in $(seq 1 "$duration"); do
    for _ in $(seq 1 "$reqs_per_sec"); do
      curl -sS -o /dev/null -w '' -X POST "${INGEST_URL}/ingest" \
        -H 'Content-Type: application/json' \
        -H "X-Tenant-ID: ${TENANT}" \
        -d "$payload" &
    done
    wait
    total_sent=$((total_sent + rps))
    if (( sec % 5 == 0 )); then
      log "  ... ${sec}s elapsed, ~${total_sent} events sent"
    fi
  done
  wait
  echo "$total_sent"
}

# ─────────────────────────────────────────────────────────────────────────────
# Scenario 1: kill-redpanda — Broker crash mid-ingest + WAL replay
# ─────────────────────────────────────────────────────────────────────────────

scenario_kill_redpanda() {
  separator
  echo -e "${BOLD}SCENARIO 1: kill-redpanda — Broker crash + WAL replay zero-loss${NC}"
  separator

  require_service "ingestion" "${INGEST_URL}/health"

  # Phase 1: baseline — send events while everything is healthy
  log "Phase 1: Sending 500 baseline events to establish WAL + Kafka flow..."
  local payload_50
  payload_50="$(make_event_payload 50 "$TENANT")"
  for _ in $(seq 1 10); do
    curl -sS -o /dev/null -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' \
      -H "X-Tenant-ID: ${TENANT}" \
      -d "$payload_50" &
  done
  wait
  sleep 3

  local ch_before
  ch_before="$(count_events_in_ch)"
  log "ClickHouse rows before kill: ${ch_before}"

  # Phase 2: kill Redpanda
  log "Phase 2: Killing Redpanda container..."
  "${COMPOSE[@]}" kill redpanda 2>/dev/null || "${COMPOSE[@]}" stop redpanda
  sleep 2

  # Phase 3: send events while broker is down (WAL accepts, produce fails)
  log "Phase 3: Sending 200 events with broker DOWN (WAL should buffer)..."
  local wal_events=0
  for _ in $(seq 1 4); do
    local code
    code="$(curl -sS -o /dev/null -w "%{http_code}" -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' \
      -H "X-Tenant-ID: ${TENANT}" \
      -d "$payload_50")"
    if [[ "$code" == "202" ]]; then
      wal_events=$((wal_events + 50))
    else
      warn "Got HTTP ${code} during broker outage (expected 202 from WAL)"
    fi
    sleep 0.5
  done
  log "Events accepted by WAL during outage: ${wal_events}"

  local produce_errors_during
  produce_errors_during="$(metric_val "$METRICS_INGEST" 'kafka_produce_errors_total')"
  log "kafka_produce_errors_total during outage: ${produce_errors_during:-0}"

  # Phase 4: wait for the configured downtime, then restart Redpanda
  log "Phase 4: Broker down for ${REDPANDA_DOWN_SEC}s total..."
  sleep "$REDPANDA_DOWN_SEC"

  log "Restarting Redpanda..."
  "${COMPOSE[@]}" start redpanda
  wait_for_healthy "redpanda" 90

  # Phase 5: restart ingestion for WAL replay
  log "Phase 5: Restart ingestion to trigger WAL replay..."
  warn ">>> Please restart ingestion now: cargo run -p ingestion"
  warn ">>> Or kill the running process and re-launch."
  warn ">>> Waiting 15s for ingestion to come back up..."
  sleep 15

  if curl -sf "${INGEST_URL}/health" >/dev/null 2>&1; then
    local replay_count
    replay_count="$(metric_val "$METRICS_INGEST" 'wal_replay_events_total')"
    log "wal_replay_events_total: ${replay_count:-0}"
  else
    warn "Ingestion not back yet — check WAL replay manually after restart"
  fi

  # Phase 6: let consumer catch up, then verify counts
  log "Phase 6: Waiting 15s for consumer to flush to ClickHouse..."
  sleep 15

  local ch_after
  ch_after="$(count_events_in_ch)"
  local dlq_count
  dlq_count="$(count_dlq)"
  local total_landed=$((ch_after + dlq_count))
  local total_sent=$((500 + wal_events))

  separator
  echo -e "${BOLD}RESULTS — kill-redpanda${NC}"
  echo "  Events sent (baseline + outage):  ${total_sent}"
  echo "  ClickHouse rows after recovery:   ${ch_after}"
  echo "  DLQ messages:                     ${dlq_count}"
  echo "  Total landed (CH + DLQ):          ${total_landed}"
  echo "  kafka_produce_errors_total:       ${produce_errors_during:-0}"

  if (( total_landed >= total_sent )); then
    pass "Zero data loss confirmed (landed >= sent). At-least-once semantics: duplicates possible."
  else
    fail "Potential data loss: sent=${total_sent}, landed=${total_landed} (delta=$((total_sent - total_landed)))"
    warn "Check: is ingestion restarted? Did WAL replay complete? Consumer lag drained?"
  fi
  separator
}

# ─────────────────────────────────────────────────────────────────────────────
# Scenario 2: throttle-clickhouse — Pause CH, trigger circuit breaker + overflow
# ─────────────────────────────────────────────────────────────────────────────

scenario_throttle_clickhouse() {
  separator
  echo -e "${BOLD}SCENARIO 2: throttle-clickhouse — Circuit breaker + Redis overflow${NC}"
  separator

  require_service "ingestion" "${INGEST_URL}/health"
  require_service "consumer metrics" "$METRICS_CONSUMER"

  # Baseline metrics
  local cb_before overflow_before ch_before
  cb_before="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
  overflow_before="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
  ch_before="$(count_events_in_ch)"
  log "Baseline — breaker_open: ${cb_before:-0}, overflow: ${overflow_before:-0}, CH rows: ${ch_before}"

  # Phase 1: pause ClickHouse (simulates network hang, not clean stop)
  log "Phase 1: Pausing ClickHouse container (${CH_PAUSE_SEC}s)..."
  "${COMPOSE[@]}" pause clickhouse

  # Phase 2: send sustained traffic to trigger breaker + overflow
  log "Phase 2: Sending 1000 events to trigger circuit breaker..."
  local payload_50
  payload_50="$(make_event_payload 50 "$TENANT")"
  for _ in $(seq 1 20); do
    curl -sS -o /dev/null -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' \
      -H "X-Tenant-ID: ${TENANT}" \
      -d "$payload_50" &
  done
  wait

  log "Waiting ${CH_PAUSE_SEC}s for breaker to open + overflow to fill..."
  local elapsed=0
  while (( elapsed < CH_PAUSE_SEC )); do
    sleep 5
    elapsed=$((elapsed + 5))
    local cb_now overflow_now
    cb_now="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
    overflow_now="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
    log "  ${elapsed}s — breaker_open: ${cb_now:-?}, overflow: ${overflow_now:-?}"
  done

  local cb_during overflow_during
  cb_during="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
  overflow_during="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"

  # Phase 3: unpause ClickHouse — breaker should half-open and drain
  log "Phase 3: Unpausing ClickHouse..."
  "${COMPOSE[@]}" unpause clickhouse
  wait_for_healthy "clickhouse" 60

  log "Waiting 30s for breaker recovery + overflow drain..."
  sleep 30

  local cb_after overflow_after ch_after
  cb_after="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
  overflow_after="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
  ch_after="$(count_events_in_ch)"

  separator
  echo -e "${BOLD}RESULTS — throttle-clickhouse${NC}"
  echo "  ClickHouse rows before:           ${ch_before}"
  echo "  ClickHouse rows after:            ${ch_after}"
  echo "  Circuit breaker open (during):    ${cb_during:-?}"
  echo "  Circuit breaker open (after):     ${cb_after:-?}"
  echo "  Overflow depth (during pause):    ${overflow_during:-?}"
  echo "  Overflow depth (after drain):     ${overflow_after:-?}"

  if [[ "${cb_during:-0}" == "1" ]]; then
    pass "Circuit breaker opened during ClickHouse pause"
  else
    fail "Circuit breaker did not open (got: ${cb_during:-?})"
  fi

  if [[ "${cb_after:-1}" == "0" ]] || [[ -z "${cb_after}" ]]; then
    pass "Circuit breaker closed after recovery"
  else
    warn "Circuit breaker may still be open (${cb_after}) — check consumer lag"
  fi

  local overflow_during_int="${overflow_during%%.*}"
  local overflow_after_int="${overflow_after%%.*}"
  overflow_during_int="${overflow_during_int:-0}"
  overflow_after_int="${overflow_after_int:-0}"

  if (( overflow_during_int > 0 )); then
    pass "Redis overflow buffered events during pause (depth: ${overflow_during_int})"
  else
    warn "Overflow was 0 during pause — breaker may have opened before events were enqueued"
  fi

  if (( overflow_after_int < overflow_during_int )); then
    pass "Overflow drained on recovery (${overflow_during_int} → ${overflow_after_int})"
  else
    warn "Overflow did not decrease — drain may need more time"
  fi
  separator
}

# ─────────────────────────────────────────────────────────────────────────────
# Scenario 3: load-10k — Sustained 10k events/sec throughput
# ─────────────────────────────────────────────────────────────────────────────

scenario_load_10k() {
  separator
  echo -e "${BOLD}SCENARIO 3: load-10k — Sustained 10,000 events/sec${NC}"
  separator

  require_service "ingestion" "${INGEST_URL}/health"
  require_service "consumer metrics" "$METRICS_CONSUMER"

  log "Tip: set BATCH_SIZE=5000 on the consumer for 10k/s throughput"
  log "     (default is 1000; restart consumer with BATCH_SIZE=5000)"

  local ch_before
  ch_before="$(count_events_in_ch)"
  local start_time; start_time="$(date +%s)"

  # Run the load generator (100 events/request × 100 req/s = 10k events/s)
  log "Starting ${LOAD_DURATION_SEC}s sustained load..."
  local total_sent
  total_sent="$(start_ingest_load 10000 "$LOAD_DURATION_SEC" 100)"

  local end_time; end_time="$(date +%s)"
  local wall_sec=$((end_time - start_time))

  # Let consumer catch up
  log "Load complete. Waiting 30s for consumer to flush remaining batches..."
  sleep 30

  local ch_after
  ch_after="$(count_events_in_ch)"
  local ch_new=$((ch_after - ch_before))
  local actual_rate=0
  if (( wall_sec > 0 )); then
    actual_rate=$((total_sent / wall_sec))
  fi

  # Capture key metrics
  local p99_flush consumer_lag overflow
  p99_flush="$(metric_val "$METRICS_CONSUMER" 'clickhouse_flush_duration_seconds\{quantile="0.99"\}')"
  consumer_lag="$(metric_val "$METRICS_CONSUMER" 'kafka_consumer_lag_events')"
  overflow="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"

  separator
  echo -e "${BOLD}RESULTS — load-10k${NC}"
  echo "  Target rate:                      10,000 events/sec"
  echo "  Duration:                         ${LOAD_DURATION_SEC}s"
  echo "  Total events sent:                ${total_sent}"
  echo "  Wall-clock time:                  ${wall_sec}s"
  echo "  Actual send rate:                 ${actual_rate} events/sec"
  echo "  ClickHouse rows before:           ${ch_before}"
  echo "  ClickHouse rows after:            ${ch_after}"
  echo "  New CH rows:                      ${ch_new}"
  echo "  CH flush p99:                     ${p99_flush:-N/A}s"
  echo "  Kafka consumer lag:               ${consumer_lag:-N/A}"
  echo "  Redis overflow depth:             ${overflow:-0}"

  if (( ch_new > 0 )); then
    local throughput_pct=$((ch_new * 100 / total_sent))
    echo "  Delivery rate:                    ${throughput_pct}%"
    if (( throughput_pct >= 95 )); then
      pass "≥95% delivery rate under 10k/s load"
    elif (( throughput_pct >= 80 )); then
      warn "80-95% delivery — consumer may need BATCH_SIZE=5000 or more partitions"
    else
      fail "<80% delivery — check consumer backpressure, CH write capacity"
    fi
  else
    warn "No new CH rows — is the consumer running? Check consumer lag."
  fi
  separator
}

# ─────────────────────────────────────────────────────────────────────────────
# Orchestration
# ─────────────────────────────────────────────────────────────────────────────

usage() {
  cat <<'EOF'
Usage: ./chaos/run_chaos.sh <scenario>

Scenarios:
  kill-redpanda       Kill Redpanda mid-ingest, restart, verify zero-loss via WAL replay
  throttle-clickhouse Pause ClickHouse to trigger circuit breaker + Redis overflow, verify drain
  load-10k            Sustained 10k events/sec — capture throughput + latency metrics
  all                 Run all three scenarios sequentially

Environment:
  INGEST_URL          (default: http://localhost:8080)
  METRICS_CONSUMER    (default: http://localhost:9091/metrics)
  LOAD_DURATION_SEC   (default: 30)
  REDPANDA_DOWN_SEC   (default: 30)
  CH_PAUSE_SEC        (default: 30)
EOF
}

case "${1:-help}" in
  kill-redpanda)       scenario_kill_redpanda ;;
  throttle-clickhouse) scenario_throttle_clickhouse ;;
  load-10k)            scenario_load_10k ;;
  all)
    scenario_kill_redpanda
    echo ""
    scenario_throttle_clickhouse
    echo ""
    scenario_load_10k
    ;;
  help|-h|--help) usage ;;
  *)
    echo "Unknown scenario: $1" >&2
    usage
    exit 1
    ;;
esac
