# agent-replay-engine

> Deterministic replay of AI agent runs using append-only event logs and frozen mock tool responses.
> Part of the TraceForge observability suite.

**Status**: Active · Day 44 scaffold
**Owner**: AkshantVats
**Builds on**: tool-call-analyzer (event schema and `BillingEvent` envelope conventions from `pkg/dualwrite`)

---

## Problem Statement

An agent run is non-deterministic by default. The model issues tool calls; the tool calls hit live APIs; the APIs return data that changes over time. Running the same agent twice on the same prompt returns different outputs if any tool returns different data. This makes debugging impossible: a bug that appeared during a live run cannot be reproduced after the fact because the tool responses are gone.

Event sourcing solves this by recording every step of an agent run as an immutable, append-only log entry. To replay the run, you feed the model the same prompt and intercept every tool call — instead of forwarding it to the live API, you return the frozen response from the event log. The model receives the same inputs it did originally. The output is deterministic.

This mirrors a lesson from Wayfair price replay: replay mocks are idempotent consumers, fed frozen inputs so re-running never depends on what a live system returns *now*. Today's code implements the same principle for agent tool calls — freeze the response, replay against the freeze, never the live API.

---

## Event Model

Each entry in the event log is an `AgentEvent`. An `AgentEvent` captures one discrete step in the agent's execution:

```go
type EventKind string

const (
    KindPrompt        EventKind = "prompt"         // initial user message
    KindModelTurn     EventKind = "model_turn"     // model response (text + tool calls issued)
    KindToolCall      EventKind = "tool_call"      // tool call issued by model
    KindToolResponse  EventKind = "tool_response"  // tool call result received
    KindFinalOutput   EventKind = "final_output"   // agent's terminal output
)

type AgentEvent struct {
    SeqNum    int64             `json:"seq_num"`    // monotonic, 1-based
    SpanID    string            `json:"span_id"`    // matches tool-call-analyzer span_id
    TraceID   string            `json:"trace_id"`   // groups events in one run
    Kind      EventKind         `json:"kind"`
    Timestamp int64             `json:"timestamp_ns"` // Unix nanoseconds (frozen on record)
    ToolName  string            `json:"tool_name,omitempty"`
    InputHash string            `json:"input_hash,omitempty"` // SHA-256 of tool call input JSON
    Payload   json.RawMessage   `json:"payload"`    // kind-specific data (opaque bytes)
}
```

**Why `InputHash` not raw input**: the raw tool call input may contain secrets (API keys in headers, PII in arguments). The hash identifies the call for lookup purposes without storing the sensitive content. A separate vault (out of scope for Day 44) can store the full input if needed for debugging.

**Why `json.RawMessage` for Payload**: different event kinds have radically different payload shapes. `KindPrompt` payload is `{"text": "..."}`. `KindToolResponse` payload is the raw API response body. Using `json.RawMessage` avoids a discriminated union type and preserves exact bytes without re-serializing.

---

## Storage Format

The event log is a JSON Lines (`.jsonl`) file — one `AgentEvent` per line, in `seq_num` order.

```
{"seq_num":1,"span_id":"...","trace_id":"...","kind":"prompt","timestamp_ns":...,"payload":{"text":"..."}}
{"seq_num":2,"span_id":"...","trace_id":"...","kind":"model_turn","timestamp_ns":...,"payload":{"tool_calls":[...]}}
{"seq_num":3,"span_id":"...","trace_id":"...","kind":"tool_call","tool_name":"search_web","input_hash":"sha256:...","timestamp_ns":...,"payload":{...}}
{"seq_num":4,"span_id":"...","trace_id":"...","kind":"tool_response","tool_name":"search_web","input_hash":"sha256:...","timestamp_ns":...,"payload":{...}}
```

**Why JSON Lines**: one event per line means the log can be streamed, appended to without re-writing the file, and read by any tool that handles newline-delimited JSON (jq, grep, awk). Binary formats (protobuf, MessagePack) are faster but require schema files and tooling to inspect. For a debug-oriented system, human-readability beats throughput.

**Why not SQLite**: SQLite supports arbitrary queries, but the event log has one primary access pattern: sequential read from `seq_num = 1`. JSON Lines is optimal for this pattern and has zero dependency footprint.

---

## Mock Tool Architecture

The `ToolMocker` intercepts tool calls during replay and returns frozen responses:

