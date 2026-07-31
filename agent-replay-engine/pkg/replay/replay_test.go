// SPDX-License-Identifier: MIT
package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
	"github.com/akshantvats/agent-replay-engine/pkg/mocker"
)

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// sevenStepLog builds a recorded run with 7 tool calls and a final_output,
// mirroring the "stop before step 7" scenario from the Day 46 blog: a
// three-call log would be too small to exercise a mid-run stop.
func sevenStepLog(t *testing.T) eventlog.EventLog {
	t.Helper()
	var log eventlog.EventLog
	seq := int64(1)

	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindPrompt,
		Payload: rawJSON(t, map[string]any{"text": "roll out config v9"}),
	})
	seq++

	for i := 1; i <= 7; i++ {
		toolName := "deploy_shard"
		inputHash := "hash-shard-" + string(rune('0'+i))
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindToolCall,
			ToolName: toolName, InputHash: inputHash,
			Payload: rawJSON(t, map[string]any{"shard": i}),
		})
		seq++
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindToolResponse,
			ToolName: toolName, InputHash: inputHash,
			Payload: rawJSON(t, map[string]any{"shard": i, "status": "ok"}),
		})
		seq++
	}

	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: "trace-1", Kind: eventlog.KindFinalOutput,
		Payload: rawJSON(t, map[string]any{"text": "rolled out 7 shards"}),
	})

	return log
}

func TestRunToCompletion(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if result.Err != nil {
		t.Fatalf("Run: unexpected error: %v", result.Err)
	}
	if result.StoppedEarly {
		t.Fatalf("Run: StoppedEarly = true, want false for stopAtStep=0")
	}
	if result.StepsRun != 7 {
		t.Fatalf("StepsRun = %d, want 7", result.StepsRun)
	}
	if result.Output != "rolled out 7 shards" {
		t.Fatalf("Output = %q, want %q", result.Output, "rolled out 7 shards")
	}
	if len(result.CallHistory) != 7 {
		t.Fatalf("len(CallHistory) = %d, want 7", len(result.CallHistory))
	}
}

func TestRunStopsBeforeBlastRadius(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	// Stop before step 7 — the scenario from the Day 46 blog post.
	result := Run(log, m, 6)
	if result.Err != nil {
		t.Fatalf("Run: unexpected error: %v", result.Err)
	}
	if !result.StoppedEarly {
		t.Fatalf("StoppedEarly = false, want true for stopAtStep=6 on a 7-step log")
	}
	if result.StepsRun != 6 {
		t.Fatalf("StepsRun = %d, want 6", result.StepsRun)
	}
	if result.Output != "" {
		t.Fatalf("Output = %q, want empty — replay halted before final_output", result.Output)
	}
	if len(result.CallHistory) != 6 {
		t.Fatalf("len(CallHistory) = %d, want 6", len(result.CallHistory))
	}
}

func TestRunStopAtStepBeyondLogLengthRunsToCompletion(t *testing.T) {
	log := sevenStepLog(t)
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 100)
	if result.Err != nil {
		t.Fatalf("Run: unexpected error: %v", result.Err)
	}
	if result.StoppedEarly {
		t.Fatalf("StoppedEarly = true, want false when stopAtStep exceeds log length")
	}
	if result.StepsRun != 7 {
		t.Fatalf("StepsRun = %d, want 7", result.StepsRun)
	}
}

func TestRunUnknownToolCallReturnsError(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-1", Kind: eventlog.KindToolCall,
			ToolName: "deploy_shard", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"shard": 1})},
		// No matching tool_response recorded — LoadFromLog will skip this
		// pairing, so Respond must return mocker.ErrUnknownCall.
	}
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if result.Err == nil {
		t.Fatal("Run: expected error for unrecorded tool call, got nil")
	}
	if !errors.Is(result.Err, mocker.ErrUnknownCall) {
		t.Fatalf("Run: err = %v, want wrapping mocker.ErrUnknownCall", result.Err)
	}
	if result.StepsRun != 0 {
		t.Fatalf("StepsRun = %d, want 0", result.StepsRun)
	}
}

