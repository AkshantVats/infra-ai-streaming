# E2E proof — k3d full stack (M1)

Automated log from `./scripts/e2e-k3d-full.sh`.

## Test matrix — run `20260525T135429Z` (all GREEN)

| # | Command | Status | Runtime / notes |
|---|---------|--------|-----------------|
| 1 | `cargo test -p ingestion` | **GREEN** | 22 tests, ~0.04s |
| 2 | `cd consumer && go test ./...` | **GREEN** | cached OK |
| 3 | `bash -n scripts/*.sh chaos/*.sh` | **GREEN** | all scripts |
| 4 | `helm template … -f values-m1.yaml` | **GREEN** | renders cleanly |
| 5a | `HELM_WAIT_TIMEOUT=2m` deploy (prior run) | **GREEN** | `--wait=false` + per-pod `kubectl wait` |
| 5b | `smoke-k8s-e2e` | **GREEN** | ok |
| 5c | `chaos C1 kill-redpanda` | **GREEN** | ~168s (standalone timing) |
| 5c | `chaos C2 throttle-clickhouse` | **GREEN** | ~89s; breaker/overflow may warn, exit 0 |
| 5c | `chaos load-m1` (1000 events / 10s) | **GREEN** | ~15–18s |
| 5d | HPA status check | **GREEN** | no HPA on M1 (expected) |

### Chaos root cause (YELLOW → GREEN)

Bare bash `wait` after background curls also waited on **kubectl port-forward** jobs (never exit) → scripts hung until perl alarm (exit 142). Fixed with `disown` on port-forwards and `wait_pids` on curl PIDs only.

### Final `kubectl get pods -n lensai`

```
lensai-redis                 1/1 Running
lensai-redpanda-0            1/1 Running
lensai-clickhouse-0          1/1 Running
lensai-ingestion             1/1 Running
lensai-consumer              1/1 Running
lensai-prometheus            1/1 Running
lensai-redpanda-init         Completed
lensai-clickhouse-init       Completed
```

## Prior deploy fixes (commit 02743ae)

- Helm `dig` probe paths, Redpanda FQDN advertise, init jobs non-hook, M1 memory 1G.
