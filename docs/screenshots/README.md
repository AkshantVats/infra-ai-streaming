# Grafana screenshots

Captured from the Compose + host-binary path (`./scripts/run.sh --profile m1 --target compose`, then `go run` consumer + `cargo run -p ingestion`).

| File | Dashboard | URL |
|------|-----------|-----|
| `grafana-e2e-local.png` | AI Inference Observability — Local E2E | http://localhost:3000/d/ai-inference-e2e-local |
| `grafana-product-slo.png` | AI Inference — Product SLOs | http://localhost:3000/d/ai-inference-product |

## Access (local)

```bash
# 1) Dependencies (Redis, Redpanda, ClickHouse, Prometheus, Grafana)
./scripts/run.sh --profile m1 --target compose

# 2) Host apps (separate terminals)
set -a && source deploy/compose/values-m1.env && set +a
cd consumer && go run ./cmd/consumer
# other terminal: cargo run -p ingestion

# 3) Grafana UI
open http://localhost:3000   # admin / admin — click Skip on password-change prompt
```

k3d/Helm E2E does not ship Grafana in-cluster; use Compose for dashboard screenshots or port-forward Prometheus and run Grafana from Compose only.
