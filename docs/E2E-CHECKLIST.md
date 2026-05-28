# E2E checklist (M1-first)

Use this before claiming a green full-stack run on a MacBook (M1/M2/M3) or any laptop with ≤16 GB RAM.

## Preconditions

1. **Branch** is up to date with `main`.
2. **No compose + k3d together** — tear down Compose first:
   ```bash
   docker compose --env-file deploy/.env -f deploy/docker-compose.yml down
   ```
3. **Stale processes** — stop any prior `./scripts/run.sh` / `./scripts/e2e-k3d-full.sh`, stuck `helm upgrade`, or orphan k3d runs.
4. **Docker RAM** — allocate ≥8 GB to Docker Desktop.

## One command (default M1 matrix)

```bash
./scripts/run.sh --profile m1
```

Uses `deploy/helm/lensai/values-m1.yaml` by default.

| Override | Purpose |
|----------|---------|
| `--skip-chaos` | Smoke only, no chaos steps |
| `HELM_WAIT_TIMEOUT=2m` | Short Helm chart timeout (no global `--wait`) |
| `POD_WAIT_TIMEOUT=120s` | Per-workload `kubectl wait` |
| `CONTINUE_ON_FAIL=1` | Run all steps; summary still shows RED steps |
| `SKIP_DEPLOY=1` | Cluster already up; skip k3d recreate |
| `SKIP_PREFLIGHT=1` | Skip host unit tests |

## Never on M1

- **Do not** `helm upgrade` with bare `values.yaml` or `values-k3d.yaml` on a laptop — use **`values-m1.yaml`** or `./scripts/run.sh --profile m1`.
- **Do not** run full Compose stack and k3d at the same time (OOM risk).

## What the matrix covers

| Phase | Steps |
|-------|--------|
| A Preflight | `cargo test -p ingestion`, `go test ./...`, `helm template`, `bash -n` scripts |
| B Deploy | compose down → k3d up → helm (profile values) → wait pods + init jobs |
| C Cluster | smoke-k8s-e2e → chaos C1/C2 → load-m1 → HPA status (N/A on M1) |

Topics created by init job: `ai_inference_events`, `ai_inference_dlq`, `ai_anomalies`.

## Proof log

Append-only results: [`E2E-PROOF-K3D.md`](E2E-PROOF-K3D.md).

Production readiness cross-ref: [`PRODUCTION-READINESS-CHECKLIST.md`](PRODUCTION-READINESS-CHECKLIST.md).
