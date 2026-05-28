#!/usr/bin/env bash
# k8s smoke: assumes helm release "lensai" in namespace lensai.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NS="${K8S_NAMESPACE:-lensai}"
RELEASE="${HELM_RELEASE:-lensai}"

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
sleep 3

echo "==> Health checks"
curl -sf http://localhost:8080/health | head -c 200
echo ""
curl -sf http://localhost:9091/health
echo ""

echo "==> POST /ingest"
ts_ms="$(python3 -c 'import time; print(int(time.time()*1000))')"
code="$(curl -sS -o /tmp/ingest-body.txt -w '%{http_code}' -X POST http://localhost:8080/ingest \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: demo' \
  -d "{\"events\":[{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts_ms},\"latency_ms\":100,\"prompt_tokens\":10,\"completion_tokens\":5,\"cost_usd\":0.01,\"status\":\"success\"}]}")"
echo "HTTP ${code} — $(cat /tmp/ingest-body.txt)"
[[ "$code" == "202" ]] || { echo "FAIL: expected 202" >&2; exit 1; }

CH_POD="$(kubectl get pods -n "${NS}" -l app.kubernetes.io/component=clickhouse -o jsonpath='{.items[0].metadata.name}')"
echo "==> Waiting for ClickHouse rows (up to 45s)"
for _ in $(seq 1 45); do
  count="$(kubectl exec -n "${NS}" "$CH_POD" -- clickhouse-client --query \
    "SELECT count() FROM infra_ai.inference_events WHERE tenant_id='demo' AND cost_usd > 0" 2>/dev/null || echo 0)"
  if [[ "${count:-0}" -gt 0 ]]; then
    echo "    ClickHouse OK (${count} rows)"
    kubectl exec -n "${NS}" "$CH_POD" -- clickhouse-client --query \
      "SELECT tenant_id, model_id, cost_usd FROM infra_ai.inference_events WHERE tenant_id='demo' ORDER BY timestamp DESC LIMIT 3"
    break
  fi
  sleep 1
done

echo "==> Consumer lag metric"
curl -sf http://localhost:9091/metrics | grep -E '^kafka_consumer_lag_events' | head -3 || echo "WARN: lag metric not found yet"

echo "==> HPA status"
kubectl get hpa -n "${NS}" 2>/dev/null || true

echo "==> k8s smoke complete"
