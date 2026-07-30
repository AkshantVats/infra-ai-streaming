// SPDX-License-Identifier: MIT
package mocker

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
)

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestMockerDeterministicLookup(t *testing.T) {
	log := eventlog.EventLog{
		{
			SeqNum: 1, Kind: eventlog.KindToolCall,
			ToolName: "search_web", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"query": "kafka"}),
		},
		{
			SeqNum: 2, Kind: eventlog.KindToolResponse,
			ToolName: "search_web", InputHash: "hash-1",
			Payload: rawJSON(t, map[string]any{"results": []string{"a", "b"}}),
		},
	}

	m, err := LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	got, err := m.Respond("search_web", "hash-1")
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}

	want := rawJSON(t, map[string]any{"results": []string{"a", "b"}})
	if !reflect.DeepEqual(normalizeJSON(t, got), normalizeJSON(t, want)) {
		t.Errorf("got payload %s, want %s", got, want)
	}
}

func normalizeJSON(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal for comparison: %v", err)
	}
	return v
}

func TestMockerUnknownCallReturnsError(t *testing.T) {
	m, err := LoadFromLog(eventlog.EventLog{})
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	_, err = m.Respond("search_web", "hash-nonexistent")
	if !errors.Is(err, ErrUnknownCall) {
		t.Errorf("got err %v, want ErrUnknownCall", err)
	}
}

func TestMockerCompositeKeyIsolation(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, Kind: eventlog.KindToolCall, ToolName: "search_web", InputHash: "shared-hash",
			Payload: rawJSON(t, map[string]any{"query": "kafka"})},
		{SeqNum: 2, Kind: eventlog.KindToolResponse, ToolName: "search_web", InputHash: "shared-hash",
			Payload: rawJSON(t, map[string]any{"source": "web"})},
		{SeqNum: 3, Kind: eventlog.KindToolCall, ToolName: "search_news", InputHash: "shared-hash",
			Payload: rawJSON(t, map[string]any{"query": "kafka"})},
		{SeqNum: 4, Kind: eventlog.KindToolResponse, ToolName: "search_news", InputHash: "shared-hash",
			Payload: rawJSON(t, map[string]any{"source": "news"})},
	}

	m, err := LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	webResp, err := m.Respond("search_web", "shared-hash")
	if err != nil {
		t.Fatalf("Respond(search_web): %v", err)
	}
	newsResp, err := m.Respond("search_news", "shared-hash")
	if err != nil {
		t.Fatalf("Respond(search_news): %v", err)
	}

	webVal := normalizeJSON(t, webResp).(map[string]any)
	newsVal := normalizeJSON(t, newsResp).(map[string]any)

	if webVal["source"] != "web" {
		t.Errorf("search_web returned source=%v, want web (cross-lookup with search_news)", webVal["source"])
	}
	if newsVal["source"] != "news" {
		t.Errorf("search_news returned source=%v, want news (cross-lookup with search_web)", newsVal["source"])
	}
}

func TestMockerCallHistoryOrdered(t *testing.T) {
	log := eventlog.EventLog{
		{SeqNum: 1, Kind: eventlog.KindToolCall, ToolName: "A", InputHash: "h",
			Payload: rawJSON(t, map[string]any{})},
		{SeqNum: 2, Kind: eventlog.KindToolResponse, ToolName: "A", InputHash: "h",
			Payload: rawJSON(t, map[string]any{"n": 1})},
		{SeqNum: 3, Kind: eventlog.KindToolCall, ToolName: "B", InputHash: "h",
			Payload: rawJSON(t, map[string]any{})},
		{SeqNum: 4, Kind: eventlog.KindToolResponse, ToolName: "B", InputHash: "h",
			Payload: rawJSON(t, map[string]any{"n": 2})},
		{SeqNum: 5, Kind: eventlog.KindToolCall, ToolName: "C", InputHash: "h",
			Payload: rawJSON(t, map[string]any{})},
		{SeqNum: 6, Kind: eventlog.KindToolResponse, ToolName: "C", InputHash: "h",
			Payload: rawJSON(t, map[string]any{"n": 3})},
	}

	m, err := LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	if _, err := m.Respond("A", "h"); err != nil {
		t.Fatalf("Respond(A): %v", err)
	}
	if _, err := m.Respond("B", "h"); err != nil {
		t.Fatalf("Respond(B): %v", err)
	}
	if _, err := m.Respond("C", "h"); err != nil {
		t.Fatalf("Respond(C): %v", err)
	}

	want := []string{compositeKey("A", "h"), compositeKey("B", "h"), compositeKey("C", "h")}
	got := m.CallHistory()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got call history %v, want %v", got, want)
	}
}

func TestMockerConcurrentRespond(t *testing.T) {
	const n = 10

	log := make(eventlog.EventLog, 0, n*2)
	for i := 0; i < n; i++ {
		tool := fmt.Sprintf("tool-%d", i)
		hash := fmt.Sprintf("hash-%d", i)
		log = append(log,
			eventlog.AgentEvent{
				SeqNum: int64(2*i + 1), Kind: eventlog.KindToolCall,
				ToolName: tool, InputHash: hash,
				Payload: rawJSON(t, map[string]any{}),
			},
			eventlog.AgentEvent{
				SeqNum: int64(2*i + 2), Kind: eventlog.KindToolResponse,
				ToolName: tool, InputHash: hash,
				Payload: rawJSON(t, map[string]any{"index": i}),
			},
		)
	}

	m, err := LoadFromLog(log)
	if err != nil {
		t.Fatalf("LoadFromLog: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, n)
	results := make([]json.RawMessage, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tool := fmt.Sprintf("tool-%d", i)
			hash := fmt.Sprintf("hash-%d", i)
			resp, err := m.Respond(tool, hash)
			errs[i] = err
			results[i] = resp
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d: Respond error: %v", i, errs[i])
			continue
		}
		val := normalizeJSON(t, results[i]).(map[string]any)
		gotIndex, ok := val["index"].(float64)
		if !ok || int(gotIndex) != i {
			t.Errorf("goroutine %d: got index %v, want %d", i, val["index"], i)
		}
	}

	if got := len(m.CallHistory()); got != n {
		t.Errorf("got %d call history entries, want %d", got, n)
	}
}
