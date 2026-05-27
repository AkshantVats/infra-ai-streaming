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
# M1/k3d: ClickHouse can take >3 min to become Ready after scale 0→1.
CH_READY_TIMEOUT_SEC="${CH_READY_TIMEOUT_SEC:-300}"
CH_SCALE_DOWN_WAIT_SEC="${CH_SCALE_DOWN_WAIT_SEC:-90}"
REDPANDA_READY_TIMEOUT_SEC="${REDPANDA_READY_TIMEOUT_SEC:-300}"
REDPANDA_SCALE_DOWN_WAIT_SEC="${REDPANDA_SCALE_DOWN_WAIT_SEC:-90}"
INGEST_ROLLOUT_TIMEOUT_SEC="${INGEST_ROLLOUT_TIMEOUT_SEC:-300}"
C2_BURST_PARALLEL="${C2_BURST_PARALLEL:-30}"
C2_BURST_ROUNDS="${C2_BURST_ROUNDS:-30}"
C2_FLUSH_PAUSE_SEC="${C2_FLUSH_PAUSE_SEC:-3}"
CH_RECOVERY_WAIT_SEC="${CH_RECOVERY_WAIT_SEC:-120}"
WAL_DRAIN_WAIT_SEC="${WAL_DRAIN_WAIT_SEC:-180}"
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

wait_ingest_ready() {
  local timeout="${1:-30}"
  local i=0
  while (( i < timeout )); do
    if curl -sf "${CURL_OPTS[@]}" "${INGEST_URL}/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  return 1
}

start_port_forwards() {
  if wait_ingest_ready 3; then
    log "Ingestion already reachable at ${INGEST_URL}"
    return 0
  fi
  log "Starting port-forwards (ingestion :8080, consumer :9091)"
  kubectl port-forward -n "${NS}" svc/ingestion 8080:8080 >/tmp/pf-ing-k8s.log 2>&1 &
  PF_ING=$!
  kubectl port-forward -n "${NS}" svc/consumer 9091:9091 >/tmp/pf-con-k8s.log 2>&1 &
  PF_CON=$!
  sleep 2
  disown_port_forwards
  if ! wait_ingest_ready 45; then
    fail "port-forward failed; see /tmp/pf-ing-k8s.log and /tmp/pf-con-k8s.log"
    return 1
  fi
}

disown_port_forwards() {
  # Bare `wait` would block on port-forward jobs; disown them so only curl PIDs are waited on.
  [[ -n "$PF_ING" ]] && disown "$PF_ING" 2>/dev/null || true
  [[ -n "$PF_CON" ]] && disown "$PF_CON" 2>/dev/null || true
}

stop_port_forwards() {
  [[ -n "$PF_ING" ]] && kill "$PF_ING" 2>/dev/null || true
  [[ -n "$PF_CON" ]] && kill "$PF_CON" 2>/dev/null || true
  PF_ING="" PF_CON=""
}

refresh_port_forwards() {
  stop_port_forwards
  start_port_forwards
}

ingestion_logs() {
  kubectl logs -n "${NS}" -l "app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=ingestion" --tail=60 2>&1 || true
}

consumer_logs() {
  kubectl logs -n "${NS}" -l "app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=consumer" --tail=60 2>&1 || true
}

metric_max() {
  local url="$1" pattern="$2"
  curl -sf "${CURL_OPTS[@]}" "$url" 2>/dev/null | awk -v p="^${pattern}" '$1 ~ p { if ($2+0 > m) m=$2+0 } END { print (m==""?0:m) }'
}

post_ingest_batch() {
  local n="${1:-50}" tenant="${2:-$TENANT}"
  local payload code
  payload="$(make_event_payload "$n" "$tenant")"
  code="$(curl -sS "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" -X POST "${INGEST_URL}/ingest" \
    -H 'Content-Type: application/json' -H "X-Tenant-ID: ${tenant}" -d "$payload")"
  [[ "$code" == "202" ]]
}

flood_ingest_burst() {
  local payload="$1"
  local parallel="${2:-${C2_BURST_PARALLEL}}"
  local -a pids=()
  local i
  for (( i = 0; i < parallel; i++ )); do
    curl -sS "${CURL_OPTS[@]}" -o /dev/null -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload" &
    pids+=($!)
  done
  wait_pids "${pids[@]}"
}

