# Documentation images

## Architecture diagram (optional)

**Filename:** `architecture.png`

Capture from the mermaid diagram in the root [README](../../README.md) (export via your editor or [mermaid.live](https://mermaid.live)), or screenshot the **Local E2E** Grafana dashboard with ingest + consumer running.

Until committed, the README keeps an HTML comment placeholder so links stay valid.

## Grafana E2E screenshot

**Filename:** `grafana-e2e.png`

After `./scripts/run.sh --profile m1` or compose + smoke:

1. Open http://localhost:3000 (`admin` / `admin`).
2. Dashboard **AI Inference — Local E2E** (or Product SLO with traffic).
3. Export PNG and commit here; uncomment the image line in README if added.

See also [`../screenshots/README.md`](../screenshots/README.md) for panel-level captures.
