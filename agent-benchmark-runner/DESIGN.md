# agent-benchmark-runner

> Task-YAML-driven comparison of two agent runs against the same scenario, graded
> against explicit, machine-checkable success criteria.
> Part of the TraceForge observability suite.

## Problem Statement

"Agent A feels better than agent B" is not a benchmark result — it is an opinion formed
from whichever transcripts someone happened to read. Two agent configurations (a prompt
change, a model swap, a new tool) cannot be compared honestly unless they are run against
the *same* task: the same prompt, the same seed, the same allowed tools, and the same
pass/fail bar. Without that, a "better" result might just mean an easier task landed on
one side of the comparison.

`agent-benchmark-runner` fixes the scenario as data (a task YAML file) and fixes the bar
as data (a list of success criteria), so that comparing agent A to agent B is a function
of two recorded run outcomes and one task file — not a matter of judgment calls made
after the fact.

## Why the Seed Is Part of the Task, Not the Run

A tempting alternative design lets each agent run supply its own seed. Day 51 rejects
that: the seed lives on the `Task`, not on either agent's invocation. If agent A and
agent B could each pick their own seed, a divergence between their tool call sequences
would be ambiguous — did the agents actually behave differently, or did they just draw
different random tool orderings from different seeds? Pinning the seed to the task means
any observed difference is attributable to the agents under test, which is the entire
point of running the comparison.

## Task YAML

```yaml
task_id: checkout-happy-path
seed: 42
prompt: "Complete a checkout for cart ID 8842."
timeout_seconds: 30
tools_allowed:
  - check_inventory
  - charge_payment
success_criteria:
  - type: final_output_contains
    value: "order confirmed"
  - type: tool_call_sequence
    values: ["check_inventory", "charge_payment"]
  - type: max_tool_calls
    max: 5
```

| Field | Meaning |
|---|---|
| `task_id` | Stable identifier, used to correlate results across benchmark runs |
| `seed` | Fixed for both agents under comparison — see rationale above |
| `prompt` | The exact instruction given to both agents |
| `timeout_seconds` | Wall-clock budget per run; the runner (not this package) enforces it |
| `tools_allowed` | Advisory list of tools the scenario expects to be available |
| `success_criteria` | Ordered list of gradeable assertions — see below |

## Success Criteria — Design Decision

**Options considered:**

| Option | Description | Rejected because |
|---|---|---|
| Free-form assertion scripts (Python/Lua snippets in the YAML) | Maximum expressiveness | Turns the task file into an executable payload; unsafe to load from untrusted sources and hard to diff in review |
| A single "LLM judge" criterion that scores the whole transcript | Handles fuzzy, open-ended tasks well | Non-deterministic grading defeats the purpose of a *repeatable* benchmark; a re-run can pass or fail with no code change |
| A small closed set of typed criteria (`final_output_contains`, `final_output_exact`, `tool_call_sequence`, `max_tool_calls`) | Deterministic, diffable, safe to load as plain YAML | — (chosen) |

**Decision:** a closed set of typed criteria, each with `Value`, `Values`, or `Max`
fields depending on `Type` — no interpreted code in task files. `pkg/task.Criterion`
validates that the fields required by a given `Type` are present before a task is
considered runnable, so a malformed task fails fast at load time instead of silently
grading nothing.

**Consequences:** the criteria set is deliberately not exhaustive. Tasks that need fuzzy
grading (e.g. "is this summary accurate") are out of scope for Day 51 — a future day can
add a new `CriterionType` without changing the YAML shape or the grading contract, since
`Evaluate` already switches on `Type` and returns a typed `Result` either way.

## Grading a Single Run

`pkg/criteria.Evaluate` grades one `Criterion` against one `RunOutcome` (a final output
string plus a tool call sequence) and returns a `Result{Criterion, Passed, Detail}`. An
unrecognized `CriterionType` fails closed — it does not panic and does not silently pass
— so a typo in a task file shows up as a failing criterion with an explanatory `Detail`,
not a runner crash or a false positive.

`EvaluateAll` grades every criterion in a task against one outcome; `AllPassed` reduces
that to a single pass/fail bit for the run.

## Comparing Two Agents

`pkg/compare.Compare` takes a `Task` and two `AgentRun` values (an agent name plus a
`RunOutcome` each) and returns a `Result` carrying:

- Each agent's full criterion-by-criterion grade (`ResultsA` / `ResultsB`) and overall
  pass/fail (`PassedA` / `PassedB`) — graded independently, since it is expected and
  unremarkable for one agent to pass while the other fails.
- A `Divergence`: the first index at which the two agents' tool call sequences disagree,
  either because they called a different tool at the same step or because one sequence
  ended before the other. `nil` means the sequences matched all the way through.

