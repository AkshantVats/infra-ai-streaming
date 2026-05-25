#!/usr/bin/env bash
# scripts/e2e-k3d-full.sh — One-command M1 E2E: preflight → k3d deploy → sequential cluster tests.
#
#   ./scripts/e2e-k3d-full.sh
#
# Why Helm waits: `helm upgrade --install --wait` blocks until Deployments, StatefulSets,
# and hook Jobs report ready. On M1, Redpanda/consumer CrashLoopBackOff (OOM, tight probes)
# kept Helm waiting until the default timeout (15m). This script uses a short Helm timeout
# (default 2m, no global --wait), then polls each critical workload with kubectl wait
# (default 120s) and prints describe/logs on failure so you fail fast with diagnostics.
#
# Environment:
#   CONTINUE_ON_FAIL=1        keep going after a failed step (default: fail-fast)
#   SKIP_DEPLOY=1             skip Phase B (cluster already up)
#   SKIP_PREFLIGHT=1          skip Phase A unit tests
#   HELM_WAIT_TIMEOUT=2m      helm upgrade --timeout (no --wait; we poll pods explicitly)
#   POD_WAIT_TIMEOUT=120s     per-workload kubectl wait timeout
#   K3D_CLUSTER               default: lensai
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NS="${K8S_NAMESPACE:-lensai}"
RELEASE="${HELM_RELEASE:-lensai}"
CLUSTER="${K3D_CLUSTER:-lensai}"
HELM_CHART="deploy/helm/lensai"
VALUES_M1="${HELM_CHART}/values-m1.yaml"
PROOF="${ROOT}/docs/E2E-PROOF-K3D.md"
CONTINUE_ON_FAIL="${CONTINUE_ON_FAIL:-0}"
HELM_WAIT_TIMEOUT="${HELM_WAIT_TIMEOUT:-2m}"
POD_WAIT_TIMEOUT="${POD_WAIT_TIMEOUT:-120s}"
INIT_JOB_WAIT_TIMEOUT="${INIT_JOB_WAIT_TIMEOUT:-180s}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"

declare -a STEP_NAMES=()
declare -a STEP_STATUS=()
declare -a STEP_DETAIL=()

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${CYAN}[e2e]${NC} $*"; }
record_step() {
  local name="$1" status="$2" detail="${3:-}"
  STEP_NAMES+=("$name")
  STEP_STATUS+=("$status")
  STEP_DETAIL+=("$detail")
}

proof_append() {
  {
    echo ""
    echo "## Run ${RUN_ID}"
    echo ""
    echo '```'
    cat "$PROOF_TMP"
    echo '```'
  } >>"$PROOF"
}

run_logged() {
  local desc="$1"
  shift
  echo ""
  log "▶ ${desc}"
  echo "\$ $*" >>"$PROOF_TMP"
  set +e
  "$@" >>"$PROOF_TMP" 2>&1
  local ec=$?
  set -e
  echo "exit=${ec}" >>"$PROOF_TMP"
  return "$ec"
}

run_step() {
  local name="$1" status_on_fail="${2:-RED}"
  shift 2
  run_logged "$name" "$@"
  local ec=$?
  if [[ "$ec" -eq 0 ]]; then
    record_step "$name" "GREEN" "ok"
    log "✓ ${name}"
    return 0
  fi
  if [[ "$status_on_fail" == "YELLOW" ]]; then
    record_step "$name" "YELLOW" "exit ${ec}"
    log "⚠ ${name} (non-fatal)"
    [[ "$CONTINUE_ON_FAIL" == "1" ]] && return 0
    return "$ec"
  fi
  record_step "$name" "RED" "exit ${ec}"
  log "✗ ${name} (exit ${ec})"
  [[ "$CONTINUE_ON_FAIL" == "1" ]] && return 0
  return "$ec"
}

require_cmd() {
  local c="$1"
  command -v "$c" >/dev/null 2>&1 || { echo "Missing required command: $c" >&2; return 1; }
}

kill_stale_e2e() {
  local self=$$
  local pid
  while read -r pid; do
    [[ -z "$pid" || "$pid" == "$self" ]] && continue
    log "Stopping stale e2e process pid=${pid}"
    kill "$pid" 2>/dev/null || true
  done < <(pgrep -f '[/]scripts/e2e-k3d-full\.sh' 2>/dev/null || true)
}

