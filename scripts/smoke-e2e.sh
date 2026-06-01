#!/usr/bin/env bash
# E2E smoke: compose health, topics, unit tests, multi-scenario HTTP ingest,
# consumer stdout verification, Prometheus metrics check, and optional
# ClickHouse row validation.
# Ingestion + consumer services must be running for the live HTTP checks.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE=(docker compose --env-file deploy/.env -f deploy/docker-compose.yml)
ENV_FILE="deploy/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "==> Copying deploy/.env.example -> deploy/.env"
  cp deploy/.env.example "$ENV_FILE"
fi

echo "==> Starting compose stack"
"${COMPOSE[@]}" up -d

echo "==> Waiting for long-running services (up to ~120s)"
deadline=$((SECONDS + 120))
while (( SECONDS < deadline )); do
  unhealthy="$("${COMPOSE[@]}" ps --format json 2>/dev/null | python3 -c "
import json, sys
bad = []
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    o = json.loads(line)
    name = o.get('Service') or o.get('Name', '?')
    state = (o.get('State') or '').lower()
    health = (o.get('Health') or '').lower()
    if state in ('exited', 'dead'):
        if name not in ('redpanda-init', 'clickhouse-init'):
            bad.append(f'{name}:{state}')
    elif state == 'running' and health and health not in ('healthy', ''):
        if name not in ('redpanda-init', 'clickhouse-init'):
            bad.append(f'{name}:{health}')
if bad:
    print(' '.join(bad))
" 2>/dev/null || true)"
  if [[ -z "$unhealthy" ]]; then
    break
  fi
  sleep 3
done

echo "==> Compose services"
"${COMPOSE[@]}" ps

echo "==> Verifying Kafka topics"
"${COMPOSE[@]}" exec -T redpanda rpk topic list | tee /tmp/rpk-topics.txt
grep -q ai_inference_events /tmp/rpk-topics.txt
grep -q ai_inference_dlq /tmp/rpk-topics.txt
grep -q ai_anomalies /tmp/rpk-topics.txt

echo "==> Kafka topic partition count (ai_inference_events)"
"${COMPOSE[@]}" exec -T redpanda rpk topic describe ai_inference_events 2>/dev/null \
  | grep -i "partition\|Partitions" || echo "    WARN: could not describe topic partition count"

echo "==> Unit tests (no compose required for Go)"
cargo test -p ingestion
(cd consumer && go test ./...)

