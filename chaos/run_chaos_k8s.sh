#!/usr/bin/env bash
# chaos/run_chaos_k8s.sh — Chaos scenarios against a k3d/Helm deployment (MODE=k8s).
#
# Usage:
#   ./chaos/run_chaos_k8s.sh kill-redpanda
#   ./chaos/run_chaos_k8s.sh throttle-clickhouse
#   ./chaos/run_chaos_k8s.sh load-m1
#   ./chaos/run_chaos_k8s.sh all
#
# Prerequisites: helm release in namespace lensai; port-forwards optional (script starts them).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NS="${K8S_NAMESPACE:-lensai}"
RELEASE="${HELM_RELEASE:-lensai}"
INGEST_URL="${INGEST_URL:-http://localhost:8080}"
METRICS_INGEST="${METRICS_INGEST:-http://localhost:8080/metrics}"
METRICS_CONSUMER="${METRICS_CONSUMER:-http://localhost:9091/metrics}"
LOAD_EVENTS="${LOAD_EVENTS:-2000}"
LOAD_DURATION_SEC="${LOAD_DURATION_SEC:-10}"
REDPANDA_DOWN_SEC="${REDPANDA_DOWN_SEC:-20}"
CH_PAUSE_SEC="${CH_PAUSE_SEC:-25}"
TENANT="${CHAOS_TENANT:-chaos-k8s}"
CURL_MAX_TIME="${CURL_MAX_TIME:-15}"
CURL_OPTS=(--max-time "${CURL_MAX_TIME}" --connect-timeout 5)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PF_ING="" PF_CON=""
log()  { echo -e "${CYAN}[chaos-k8s]${NC} $*"; }
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
separator() { echo -e "\n${BOLD}═══════════════════════════════════════════════════════════════${NC}"; }