wait_ch_tenant_growth() {
  local baseline="$1" min_new="$2" timeout="${3:-${CH_RECOVERY_WAIT_SEC}}"
  local i=0 target=$((baseline + min_new))
  while (( i < timeout )); do
    local now
    now="$(count_events_in_ch "$TENANT" | tr -cd '0-9')"
    now="${now:-0}"
    if (( now >= target )); then
      echo "$now"
      return 0
    fi
    sleep 3
    i=$((i + 3))
  done
  count_events_in_ch "$TENANT" | tr -cd '0-9'
  return 1
}

wait_consumer_rollout() {
  local dep timeout="${INGEST_ROLLOUT_TIMEOUT_SEC}"
  dep="$(deploy_name consumer)"
  [[ -n "$dep" ]] || return 0
  log "Rollout restart consumer (${dep}) to reconnect after broker restore..."
  kubectl rollout restart "deployment/${dep}" -n "${NS}"
  kubectl rollout status "deployment/${dep}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1 || true
  kubectl wait --for=condition=available "deployment/${dep}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1 || true
}

wait_ingestion_rollout() {
  local dep timeout="${INGEST_ROLLOUT_TIMEOUT_SEC}"
  dep="$(deploy_name ingestion)"
  [[ -n "$dep" ]] || { fail "ingestion deployment not found"; return 1; }
  log "Waiting for ingestion rollout (${dep}, ${timeout}s)..."
  if ! kubectl rollout status "deployment/${dep}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1; then
    fail "ingestion rollout did not complete within ${timeout}s"
    ingestion_logs
    return 1
  fi
  if ! kubectl wait --for=condition=available "deployment/${dep}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1; then
    fail "ingestion deployment not Available within ${timeout}s"
    ingestion_logs
    return 1
  fi
  refresh_port_forwards || return 1
  if ! wait_ingest_ready 60; then
    fail "ingestion /health not reachable after rollout (see /tmp/pf-ing-k8s.log)"
    ingestion_logs
    return 1
  fi
  pass "Ingestion deployment ready and /health OK"
  return 0
}

wait_pids() {
  local pid
  for pid in "$@"; do
    wait "$pid" 2>/dev/null || true
  done
}

wait_clickhouse_gone() {
  local timeout="${1:-${CH_SCALE_DOWN_WAIT_SEC}}"
  local selector="app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=clickhouse"
  log "Waiting for ClickHouse pod termination (${timeout}s)..."
  if kubectl wait --for=delete pod -l "${selector}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1; then
    return 0
  fi
  # Pod may already be gone
  local count
  count="$(kubectl get pods -n "${NS}" -l "${selector}" --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  [[ "${count:-0}" == "0" ]]
}

ensure_clickhouse_up() {
  local sts
  sts="$(sts_name clickhouse)"
  [[ -n "$sts" ]] || return 0
  local desired
  desired="$(kubectl get sts "$sts" -n "${NS}" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo 1)"
  if [[ "${desired:-0}" == "0" ]]; then
    log "Restoring ClickHouse replicas (was scaled to 0)..."
    kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=1
    wait_clickhouse_ready "${CH_READY_TIMEOUT_SEC}" || return 1
  fi
  return 0
}

require_ingest() {
  if ! wait_ingest_ready 15; then
    fail "ingestion not reachable at ${INGEST_URL}/health (see /tmp/pf-ing-k8s.log)"
    exit 1
  fi
}

wait_redpanda_gone() {
  local timeout="${1:-${REDPANDA_SCALE_DOWN_WAIT_SEC}}"
  local selector="app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=redpanda"
  log "Waiting for Redpanda pod termination (${timeout}s)..."
  if kubectl wait --for=delete pod -l "${selector}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1; then
    return 0
  fi
  local count
  count="$(kubectl get pods -n "${NS}" -l "${selector}" --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  [[ "${count:-0}" == "0" ]]
}

wait_redpanda_ready() {
  local timeout="${1:-${REDPANDA_READY_TIMEOUT_SEC}}"
  local selector="app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=redpanda"
  log "Waiting for redpanda pod ready (${timeout}s)..."
  if kubectl wait --for=condition=ready pod -l "${selector}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1; then
    pass "Redpanda ready"
    return 0
  fi
  fail "Redpanda not ready after ${timeout}s"
  kubectl get pods -n "${NS}" -l "${selector}" -o wide 2>&1 || true
  kubectl describe pod -n "${NS}" -l "${selector}" 2>&1 | tail -40 || true
  return 1
}