if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
  echo "==> Ingestion reachable — running HTTP ingest scenario tests"
  ts_ms="$(python3 -c 'import time; print(int(time.time()*1000))')"

  # --- Scenario 1: single happy-path event → expect 202 ---
  echo "  [1/5] Single valid event → expect 202"
  resp="$(curl -sS -w '\n%{http_code}' -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' \
    -H 'X-Tenant-ID: demo' \
    -d "{\"events\":[{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts_ms},\"latency_ms\":342,\"prompt_tokens\":512,\"completion_tokens\":128,\"cost_usd\":0.00423,\"status\":\"success\"}]}")"
  body="${resp%$'\n'*}"; code="${resp##*$'\n'}"
  echo "    HTTP $code — $body"
  if [[ "$code" != "202" ]]; then
    echo "FAIL: expected 202, got $code" >&2; exit 1
  fi

  # --- Scenario 2: batch of 3 events → expect 202 ---
  echo "  [2/5] Batch of 3 events → expect 202"
  ts2="$(python3 -c 'import time; print(int(time.time()*1000))')"
  ts3="$(python3 -c 'import time; print(int(time.time()*1000)+1)')"
  ts4="$(python3 -c 'import time; print(int(time.time()*1000)+2)')"
  batch_payload="{\"events\":["
  batch_payload+="{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts2},\"latency_ms\":100,\"prompt_tokens\":10,\"completion_tokens\":5,\"cost_usd\":0.001},"
  batch_payload+="{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts3},\"latency_ms\":200,\"prompt_tokens\":20,\"completion_tokens\":10,\"cost_usd\":0.002},"
  batch_payload+="{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts4},\"latency_ms\":300,\"prompt_tokens\":30,\"completion_tokens\":15,\"cost_usd\":0.003}"
  batch_payload+="]}"
  resp="$(curl -sS -w '\n%{http_code}' -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' \
    -H 'X-Tenant-ID: demo' \
    -d "$batch_payload")"
  code="${resp##*$'\n'}"
  echo "    HTTP $code"
  if [[ "$code" != "202" ]]; then
    echo "FAIL: batch of 3 events returned $code (expected 202)" >&2; exit 1
  fi

  # --- Scenario 3: missing X-Tenant-ID header → expect 400 ---
  echo "  [3/5] Missing X-Tenant-ID → expect 400"
  resp="$(curl -sS -w '\n%{http_code}' -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' \
    -d "{\"events\":[{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts_ms},\"latency_ms\":1,\"prompt_tokens\":1,\"completion_tokens\":1,\"cost_usd\":0.0}]}")"
  code="${resp##*$'\n'}"
  echo "    HTTP $code"
  if [[ "$code" != "400" ]]; then
    echo "FAIL: expected 400 for missing tenant header, got $code" >&2; exit 1
  fi

  # --- Scenario 4: malformed JSON body → expect 400 ---
  echo "  [4/5] Malformed JSON body → expect 400"
  resp="$(curl -sS -w '\n%{http_code}' -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' \
    -H 'X-Tenant-ID: demo' \
    -d 'not-json-at-all')"
  code="${resp##*$'\n'}"
  echo "    HTTP $code"
  if [[ "$code" != "400" ]]; then
    echo "FAIL: expected 400 for malformed JSON, got $code" >&2; exit 1
  fi

  # --- Scenario 5: wrong Content-Type (text/plain) → expect 4xx ---
  echo "  [5/5] Wrong Content-Type (text/plain) → expect 4xx"
  resp="$(curl -sS -w '\n%{http_code}' -X POST http://localhost:8080/ingest \
    -H 'Content-Type: text/plain' \
    -H 'X-Tenant-ID: demo' \
    -d 'hello world')"
  code="${resp##*$'\n'}"
  echo "    HTTP $code"
  if [[ "${code:0:1}" != "4" ]]; then
    echo "FAIL: expected 4xx for wrong content-type, got $code" >&2; exit 1
  fi

  echo "==> All HTTP ingest scenarios passed"

  # --- ClickHouse row verification ---
  echo "==> Waiting for ClickHouse rows (up to 15s)"
  ch_ok=0
  for _ in $(seq 1 15); do
    count="$("${COMPOSE[@]}" exec -T clickhouse clickhouse-client --query \
      "SELECT count() FROM infra_ai.inference_events WHERE tenant_id='demo' AND cost_usd > 0" 2>/dev/null || echo 0)"
    if [[ "${count:-0}" -gt 0 ]]; then
      ch_ok=1
      break
    fi
    sleep 1
  done
  if [[ "$ch_ok" -eq 1 ]]; then
    echo "    ClickHouse rows OK (demo tenant, cost_usd > 0)"
    "${COMPOSE[@]}" exec -T clickhouse clickhouse-client --query \
      "SELECT tenant_id, model_id, cost_usd FROM infra_ai.inference_events WHERE tenant_id='demo' ORDER BY timestamp DESC LIMIT 3"
  else
    echo "WARN: no ClickHouse rows yet (start consumer: cd consumer && go run ./cmd/consumer)" >&2
  fi

  # --- Consumer stdout verification ---
  echo "==> Checking consumer container stdout for event-processing keywords"
  consumer_log="$("${COMPOSE[@]}" logs --tail=50 consumer 2>/dev/null || true)"
  if echo "$consumer_log" | grep -qiE "processed|handoff|kafka_records|consumer_started"; then
    echo "    Consumer log contains expected event-processing output"
  else
    echo "    WARN: consumer log does not contain event-processing keywords (consumer may not be running)" >&2
  fi
else
  echo "==> Skipping live /ingest tests (ingestion not reachable on :8080)"
  echo "    Terminal A: cd consumer && go run ./cmd/consumer"
  echo "    Terminal B: cargo run -p ingestion"
  echo "    Re-run this script or curl manually (see README)."
fi

echo "==> Prometheus metrics smoke"
if curl -sf http://localhost:8080/metrics 2>/dev/null | grep -q "ingestion_"; then
  echo "    ingestion /metrics OK — ingestion_ metric family present on :8080"
elif curl -sf http://localhost:8080/metrics >/dev/null 2>&1; then
  echo "    ingestion /metrics endpoint reachable on :8080"
else
  echo "    WARN: ingestion /metrics not reachable on :8080" >&2
fi

if curl -sf http://localhost:9091/metrics 2>/dev/null | grep -q clickhouse_batch_size; then
  echo "    consumer /metrics OK — clickhouse_batch_size present on :9091"
elif curl -sf http://localhost:9091/metrics >/dev/null 2>&1; then
  echo "    consumer /metrics reachable on :9091 (clickhouse_batch_size not yet seen)"
else
  echo "    WARN: consumer /metrics not reachable on :9091 (go run ./cmd/consumer)" >&2
fi

echo "==> Smoke complete"