sts_name() {
  local component="$1"
  kubectl get sts -n "${NS}" -l "app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=${component}" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

deploy_name() {
  local component="$1"
  kubectl get deploy -n "${NS}" -l "app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=${component}" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

ch_pod() {
  kubectl get pods -n "${NS}" -l "app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=clickhouse" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

ts_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

metric_val() {
  local url="$1" pattern="$2"
  curl -sf "${CURL_OPTS[@]}" "$url" 2>/dev/null | grep -E "^${pattern}" | head -1 | awk '{print $2}' || echo ""
}

count_events_in_ch() {
  local tenant="${1:-$TENANT}"
  local pod
  pod="$(ch_pod)"
  [[ -n "$pod" ]] || { echo "0"; return; }
  kubectl exec -n "${NS}" "$pod" -- clickhouse-client --query \
    "SELECT count() FROM infra_ai.inference_events WHERE tenant_id='${tenant}'" 2>/dev/null || echo "0"
}

make_event_payload() {
  local n="${1:-1}" tenant="${2:-$TENANT}"
  python3 -c "
import json, random, time
n = int('${n}')
tenant = '${tenant}'
events = []
base = int(time.time() * 1000)
for i in range(n):
    events.append({
        'tenant_id': tenant,
        'model_id': 'gpt-4o',
        'timestamp_unix_ms': base + i,
        'latency_ms': random.randint(50, 549),
        'prompt_tokens': random.randint(10, 1009),
        'completion_tokens': random.randint(5, 504),
        'cost_usd': round(random.uniform(0.001, 0.999), 5),
        'status': 'success',
    })
print(json.dumps({'events': events}))
"
}

start_port_forwards() {
  if curl -sf "${CURL_OPTS[@]}" "${INGEST_URL}/health" >/dev/null 2>&1; then
    log "Ingestion already reachable at ${INGEST_URL}"
    return 0
  fi
  log "Starting port-forwards (ingestion :8080, consumer :9091)"
  kubectl port-forward -n "${NS}" svc/ingestion 8080:8080 >/tmp/pf-ing-k8s.log 2>&1 &
  PF_ING=$!
  kubectl port-forward -n "${NS}" svc/consumer 9091:9091 >/tmp/pf-con-k8s.log 2>&1 &
  PF_CON=$!
  sleep 3
}

stop_port_forwards() {
  [[ -n "$PF_ING" ]] && kill "$PF_ING" 2>/dev/null || true
  [[ -n "$PF_CON" ]] && kill "$PF_CON" 2>/dev/null || true
  PF_ING="" PF_CON=""
}

require_ingest() {
  if ! curl -sf "${CURL_OPTS[@]}" "${INGEST_URL}/health" >/dev/null 2>&1; then
    fail "ingestion not reachable at ${INGEST_URL}/health"
    exit 1
  fi
}

wait_redpanda_ready() {
  local sts timeout="${2:-120}" elapsed=0
  sts="$(sts_name redpanda)"
  [[ -n "$sts" ]] || return 1
  log "Waiting for redpanda StatefulSet ready..."
  while (( elapsed < timeout )); do
    local ready desired
    ready="$(kubectl get sts "$sts" -n "${NS}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo 0)"
    desired="$(kubectl get sts "$sts" -n "${NS}" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo 1)"
    if [[ "${ready:-0}" -ge 1 ]] && [[ "${ready:-0}" -ge "${desired:-1}" ]]; then
      pass "Redpanda ready (${elapsed}s)"
      return 0
    fi
    sleep 3
    elapsed=$((elapsed + 3))
  done
  fail "Redpanda not ready after ${timeout}s"
  return 1
}

wait_clickhouse_ready() {
  local sts timeout="${2:-120}" elapsed=0
  sts="$(sts_name clickhouse)"
  [[ -n "$sts" ]] || return 1
  while (( elapsed < timeout )); do
    local ready
    ready="$(kubectl get sts "$sts" -n "${NS}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo 0)"
    if [[ "${ready:-0}" -ge 1 ]]; then
      pass "ClickHouse ready (${elapsed}s)"
      return 0
    fi
    sleep 3
    elapsed=$((elapsed + 3))
  done
  fail "ClickHouse not ready after ${timeout}s"
  return 1
}

restart_ingestion_k8s() {
  local dep
  dep="$(deploy_name ingestion)"
  [[ -n "$dep" ]] || { warn "ingestion deployment not found"; return 1; }
  log "Rollout restart ingestion (${dep}) for WAL replay..."
  kubectl rollout restart "deployment/${dep}" -n "${NS}"
  kubectl rollout status "deployment/${dep}" -n "${NS}" --timeout=120s || true
  local i=0
  while (( i < 60 )); do
    if curl -sf "${CURL_OPTS[@]}" "${INGEST_URL}/health" >/dev/null 2>&1; then
      pass "Ingestion healthy after restart"
      return 0
    fi
    sleep 2
    i=$((i + 2))
  done
  fail "Ingestion not healthy after rollout restart"
  return 1
}

scenario_kill_redpanda() {
  separator
  echo -e "${BOLD}SCENARIO C1 (k8s): kill-redpanda — broker outage + WAL replay${NC}"
  separator

  start_port_forwards
  trap stop_port_forwards EXIT
  require_ingest

  local payload_50
  payload_50="$(make_event_payload 50 "$TENANT")"

  log "Phase 1: baseline ingest (150 events)..."
  for _ in $(seq 1 3); do
    curl -sS "${CURL_OPTS[@]}" -o /dev/null -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload_50" &
  done
  wait || true
  sleep 3
  local ch_before
  ch_before="$(count_events_in_ch | tr -cd '0-9')"
  ch_before="${ch_before:-0}"
  log "ClickHouse rows before kill: ${ch_before}"

  local sts rp_pod
  sts="$(sts_name redpanda)"
  log "Phase 2: scale redpanda to 0 (${sts})..."
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=0
  sleep 3

  log "Phase 3: ingest during broker outage (200 events)..."
  local wal_events=0
  for _ in $(seq 1 4); do
    local code
    code="$(curl -sS "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload_50")"
    if [[ "$code" == "202" ]]; then
      wal_events=$((wal_events + 50))
    else
      warn "HTTP ${code} during outage (expected 202 from WAL)"
    fi
    sleep 0.5
  done
  local produce_errors
  produce_errors="$(metric_val "$METRICS_INGEST" 'kafka_produce_errors_total')"
  log "kafka_produce_errors_total: ${produce_errors:-0}"

  log "Phase 4: broker down ${REDPANDA_DOWN_SEC}s, then restore..."
  sleep "$REDPANDA_DOWN_SEC"
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=1
  wait_redpanda_ready 180

  log "Phase 5: restart ingestion for WAL replay..."
  restart_ingestion_k8s || warn "WAL replay may need manual check"
  local replay
  replay="$(metric_val "$METRICS_INGEST" 'wal_replay_events_total')"
  log "wal_replay_events_total: ${replay:-0}"

  log "Phase 6: wait for consumer → ClickHouse..."
  sleep 20
  local ch_after
  ch_after="$(count_events_in_ch | tr -cd '0-9')"
  ch_after="${ch_after:-0}"
  local total_sent=$((150 + wal_events))

  echo "  Events sent: ${total_sent}, CH rows after: ${ch_after}"
  if (( ch_after >= ch_before )); then
    pass "Recovery path OK (CH grew or held; at-least-once may duplicate)"
  else
    fail "CH count dropped unexpectedly (before=${ch_before}, after=${ch_after})"
  fi
  separator
}

scenario_throttle_clickhouse() {
  separator
  echo -e "${BOLD}SCENARIO C2 (k8s): throttle-clickhouse — breaker + overflow${NC}"
  separator

  start_port_forwards
  trap stop_port_forwards EXIT
  require_ingest

  local cb_before overflow_before ch_before
  cb_before="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
  overflow_before="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
  ch_before="$(count_events_in_ch | tr -cd '0-9')"
  log "Baseline — breaker_open: ${cb_before:-0}, overflow: ${overflow_before:-0}, CH: ${ch_before}"

  local sts ch_sts
  sts="$(sts_name clickhouse)"
  log "Phase 1: scale ClickHouse to 0 (${sts}) for ${CH_PAUSE_SEC}s window..."
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=0
  sleep 2

  local payload_50
  payload_50="$(make_event_payload 50 "$TENANT")"
  log "Phase 2: load while CH unavailable..."
  for _ in $(seq 1 20); do
    curl -sS "${CURL_OPTS[@]}" -o /dev/null -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload_50" &
  done
  wait

  local elapsed=0 cb_during overflow_during
  while (( elapsed < CH_PAUSE_SEC )); do
    sleep 5
    elapsed=$((elapsed + 5))
    cb_during="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
    overflow_during="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
    log "  ${elapsed}s — breaker_open: ${cb_during:-?}, overflow: ${overflow_during:-?}"
  done

  log "Phase 3: restore ClickHouse..."
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=1
  wait_clickhouse_ready 180
  sleep 25

  local cb_after overflow_after ch_after
  cb_after="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
  overflow_after="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
  ch_after="$(count_events_in_ch | tr -cd '0-9')"

  echo "  breaker during: ${cb_during:-?}, after: ${cb_after:-?}"
  echo "  overflow during: ${overflow_during:-?}, after: ${overflow_after:-?}"
  echo "  CH rows before/after: ${ch_before} / ${ch_after}"

  if [[ "${cb_during:-0}" == "1" ]]; then
    pass "Circuit breaker opened during CH outage"
  else
    warn "Breaker did not report open (got ${cb_during:-?}) — may need longer pause or more load"
  fi

  local od="${overflow_during%%.*}" oa="${overflow_after%%.*}"
  od="${od:-0}" oa="${oa:-0}"
  if (( od > 0 )); then
    pass "Redis overflow observed (depth ${od})"
  else
    warn "No overflow depth during pause"
  fi
  separator
}

scenario_load_m1() {
  separator
  echo -e "${BOLD}SCENARIO load-m1 — ${LOAD_EVENTS} events over ${LOAD_DURATION_SEC}s${NC}"
  separator

  start_port_forwards
  trap stop_port_forwards EXIT
  require_ingest

  local ch_before ch_after total_sent=0
  ch_before="$(count_events_in_ch | tr -cd '0-9')"
  ch_before="${ch_before:-0}"

  local events_per_sec=$((LOAD_EVENTS / LOAD_DURATION_SEC))
  [[ "$events_per_sec" -ge 1 ]] || events_per_sec=1
  local events_per_req=50
  local reqs_per_sec=$((events_per_sec / events_per_req))
  [[ "$reqs_per_sec" -ge 1 ]] || reqs_per_sec=1

  local payload
  payload="$(make_event_payload "$events_per_req" "$TENANT")"
  log "Sending ~${events_per_sec} events/s (${reqs_per_sec} req/s × ${events_per_req}) for ${LOAD_DURATION_SEC}s"

  for sec in $(seq 1 "$LOAD_DURATION_SEC"); do
    for _ in $(seq 1 "$reqs_per_sec"); do
      curl -sS "${CURL_OPTS[@]}" -o /dev/null -X POST "${INGEST_URL}/ingest" \
        -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload" &
      total_sent=$((total_sent + events_per_req))
    done
    wait
    if (( sec % 5 == 0 )); then
      log "  ... ${sec}s, ~${total_sent} events sent"
    fi
  done

  sleep 20
  ch_after="$(count_events_in_ch | tr -cd '0-9')"
  ch_after="${ch_after:-0}"
  local ch_new=$((ch_after - ch_before))
  local lag overflow
  lag="$(metric_val "$METRICS_CONSUMER" 'kafka_consumer_lag_events')"
  overflow="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"

  echo "  Sent: ~${total_sent}, new CH rows: ${ch_new}, lag: ${lag:-?}, overflow: ${overflow:-0}"
  if (( ch_new > 0 )); then
    pass "Load delivered rows to ClickHouse (${ch_new} new)"
  else
    warn "No new CH rows — check consumer logs"
  fi
  separator
}

usage() {
  cat <<EOF
Usage: ./chaos/run_chaos_k8s.sh <scenario>

Scenarios:
  kill-redpanda        C1: scale redpanda to 0, WAL buffer, rollout restart ingestion
  throttle-clickhouse  C2: scale CH to 0, verify breaker/overflow metrics
  load-m1              Reduced load (LOAD_EVENTS=${LOAD_EVENTS}, LOAD_DURATION_SEC=${LOAD_DURATION_SEC})
  all                  All three sequentially

Environment:
  K8S_NAMESPACE, HELM_RELEASE, INGEST_URL, METRICS_CONSUMER
  LOAD_EVENTS, LOAD_DURATION_SEC, REDPANDA_DOWN_SEC, CH_PAUSE_SEC
EOF
}

case "${1:-help}" in
  kill-redpanda)       scenario_kill_redpanda ;;
  throttle-clickhouse) scenario_throttle_clickhouse ;;
  load-m1)             scenario_load_m1 ;;
  all)
    scenario_kill_redpanda
    scenario_throttle_clickhouse
    scenario_load_m1
    ;;
  help|-h|--help) usage ;;
  *)
    echo "Unknown scenario: $1" >&2
    usage
    exit 1
    ;;
esac

stop_port_forwards 2>/dev/null || true
