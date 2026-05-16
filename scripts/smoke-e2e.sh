#!/usr/bin/env bash
# Day 4 E2E smoke: compose health, topics, optional ingest (ingestion + consumer must be running).
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
"${COMPOSE[@]}" run --rm redpanda rpk topic list --brokers redpanda:9092 | tee /tmp/rpk-topics.txt
grep -q ai_inference_events /tmp/rpk-topics.txt
grep -q ai_inference_dlq /tmp/rpk-topics.txt

echo "==> Unit tests (no compose required for Go)"
cargo test -p ingestion
go test ./consumer/...

if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
  echo "==> Ingestion reachable — posting test event"
  ts_ms="$(python3 -c 'import time; print(int(time.time()*1000))')"
  resp="$(curl -sS -w '\n%{http_code}' -X POST http://localhost:8080/ingest \
    -H 'Content-Type: application/json' \
    -H 'X-Tenant-ID: demo' \
    -d "{\"events\":[{\"tenant_id\":\"demo\",\"model_id\":\"gpt-4o\",\"timestamp_unix_ms\":${ts_ms},\"latency_ms\":342,\"prompt_tokens\":512,\"completion_tokens\":128,\"cost_usd\":0.00423,\"status\":\"success\"}]}")"
  body="${resp%$'\n'*}"
  code="${resp##*$'\n'}"
  echo "HTTP $code — $body"
  if [[ "$code" != "202" ]]; then
    echo "WARN: expected 202 from /ingest (start: cargo run -p ingestion)" >&2
  else
    echo "==> Check Go consumer stdout for: cost_usd=0.00423 tenant_id=demo model_id=gpt-4o"
  fi
else
  echo "==> Skipping /ingest (start ingestion: cargo run -p ingestion)"
  echo "    Then in another terminal: go run ./consumer/cmd/consumer"
  echo "    Re-run this script or curl manually (see README)."
fi

echo "==> Prometheus metrics smoke"
if curl -sf http://localhost:8080/metrics | head -5; then
  echo "    ingestion /metrics OK on :8080"
else
  echo "    WARN: ingestion /metrics not reachable on :8080" >&2
fi

echo "==> Smoke complete"
