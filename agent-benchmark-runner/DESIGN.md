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
```

## Acceptance Criteria

```bash
gofmt -l .           # empty
go vet ./...         # exits 0
go test -race ./...  # exits 0, 22 tests pass
```

## Series Navigation

Previous: Day 50 — agent-replay-engine: CI Smoke Test Against a Sample Bundle + On-Call Runbook
Next: Day 52 — TBD