```go
type ToolMocker struct {
    responses map[string]json.RawMessage // key: SHA-256(tool_name + ":" + input_hash)
    calls     []string                   // ordered call history for assertions
}

// LoadFromLog builds a ToolMocker from a recorded event log.
// It reads KindToolCall + KindToolResponse pairs in seq_num order.
// For each pair, it stores responses[hash(tool_name+input_hash)] = response_payload.
func LoadFromLog(log EventLog) (*ToolMocker, error)

// Respond looks up the frozen response for a tool call.
// Returns ErrUnknownCall if no matching record exists in the log.
// Returns the raw payload bytes if found — caller deserializes to expected type.
func (m *ToolMocker) Respond(toolName string, inputHash string) (json.RawMessage, error)

// CallHistory returns the list of hashes in the order Respond was called.
// Used in replay assertions to verify the model issued the same tool calls in the same order.
func (m *ToolMocker) CallHistory() []string
```

**Key design decision — composite key**: the lookup key is `SHA-256(tool_name + ":" + input_hash)` rather than just `input_hash`. Two different tools can receive the same input (e.g., both `search_web` and `search_news` receive `{"query": "kafka"}`) — the tool name in the key prevents a collision returning the wrong tool's response.

**Key design decision — `ErrUnknownCall`**: when the model issues a tool call that has no frozen response, the mocker returns a typed error rather than an empty response or a panic. The replay runner decides how to handle this: halt the replay (strict mode) or forward to the live API (lenient mode). Day 44 implements strict mode only.

---

## Determinism Rules

For a replay to be deterministic, three categories of state must be frozen:

**1. Tool responses** (always frozen in replay)
Every `KindToolResponse` payload is frozen at record time. The replay mocker serves these payloads verbatim. No tool call reaches a live API during replay.

**2. Timestamps** (frozen in the event log, not re-used by the model)
`timestamp_ns` in each event is the wall-clock time at record time. The replay runner does not inject these into the model's context — they are metadata for the observer (latency analysis, cost attribution) not inputs to the model. The model's behaviour must not depend on wall-clock time.

**3. Sequence** (enforced by seq_num ordering)
Tool calls must be replayed in the same order as recorded. If the model issues tool calls in a different order during replay, the mocker's call history diverges from the recorded history — detected by comparing `CallHistory()` to the recorded `KindToolCall` sequence. Order divergence is a replay failure, not a warning.

**What is NOT frozen**: the model weights. A replay does not guarantee the same model weights are in use. If the model has been updated between the record run and the replay, the model may issue different tool calls given the same responses. This is intentional — replay detects model-induced behaviour changes, which is a valid use case (regression testing prompts across model versions).

---

## Replay Algorithm

```
function replay(log: EventLog, mocker: ToolMocker, model: ModelClient) -> ReplayResult:
    prompt = log.first(KindPrompt).payload.text
    session = model.start_session(prompt)

    for step in session:
        if step is FinalOutput:
            recorded = log.first(KindFinalOutput).payload.text
            return ReplayResult{
                Output: step.text,
                Matches: step.text == recorded,
                CallHistory: mocker.CallHistory(),
            }

        if step is ToolCall:
            response, err = mocker.Respond(step.toolName, step.inputHash)
            if err == ErrUnknownCall:
                return ReplayResult{Error: fmt.Errorf("unknown tool call: %s %s", step.toolName, step.inputHash)}
            session.injectToolResponse(step.toolName, response)

    return ReplayResult{Error: errors.New("session ended without FinalOutput")}
```

The replay runner does not need to understand the model's internal reasoning — it only needs to intercept tool calls and inject frozen responses. This makes it model-agnostic: the same replay engine works for OpenAI function calling, Anthropic tool use, and any future tool call protocol.

---

## Diff Algorithm

`traceforge diff --log <path> --trace-a <id> --trace-b <id>` finds the first point where two
recorded traces disagree — the "first diverging span" a rider-ETA A/B test or a route-change
regression needs to localize before reading either trace end to end.

The comparison is **structural, not textual**: it walks each trace's `tool_call` events in
`seq_num` order and compares `ToolName` + `InputHash` at each step — the same composite key
`pkg/mocker` uses to serve frozen responses. Two steps "match" exactly when replay would serve
the same mocked response for both. This is deliberate: a textual diff of the raw JSON payloads
would flag every differing timestamp or request ID as a divergence even when the two traces made
the same tool call with the same effective input. `InputHash` is already the field that strips
that noise — `pkg/diff` diffs on it instead of re-deriving its own comparison key.

```
function compare(a: EventLog, b: EventLog) -> Result:
    callsA = a.tool_call_events_in_seq_order()
    callsB = b.tool_call_events_in_seq_order()

    for i in 0..min(len(callsA), len(callsB)):
        if callsA[i].tool_name != callsB[i].tool_name:
            return divergence(step=i+1, reason=TOOL_NAME, ...)
        if callsA[i].input_hash != callsB[i].input_hash:
            return divergence(step=i+1, reason=INPUT_HASH, ...)

    if len(callsA) != len(callsB):
        return divergence(step=min(len)+1, reason=MISSING_IN_SHORTER_TRACE, ...)

    return Result{no divergence}
```