diagnose_not_ready() {
  local component="${1:-}"
  local selector="app.kubernetes.io/instance=${RELEASE}"
  if [[ -n "$component" ]]; then
    selector+=",app.kubernetes.io/component=${component}"
  fi
  {
    echo "=== FAIL: pods not ready (component=${component:-all}) ==="
    echo "--- kubectl get pods -n ${NS} ---"
    kubectl get pods -n "${NS}" -o wide 2>&1 || true
    echo "--- kubectl describe pod (not Ready) ---"
    kubectl get pods -n "${NS}" -l "${selector}" -o name 2>/dev/null | while read -r pod; do
      local ready
      ready="$(kubectl get -n "${NS}" "$pod" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo Unknown)"
      [[ "$ready" == "True" ]] && continue
      kubectl describe -n "${NS}" "$pod" 2>&1 || true
      echo "--- kubectl logs ${pod} (current) ---"
      kubectl logs -n "${NS}" "$pod" --tail=80 2>&1 || true
      echo "--- kubectl logs ${pod} (previous) ---"
      kubectl logs -n "${NS}" "$pod" --previous --tail=40 2>&1 || true
    done
    echo "Hints: Redpanda OOM → raise values-m1 redpanda limits; consumer → KAFKA_BROKERS / CLICKHOUSE_DSN; ImagePullBackOff → ./deploy/k3d/up.sh"
  } | tee -a "$PROOF_TMP"
}

wait_component_pods() {
  local component="$1"
  local selector="app.kubernetes.io/instance=${RELEASE},app.kubernetes.io/component=${component}"
  log "kubectl wait ready: ${component} (${POD_WAIT_TIMEOUT})"
  if kubectl wait --for=condition=ready pod -l "${selector}" -n "${NS}" --timeout="${POD_WAIT_TIMEOUT}" >>"$PROOF_TMP" 2>&1; then
    return 0
  fi
  diagnose_not_ready "${component}"
  return 1
}

wait_init_jobs() {
  local ec=0
  local job
  for job in "${RELEASE}-redpanda-init" "${RELEASE}-clickhouse-init"; do
    log "kubectl wait complete: ${job} (${INIT_JOB_WAIT_TIMEOUT})"
    if kubectl wait --for=condition=complete "job/${job}" -n "${NS}" --timeout="${INIT_JOB_WAIT_TIMEOUT}" >>"$PROOF_TMP" 2>&1; then
      continue
    fi
    {
      echo "=== FAIL: job/${job} not complete within ${INIT_JOB_WAIT_TIMEOUT} ==="
      kubectl describe job -n "${NS}" "${job}" 2>&1 || true
      kubectl logs -n "${NS}" "job/${job}" --tail=80 2>&1 || true
    } | tee -a "$PROOF_TMP"
    ec=1
  done
  return "$ec"
}

wait_cluster_ready() {
  local ec=0
  local infra=(redis redpanda clickhouse prometheus)
  local apps=(ingestion consumer)
  for c in "${infra[@]}"; do
    wait_component_pods "$c" || ec=1
  done
  wait_init_jobs || ec=1
  for c in "${apps[@]}"; do
    wait_component_pods "$c" || ec=1
  done
  kubectl get pods,jobs,hpa -n "${NS}" >>"$PROOF_TMP" 2>&1 || true
  if [[ "$ec" -ne 0 ]]; then
    log "${RED}Cluster not ready within ${POD_WAIT_TIMEOUT} per workload${NC}"
    [[ "$CONTINUE_ON_FAIL" == "1" ]] && return 0
    return 1
  fi
  return 0
}