func TestRunMissingFinalOutputReturnsError(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-1", Kind: eventlog.KindToolCall,
			ToolName: "deploy_shard", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"shard": 1})},
		{SeqNum: 2, TraceID: "trace-1", Kind: eventlog.KindToolResponse,
			ToolName: "deploy_shard", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"status": "ok"})},
		// No final_output event.
	}
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if !errors.Is(result.Err, ErrNoFinalOutput) {
		t.Fatalf("Run: err = %v, want ErrNoFinalOutput", result.Err)
	}
	if result.StepsRun != 1 {
		t.Fatalf("StepsRun = %d, want 1", result.StepsRun)
	}
}

func TestRunEmptyTraceReturnsError(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-1", Kind: eventlog.KindPrompt,
			Payload: rawJSON(t, map[string]any{"text": "hello"})},
	}
	m, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	result := Run(log, m, 0)
	if !errors.Is(result.Err, ErrEmptyTrace) {
		t.Fatalf("Run: err = %v, want ErrEmptyTrace", result.Err)
	}
}

// nStepLog builds a recorded run with n tool calls and a final_output,
// generalizing sevenStepLog for streaming/perf tests that need a larger
// trace than the Day 46 blast-radius scenario calls for.
func nStepLog(t *testing.T, traceID string, n int) eventlog.EventLog {
	t.Helper()
	var log eventlog.EventLog
	seq := int64(1)

	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: traceID, Kind: eventlog.KindPrompt,
		Payload: rawJSON(t, map[string]any{"text": "replay this run"}),
	})
	seq++

	for i := 1; i <= n; i++ {
		toolName := "deploy_shard"
		inputHash := fmt.Sprintf("hash-shard-%d", i)
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: traceID, Kind: eventlog.KindToolCall,
			ToolName: toolName, InputHash: inputHash,
			Payload: rawJSON(t, map[string]any{"shard": i}),
		})
		seq++
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: traceID, Kind: eventlog.KindToolResponse,
			ToolName: toolName, InputHash: inputHash,
			Payload: rawJSON(t, map[string]any{"shard": i, "status": "ok"}),
		})
		seq++
	}

	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: traceID, Kind: eventlog.KindFinalOutput,
		Payload: rawJSON(t, map[string]any{"text": fmt.Sprintf("rolled out %d shards", n)}),
	})

	return log
}

// benchNStepLog is nStepLog's *testing.B-safe twin: benchmarks run outside
// the test framework's per-call setup, so helpers they use can't take a
// *testing.T (t.Fatalf relies on runtime.Goexit bookkeeping the testing
// package sets up around a real test, not around a bare struct literal).
func benchNStepLog(traceID string, n int) eventlog.EventLog {
	must := func(v any) json.RawMessage {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return b
	}

	var log eventlog.EventLog
	seq := int64(1)
	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: traceID, Kind: eventlog.KindPrompt,
		Payload: must(map[string]any{"text": "replay this run"}),
	})
	seq++
	for i := 1; i <= n; i++ {
		toolName := "deploy_shard"
		inputHash := fmt.Sprintf("hash-shard-%d", i)
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: traceID, Kind: eventlog.KindToolCall,
			ToolName: toolName, InputHash: inputHash,
			Payload: must(map[string]any{"shard": i}),
		})
		seq++
		log = append(log, eventlog.AgentEvent{
			SeqNum: seq, TraceID: traceID, Kind: eventlog.KindToolResponse,
			ToolName: toolName, InputHash: inputHash,
			Payload: must(map[string]any{"shard": i, "status": "ok"}),
		})
		seq++
	}
	log = append(log, eventlog.AgentEvent{
		SeqNum: seq, TraceID: traceID, Kind: eventlog.KindFinalOutput,
		Payload: must(map[string]any{"text": fmt.Sprintf("rolled out %d shards", n)}),
	})
	return log
}

func jsonlBytes(t *testing.T, log eventlog.EventLog) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	return buf.Bytes()
}