**Why check `ToolName` before `InputHash`**: when both fields differ at the same step, a
different tool being called at all is the more fundamental divergence — reporting "different
tool" is more actionable than "different input," even though either alone would already prove
the traces disagree.

**Why a length mismatch (with every shared step matching) still counts as a divergence**: two
traces where the shorter one is a strict prefix of the longer one did not "agree" — one run took
an extra step the other never took. Reporting that as `Found() == false` would hide a real
behavioral difference (e.g. rider-a's route needed a reroute call rider-b's never triggered).

## Fault Injection

A recorded log only contains the response that actually happened at record time — almost always
success. That leaves an agent's error-handling path (a tool timeout, a 500 from the backing
service, a stale cache hit) untested unless the failure happened to occur live and get recorded,
which is rare and not something you can aim at a specific step on demand.

`pkg/fault` defines three injectable failure kinds — `timeout`, `http_500`, `stale_cache` — each
with a sentinel error (`fault.ErrTimeout`, `fault.ErrHTTP500`, `fault.ErrStaleCache`).
`pkg/mocker.ToolMocker.Inject(atStep int, kind fault.Kind)` configures the mocker to fail its
`atStep`'th `Respond` call (1-based, counting every call regardless of tool name) with that
fault's error instead of serving the step's recorded response — checked before the frozen-response
lookup, so injection overrides even a step with a perfectly good recording.

```
function Respond(toolName, inputHash) -> (payload, err):
    callCount += 1
    if inject is set and callCount == inject.atStep:
        return nil, inject.kind.Err()
    return lookup(toolName, inputHash)  // existing frozen-response path
```

