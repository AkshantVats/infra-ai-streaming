// SPDX-License-Identifier: MIT
// Package eventlog defines the AgentEvent log types used to record and
// replay AI agent runs deterministically. See DESIGN.md at the repo root
// for the full event model and storage format.
package eventlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// EventKind identifies the discrete step an AgentEvent captures.
type EventKind string

const (
	KindPrompt       EventKind = "prompt"        // initial user message
	KindModelTurn    EventKind = "model_turn"    // model response (text + tool calls issued)
	KindToolCall     EventKind = "tool_call"     // tool call issued by model
	KindToolResponse EventKind = "tool_response" // tool call result received
	KindFinalOutput  EventKind = "final_output"  // agent's terminal output
)

// ErrNotFound is returned by First when no event of the requested kind exists.
var ErrNotFound = errors.New("eventlog: no event of requested kind found")

// AgentEvent is one immutable, append-only entry in an agent run's event log.
type AgentEvent struct {
	SeqNum    int64           `json:"seq_num"`  // monotonic, 1-based
	SpanID    string          `json:"span_id"`  // matches tool-call-analyzer span_id
	TraceID   string          `json:"trace_id"` // groups events in one run
	Kind      EventKind       `json:"kind"`
	Timestamp int64           `json:"timestamp_ns"` // Unix nanoseconds (frozen on record)
	ToolName  string          `json:"tool_name,omitempty"`
	InputHash string          `json:"input_hash,omitempty"` // SHA-256 of tool call input JSON
	Payload   json.RawMessage `json:"payload"`              // kind-specific data (opaque bytes)
}

// EventLog is an ordered slice of AgentEvents sorted by SeqNum.
type EventLog []AgentEvent

// ReadJSONL reads an event log from a JSON Lines reader.
// Events are sorted by SeqNum before returning.
func ReadJSONL(r io.Reader) (EventLog, error) {
	var log EventLog

	scanner := bufio.NewScanner(r)
	// Payloads can be arbitrarily large API responses; grow the buffer well
	// past bufio's 64KiB default so a large tool response doesn't error out.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 16*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev AgentEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("eventlog: line %d: %w", lineNum, err)
		}
		log = append(log, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("eventlog: reading jsonl: %w", err)
	}

	sort.SliceStable(log, func(i, j int) bool {
		return log[i].SeqNum < log[j].SeqNum
	})

	return log, nil
}

// WriteJSONL writes the event log to w in JSON Lines format, one event per line.
func (log EventLog) WriteJSONL(w io.Writer) error {
	bw := bufio.NewWriter(w)
	for _, ev := range log {
		b, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("eventlog: marshal seq_num=%d: %w", ev.SeqNum, err)
		}
		if _, err := bw.Write(b); err != nil {
			return fmt.Errorf("eventlog: write seq_num=%d: %w", ev.SeqNum, err)
		}
		if err := bw.WriteByte('\n'); err != nil {
			return fmt.Errorf("eventlog: write newline seq_num=%d: %w", ev.SeqNum, err)
		}
	}
	return bw.Flush()
}

// First returns the first event of the given kind (lowest SeqNum), or ErrNotFound.
func (log EventLog) First(kind EventKind) (AgentEvent, error) {
	var found AgentEvent
	has := false
	for _, ev := range log {
		if ev.Kind != kind {
			continue
		}
		if !has || ev.SeqNum < found.SeqNum {
			found = ev
			has = true
		}
	}
	if !has {
		return AgentEvent{}, ErrNotFound
	}
	return found, nil
}

// AllOfKind returns all events of the given kind in SeqNum order.
func (log EventLog) AllOfKind(kind EventKind) []AgentEvent {
	var out []AgentEvent
	// log is expected to already be SeqNum-sorted (ReadJSONL guarantees this);
	// a defensive stable sort of the filtered subset keeps the contract even
	// if a caller builds an EventLog by hand out of order.
	for _, ev := range log {
		if ev.Kind == kind {
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].SeqNum < out[j].SeqNum
	})
	return out
}

// Validate checks ordering and uniqueness invariants.
// Returns an error if SeqNums are not strictly monotonic (in log order) or
// any SpanID is duplicated across events.
func (log EventLog) Validate() error {
	seen := make(map[string]int64, len(log)) // span_id -> seq_num first seen at
	var prevSeq int64
	first := true

	for _, ev := range log {
		if !first && ev.SeqNum <= prevSeq {
			return fmt.Errorf("eventlog: non-monotonic seq_num: %d follows %d", ev.SeqNum, prevSeq)
		}
		prevSeq = ev.SeqNum
		first = false

		if ev.SpanID == "" {
			continue
		}
		if prior, ok := seen[ev.SpanID]; ok {
			return fmt.Errorf("eventlog: duplicate span_id %q at seq_num %d (first seen at %d)", ev.SpanID, ev.SeqNum, prior)
		}
		seen[ev.SpanID] = ev.SeqNum
	}

	return nil
}