func TestRunFromReaderMatchesRunToCompletion(t *testing.T) {
	log := sevenStepLog(t)
	data := jsonlBytes(t, log)

	batchM, err := mocker.LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}
	want := Run(log, batchM, 0)

	streamM, sawAny, err := mocker.LoadFromReader(bytes.NewReader(data), "trace-1")
	if err != nil {
		t.Fatalf("mocker.LoadFromReader: %v", err)
	}
	if !sawAny {
		t.Fatal("sawAny = false, want true")
	}
	got, err := RunFromReader(bytes.NewReader(data), "trace-1", streamM, 0)
	if err != nil {
		t.Fatalf("RunFromReader: %v", err)
	}

	if got.Output != want.Output {
		t.Errorf("Output = %q, want %q", got.Output, want.Output)
	}
	if got.StepsRun != want.StepsRun {
		t.Errorf("StepsRun = %d, want %d", got.StepsRun, want.StepsRun)
	}
	if got.StoppedEarly != want.StoppedEarly {
		t.Errorf("StoppedEarly = %v, want %v", got.StoppedEarly, want.StoppedEarly)
	}
	if len(got.CallHistory) != len(want.CallHistory) {
		t.Errorf("len(CallHistory) = %d, want %d", len(got.CallHistory), len(want.CallHistory))
	}
}

func TestRunFromReaderMatchesRunStoppedEarly(t *testing.T) {
	log := sevenStepLog(t)
	data := jsonlBytes(t, log)

	streamM, _, err := mocker.LoadFromReader(bytes.NewReader(data), "trace-1")
	if err != nil {
		t.Fatalf("mocker.LoadFromReader: %v", err)
	}
	got, err := RunFromReader(bytes.NewReader(data), "trace-1", streamM, 6)
	if err != nil {
		t.Fatalf("RunFromReader: %v", err)
	}
	if !got.StoppedEarly {
		t.Fatal("StoppedEarly = false, want true for stopAtStep=6 on a 7-step trace")
	}
	if got.StepsRun != 6 {
		t.Fatalf("StepsRun = %d, want 6", got.StepsRun)
	}
	if got.Output != "" {
		t.Fatalf("Output = %q, want empty — replay halted before final_output", got.Output)
	}
}

func TestRunFromReaderEmptyTrace(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-1", Kind: eventlog.KindPrompt, Payload: rawJSON(t, map[string]any{"text": "hi"})},
	}
	data := jsonlBytes(t, log)

	m, sawAny, err := mocker.LoadFromReader(bytes.NewReader(data), "trace-1")
	if err != nil {
		t.Fatalf("mocker.LoadFromReader: %v", err)
	}
	if !sawAny {
		t.Fatal("sawAny = false, want true")
	}
	result, err := RunFromReader(bytes.NewReader(data), "trace-1", m, 0)
	if err != nil {
		t.Fatalf("RunFromReader: %v", err)
	}
	if !errors.Is(result.Err, ErrEmptyTrace) {
		t.Fatalf("result.Err = %v, want ErrEmptyTrace", result.Err)
	}
}

// trackingReader counts bytes actually pulled from the underlying reader,
// so a test can prove RunFromReader stopped reading r rather than merely
// stopped returning results after reading it all.
type trackingReader struct {
	r    io.Reader
	read int
}

func (t *trackingReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	t.read += n
	return n, err
}

func TestRunFromReaderStopsEarlyWithoutReadingWholeStream(t *testing.T) {
	// Large enough that the scanner's first chunk read can't plausibly
	// cover the whole file — proves stopping at step 1 skips reading the
	// remaining ~4,999 steps' worth of bytes, not just skips reporting them.
	const n = 5000
	log := nStepLog(t, "trace-1", n)
	data := jsonlBytes(t, log)

	tr := &trackingReader{r: bytes.NewReader(data)}
	m, sawAny, err := mocker.LoadFromReader(bytes.NewReader(data), "trace-1")
	if err != nil {
		t.Fatalf("mocker.LoadFromReader: %v", err)
	}
	if !sawAny {
		t.Fatal("sawAny = false, want true")
	}

	result, err := RunFromReader(tr, "trace-1", m, 1)
	if err != nil {
		t.Fatalf("RunFromReader: %v", err)
	}
	if !result.StoppedEarly {
		t.Fatal("StoppedEarly = false, want true for stopAtStep=1")
	}

	if tr.read >= len(data) {
		t.Fatalf("trackingReader consumed %d of %d bytes — RunFromReader read the whole stream instead of stopping early", tr.read, len(data))
	}
}