**Why key by call number, not tool name**: every other addressing scheme in this package (diff's
`StepIndex`, replay's `--stop-at-step`) already addresses a run by its position in the tool-call
sequence. Keying injection the same way means "fail step 4" means the same thing whether you're
stopping before it, diffing around it, or now failing it — one mental model, not three.

**Why this needed no change to `pkg/replay.Run`**: `Run` already treats any error `Respond`
returns as a replay failure and wraps it with the step number (`replay: step %d: %w`) — that
existing generic handling covers `ErrUnknownCall` and an injected fault identically. Verifying an
agent's error path is exercising a code path that was already there for the messier "log has a
gap" case; fault injection just gives you a deliberate way to trigger it.

**Deliberate scope cut**: the CLI (`traceforge replay --inject-timeout <step>`) only exposes the
`timeout` kind this round. `http_500` and `stale_cache` are implemented in `pkg/fault` and
`ToolMocker.Inject` — a library caller can use either today — but wiring `--inject-http-500` and
`--inject-stale-cache` CLI flags is future scope, the same kind of cut Day 47 made on
`tool_response` payload diffing: land the mechanism generally, expose one concrete CLI path now.

## Streaming Replay

`Run` (Day 46) takes an already-loaded `eventlog.EventLog`: the CLI got there via `eventlog.ReadJSONL`, which reads the entire log file, sorts it by `seq_num`, and hands back one in-memory slice — then `FilterByTraceID` makes a second slice scoped to the trace being replayed. Fine for a single recorded run's log file. Wrong for the production shape: one shared, rolling log file holding every trace ever recorded, where replaying trace-1 means finding it inside everyone else's traces too. `ReadJSONL` buffers all of them to find one.

Day 49's target — "Replay perf: 100-step trace <3s; streaming parser memory profile" — is really a memory-bound problem wearing a perf-bound description: in-process replay of a 100-step trace is microsecond-scale regardless of how the log is read (`pkg/replay.TestRunFromReader100StepTraceUnderThreeSeconds` is a regression guard, not a real bottleneck). The actual risk is a laptop-scale `traceforge replay` OOMing against a multi-GB shared log file that happens to contain the one 100-step trace someone needs to debug.

**`eventlog.Scanner`** reads the log one JSON Lines record at a time, `bufio.Scanner`-style, and never buffers the file or sorts it — it trusts the recorder's append-only ordering guarantee instead. **`mocker.LoadFromReader`** streams a log through a `Scanner` and builds a `ToolMocker` scoped to one `trace_id` in a single pass, using an index no bigger than that trace's own tool calls (a `pending` map resolves a `tool_call`/`tool_response` pair regardless of which arrives first in the stream). **`replay.RunFromReader`** streams the tool-call sequence directly, the way `Run` walks an already-loaded slice.

```
function RunFromReader(r, traceID, mocker, stopAtStep):
    for event in Scanner(r):
        if event.trace_id != traceID: continue
        if event.kind == TOOL_CALL:
            if stopAtStep > 0 and steps >= stopAtStep:
                return Result{StoppedEarly: true, ...}   # stop reading r right here
            mocker.Respond(event.tool_name, event.input_hash)
            steps += 1
        if event.kind == FINAL_OUTPUT and finalPayload is unset:
            finalPayload = event.payload
    # reached EOF without stopping early
    return Result{Output: finalPayload.text, ...}
```

**The `traceforge replay` CLI now streams by default** (see `cmd/traceforge/main.go`), reading `--log` twice — once for `LoadFromReader`, once for `RunFromReader` — rather than once for `ReadJSONL`. Two sequential passes instead of one traded for never holding the whole file at once.

### What the benchmark actually shows

`BenchmarkRunBatchVsStream` (`pkg/replay/replay_test.go`) replays a 100-step target trace out of a shared log with 50 other 200-step traces mixed in (51 traces, ~3.7MB) — `go test ./pkg/replay/ -bench BenchmarkRunBatchVsStream -benchmem -run '^$'`:

| Path | Time/op | Memory/op | Allocs/op |
|---|---|---|---|
| `batch` (`ReadJSONL` + `Run`) | 104ms | 22.9MB | 223,627 |
| `stream` (`RunFromReader`) | 145ms | 18.4MB | 446,678 |

Streaming is **not** the strict win a "just stream it" instinct predicts. It uses ~20% less peak memory — the number that matters for not OOMing — but it's ~40% slower and allocates 2x more, because two single-pass scans each decoding every line into its own `AgentEvent` cost more than one `ReadJSONL` pass with slice growth followed by a filter. The honest tradeoff: streaming caps peak memory independent of how many *other* traces share the log file, at a real CPU and allocation cost, not a free upgrade.

Where streaming wins without qualification is `--stop-at-step` on a large file: it stops reading `r` the moment the step limit is hit, instead of reading to EOF first. `TestRunFromReaderStopsEarlyWithoutReadingWholeStream` proves it structurally (a `trackingReader` records exactly how many bytes were pulled from the underlying reader); measured against a 5,000-step / 1.78MB trace with `--stop-at-step 1`, `RunFromReader` reads 65,536 bytes before returning — 3.7% of the file, exactly one scanner buffer's worth, regardless of how large the remaining 4,999 steps are. `Run`'s old path read and sorted the whole 1.78MB first no matter where `--stop-at-step` cut it off.

## CI Smoke Test and the On-Call Runbook

Every package's unit tests run against in-memory `eventlog.EventLog` fixtures built by hand in
Go — that proves each package's logic is correct in isolation, but it never runs the actual
`traceforge` binary the way an on-call engineer would type it: as a compiled CLI, reading a real
file off disk, exiting with a real status code. A regression in flag parsing, file handling, or
how `main.go` wires the packages together could pass every package's tests and still break the
CLI a human runs at 3am.

`testdata/sample_run.jsonl` is a small, checked-in two-trace bundle (`trace-checkout-1001` /
`trace-checkout-1002`, sharing an identical `check_inventory` call and diverging at
`charge_payment`) built specifically to exercise every CLI path in one fixture: full replay,
`--stop-at-step`, `diff`, and `--inject-timeout`. `scripts/smoke_test.sh` builds the real binary
with `go build` and runs all four against it, asserting on actual stdout/stderr and exit codes —
CI runs this as its own step (`Go test (agent-replay-engine)` still covers the package-level
unit suite; the smoke test is additive, not a replacement).

The seven-step on-call runbook in the README exists for the same reason the smoke test does:
every debugging session so far (Day 46's `--stop-at-step`, Day 47's `diff`, Day 48's
`--inject-timeout`) added one more capability to this CLI, but nothing tied them together into
an order an exhausted on-call engineer can follow without re-deriving it. The runbook is that
fixed order, and step 7 — re-running `scripts/smoke_test.sh` against the same fixture CI checks
— is deliberately the same command in both places: what passes locally passes in CI, so a fix
isn't "done" until that one script says so.

## Scope for Day 44

Day 44 delivers the event log types (`pkg/eventlog`) and mock tool architecture (`pkg/mocker`). The replay runner and model client integration are Day 45+. The goal for Day 44 is a compilable, tested foundation that the replay runner can build on.

---

## Deviation from the 150-day plan

The plan names this repo `AkshantVats/agent-replay-engine` (new standalone repo). That repo is not accessible in this build environment's scoped GitHub access, so — following the precedent set by `tool-call-analyzer` (also originally planned as a standalone repo, Day 37) — this is implemented as a self-contained Go module living in the `agent-replay-engine/` subdirectory of `AkshantVats/infra-ai-streaming`, with its own `go.mod` (`module github.com/akshantvats/agent-replay-engine`) so it can be extracted to a standalone repo later with no import path changes beyond the module boundary.

---

## Series Navigation

Previous: Day 49 — agent-replay-engine: Streaming Replay — `eventlog.Scanner` + `RunFromReader`
Day 50 — agent-replay-engine: CI smoke test against a sample bundle + on-call runbook
Next: Day 51 — TBD
