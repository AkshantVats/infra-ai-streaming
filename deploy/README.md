# Deploy

Two paths: **Docker Compose** (fast local dev) and **k3d + Helm** (full stack in Kubernetes with lag-driven HPA).

**Recommended entry point:** `./scripts/run.sh` — see profile table in root [README.md](../README.md).

---

## Configuration profiles

| Profile | Helm values | Compose env |
|---------|-------------|-------------|
| `m1` (default for E2E) | `helm/lensai/values-m1.yaml` | `compose/values-m1.env` |
| `dev` | `helm/lensai/values-dev.yaml` | `compose/values-dev.env` |
| `default` | `helm/lensai/values.yaml` | `compose/values-dev.env` |
| `k3d` | `helm/lensai/values-k3d.yaml` | — |
| custom | Copy `helm/lensai/values.example.yaml` → `values.mycluster.yaml` | Copy `deploy/.env.example` → `deploy/.env` |

```bash
./scripts/run.sh --profile m1                    # full k3d E2E
./scripts/run.sh --profile m1 --target compose   # Docker Compose only
./scripts/run.sh --values path/to/custom.yaml --target helm
```

---

## Docker Compose (primary dev loop)

[`docker-compose.yml`](docker-compose.yml) runs:

| Service | Host ports | Purpose |
|---------|------------|---------|
| **redis** | 6379 | Rate limiting (ingestion) |
| **redpanda** | 9092, 9644 | Event bus |
| **redpanda-init** | — (one-shot) | Topics `ai_inference_events`, `ai_inference_dlq`, `ai_anomalies` |
| **clickhouse** | 8123, 9000 | Analytical store |
| **clickhouse-init** | — (one-shot) | DDL from `clickhouse/init.sql` |
| **prometheus** | 9090 | Scrapes host `:8080` / `:9091` |
| **grafana** | 3000 | Dashboards (`admin` / `admin`) |

Classic workflow (without `run.sh`):

```bash
cp deploy/.env.example deploy/.env
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

E2E with host binaries: root `README.md` or [`../scripts/smoke-e2e.sh`](../scripts/smoke-e2e.sh).

---

## One-command E2E on M1

```bash
./scripts/run.sh --profile m1
# equivalent: HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh
```

Optional: `CONTINUE_ON_FAIL=1`, `SKIP_DEPLOY=1`, `--skip-chaos`. Proof log: [`docs/E2E-PROOF-K3D.md`](../docs/E2E-PROOF-K3D.md).

**Helm wait strategy:** the E2E script uses a short Helm timeout (default **2m**, no global `--wait`), then `kubectl wait` per critical workload (default **120s** each). On failure it prints `kubectl describe` and logs.

Override: `HELM_WAIT_TIMEOUT=2m` `POD_WAIT_TIMEOUT=120s`.

M1 limits: [`helm/lensai/values-m1.yaml`](helm/lensai/values-m1.yaml). If k3d OOMs, raise Docker memory or stop compose first.

---

## k3d + Helm — manual steps

For step-by-step debugging, use **`values-m1.yaml`** on laptops (not default `values.yaml`).

**Prerequisites:** Docker ≥ 8 GB RAM, `k3d`, `helm`, `kubectl`, Rust 1.86, Go 1.22+.

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml down
./deploy/k3d/up.sh
helm dependency update deploy/helm/lensai
HELM_WAIT_TIMEOUT=2m helm upgrade --install lensai deploy/helm/lensai \
  -n lensai --create-namespace \
  -f deploy/helm/lensai/values-m1.yaml \
  --timeout "${HELM_WAIT_TIMEOUT}" \
  --wait=false
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=lensai -n lensai --timeout=120s
./scripts/smoke-k8s-e2e.sh
```

**Full HPA demo (workstation only):** `values-k3d.yaml` with `--wait --timeout 10m` — not recommended on M1.

### k3d artifacts

| Artifact | Purpose |
|----------|---------|
| [`k3d/cluster.yaml`](k3d/cluster.yaml) | Single-server cluster, LB ports 8080 / 9091 |
| [`helm/lensai/`](helm/lensai/) | Umbrella chart + profile values |
| [`docker/Dockerfile.ingestion`](docker/Dockerfile.ingestion) | Rust ingestion image |
| [`docker/Dockerfile.consumer`](docker/Dockerfile.consumer) | Go consumer image |
| [`tenant-limits.example.json`](tenant-limits.example.json) | Source for ConfigMap → `TENANT_LIMITS_PATH` |

**HPA:** Consumer scales on external metric `kafka_consumer_lag_sum` (Prometheus adapter over `kafka_consumer_lag_events`).

### Troubleshooting (k3d)

| Symptom | Check |
|---------|--------|
| Pods `ImagePullBackOff` | Run `./deploy/k3d/up.sh` (`pullPolicy: Never` in M1/k3d values) |
| OOM | Reduce Docker load or raise Docker memory to 8 GB+ |
| HPA `<unknown>` | `kubectl logs -n lensai -l app.kubernetes.io/name=prometheus-adapter` |
| Init job failed | `kubectl logs -n lensai job/lensai-redpanda-init` / `lensai-clickhouse-init` |
| CH empty | Wait for consumer; check consumer pod logs |

### Manual verify (no k3d)

```bash
helm template lensai deploy/helm/lensai -f deploy/helm/lensai/values-m1.yaml > /tmp/lensai.yaml
cargo test -p ingestion && (cd consumer && go test ./...)
```

---

## Prometheus scrape (Compose)

Ingestion metrics on **`HTTP_PORT` (8080)** at `/metrics`. Prometheus in compose scrapes `host.docker.internal:8080` and `:9091` for the consumer.

## Resource note

ClickHouse + Grafana need ~**8 GB** Docker RAM. Consumer-only: `docker compose … up -d redis redpanda redpanda-init`.

Environment variables: [`deploy/.env.example`](.env.example) and [`ingestion/src/config.rs`](../ingestion/src/config.rs). Do not commit `.env`.