func TestRunFromReader100StepTraceUnderThreeSeconds(t *testing.T) {
	// Regression guard for the Day 49 perf target: a 100-step trace must
	// replay in under 3s. In-process replay is µs-scale, so this mostly
	// catches an accidental O(n²) regression (e.g. re-scanning the reader
	// per step) rather than genuine slowness.
	const n = 100
	log := nStepLog(t, "trace-1", n)
	data := jsonlBytes(t, log)

	m, _, err := mocker.LoadFromReader(bytes.NewReader(data), "trace-1")
	if err != nil {
		t.Fatalf("mocker.LoadFromReader: %v", err)
	}

	start := time.Now()
	result, err := RunFromReader(bytes.NewReader(data), "trace-1", m, 0)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunFromReader: %v", err)
	}
	if result.Err != nil {
		t.Fatalf("RunFromReader: result.Err = %v", result.Err)
	}
	if result.StepsRun != n {
		t.Fatalf("StepsRun = %d, want %d", result.StepsRun, n)
	}
	if elapsed >= 3*time.Second {
		t.Fatalf("replay of %d-step trace took %s, want < 3s", n, elapsed)
	}
}

// benchSharedLog builds a shared, multi-tenant-style log file — the
// production shape this benchmark cares about: one target trace plus
// noiseTraces other traces recorded in the same file, each noiseSteps
// long. A real deployment appends every run to one rolling log, so
// replaying trace-1 in production means finding it inside everyone
// else's traces too, not inside a file that only ever held trace-1.
func benchSharedLog(noiseTraces, noiseSteps, targetSteps int) []byte {
	var log eventlog.EventLog
	for i := 0; i < noiseTraces; i++ {
		log = append(log, benchNStepLog(fmt.Sprintf("noise-trace-%d", i), noiseSteps)...)
	}
	log = append(log, benchNStepLog("trace-1", targetSteps)...)

	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// BenchmarkRunBatchVsStream reports the allocation profile Run (ReadJSONL
// the whole shared log, then filter down to one trace) trades against
// RunFromReader (stream the shared log, only ever retaining the target
// trace's own events) — the numbers cited in the Day 49 blog post's
// "streaming parser memory profile" came from
// `go test -bench BenchmarkRunBatchVsStream -benchmem ./pkg/replay/`.
// The shared-log shape (50 other traces alongside the one being replayed)
// is what makes the memory difference show up: Run's ReadJSONL buffers
// every trace in the file before FilterByTraceID discards the other 50,
// so its peak memory scales with total file size. RunFromReader never
// buffers a line past reading whether it belongs to trace-1.
func BenchmarkRunBatchVsStream(b *testing.B) {
	const noiseTraces = 50
	const noiseSteps = 200
	const targetSteps = 100 // matches the Day 49 perf target trace length
	data := benchSharedLog(noiseTraces, noiseSteps, targetSteps)
	b.Logf("shared log: %d traces, %d bytes total, target trace has %d steps", noiseTraces+1, len(data), targetSteps)

	b.Run("batch/ReadJSONL+Run", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			full, err := eventlog.ReadJSONL(bytes.NewReader(data))
			if err != nil {
				b.Fatalf("ReadJSONL: %v", err)
			}
			scoped := full.FilterByTraceID("trace-1")
			m, err := mocker.LoadFromLog(scoped)
			if err != nil {
				b.Fatalf("LoadFromLog: %v", err)
			}
			if r := Run(scoped, m, 0); r.Err != nil {
				b.Fatalf("Run: %v", r.Err)
			}
		}
	})

	b.Run("stream/RunFromReader", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m, _, err := mocker.LoadFromReader(bytes.NewReader(data), "trace-1")
			if err != nil {
				b.Fatalf("mocker.LoadFromReader: %v", err)
			}
			r, err := RunFromReader(bytes.NewReader(data), "trace-1", m, 0)
			if err != nil {
				b.Fatalf("RunFromReader: %v", err)
			}
			if r.Err != nil {
				b.Fatalf("RunFromReader: %v", r.Err)
			}
		}
	})
}
