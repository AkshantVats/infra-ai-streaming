# E2E proof — k3d full stack (M1)

Automated log from `./scripts/e2e-k3d-full.sh`. Each section is one run.

## Test matrix (latest run 20260525T122506Z deploy + 20260525T132252Z cluster tests)

| # | Command | Status | Notes |
|---|---------|--------|-------|
| 1 | `cargo test -p ingestion` | **GREEN** | 22 tests passed |
| 2 | `cd consumer && go test ./...` | **GREEN** | cached OK |
| 3 | `bash -n scripts/*.sh chaos/*.sh` | **GREEN** | all scripts |
| 4 | `helm template … -f values-m1.yaml` | **GREEN** | after `dig` probe fix |
| 5 | `HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh` | **GREEN** deploy / **YELLOW** chaos | Helm `--wait=false`; `kubectl wait` per workload; smoke **GREEN**; chaos hit 300s/180s/120s alarms (still investigating hang in `kill-redpanda`) |

### Final `kubectl get pods -n lensai` (after deploy fix)

```
lensai-redis          1/1 Running
lensai-redpanda-0     1/1 Running
lensai-clickhouse-0   1/1 Running   (0/0 if chaos C2 scaled CH down)
lensai-ingestion      1/1 Running
lensai-consumer       1/1 Running
lensai-prometheus     1/1 Running
*-init jobs           Completed
```

### Root causes fixed

1. **Helm `dig` probe paths** — wrong key order broke `helm template` / install.
2. **Redpanda DNS** — bare hostname `redpanda` does not resolve on StatefulSet pods; use `$(hostname -f)` for RPC/Kafka advertise.
3. **Helm hook wait** — post-install init jobs blocked `helm upgrade` for 2m; hooks removed, init jobs are regular Jobs, e2e waits with `kubectl wait job/...`.
4. **Init job polling** — 120s loop started after infra ready but jobs began at install; fixed with per-job `kubectl wait` (180s).
5. **M1 memory** — Redpanda `1G` `--memory`, 1Gi request / 1.5Gi limit.

See appended run logs below for full command output.
