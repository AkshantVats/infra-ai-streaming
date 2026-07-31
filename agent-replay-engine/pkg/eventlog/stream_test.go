// SPDX-License-Identifier: MIT
package eventlog

import (
	"bytes"
	"strings"
	"testing"
)

func TestScannerReadsEventsInFileOrder(t *testing.T) {
	var log EventLog
	for i := int64(5); i >= 1; i-- {
		log = append(log, AgentEvent{SeqNum: i, Kind: KindToolCall, Payload: mustPayload(t, map[string]any{})})
	}
	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}

	sc := NewScanner(&buf)
	var got []int64
	for sc.Scan() {
		got = append(got, sc.Event().SeqNum)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Scanner does not re-sort — it must preserve exactly the file's
	// descending SeqNum order that ReadJSONL would have sorted away.
	want := []int64{5, 4, 3, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d: got seq_num %d, want %d (order %v)", i, got[i], want[i], got)
		}
	}
}

func TestScannerSkipsBlankLines(t *testing.T) {
	input := "\n" +
		`{"seq_num":1,"kind":"prompt","payload":{}}` + "\n" +
		"   \n" +
		`{"seq_num":2,"kind":"tool_call","payload":{}}` + "\n" +
		"\n"

	sc := NewScanner(strings.NewReader(input))
	var count int
	for sc.Scan() {
		count++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if count != 2 {
		t.Fatalf("got %d events, want 2 (blank lines should be skipped)", count)
	}
}

func TestScannerReturnsErrOnMalformedLine(t *testing.T) {
	input := `{"seq_num":1,"kind":"prompt","payload":{}}` + "\n" +
		`not json` + "\n"

	sc := NewScanner(strings.NewReader(input))
	if !sc.Scan() {
		t.Fatalf("Scan: expected first line to succeed, err=%v", sc.Err())
	}
	if sc.Scan() {
		t.Fatalf("Scan: expected malformed line to stop the scan")
	}
	if sc.Err() == nil {
		t.Fatal("Err: expected error for malformed line, got nil")
	}
}

func TestScannerEmptyInputYieldsNoEvents(t *testing.T) {
	sc := NewScanner(strings.NewReader(""))
	if sc.Scan() {
		t.Fatal("Scan: expected false on empty input")
	}
	if sc.Err() != nil {
		t.Fatalf("Err: got %v, want nil for empty input (not an error)", sc.Err())
	}
}

func TestScannerHandlesLargePayload(t *testing.T) {
	// One tool response can legitimately be a multi-MB API dump; the
	// scanner's buffer must grow to accommodate a single such line the
	// same way ReadJSONL's does.
	big := strings.Repeat("x", 5*1024*1024)
	ev := AgentEvent{SeqNum: 1, Kind: KindToolResponse, Payload: mustPayload(t, map[string]any{"data": big})}
	var log EventLog
	log = append(log, ev)
	var buf bytes.Buffer
	if err := log.WriteJSONL(&buf); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}

	sc := NewScanner(&buf)
	if !sc.Scan() {
		t.Fatalf("Scan: expected one event, err=%v", sc.Err())
	}
	if len(sc.Event().Payload) < 5*1024*1024 {
		t.Fatalf("payload truncated: got %d bytes", len(sc.Event().Payload))
	}
}
