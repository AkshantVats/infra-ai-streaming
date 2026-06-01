# E2E proof — k3d (summary)

Full command transcripts are **not** stored in git. They appear in GitHub Actions logs and the `e2e-k3d-proof-tail` artifact when a workflow fails.

## Test matrix (reference)

| Step | PR CI (`ci.yml` ? `e2e-k3d`) | Weekly (`e2e-k3d-dispatch.yml`) |
|------|------------------------------|----------------------------------|
| `cargo test -p ingestion` | via `run.sh` preflight | same |
| `go test ./...` | via `run.sh` preflight | same |
| `helm template` / deploy | k3d + Helm `values-m1` | same |
| `smoke-k8s-e2e.sh` | yes | yes (+ chaos unless `SKIP_CHAOS=1`) |
| Chaos C1/C2/load | no (PR job uses default `run.sh`) | yes on scheduled run |

**Last known-good (local M1):** 2026-05-27 · ~8 min wall · all matrix steps GREEN (chaos breaker/overflow may WARN on low RAM).

## Reproduce locally

```bash
./scripts/run.sh --profile m1
# lighter: ./scripts/run.sh --profile m1 --skip-chaos
```

Direct script (same deploy path): `HELM_WAIT_TIMEOUT=5m POD_WAIT_TIMEOUT=300s ./scripts/e2e-k3d-full.sh`

## Troubleshooting smoke flakes

`scripts/smoke-k8s-e2e.sh` retries port-forward warmup and HTTP checks (`SMOKE_CURL_RETRIES`, `SMOKE_CH_WAIT_SEC`). Curl exit **22** usually means `/health` or `/ingest` was hit before port-forward was ready — re-run or increase `SMOKE_PF_WARMUP_SEC`.