wait_clickhouse_ready() {
  local timeout="${1:-${CH_READY_TIMEOUT_SEC}}"
  local selector="app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=clickhouse"
  log "Waiting for ClickHouse pod ready (${timeout}s)..."
  if kubectl wait --for=condition=ready pod -l "${selector}" -n "${NS}" --timeout="${timeout}s" >/dev/null 2>&1; then
    pass "ClickHouse ready"
    return 0
  fi
  fail "ClickHouse not ready after ${timeout}s"
  kubectl get pods -n "${NS}" -l "${selector}" -o wide 2>&1 || true
  kubectl describe pod -n "${NS}" -l "${selector}" 2>&1 | tail -40 || true
  return 1
}

restart_ingestion_k8s() {
  local dep
  dep="$(deploy_name ingestion)"
  [[ -n "$dep" ]] || { fail "ingestion deployment not found"; return 1; }
  log "Rollout restart ingestion (${dep}) for WAL replay..."
  kubectl rollout restart "deployment/${dep}" -n "${NS}"
  wait_ingestion_rollout
}

scenario_kill_redpanda() {
  separator
  echo -e "${BOLD}SCENARIO C1 (k8s): kill-redpanda — broker outage + WAL replay${NC}"
  separator

  ensure_clickhouse_up || exit 1
  start_port_forwards || exit 1
  trap stop_port_forwards EXIT
  require_ingest

  local payload_50
  payload_50="$(make_event_payload 50 "$TENANT")"

  log "Phase 1: baseline ingest (150 events)..."
  local -a pids=()
  for _ in $(seq 1 3); do
    curl -sS "${CURL_OPTS[@]}" -o /dev/null -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload_50" &
    pids+=($!)
  done
  wait_pids "${pids[@]}"
  sleep 3
  local ch_before
  ch_before="$(count_events_in_ch "$TENANT" | tr -cd '0-9')"
  ch_before="${ch_before:-0}"
  log "ClickHouse rows before kill: ${ch_before}"

  local sts
  sts="$(sts_name redpanda)"
  log "Phase 2: scale redpanda to 0 (${sts})..."
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=0
  wait_redpanda_gone "${REDPANDA_SCALE_DOWN_WAIT_SEC}" || warn "Redpanda pod may still terminating"

  log "Phase 3: ingest during broker outage (200 events)..."
  local wal_events=0
  for _ in $(seq 1 4); do
    local code
    code="$(curl -sS "${CURL_OPTS[@]}" -o /dev/null -w "%{http_code}" -X POST "${INGEST_URL}/ingest" \
      -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload_50")"
    if [[ "$code" == "202" ]]; then
      wal_events=$((wal_events + 50))
    else
      fail "HTTP ${code} during outage (expected 202 from WAL)"
      exit 1
    fi
    sleep 0.5
  done
  if (( wal_events < 1 )); then
    fail "No events accepted into WAL during broker outage"
    exit 1
  fi
  pass "WAL accepted ${wal_events} events during outage (HTTP 202)"

  local wal_pending_outage produce_errors
  wal_pending_outage="$(metric_val "$METRICS_INGEST" 'wal_segments_pending')"
  produce_errors="$(metric_val "$METRICS_INGEST" 'kafka_produce_errors_total')"
  log "wal_segments_pending (outage): ${wal_pending_outage:-?}, kafka_produce_errors_total: ${produce_errors:-0}"
  if [[ "${wal_pending_outage:-0}" == "0" ]]; then
    fail "Expected wal_segments_pending > 0 while broker is down"
    ingestion_logs
    exit 1
  fi
  pass "WAL backlog visible (wal_segments_pending=${wal_pending_outage})"

  log "Phase 4: broker down ${REDPANDA_DOWN_SEC}s, restore Redpanda..."
  sleep "$REDPANDA_DOWN_SEC"
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=1
  sleep 3
  wait_redpanda_ready "${REDPANDA_READY_TIMEOUT_SEC}"
  sleep 15

  log "Phase 5: restart ingestion for WAL replay (PVC on M1 values-m1)..."
  restart_ingestion_k8s

  local replay wal_pending_now
  replay="$(metric_val "$METRICS_INGEST" 'wal_replay_events_total')"
  wal_pending_now="$(metric_val "$METRICS_INGEST" 'wal_segments_pending')"
  log "wal_replay_events_total: ${replay:-0}, wal_segments_pending: ${wal_pending_now:-?}"
  if [[ "${replay:-0}" == "0" ]] && (( wal_events > 0 )); then
    fail "Expected wal_replay_events_total > 0 after restart (enable ingestion.wal.persistence on k8s)"
    ingestion_logs
    exit 1
  fi
  pass "WAL replay on startup (wal_replay_events_total=${replay:-0})"

  wait_consumer_rollout
  refresh_port_forwards || exit 1

  log "Phase 5b: wait for WAL drain + consumer → ClickHouse (up to ${WAL_DRAIN_WAIT_SEC}s)..."
  local i=0 ch_now
  local min_expected=$((wal_events / 2))
  [[ "$min_expected" -ge 50 ]] || min_expected=50
  while (( i < WAL_DRAIN_WAIT_SEC )); do
    wal_pending_now="$(metric_val "$METRICS_INGEST" 'wal_segments_pending')"
    ch_now="$(count_events_in_ch "$TENANT" | tr -cd '0-9')"
    ch_now="${ch_now:-0}"
    if [[ "${wal_pending_now:-1}" == "0" ]] && (( ch_now >= ch_before + min_expected )); then
      pass "WAL drained and ClickHouse caught up (${ch_before} → ${ch_now})"
      break
    fi
    sleep 5
    i=$((i + 5))
    log "  ${i}s — wal_segments_pending: ${wal_pending_now:-?}, CH: ${ch_now}"
  done
  if [[ "${wal_pending_now:-1}" != "0" ]] || (( ch_now < ch_before + min_expected )); then
    fail "Recovery incomplete (pending=${wal_pending_now:-?}, CH=${ch_now:-0}, need +${min_expected})"
    ingestion_logs
    consumer_logs
    exit 1
  fi
  if ! post_ingest_batch 50; then
    fail "post-recovery ingest did not return 202"
    ingestion_logs
    exit 1
  fi
  pass "Post-recovery ingest accepted (HTTP 202)"

  local ch_after min_expected total_sent
  min_expected=$((wal_events / 2))
  [[ "$min_expected" -ge 50 ]] || min_expected=50
  total_sent=$((150 + wal_events))
  log "Phase 6: wait for ≥${min_expected} new CH rows (tenant ${TENANT}, up to ${CH_RECOVERY_WAIT_SEC}s)..."
  ensure_clickhouse_up || exit 1
  if ch_after="$(wait_ch_tenant_growth "$ch_before" "$min_expected" "$CH_RECOVERY_WAIT_SEC")"; then
    pass "ClickHouse grew: ${ch_before} → ${ch_after} (sent ~${total_sent} during scenario)"
  else
    ch_after="${ch_after:-$(count_events_in_ch "$TENANT" | tr -cd '0-9')}"
    fail "ClickHouse did not grow enough (before=${ch_before}, after=${ch_after}, need +${min_expected})"
    ingestion_logs
    consumer_logs
    exit 1
  fi
  separator
}

