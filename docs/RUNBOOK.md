# Runbook — infra-ai-streaming

Operational quick reference for the **Rust ingestion → Kafka (Redpanda) → Go consumer → ClickHouse** stack.

This document is intentionally **procedural**: symptoms → checks → actions. For deeper context, see:

- `docs/ARCHITECTURE.md` (evergreen summary)
- `docs/ARCHITECTURE-AND-FLOWS.md` (full walkthrough + troubleshooting matrix)
- `docs/E2E-CHECKLIST.md` and `docs/E2E-PROOF-K3D.md`
- `docs/DATA-RETENTION.md` — ClickHouse TTL, WAL PVC, Kafka retention

---

## Golden commands (grab these first)

- **Cluster state**:

```bash
kubectl get pods,svc,deploy,sts,job,hpa -n lensai -o wide
kubectl get events -n lensai --sort-by=.lastTimestamp | tail -n 30
```

- **Logs (current + previous)**:

```bash
kubectl logs -n lensai deploy/lensai-ingestion --tail=200
kubectl logs -n lensai deploy/lensai-consumer --tail=200
kubectl logs -n lensai deploy/lensai-ingestion --previous --tail=200 || true
kubectl logs -n lensai deploy/lensai-consumer --previous --tail=200 || true
```

- **Port-forward for quick checks**:

```bash
kubectl -n lensai port-forward svc/ingestion 8080:8080
kubectl -n lensai port-forward svc/consumer 9091:9091
curl -sf http://localhost:8080/health
curl -sf http://localhost:9091/metrics | head
```

---

## M1 “full E2E” (k3d) run

This is the canonical “works on laptop” flow.

```bash
HELM_WAIT_TIMEOUT=2m ./scripts/e2e-k3d-full.sh
```

Notes:

- Uses `deploy/helm/lensai/values-m1.yaml` (low-RAM safe defaults).
- Appends proof logs into `docs/E2E-PROOF-K3D.md`.
- If it fails, re-run with `CONTINUE_ON_FAIL=1` to collect more diagnostics in one pass.

---

## Common failures

### Redpanda OOM / CrashLoopBackOff

**Symptoms**

- `kubectl get pods -n lensai` shows `CrashLoopBackOff` for the Redpanda pod.
- Helm installs “hang” because infra pods never become Ready.

**Checks**

```bash
kubectl describe pod -n lensai -l app.kubernetes.io/component=redpanda | sed -n '1,200p'
kubectl logs -n lensai -l app.kubernetes.io/component=redpanda --tail=200
```

**Actions**

- On k3d/M1, prefer `values-m1.yaml` (already tuned).
- If still failing, raise `redpanda.resources.limits.memory` in `deploy/helm/lensai/values-m1.yaml`.
- Verify your k3d node has enough memory allocated (Docker Desktop settings).

---

### ClickHouse slow / inserts failing

**Symptoms**

- Consumer lag rises, breaker opens, overflow depth grows.
- `clickhouse_write_errors_total` increases.

**Checks**

```bash
kubectl logs -n lensai -l app.kubernetes.io/component=consumer --tail=200
kubectl logs -n lensai -l app.kubernetes.io/component=clickhouse --tail=200
kubectl exec -n lensai -it sts/lensai-clickhouse -- bash -lc 'clickhouse-client -q "SELECT 1"'
```

**Actions**

- Increase ClickHouse memory/CPU in `values-m1.yaml` (limits first, then requests).
- If disk is saturated, reduce load (fewer events) and confirm k3d storage is healthy.
- If the breaker is open, fixing ClickHouse should allow overflow drain to catch up.

---

### ImagePullBackOff (k3d)

**Symptoms**

- Ingestion/consumer pods stuck with `ImagePullBackOff`.

**Checks**

```bash
kubectl describe pod -n lensai -l app.kubernetes.io/component=ingestion | sed -n '1,200p'
kubectl describe pod -n lensai -l app.kubernetes.io/component=consumer | sed -n '1,200p'
```

**Actions**

- Rebuild and import images into k3d:

```bash
./deploy/k3d/up.sh
```

- Ensure `values-m1.yaml` sets `image.pullPolicy: Never` for ingestion/consumer (it should).

---

### Ingestion not Ready / probe failures

**Symptoms**

- Ingestion pod restarts; readiness fails; `/health` not responding.

**Checks**

```bash
kubectl logs -n lensai -l app.kubernetes.io/component=ingestion --tail=200
kubectl describe pod -n lensai -l app.kubernetes.io/component=ingestion | sed -n '1,220p'
```

**Actions**

- Confirm env wiring: `KAFKA_BROKERS`, `REDIS_URL`, `WAL_DIR`, `TENANT_LIMITS_PATH`.
- If WAL uses PVC, confirm it’s bound:

```bash
kubectl get pvc -n lensai
```

- If the pod is too slow to start on M1, increase probe `initialDelaySeconds` in `values-m1.yaml`.

---

### Consumer not Ready / cannot reach Kafka or ClickHouse

**Symptoms**

- Consumer pod Ready is false; logs show broker/CH connection errors.

**Checks**

```bash
kubectl logs -n lensai -l app.kubernetes.io/component=consumer --tail=200
kubectl exec -n lensai -it deploy/lensai-consumer -- sh -lc 'printenv | egrep "KAFKA|CLICKHOUSE|REDIS"'
```

**Actions**

- Validate DSNs in Helm values: `kafka.brokers`, `clickhouseDsn`, `redisUrl`.
- If ClickHouse is not ready, fix ClickHouse first; the breaker/overflow will cover brief outages but not a permanent down state.

---

## Data/persistence notes (k3d)

- Ingestion WAL PVC is enabled for M1 runs. Storage class behavior varies by k3d setup.
- If PVCs are Pending, ensure the default storage class exists:

```bash
kubectl get storageclass
kubectl describe pvc -n lensai
```

If your k3d cluster has no default storage class, set `ingestion.wal.persistence.storageClassName` in `values-m1.yaml` to a valid class.