# ── Phase A: Preflight ───────────────────────────────────────────────────────
phase_a() {
  log "${BOLD}Phase A — Preflight (host)${NC}"
  if [[ "${SKIP_PREFLIGHT:-}" == "1" ]]; then
    record_step "preflight-skip" "YELLOW" "SKIP_PREFLIGHT=1"
    return 0
  fi
  run_step "cargo test ingestion" RED cargo test -p ingestion
  run_step "go test consumer" RED bash -c 'cd consumer && go test ./...'
  run_step "helm template values-m1" RED helm template "${RELEASE}" "${HELM_CHART}" \
    -f "${VALUES_M1}" --namespace "${NS}" >/dev/null
  local sh
  for sh in scripts/*.sh chaos/*.sh; do
    [[ -f "$sh" ]] || continue
    run_step "bash -n ${sh}" RED bash -n "$sh"
  done
  run_step "check docker" RED require_cmd docker
  run_step "check k3d" RED require_cmd k3d
  run_step "check helm" RED require_cmd helm
  run_step "check kubectl" RED require_cmd kubectl
}

# ── Phase B: Deploy ──────────────────────────────────────────────────────────
phase_b() {
  log "${BOLD}Phase B — Deploy (k3d + Helm M1)${NC}"
  if [[ "${SKIP_DEPLOY:-}" == "1" ]]; then
    record_step "deploy-skip" "YELLOW" "SKIP_DEPLOY=1"
    wait_cluster_ready
    return 0
  fi

  run_step "docker compose down" YELLOW bash -c \
    'docker compose --env-file deploy/.env -f deploy/docker-compose.yml down 2>/dev/null || true'

  if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER} "; then
    run_step "k3d cluster delete ${CLUSTER}" YELLOW k3d cluster delete "${CLUSTER}" || true
  fi

  run_step "k3d up (cluster + images)" RED ./deploy/k3d/up.sh
  run_step "helm dependency update" RED helm dependency update "${HELM_CHART}"
  run_step "helm upgrade --install (values-m1)" RED helm upgrade --install "${RELEASE}" "${HELM_CHART}" \
    -n "${NS}" --create-namespace \
    -f "${VALUES_M1}" \
    --timeout "${HELM_WAIT_TIMEOUT}" \
    --wait=false \
    --wait-for-jobs=false

  if run_logged "wait cluster ready (${POD_WAIT_TIMEOUT}/workload)" wait_cluster_ready; then
    record_step "wait pods + init jobs" "GREEN" "all critical workloads ready"
  else
    record_step "wait pods + init jobs" "RED" "see diagnostics in proof log"
    return 1
  fi
}

# ── Phase C: Tests on cluster ────────────────────────────────────────────────
phase_c() {
  log "${BOLD}Phase C — Tests on cluster (sequential)${NC}"

  export SKIP_UNIT_TESTS=1
  run_step "smoke-k8s-e2e" RED ./scripts/smoke-k8s-e2e.sh

  run_step "chaos C1 kill-redpanda (k8s)" RED ./chaos/run_chaos_k8s.sh kill-redpanda
  run_step "chaos C2 throttle-clickhouse (k8s)" RED ./chaos/run_chaos_k8s.sh throttle-clickhouse
  run_step "chaos load-m1 (k8s)" RED env LOAD_EVENTS=1000 LOAD_DURATION_SEC=10 ./chaos/run_chaos_k8s.sh load-m1

  run_step "HPA status" YELLOW bash -c "
    kubectl get hpa -n '${NS}' 2>&1 || echo 'No HPA (expected on M1 values-m1)'
    curl -sf http://localhost:9091/metrics 2>/dev/null | grep -E '^kafka_consumer_lag_events' | head -3 || true
  "
}

print_summary() {
  echo ""
  echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${BOLD}E2E summary (run ${RUN_ID})${NC}"
  printf "${BOLD}%-42s %-8s %s${NC}\n" "STEP" "STATUS" "DETAIL"
  local i
  for i in "${!STEP_NAMES[@]}"; do
    local st="${STEP_STATUS[$i]}"
    local color="${NC}"
    case "$st" in
      GREEN)  color="${GREEN}" ;;
      YELLOW) color="${YELLOW}" ;;
      RED)    color="${RED}" ;;
    esac
    printf "%-42s ${color}%-8s${NC} %s\n" "${STEP_NAMES[$i]}" "$st" "${STEP_DETAIL[$i]:-}"
  done
  echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
  echo "Proof appended: ${PROOF}"
}

# ── Main ─────────────────────────────────────────────────────────────────────
mkdir -p "$(dirname "$PROOF")"
PROOF_TMP="$(mktemp)"
trap 'rm -f "$PROOF_TMP"' EXIT

kill_stale_e2e

if [[ ! -f "$PROOF" ]]; then
  cat >"$PROOF" <<EOF
# E2E proof — k3d full stack (M1)

Automated log from \`./scripts/e2e-k3d-full.sh\`. Each section is one run.

## Test matrix (preflight + full e2e)

| # | Command | Purpose |
|---|---------|---------|
| 1 | \`cargo test -p ingestion\` | Rust ingestion unit tests |
| 2 | \`cd consumer && go test ./...\` | Go consumer unit tests |
| 3 | \`bash -n scripts/*.sh chaos/*.sh\` | Shell syntax check |
| 4 | \`helm template … -f values-m1.yaml\` | Chart renders on M1 values |
| 5 | \`./scripts/e2e-k3d-full.sh\` | Full k3d deploy + smoke + chaos |

EOF
fi

{
  echo "Started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "Host: $(uname -a)"
  echo "Branch: $(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
  echo "CONTINUE_ON_FAIL=${CONTINUE_ON_FAIL}"
  echo "HELM_WAIT_TIMEOUT=${HELM_WAIT_TIMEOUT}"
  echo "POD_WAIT_TIMEOUT=${POD_WAIT_TIMEOUT}"
} >"$PROOF_TMP"

main_ec=0
phase_a || main_ec=1
if [[ "$main_ec" -eq 0 ]] || [[ "$CONTINUE_ON_FAIL" == "1" ]]; then
  phase_b || main_ec=1
fi
if [[ "$main_ec" -eq 0 ]] || [[ "$CONTINUE_ON_FAIL" == "1" ]]; then
  phase_c || main_ec=1
fi

proof_append
print_summary
exit "$main_ec"