scenario_throttle_clickhouse() {
  separator
  echo -e "${BOLD}SCENARIO C2 (k8s): throttle-clickhouse — breaker + overflow${NC}"
  separator

  ensure_clickhouse_up || exit 1
  start_port_forwards || exit 1
  trap 'stop_port_forwards; ensure_clickhouse_up' EXIT
  require_ingest

  local cb_before overflow_before ch_before
  cb_before="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
  overflow_before="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
  ch_before="$(count_events_in_ch "$TENANT" | tr -cd '0-9')"
  log "Baseline — breaker_open: ${cb_before:-0}, overflow: ${overflow_before:-0}, CH: ${ch_before}"

  local sts
  sts="$(sts_name clickhouse)"
  log "Phase 1: scale ClickHouse to 0 (${sts}) for ${CH_PAUSE_SEC}s window..."
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=0
  wait_clickhouse_gone "${CH_SCALE_DOWN_WAIT_SEC}" || warn "ClickHouse pod may still terminating"

  local payload_50
  payload_50="$(make_event_payload 50 "$TENANT")"
  log "Phase 2: sustained ingest while CH unavailable (${C2_BURST_ROUNDS}×${C2_BURST_PARALLEL} curls/round)..."
  local cb_during=0 overflow_during=0 cb_max=0 overflow_max=0
  local ch_errors_start ch_errors_peak lag_peak
  ch_errors_start="$(metric_val "$METRICS_CONSUMER" 'clickhouse_write_errors_total')"
  ch_errors_start="${ch_errors_start%%.*}"
  ch_errors_start="${ch_errors_start:-0}"
  ch_errors_peak="$ch_errors_start"
  lag_peak="$(metric_val "$METRICS_CONSUMER" 'kafka_consumer_lag_events')"
  lag_peak="${lag_peak%%.*}"
  lag_peak="${lag_peak:-0}"
  local round
  for (( round = 1; round <= C2_BURST_ROUNDS; round++ )); do
    flood_ingest_burst "$payload_50" "${C2_BURST_PARALLEL}"
    sleep "${C2_FLUSH_PAUSE_SEC}"
    cb_during="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
    overflow_during="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
    local ch_err_now lag_now
    ch_err_now="$(metric_max "$METRICS_CONSUMER" 'clickhouse_write_errors_total')"
    ch_err_now="${ch_err_now%%.*}"
    ch_err_now="${ch_err_now:-0}"
    (( ch_err_now > ch_errors_peak )) && ch_errors_peak=$ch_err_now
    lag_now="$(metric_max "$METRICS_CONSUMER" 'kafka_consumer_lag_events')"
    lag_now="${lag_now%%.*}"
    lag_now="${lag_now:-0}"
    (( lag_now > lag_peak )) && lag_peak=$lag_now
    [[ "${cb_during:-0}" == "1" ]] && cb_max=1
    local od="${overflow_during%%.*}"
    od="${od:-0}"
    (( od > overflow_max )) && overflow_max=$od
    log "  round ${round}/${C2_BURST_ROUNDS} — breaker_open: ${cb_during:-0}, overflow: ${overflow_during:-0}, ch_errors: ${ch_errors_peak} (Δ=$((ch_errors_peak - ch_errors_start))), lag: ${lag_now}"
    if [[ "$cb_max" == "1" ]] || (( overflow_max > 0 )); then
      break
    fi
  done
  local ch_errors_delta=$((ch_errors_peak - ch_errors_start))

  log "Phase 3: restore ClickHouse..."
  kubectl scale "statefulset/${sts}" -n "${NS}" --replicas=1
  sleep 3
  if ! wait_clickhouse_ready "${CH_READY_TIMEOUT_SEC}"; then
    ensure_clickhouse_up || true
    exit 1
  fi
  sleep 15

  local cb_after overflow_after ch_after
  cb_after="$(metric_val "$METRICS_CONSUMER" 'circuit_breaker_state\{state="open"\}')"
  overflow_after="$(metric_val "$METRICS_CONSUMER" 'redis_overflow_depth')"
  ch_after="$(count_events_in_ch "$TENANT" | tr -cd '0-9')"

  echo "  breaker max: ${cb_max}, overflow max: ${overflow_max}, ch_errors Δ: ${ch_errors_delta}, lag peak: ${lag_peak}"
  echo "  breaker after: ${cb_after:-?}, overflow after: ${overflow_after:-?}"
  echo "  CH rows before/after: ${ch_before} / ${ch_after}"

  if [[ "$cb_max" == "1" ]]; then
    pass "Circuit breaker opened during CH outage"
  elif (( overflow_max > 0 )); then
    pass "Redis overflow observed during CH outage (max depth ${overflow_max})"
  elif (( ch_errors_delta >= 5 )); then
    pass "ClickHouse write errors increased by ${ch_errors_delta} (breaker threshold reached)"
  elif (( lag_peak >= 200 )); then
    pass "Consumer lag backlog during CH outage (peak ${lag_peak} events)"
  else
    fail "No breaker, overflow, CH errors (Δ=${ch_errors_delta}), or lag (peak=${lag_peak}) during CH outage"
    consumer_logs
    exit 1
  fi

  if (( overflow_max > 0 )); then
    pass "Redis overflow depth peaked at ${overflow_max}"
  fi
  separator
}

scenario_load_m1() {
  separator
  echo -e "${BOLD}SCENARIO load-m1 — ${LOAD_EVENTS} events over ${LOAD_DURATION_SEC}s${NC}"
  separator

  ensure_clickhouse_up
  start_port_forwards
  trap stop_port_forwards EXIT
  require_ingest

  local ch_before ch_after total_sent=0
  ch_before="$(count_events_in_ch "$TENANT" | tr -cd '0-9')"
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
    local -a pids=()
    for _ in $(seq 1 "$reqs_per_sec"); do
      curl -sS "${CURL_OPTS[@]}" -o /dev/null -X POST "${INGEST_URL}/ingest" \
        -H 'Content-Type: application/json' -H "X-Tenant-ID: ${TENANT}" -d "$payload" &
      pids+=($!)
      total_sent=$((total_sent + events_per_req))
    done
    wait_pids "${pids[@]}"
    if (( sec % 5 == 0 )); then
      log "  ... ${sec}s, ~${total_sent} events sent"
    fi
  done

  sleep 12
  ch_after="$(count_events_in_ch "$TENANT" | tr -cd '0-9')"
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
  CH_READY_TIMEOUT_SEC (default: 300), CH_SCALE_DOWN_WAIT_SEC (default: 90)
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
