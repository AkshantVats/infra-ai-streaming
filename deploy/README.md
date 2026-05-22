# Deploy

Two paths: **Docker Compose** (fast local dev) and **k3d + Helm** (full stack in Kubernetes with lag-driven HPA).

## Docker Compose (primary dev loop)

[`docker-compose.yml`](docker-compose.yml) runs:

| Service | Host ports | Purpose |
|---------|------------|---------|
| **redis** | 6379 | Rate limiting (ingestion) |
| **redpanda** | 9092, 9644 | Event bus |
| **redpanda-init** | — (one-shot) | Topics `ai_inference_events`, `ai_inference_dlq` |
| **clickhouse** | 8123, 9000 | Analytical store |
| **clickhouse-init** | — (one-shot) | DDL from `clickhouse/init.sql` |
| **prometheus** | 9090 | Scrapes host `:8080` / `:9091` |
| **grafana** | 3000 | Dashboards (`admin` / `admin`) |

```bash
cp .env.example .env
docker compose --env-file .env -f docker-compose.yml up -d
```

From repo root:

```bash
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

E2E with host binaries: see root `README.md` or [`../scripts/smoke-e2e.sh`](../scripts/smoke-e2e.sh).

---

## k3d + Helm (Day 8 / G-07) — three commands

**Prerequisites:** Docker ≥ 8 GB RAM, `k3d`, `helm`, `kubectl`, Rust 1.86, Go 1.22+.

### 1) Cluster + images

```bash
./deploy/k3d/up.sh
```

Creates cluster `lensai`, builds `lensai/ingestion:local` and `lensai/consumer:local`, imports into k3d.

### 2) Helm install

```bash
helm dependency update deploy/helm/lensai
helm upgrade --install lensai deploy/helm/lensai \
  -n lensai --create-namespace \
  -f deploy/helm/lensai/values-k3d.yaml \
  --wait --timeout 10m
```

Stack: Redis, Redpanda, ClickHouse, **ingestion** (2 replicas, PDB, tenant ConfigMap), **consumer** (HPA on `kafka_consumer_lag_sum`), in-cluster Prometheus + prometheus-adapter.

### 3) Verify

```bash
./scripts/smoke-k8s-e2e.sh
```

Port-forward is used inside the smoke script. Optional HPA watch:

```bash
./scripts/demo-hpa-lag.sh
```

### k3d details

| Artifact | Purpose |
|----------|---------|
| [`k3d/cluster.yaml`](k3d/cluster.yaml) | Single-server cluster, LB ports 8080 / 9091 |
| [`helm/lensai/`](helm/lensai/) | Umbrella chart |
| [`docker/Dockerfile.ingestion`](docker/Dockerfile.ingestion) | Rust ingestion image |
| [`docker/Dockerfile.consumer`](docker/Dockerfile.consumer) | Go consumer image |
| [`tenant-limits.example.json`](tenant-limits.example.json) | Source for ConfigMap → `TENANT_LIMITS_PATH` |

**HPA:** Consumer Deployment scales on **external** metric `kafka_consumer_lag_sum` (Prometheus adapter over `kafka_consumer_lag_events`). Ingestion stays at 2 replicas with PDB `maxUnavailable: 1` — lag is owned by consumers, not CPU theater.

### Troubleshooting (k3d)

| Symptom | Check |
|---------|--------|
| Pods `ImagePullBackOff` | Run `./deploy/k3d/up.sh` (images must be imported; `pullPolicy: Never` in values-k3d) |
| OOM | Reduce Docker load or raise Docker memory to 8 GB+ |
| HPA `<unknown>` | `kubectl logs -n lensai -l app.kubernetes.io/name=prometheus-adapter`; verify `kubectl get --raw /apis/external.metrics.k8s.io/v1beta1` |
| Init job failed | `kubectl logs -n lensai job/lensai-redpanda-init` / `lensai-clickhouse-init` |
| CH empty | Wait for consumer; `kubectl logs -n lensai -l app.kubernetes.io/component=consumer` |

### Manual verify (no k3d in CI)

```bash
helm template lensai deploy/helm/lensai -f deploy/helm/lensai/values-k3d.yaml > /tmp/lensai.yaml
cargo test -p ingestion && (cd consumer && go test ./...)
```

---

## Prometheus scrape (Compose)

Ingestion metrics on **`HTTP_PORT` (8080)** at `/metrics`. Prometheus in compose scrapes `host.docker.internal:8080` and `:9091` for the consumer.

## Resource note

ClickHouse + Grafana need ~**8 GB** Docker RAM. Consumer-only: `docker compose … up -d redis redpanda redpanda-init`.

Environment variables: [`deploy/.env.example`](.env.example) and [`ingestion/src/config.rs`](../ingestion/src/config.rs). Do not commit `.env`.
