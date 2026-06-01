#!/usr/bin/env bash
# k8s smoke: assumes helm release "lensai" in namespace lensai.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NS="${K8S_NAMESPACE:-lensai}"
RELEASE="${HELM_RELEASE:-lensai}"
CURL_RETRIES="${SMOKE_CURL_RETRIES:-12}"
CURL_INTERVAL="${SMOKE_CURL_INTERVAL_SEC:-2}"
PF_WARMUP_SEC="${SMOKE_PF_WARMUP_SEC:-5}"
CH_WAIT_SEC="${SMOKE_CH_WAIT_SEC:-60}"

wait_http() {
  local url="$1"
  local desc="$2"
  local attempt=1
  while (( attempt <= CURL_RETRIES )); do
    if curl -sf --connect-timeout 3 --max-time 10 "$url" >/dev/null 2>&1; then
      return 0
    fi
    echo "    ${desc}: not ready (${attempt}/${CURL_RETRIES}), retry in ${CURL_INTERVAL}s"
    sleep "$CURL_INTERVAL"
    attempt=$((attempt + 1))
  done
  echo "FAIL: ${desc} unreachable at ${url}" >&2
  return 22
}

curl_post_ingest() {
  local ts_ms="$1"
  local attempt=1
  local code
  while (( attempt <= CURL_RETRIES )); do
    code="$(curl -sS --connect-timeout 3 --max-time 15 -o /tmp/ingest-body.txt -w '%{http_code}' \
      -X POST http://localhost:8080/ingest \
      -H 'Content-Type: application/json' \
      -H 'X-Tenant-ID: demo' \
      -d "{\"events\":[{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts_ms},\"latency_ms\":100,\"prompt_tokens\":10,\"completion_tokens\":5,\"cost_usd\":0.01,\"status\":\"success\"}]}")" || code="000"
    if [[ "$code" == "202" ]]; then
      echo "HTTP ${code} — $(cat /tmp/ingest-body.txt)"
      return 0
    fi
    echo "    POST /ingest returned ${code} (${attempt}/${CURL_RETRIES})"
    sleep "$CURL_INTERVAL"
    attempt=$((attempt + 1))
  done
  echo "FAIL: expected HTTP 202, got ${code:-unknown}" >&2
  [[ -f /tmp/ingest-body.txt ]] && cat /tmp/ingest-body.txt >&2 || true
  return 1
}

echo "==> Waiting for pods in namespace ${NS}"
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance="${RELEASE}" -n "${NS}" --timeout=300s 2>/dev/null || true
kubectl get pods -n "${NS}"

if [[ "${SKIP_UNIT_TESTS:-}" != "1" ]]; then
  echo "==> Unit tests (host)"
  cargo test -p ingestion
  (cd consumer && go test ./...)
else
  echo "==> Unit tests skipped (SKIP_UNIT_TESTS=1)"
fi

PF_ING="${PF_ING_PID:-}"
PF_CON="${PF_CON_PID:-}"
cleanup() {
  if [[ -n "$PF_ING" ]]; then
    kill "$PF_ING" 2>/dev/null || true
  fi
  if [[ -n "$PF_CON" ]]; then
    kill "$PF_CON" 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "==> Port-forward ingestion :8080 and consumer metrics :9091"
kubectl port-forward -n "${NS}" svc/ingestion 8080:8080 >/tmp/pf-ing.log 2>&1 &
PF_ING=$!
kubectl port-forward -n "${NS}" svc/consumer 9091:9091 >/tmp/pf-con.log 2>&1 &
PF_CON=$!
sleep "$PF_WARMUP_SEC"

echo "==> Health checks"
curl -sf http://localhost:8080/health | head -c 200 || wait_http "http://localhost:8080/health" "ingestion /health"
echo ""
wait_http "http://localhost:9091/health" "consumer /health"
curl -sf http://localhost:9091/health
echo ""

echo "==> POST /ingest"
ts_ms="$(python3 -c 'import time; print(int(time.time()*1000))')"
curl_post_ingest "$ts_ms"

CH_POD="$(kubectl get pods -n "${NS}" -l app.kubernetes.io/component=clickhouse -o jsonpath='{.items[0].metadata.name}')"
echo "==> Waiting for ClickHouse rows (up to ${CH_WAIT_SEC}s)"
ch_ok=0
for _ in $(seq 1 "$CH_WAIT_SEC"); do
  count="$(kubectl exec -n "${NS}" "$CH_POD" -- clickhouse-client --query \
    "SELECT count() FROM infra_ai.inference_events WHERE tenant_id='demo' AND cost_usd > 0" 2>/dev/null || echo 0)"
  if [[ "${count:-0}" -gt 0 ]]; then
    echo "    ClickHouse OK (${count} rows)"
    kubectl exec -n "${NS}" "$CH_POD" -- clickhouse-client --query \
      "SELECT tenant_id, model_id, cost_usd FROM infra_ai.inference_events WHERE tenant_id='demo' ORDER BY timestamp DESC LIMIT 3"
    ch_ok=1
    break
  fi
  sleep 1
done
if [[ "$ch_ok" -ne 1 ]]; then
  echo "FAIL: no demo rows in ClickHouse after ${CH_WAIT_SEC}s" >&2
  exit 1
fi

echo "==> Consumer lag metric"
curl -sf http://localhost:9091/metrics | grep -E '^kafka_consumer_lag_events' | head -3 || echo "WARN: lag metric not found yet"

echo "==> HPA status"
kubectl get hpa -n "${NS}" 2>/dev/null || true

echo "==> k8s smoke complete"
