// SPDX-License-Identifier: MIT
package eventlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func mustPayload(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func TestReadWriteRoundTrip(t *testing.T) {
	var log EventLog
	for i := int64(1); i <= 5; i++ {
		log = append(log, AgentEvent{
			SeqNum:    i,
			SpanID:    "span-" + string(rune('a'+i)),
			TraceID:   "trace-1",
			Kind:      KindModelTurn,
			Timestamp: 1000 + i,
			Payload:   mustPayload(t, map[string]any{"n": i}),
		})
	}

	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}

	got, err := ReadJSONL(&buf)
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}

	if len(got) != len(log) {
		t.Fatalf("got %d events, want %d", len(got), len(log))
	}
	for i := range log {
		if got[i].SeqNum != log[i].SeqNum {
			t.Errorf("event %d: SeqNum order not preserved: got %d want %d", i, got[i].SeqNum, log[i].SeqNum)
		}
		if !reflect.DeepEqual(got[i], log[i]) {
			t.Errorf("event %d: not equal after round trip.\ngot:  %+v\nwant: %+v", i, got[i], log[i])
		}
	}
}

func TestReadJSONLSortsBySeqNum(t *testing.T) {
	var log EventLog
	for i := int64(5); i >= 1; i-- {
		log = append(log, AgentEvent{
			SeqNum:  i,
			Kind:    KindToolCall,
			Payload: mustPayload(t, map[string]any{}),
		})
	}

	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}

	got, err := ReadJSONL(&buf)
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}

	for i := 1; i < len(got); i++ {
		if got[i].SeqNum < got[i-1].SeqNum {
			t.Fatalf("not sorted ascending: %v", seqNums(got))
		}
	}
	want := []int64{1, 2, 3, 4, 5}
	if !reflect.DeepEqual(seqNums(got), want) {
		t.Errorf("got seq order %v, want %v", seqNums(got), want)
	}
}

func seqNums(log EventLog) []int64 {
	out := make([]int64, len(log))
	for i, ev := range log {
		out[i] = ev.SeqNum
	}
	return out
}

func TestFirstReturnsEarliestOfKind(t *testing.T) {
	log := EventLog{
		{SeqNum: 1, Kind: KindPrompt, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 5, Kind: KindToolCall, ToolName: "later", Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 3, Kind: KindToolCall, ToolName: "earlier", Payload: mustPayload(t, map[string]any{})},
	}

	got, err := log.First(KindToolCall)
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.SeqNum != 3 || got.ToolName != "earlier" {
		t.Errorf("got seq_num=%d tool_name=%q, want seq_num=3 tool_name=earlier", got.SeqNum, got.ToolName)
	}
}

func TestAllOfKindFilters(t *testing.T) {
	log := EventLog{
		{SeqNum: 1, Kind: KindPrompt, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 2, Kind: KindToolCall, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 3, Kind: KindToolResponse, ToolName: "a", Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 4, Kind: KindModelTurn, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 5, Kind: KindToolResponse, ToolName: "b", Payload: mustPayload(t, map[string]any{})},
	}

	got := log.AllOfKind(KindToolResponse)
	if len(got) != 2 {
		t.Fatalf("got %d tool_response events, want 2", len(got))
	}
	if got[0].ToolName != "a" || got[1].ToolName != "b" {
		t.Errorf("got tool_names [%s, %s] out of order, want [a, b]", got[0].ToolName, got[1].ToolName)
	}
}

func TestValidateRejectsDuplicateSpanID(t *testing.T) {
	log := EventLog{
		{SeqNum: 1, SpanID: "dup", Kind: KindPrompt, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 2, SpanID: "dup", Kind: KindModelTurn, Payload: mustPayload(t, map[string]any{})},
	}

	if err := log.Validate(); err == nil {
		t.Fatal("Validate: expected error for duplicate span_id, got nil")
	}
}

func TestValidateRejectsNonMonotonic(t *testing.T) {
	log := EventLog{
		{SeqNum: 1, SpanID: "s1", Kind: KindPrompt, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 2, SpanID: "s2", Kind: KindModelTurn, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 2, SpanID: "s3", Kind: KindToolCall, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 3, SpanID: "s4", Kind: KindFinalOutput, Payload: mustPayload(t, map[string]any{})},
	}

	if err := log.Validate(); err == nil {
		t.Fatal("Validate: expected error for non-monotonic seq_num, got nil")
	}
}

func TestEmptyLogIsValid(t *testing.T) {
	var log EventLog

	if err := log.Validate(); err != nil {
		t.Errorf("Validate on empty log: got error %v, want nil", err)
	}

	_, err := log.First(KindPrompt)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("First on empty log: got %v, want ErrNotFound", err)
	}
}

func TestFilterByTraceID(t *testing.T) {
	log := EventLog{
		{SeqNum: 3, TraceID: "trace-b", Kind: KindPrompt, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 1, TraceID: "trace-a", Kind: KindPrompt, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 2, TraceID: "trace-a", Kind: KindToolCall, Payload: mustPayload(t, map[string]any{})},
	}

	got := log.FilterByTraceID("trace-a")
	if len(got) != 2 {
		t.Fatalf("len(FilterByTraceID) = %d, want 2", len(got))
	}
	if got[0].SeqNum != 1 || got[1].SeqNum != 2 {
		t.Fatalf("FilterByTraceID not in seq_num order: %+v", got)
	}
	for _, ev := range got {
		if ev.TraceID != "trace-a" {
			t.Errorf("FilterByTraceID leaked event from trace_id=%q", ev.TraceID)
		}
	}

	if got := log.FilterByTraceID("trace-does-not-exist"); len(got) != 0 {
		t.Errorf("FilterByTraceID for unknown trace_id = %d events, want 0", len(got))
	}
}
