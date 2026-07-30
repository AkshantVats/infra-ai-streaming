// SPDX-License-Identifier: MIT
package export

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
	"github.com/akshantvats/agent-replay-engine/pkg/objectstore"
)

func mustPayload(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func sampleLog(t *testing.T, traceID string, n int) eventlog.EventLog {
	t.Helper()
	var log eventlog.EventLog
	for i := 1; i <= n; i++ {
		log = append(log, eventlog.AgentEvent{
			SeqNum:    int64(i),
			TraceID:   traceID,
			Kind:      eventlog.KindModelTurn,
			Timestamp: int64(1000 + i),
			// Repetitive payload keeps this a meaningful zstd compression
			// sanity check without needing megabytes of fixture data.
			Payload: mustPayload(t, map[string]any{"n": i, "note": strings.Repeat("payload-filler-text ", 20)}),
		})
	}
	return log
}

func TestExportRoundTrip(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}
	log := sampleLog(t, "trace-abc", 5)

	key, err := e.Export(context.Background(), log)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	r, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get %s: %v", key, err)
	}
	defer func() { _ = r.Close() }()
	compressed, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	dec, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatalf("zstd.NewReader: %v", err)
	}
	defer dec.Close()
	raw, err := dec.DecodeAll(compressed, nil)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}

	got, err := eventlog.ReadJSONL(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}
	if len(got) != len(log) {
		t.Fatalf("got %d events after round trip, want %d", len(got), len(log))
	}
	for i := range log {
		if got[i].SeqNum != log[i].SeqNum || got[i].TraceID != log[i].TraceID {
			t.Errorf("event %d: got seq_num=%d trace_id=%q, want seq_num=%d trace_id=%q",
				i, got[i].SeqNum, got[i].TraceID, log[i].SeqNum, log[i].TraceID)
		}
	}
}

func TestExportKeyFormat(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}
	log := sampleLog(t, "9f2a-trace", 47)

	key, err := e.Export(context.Background(), log)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	want := "traces/9f2a-trace/000001-000047.jsonl.zst"
	if key != want {
		t.Errorf("got key %q, want %q", key, want)
	}
}

func TestExportWritesChecksumSidecar(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}
	log := sampleLog(t, "trace-sidecar", 3)

	key, err := e.Export(context.Background(), log)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	objR, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get object: %v", err)
	}
	defer func() { _ = objR.Close() }()
	compressed, err := io.ReadAll(objR)
	if err != nil {
		t.Fatalf("ReadAll object: %v", err)
	}
	sum := sha256.Sum256(compressed)
	wantChecksum := hex.EncodeToString(sum[:])

	sumR, err := store.Get(context.Background(), key+".sha256")
	if err != nil {
		t.Fatalf("Get checksum sidecar: %v", err)
	}
	defer func() { _ = sumR.Close() }()
	gotChecksum, err := io.ReadAll(sumR)
	if err != nil {
		t.Fatalf("ReadAll checksum: %v", err)
	}

	if string(gotChecksum) != wantChecksum {
		t.Errorf("got checksum %q, want %q", gotChecksum, wantChecksum)
	}
}

func TestVerifySucceedsAfterExport(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}
	log := sampleLog(t, "trace-verify-ok", 4)

	key, err := e.Export(context.Background(), log)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if err := e.Verify(context.Background(), key); err != nil {
		t.Errorf("Verify: got %v, want nil", err)
	}
}

func TestVerifyDetectsCorruption(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}
	log := sampleLog(t, "trace-verify-corrupt", 4)

	key, err := e.Export(context.Background(), log)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Tamper with the compressed object after export without touching the
	// checksum sidecar, simulating bit-rot or a partial re-upload.
	tampered := []byte("this is not the original compressed payload")
	if err := store.Put(context.Background(), key, bytes.NewReader(tampered), int64(len(tampered))); err != nil {
		t.Fatalf("Put tampered object: %v", err)
	}

	err = e.Verify(context.Background(), key)
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("Verify on corrupted object: got %v, want ErrChecksumMismatch", err)
	}
}

func TestExportEmptyLogReturnsError(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}

	_, err := e.Export(context.Background(), eventlog.EventLog{})
	if !errors.Is(err, ErrEmptyLog) {
		t.Errorf("Export empty log: got %v, want ErrEmptyLog", err)
	}
}

func TestExportRejectsMixedTraceID(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}
	log := eventlog.EventLog{
		{SeqNum: 1, TraceID: "trace-a", Kind: eventlog.KindPrompt, Payload: mustPayload(t, map[string]any{})},
		{SeqNum: 2, TraceID: "trace-b", Kind: eventlog.KindModelTurn, Payload: mustPayload(t, map[string]any{})},
	}

	_, err := e.Export(context.Background(), log)
	if err == nil {
		t.Fatal("Export with mixed trace_id: got nil error, want non-nil")
	}
}

func TestExportCompressionRatioSanity(t *testing.T) {
	store := objectstore.NewMemoryStore()
	e := &Exporter{Store: store}
	log := sampleLog(t, "trace-ratio", 200)

	var raw bytes.Buffer
	if err := log.WriteJSONL(&raw); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}

	key, err := e.Export(context.Background(), log)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	r, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = r.Close() }()
	compressed, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(compressed) >= raw.Len() {
		t.Errorf("compressed size %d not smaller than raw size %d for repetitive payload", len(compressed), raw.Len())
	}
}
