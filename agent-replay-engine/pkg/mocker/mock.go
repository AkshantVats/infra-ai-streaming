// SPDX-License-Identifier: MIT
// Package mocker serves frozen tool responses recorded in an eventlog.EventLog
// so an agent run can be replayed deterministically without reaching any live
// tool API. See DESIGN.md at the repo root for the mock tool contract.
package mocker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
)

// ErrUnknownCall is returned by Respond when no recorded response exists for
// the given tool name / input hash pair.
var ErrUnknownCall = errors.New("mocker: no recorded response for this tool call")

// ToolMocker serves frozen tool responses from a recorded event log.
type ToolMocker struct {
	mu        sync.Mutex
	responses map[string]json.RawMessage
	history   []string // composite keys in Respond call order
}

// LoadFromLog builds a ToolMocker from a recorded EventLog.
// It pairs each KindToolCall event with the KindToolResponse event that
// shares the same ToolName and InputHash, and stores the response payload
// under the composite key. Tool calls that never received a response are
// skipped — Respond will surface those as ErrUnknownCall at replay time.
func LoadFromLog(log eventlog.EventLog) (*ToolMocker, error) {
	m := &ToolMocker{
		responses: make(map[string]json.RawMessage),
	}

	// index responses by (tool_name, input_hash) so tool_call events can be
	// paired regardless of interleaving with other event kinds in the log.
	type callKey struct {
		toolName  string
		inputHash string
	}
	responseByCall := make(map[callKey]json.RawMessage)
	for _, ev := range log.AllOfKind(eventlog.KindToolResponse) {
		if ev.ToolName == "" || ev.InputHash == "" {
			continue
		}
		k := callKey{toolName: ev.ToolName, inputHash: ev.InputHash}
		// First response wins for a given call key — later duplicates (e.g. a
		// retried call re-recorded) don't silently overwrite the original.
		if _, exists := responseByCall[k]; !exists {
			responseByCall[k] = ev.Payload
		}
	}

	for _, ev := range log.AllOfKind(eventlog.KindToolCall) {
		if ev.ToolName == "" || ev.InputHash == "" {
			continue
		}
		k := callKey{toolName: ev.ToolName, inputHash: ev.InputHash}
		payload, ok := responseByCall[k]
		if !ok {
			continue
		}
		m.responses[compositeKey(ev.ToolName, ev.InputHash)] = payload
	}

	return m, nil
}

// LoadFromReader streams a recorded event log and builds a ToolMocker
// scoped to a single traceID, without ever holding the full log in
// memory — see LoadFromLog for the batch equivalent that takes an
// already-loaded eventlog.EventLog. Peak memory is bounded by traceID's
// own tool_call/tool_response events, not by how large the log file is,
// which is what makes replaying one trace out of a multi-GB shared log
// file practical.
//
// sawAny reports whether any event of any kind matched traceID, so a
// caller can distinguish an unknown trace_id (sawAny false) from a trace
// that exists but has, say, no completed tool calls yet.
//
// r is read once, forward-only: LoadFromReader pairs a tool_call with a
// tool_response regardless of which one appears first in the stream, but
// it cannot look ahead, so a tool_call whose tool_response never arrives
// (order-dependent within the trace, not just "missing") is silently
// left unpaired — Respond will surface it as ErrUnknownCall at replay
// time, identical to LoadFromLog's behavior for an unpaired call.
func LoadFromReader(r io.Reader, traceID string) (m *ToolMocker, sawAny bool, err error) {
	m = &ToolMocker{responses: make(map[string]json.RawMessage)}

	// callKey mirrors LoadFromLog's index key. Two maps replace that
	// function's two AllOfKind passes: responseByCall for responses seen
	// so far (first response wins, as in LoadFromLog), pending for calls
	// seen before their response arrived.
	type callKey struct {
		toolName  string
		inputHash string
	}
	responseByCall := make(map[callKey]json.RawMessage)
	pending := make(map[callKey]struct{})

	sc := eventlog.NewScanner(r)
	for sc.Scan() {
		ev := sc.Event()
		if ev.TraceID != traceID {
			continue
		}
		sawAny = true

		switch ev.Kind {
		case eventlog.KindToolResponse:
			if ev.ToolName == "" || ev.InputHash == "" {
				continue
			}
			k := callKey{toolName: ev.ToolName, inputHash: ev.InputHash}
			if _, exists := responseByCall[k]; !exists {
				responseByCall[k] = ev.Payload
			}
			if _, waiting := pending[k]; waiting {
				m.responses[compositeKey(ev.ToolName, ev.InputHash)] = responseByCall[k]
				delete(pending, k)
			}
		case eventlog.KindToolCall:
			if ev.ToolName == "" || ev.InputHash == "" {
				continue
			}
			k := callKey{toolName: ev.ToolName, inputHash: ev.InputHash}
			if payload, ok := responseByCall[k]; ok {
				m.responses[compositeKey(ev.ToolName, ev.InputHash)] = payload
			} else {
				pending[k] = struct{}{}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, false, fmt.Errorf("mocker: %w", err)
	}

	return m, sawAny, nil
}

// Respond returns the frozen response payload for a tool call.
// toolName and inputHash must match a recorded KindToolCall entry that also
// has a paired KindToolResponse. Returns ErrUnknownCall if not found.
// Records the composite key in CallHistory in call order.
func (m *ToolMocker) Respond(toolName, inputHash string) (json.RawMessage, error) {
	key := compositeKey(toolName, inputHash)

	m.mu.Lock()
	defer m.mu.Unlock()

	payload, ok := m.responses[key]
	if !ok {
		return nil, fmt.Errorf("%w: tool=%q input_hash=%q", ErrUnknownCall, toolName, inputHash)
	}
	m.history = append(m.history, key)
	return payload, nil
}

// CallHistory returns the composite keys of all Respond calls made so far,
// in call order. Used to assert the model issued the same tool calls in the
// same sequence as the original run.
func (m *ToolMocker) CallHistory() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]string, len(m.history))
	copy(out, m.history)
	return out
}

// compositeKey returns SHA-256(toolName + ":" + inputHash) as a hex string.
// Combining the tool name into the key prevents two different tools that
// happen to receive an identical input from colliding on the same frozen
// response.
func compositeKey(toolName, inputHash string) string {
	sum := sha256.Sum256([]byte(toolName + ":" + inputHash))
	return hex.EncodeToString(sum[:])
}