**Why report divergence separately from pass/fail:** two agents can both pass every
success criterion while taking different paths to get there (e.g. checking inventory
twice vs. once), and two agents can both fail for unrelated reasons with no meaningful
tool-call overlap at all. Divergence answers "where did their behavior first differ,"
which is a different question from "did each one pass," and conflating the two would
lose information a benchmark report needs.

## Scope for Day 51

Day 51 delivers the task specification (`pkg/task`), single-run grading (`pkg/criteria`),
and two-agent comparison (`pkg/compare`) as a self-contained module with no runtime agent
invocation yet — `RunOutcome` is supplied directly by the caller (a human, a test, or a
future runner) rather than produced by executing a live agent. Wiring an actual agent
process up to produce a `RunOutcome`, and a CLI to drive the whole comparison from two
task YAML files, is out of scope for Day 51.

## Running a Task N Times (Day 52)

A single run against a Task is an anecdote, not a benchmark. LLM agents are
non-deterministic in practice even holding the task, seed, and prompt fixed — tool
latency jitter, retries, and provider-side sampling all inject variance a single run
can't separate from real behavior. `pkg/orchestrator.Run` executes a Task `Repetitions`
times against one agent and returns one `RunResult` per repetition; `Summarize` turns
that batch into a distribution (pass rate with a confidence interval, median and P95 of
tool-call step count) instead of one pass/fail.

### Why Bounded Parallelism

An agent run makes real outbound calls — to a model provider, to tools. Firing all N
repetitions at once turns a benchmark into a self-inflicted burst that can trip the very
rate limits the agent under test would hit in production: an unrepresentative failure
mode, not a signal about the agent. `Config.MaxParallel` bounds concurrency the same way
a load-testing tool bounds virtual users — it names its own concurrency instead of
borrowing it as a side effect of a loop. `Run` enforces the bound with a fixed-size
semaphore channel, not an unbounded goroutine-per-repetition fan-out.

**Options considered:**

| Option | Rejected because |
|---|---|
| Unbounded fan-out (one goroutine per repetition) | Fastest wall-clock, but the caller becomes the rate-limit violator — the exact failure this package exists to avoid confusing with agent behavior |
| Serial, one repetition at a time | Safe, but a 30-run batch takes 30x as long for no benefit once the downstream capacity is known |
| Fixed-size worker pool (chosen) | Bounded and predictable — the concurrency budget is a parameter instead of an accident |
| Adaptive/AIMD-style dynamic concurrency | The textbook "better" answer, but out of scope: a fixed pool sized from known downstream capacity solves the actual problem without a feedback controller tuning itself against a benchmark run |

### Why the Seed Is Derived, Not Shared, Per Repetition

Day 51 pinned the seed to the *task* so two agents being compared see the same
randomness. Day 52 runs N repetitions of *one* task; reusing that same seed N times would
make every repetition identical, and a median or P95 computed over N identical numbers is
not evidence of anything. `deriveSeed(base, i)` hashes `(base, repetitionIndex)` to give
each repetition its own reproducible draw, so a full N-run batch still reproduces
byte-for-byte from a single recorded base seed. A simple `base + i` offset was rejected:
several common PRNGs correlate poorly across seeds that differ by a small constant, which
would bias exactly the run-to-run variance this package exists to measure honestly.

### Summarizing N Runs

`Summarize` reports a pass rate with a 95% **Wilson score interval**, not the naive
normal (Wald) approximation. Wald produces nonsensical bounds — below 0, above 1, or a
zero-width interval at `k=0` or `k=n` — at exactly the small sample sizes (10–30 runs) a
benchmark batch typically has; Wilson stays within `[0, 1]` and has positive width even
at the extremes. Median and P95 step count are computed by linear interpolation between
closest ranks over repetitions whose `agentFn` call completed — a repetition whose run
errored (timeout, transport failure) contributes no statistical signal about the agent's
*behavior*, so it counts toward `N` but is excluded from `Completed`, `PassRate`, and the
step-count percentiles.

### Persistence Is an Injected Writer

`pkg/orchestrator` has no ClickHouse dependency, matching Day 51's no-I/O discipline for
`pkg/task`/`pkg/criteria`/`pkg/compare` — `Run` returns `[]RunResult` and a caller decides
whether and where to persist it. `pkg/store` sits one layer up: `Writer` is an interface
(`WriteRuns(ctx, []RunRecord) error`), and `ClickHouseWriter` is the only implementation
today, over `github.com/ClickHouse/clickhouse-go/v2`. This is also why `orchestrator`'s
tests need no database, and why `store`'s ClickHouse-touching tests live behind a
`//go:build integration` tag (skipped without `CLICKHOUSE_DSN` set) — the same pattern
`consumer/internal/clickhouse` already uses.

