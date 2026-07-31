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
	"sync"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
	"github.com/akshantvats/agent-replay-engine/pkg/fault"
)

// ErrUnknownCall is returned by Respond when no recorded response exists for
// the given tool name / input hash pair.
var ErrUnknownCall = errors.New("mocker: no recorded response for this tool call")

// injection configures a single synthetic failure to fire on a chosen
// Respond call, keyed by call number rather than tool name — a fault is
// injected at a point in the call sequence, the same way a recorded run's
// tool calls are addressed by step index everywhere else in this package.
type injection struct {
	atStep int // 1-based Respond call number the fault fires on
	kind   fault.Kind
}

// ToolMocker serves frozen tool responses from a recorded event log.
type ToolMocker struct {
	mu        sync.Mutex
	responses map[string]json.RawMessage
	history   []string // composite keys in Respond call order
	callCount int
	inject    *injection
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

// Inject configures Respond to fail its atStep'th call (1-based, counting
// every call regardless of tool name) with kind's fault instead of serving
// the normal frozen response. atStep <= 0 clears any previously configured
// injection. Call this before the replay run whose error path you want to
// verify — a recorded log only ever contains the response that actually
// happened, usually success, so this is the only way to force a step to
// fail during replay.
func (m *ToolMocker) Inject(atStep int, kind fault.Kind) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if atStep <= 0 {
		m.inject = nil
		return
	}
	m.inject = &injection{atStep: atStep, kind: kind}
}

// Respond returns the frozen response payload for a tool call.
// toolName and inputHash must match a recorded KindToolCall entry that also
// has a paired KindToolResponse. Returns ErrUnknownCall if not found.
// Records the composite key in CallHistory in call order.
//
// If Inject configured a fault for this call's position in the sequence,
// Respond returns that fault's error instead — checked before the frozen
// response lookup, so an injected fault fires even on a step that has a
// perfectly good recorded response.
func (m *ToolMocker) Respond(toolName, inputHash string) (json.RawMessage, error) {
	key := compositeKey(toolName, inputHash)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++
	if m.inject != nil && m.callCount == m.inject.atStep {
		return nil, fmt.Errorf("mocker: tool=%q input_hash=%q: %w", toolName, inputHash, m.inject.kind.Err())
	}

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
