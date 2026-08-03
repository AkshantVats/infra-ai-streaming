# Show HN: TraceForge — agent observability with money on the line (Rust + Go + ClickHouse)

## Title
Show HN: TraceForge — open-source agent observability suite (span ingest, replay, benchmark)

## Body

I've spent the last two months of a 150-day build shipping LensAI (LLM inference observability) and then hitting a follow-on problem: once you have agents making tool calls and taking multi-step actions, "did it work" stops being a single inference event and becomes a run — a sequence of steps, tool calls, and a pass/fail bar that only makes sense against a fixed task. LensAI's per-request view doesn't answer "is agent A better than agent B at this task," and it can't replay what actually happened when a run fails days later.

TraceForge is four components in one monorepo that answer that:

---

**What it is**

- **`traceforge/`** — span ingest HTTP service: PII scrub, sampling, OTLP export. The entry point every other component's data flows through.
- **`agent-replay-engine/`** — deterministic replay/diff of a recorded agent run from its event log, so a production failure becomes a local, re-runnable artifact instead of a Slack screenshot.
- **`agent-benchmark-runner/`** — runs a task YAML against one or two agents N times, grades each repetition against typed pass/fail criteria (not a vibe check), and for two agents produces a markdown + JSON + self-contained HTML divergence report.
- **`tool-call-analyzer/`** — builds a tool-call dependency graph from a run's spans and dual-writes cost/duration rollups onto LensAI's own ingest pipeline.

One `docker-compose.yml` at the repo root brings up all four plus the Redpanda/ClickHouse/OTel Collector infra they share:

```bash
docker compose up -d
docker compose --profile tools run --rm benchmark run --task <path> --agent-a-cmd '<cmd>' --agent-b-cmd '<cmd>'
docker compose --profile tools run --rm replay replay --log <path> --trace-id <id>
```

---

**Why I built it this way**

`agent-benchmark-runner`'s core bet is that "agent A feels better than agent B" isn't a benchmark result — it's an opinion. Fixing the task (prompt, seed, allowed tools) and the pass bar (a list of typed success criteria) as data means comparing two agents becomes a function of two recorded outcomes and one task file, not a transcript-reading exercise. The divergence report exists because a raw pass-rate number ("14/20 vs 9/20") doesn't tell you *where* the two agents diverged — the report's timeline diagram does.

The dual-write into LensAI's ingest pipeline (discriminated by a `source` field: `inference` for native LensAI events, `benchmark_run` for TraceForge batch completions) means a single ClickHouse table and a single Grafana dashboard answer both "what did this tenant's inference cost" and "what did benchmarking this tenant's agents cost" — no second warehouse, no join across services.

---

**Key design decisions**

1. **Raw per-repetition rows, not pre-aggregated batches.** `benchmark_runs` in ClickHouse stores one row per repetition. A pre-aggregated per-batch row would need recomputing every time a later repetition changes the median/P95; a raw row lets any consumer — this package's own `Summarize`, a Grafana panel, an ad-hoc `quantile(0.95)` query — compute its own statistic instead of trusting one baked in at write time.

2. **CostUSD stays zero on the benchmark_run dual-write.** The orchestrator's `Summary` carries no cost data, and the cost data that does exist (`pkg/report.CostReport`) is scoped to two-agent comparison output. Estimating a cost here would double-count against cost already tracked elsewhere — so the field is deliberately left at 0.0 rather than guessed.

3. **The CLI calls the orchestrator directly, not `go run` in a subprocess.** The launch-rehearsal integration test (added this week) drives the same `orchestrator.Run` → `compare.Compare` → `report.Build` wiring the real CLI does, in-process, so it's fast and hermetic instead of shelling out and parsing stdout.

4. **One `source` field, not a second table.** Adding `trace_id`/`source` to the shared `inference_events` schema — rather than standing up a `benchmark_events` table — means the existing Grafana dashboard infrastructure (ClickHouse datasource, time-range picker, tenant variable) works unmodified for TraceForge data on day one.

---

**Honest limitations**

- `agent-benchmark-runner`'s divergence report compares exactly two agents. N-way tournament comparison isn't implemented.
- The Grafana cross-product dashboard (`dashboards/traceforge-lensai-cross-product.json`) was validated against the ClickHouse schema and the Go/Rust wire format, not against a live Grafana instance with real traffic — no Grafana server was available in this build's sandbox.
- No Docker daemon was available to build/run the unified `docker-compose.yml` end-to-end for this launch; it's validated with `docker compose config` only (see Day 56's honest gap note in `DESIGN.md`).
- `agent-replay-engine` diffs recorded runs; it doesn't yet re-execute an agent live against a modified environment ("what if this tool call had failed instead").

---

**Repo**

https://github.com/akshantvats/infra-ai-streaming

Quickstart: `docker compose up -d`, then `docker compose --profile tools run --rm benchmark run --task agent-benchmark-runner/testdata/checkout-happy-path.yaml --agent-a-cmd '<cmd>' --agent-b-cmd '<cmd>'`. Dashboards in `dashboards/`, load test in `load-test/`.

Happy to answer questions about the benchmark grading model, the ClickHouse schema, or the LensAI dual-write.