`benchmark_runs` (`pkg/store/schema/001_benchmark_runs.sql`) stores one row per
repetition, not one row per batch: a pre-aggregated per-batch row would need
recomputing every time a later repetition's data shifts the median or P95, while a raw
per-repetition row lets any consumer — `Summarize`, a Grafana panel, an ad-hoc
`SELECT quantile(0.95)(step_count)` — compute its own statistic from source rows instead
of trusting a stale pre-aggregated one.

## Scope for Day 52

`pkg/orchestrator` and `pkg/store` still take `AgentFunc`/`RunOutcome` as caller-supplied
inputs — wiring an actual agent process invocation remains out of scope, as it was for
Day 51. A CLI that reads a task YAML, drives N repetitions against a real agent process,
and writes the results to ClickHouse end-to-end is a natural Day 53+ candidate, not part
of Day 52.

## Generating a Report (Day 53)

Day 51 produces a `compare.Result`; Day 52 produces a `Summary` over N repetitions.
Neither is something a reviewer reads directly — `compare.Result` is a Go struct, and a
benchmark result only earns attention if a reader can tell in five words whether
something is wrong. `pkg/report.Build` reshapes a `compare.Result` plus the two agents'
raw tool call sequences (deliberately not carried on `compare.Result` itself — see
"Comparing Two Agents" above) into a `Report`, and three render functions turn that into
the three shapes three different audiences actually need: `RenderMarkdown` for a PR
description or a Slack message, `RenderJSON` for a dashboard or an alerting rule, and
`RenderTimelineSVG` for a side-by-side visual of where two runs' tool calls parted ways.

### Why the Headline Leads With Divergence, Not Pass/Fail

`Report.Headline` reads `"14 calls vs 9, diverged at step 5"` — call counts and the
divergence point, not `PASS`/`FAIL`. A pass/fail badge only answers "did it pass," which
Day 51 already reports per agent; it says nothing about *why* two runs differ, and a
report that opens with two independent pass/fail badges makes a reader hunt through the
rest of the document to find where the runs actually split. Leading with the call-count
comparison and the divergence step answers the question a reviewer opens the report to
ask — "where did these two runs stop agreeing" — before they read a single criterion
result.

### Why a Hand-Rolled SVG, Not a Charting Dependency

`RenderTimelineSVG` writes raw `<svg>`/`<rect>`/`<text>` elements with `fmt.Fprintf`
instead of pulling in a charting or graphing library. The visual is two rows of fixed-size
boxes — one per tool call, colored by whether that step matched, diverged, or came after
the divergence point — which a few dozen lines of string formatting renders exactly as
well as a general-purpose charting library would, without adding a dependency (and its
own transitive tree) for a fixed, narrow shape of chart. A tool call sequence that ends
before the other's is drawn as a dashed empty slot at that step rather than simply
omitted, so a shorter run visually reads as "stopped here" instead of leaving an
unexplained gap in the timeline.

### Scope for Day 53

`pkg/report` renders a single `compare.Result`, not a `Summary` batch — a Day 52
orchestrator run producing N repetitions and picking a representative pair (or an
aggregate divergence view across all N) to hand to `report.Build` is a natural next step,
not part of Day 53. There is still no CLI wiring a task YAML, a live agent process, and
a rendered report together end-to-end — each of `pkg/task`, `pkg/orchestrator`, and now
`pkg/report` remains a library a future runner composes, not a runnable binary itself.

## Flame Graph Timeline — Colored by Cost (Day 54)

`RenderTimelineSVG` (Day 53) answers "where did two agents' sequences stop agreeing" —
every box is the same fixed width, and color only ever means matched, diverged, or beyond
the divergence point. It has no notion of cost, because divergence and cost are
orthogonal questions: two runs can agree on every tool call and still differ wildly in
what those calls cost. Day 54 adds `pkg/report/flame.go`, a second SVG renderer that
answers the cost question the same way a CPU flame graph answers "which function ate the
time" — width proportional to cost, color on a heat gradient, the single most expensive
call marked.

### Why `CostReport` Wraps `Report`

`CostReport` embeds `Report` and adds `CostsA`/`CostsB []float64` rather than adding those
two fields to `Report` itself. Every caller that builds a `Report` from a `compare.Result`
today — `RenderMarkdown`, `RenderJSON`, `RenderTimelineSVG`, and their tests — has no cost
data and no reason to acquire any just because a flame graph renderer now exists elsewhere
in the package. A wrapper type that a caller opts into only when it has cost data keeps
`Report`'s existing shape (and its JSON wire format) exactly as Day 53 shipped it.

### Width Is Linear in Cost, Clamped to a Floor

