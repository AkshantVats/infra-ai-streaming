# Day 6 — local test before merge (no PR until user approves)

Branches: **infra** `feat/grafana-inference-slo-dashboard` · **Profile** `feat/day-6-blogs`

---

## infra-ai-streaming

### 0. Checkout

```bash
cd ~/Desktop/github/infra-ai-streaming
git fetch origin
git checkout feat/grafana-inference-slo-dashboard
git pull --ff-only origin feat/grafana-inference-slo-dashboard 2>/dev/null || true
```

### 1. Compose stack

```bash
cp -n deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps
```

Wait until Redis, Redpanda, ClickHouse, Prometheus, Grafana are **running/healthy** (init jobs `redpanda-init`, `clickhouse-init` may show **exited** — OK).

### 2. Consumer (terminal A)

```bash
cd ~/Desktop/github/infra-ai-streaming
set -a && source deploy/.env && set +a
export KAFKA_BROKERS=127.0.0.1:9092
cd consumer && go run ./cmd/consumer
```

### 3. Ingestion (terminal B)

```bash
cd ~/Desktop/github/infra-ai-streaming
set -a && source deploy/.env && set +a
cargo run -p ingestion
```

### 4. Ingest smoke (terminal C)

```bash
curl -sS -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d '{"events":[{"tenant_id":"demo","model_id":"gpt-4o","timestamp_unix_ms":1715000000000,"latency_ms":342,"prompt_tokens":512,"completion_tokens":128,"cost_usd":0.00423,"status":"success"}]}'
```

**Look for:** HTTP 2xx; consumer log shows handoff; optional:

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml exec -T clickhouse \
  clickhouse-client --query "SELECT count(), max(cost_usd) FROM infra_ai.inference_events WHERE tenant_id='demo'"
```

### 5. Grafana

Login: **admin** / **admin**

| Dashboard | URL |
|-----------|-----|
| Product SLOs | http://localhost:3000/d/ai-inference-product |
| Local E2E | http://localhost:3000/d/ai-inference-e2e-local |

**Look for (product):**

1. **Ingest throughput by tenant** — Prometheus; line for `demo` after curl loop
2. **P99 inference latency by model** — ClickHouse; needs rows in `inference_events`
3. **Cost per hour by tenant** — ClickHouse
4. **Kafka consumer lag** — Prometheus

**If panels 2–3 are empty:** restart Grafana after datasource fix (`jsonData.host: clickhouse` in `deploy/grafana/provisioning/datasources/datasources.yml`), or Connections → ClickHouse → Save & test. Re-run ingest + wait ~30s for consumer flush.

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml restart grafana
```

Optional full script (compose + tests; start consumer/ingestion first for CH rows):

```bash
./scripts/smoke-e2e.sh
```

Prometheus targets: http://localhost:9090/targets — `ingestion` and `consumer` should be **UP** while binaries run.

---

## Profile (Day 6 blogs)

### 0. Checkout

```bash
cd ~/Desktop/github/Profile
git fetch origin
git checkout feat/day-6-blogs
git pull --ff-only origin feat/day-6-blogs 2>/dev/null || true
```

### 1. Local preview

```bash
cd ~/Desktop/github/Profile
python3 -m http.server 8765
```

### 2. URLs to open

| Post | URL |
|------|-----|
| AI Learning Day 5 | http://localhost:8765/blog/series/ai-learning/day-5-sampling-deterministic-routing.html |
| Experience 5 | http://localhost:8765/blog/series/experience/cardinality-is-the-silent-killer-roaringbitmap-lessons.html |
| Series hub | http://localhost:8765/blog/index.html |
| Home (dynamic cards) | http://localhost:8765/index.html |

**Look for:**

- Sidebar kickers: **Day 5 of N** and **Experience 5 of N**
- Cross-links between the two Day 6 posts resolve on localhost
- Mermaid diagrams render
- Hero images load: `blog/assets/covers/day-5-sampling-deterministic-routing.png`, `blog/assets/covers/cardinality-is-the-silent-killer-roaringbitmap-lessons.png`
- OG assets exist: `blog/assets/og/day-5-sampling-deterministic-routing.png`, `blog/assets/og/cardinality-is-the-silent-killer-roaringbitmap-lessons.png`

**Note:** `series-index.json` lists Experience 5 before Experience 4 in the array (nav order); confirm both posts open cleanly.

---

## After local pass

Reply in chat with sign-off, e.g. **"approved — push and open PR"** (infra / Profile / both). Agents push and open PRs only after that phrase.