`widthForCost` scales linearly from a floor (`flameMinWidth`) to a ceiling
(`flameMaxWidth`), proportional to `cost / maxCost` where `maxCost` is the single most
expensive call across both rows. Linear, not logarithmic: a real flame graph's entire
value is that box width *is* the measured quantity without a mental unit conversion, and
a log scale would make a 10x-more-expensive call look only modestly wider — exactly the
kind of understatement "wide bars are where budget died" is trying to avoid. The floor
exists so a zero-cost call still renders as a visible, clickable box instead of collapsing
to a zero-width sliver that reads as a rendering bug rather than "this one was free."

### Rows Are Independent Strips, Not Aligned Columns

Day 53's timeline aligns agent A and agent B by tool-call index, because it exists to
answer "at which step did they diverge" — a question that only makes sense column by
column. A flame graph's question is "where did the money go" *within* one run, and boxes
in a real flame graph sit contiguously next to their neighbors, not aligned to some
external index. Forcing agent A's and agent B's cost-weighted boxes into shared columns
would make two calls at the same step index render at different widths for reasons that
have nothing to do with when they ran — `RenderFlameGraphSVG` instead lays each row out
independently, one box immediately after the previous one, exactly like a CPU flame graph
stacks frames.

### One Peak Marker per Row, Not a Top-N List

Each row's single most expensive call gets a heavier stroke; nothing else is annotated. A
threshold-based "flag everything over $X" scheme would need a threshold, and different
tasks have wildly different cost distributions — there's no dollar figure that's "high"
across every benchmark. The widest, hottest box already answers "where did the budget
die" at a glance; a second annotation layer on top of that would be re-deriving a ranking
the visual already encodes.

### The Honest Gap

`agent-benchmark-runner` has no live per-call cost telemetry — `BuildCostReport` takes
`costsA`/`costsB` as explicit parameters, the same shape Day 53's `Build` used for tool
call sequences it also doesn't have another source for. A future runner that instruments
real per-tool-call dollar cost supplies it here; today's tests supply fixture costs.

## File Layout

```
DESIGN.md                          (NEW — Day 51)
README.md                          (NEW — Day 51)
go.mod / go.sum                    (NEW — Day 51: module github.com/akshantvats/agent-benchmark-runner)
testdata/
  checkout-happy-path.yaml         (NEW — Day 51: sample task fixture)
pkg/
  task/
    task.go                        (NEW — Day 51: Task, Criterion, LoadYAML/LoadFile, Validate)
    task_test.go                   (NEW — Day 51: 7 tests)
  criteria/
    criteria.go                    (NEW — Day 51: RunOutcome, Result, Evaluate/EvaluateAll/AllPassed)
    criteria_test.go               (NEW — Day 51: 9 tests)
  compare/
    compare.go                     (NEW — Day 51: AgentRun, Divergence, Compare)
    compare_test.go                (NEW — Day 51: 6 tests)
  orchestrator/
    orchestrator.go                (NEW — Day 52: AgentFunc, Config, RunResult, Run, deriveSeed)
    orchestrator_test.go           (NEW — Day 52: bounded parallelism, seed derivation, partial failure, config validation, cancellation)
    summary.go                     (NEW — Day 52: Summary, Summarize, Wilson interval, percentile)
    summary_test.go                (NEW — Day 52: Wilson interval fixtures, median/P95 fixtures)
  store/
    writer.go                      (NEW — Day 52: RunRecord, NewRunRecords, Writer, ClickHouseWriter)
    writer_test.go                 (NEW — Day 52: RunResult -> RunRecord mapping, no ClickHouse dependency)
    integration_test.go            (NEW — Day 52: //go:build integration, skips without CLICKHOUSE_DSN)
    schema/
      001_benchmark_runs.sql       (NEW — Day 52: benchmark_runs DDL)
      002_apply.sh                 (NEW — Day 52: applies schema files in order)
  report/
    report.go                      (NEW — Day 53: Report, Build, headline, RenderMarkdown, RenderJSON)
    report_test.go                 (NEW — Day 53: 7 tests)
    timeline.go                    (NEW — Day 53: RenderTimelineSVG, truncateLabel)
    timeline_test.go               (NEW — Day 53: 6 tests)
    flame.go                       (NEW — Day 54: CostReport, BuildCostReport, RenderFlameGraphSVG)
    flame_test.go                  (NEW — Day 54: 10 tests)
```

## Acceptance Criteria

```bash
gofmt -l .           # empty
go vet ./...         # exits 0
go test -race ./...  # exits 0
```

## Series Navigation

Previous: Day 53 — agent-benchmark-runner: Report Generator — Markdown, JSON, SVG Timeline
Next: Day 55 — agent-benchmark-runner: LensAI Integration — Benchmark Completion Emits Ingest Events
