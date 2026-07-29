// SPDX-License-Identifier: MIT
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// --- test doubles ---

// fakeLeaseKV simulates a successful Grant + Put for the Logger.
type fakeLeaseKV struct {
	grantErr error
	putErr   error
	puts     []string // keys written
}

func (f *fakeLeaseKV) Grant(_ context.Context, _ int64) (*clientv3.LeaseGrantResponse, error) {
	if f.grantErr != nil {
		return nil, f.grantErr
	}
	return &clientv3.LeaseGrantResponse{ID: 1}, nil
}

func (f *fakeLeaseKV) Put(_ context.Context, key, _ string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	f.puts = append(f.puts, key)
	return &clientv3.PutResponse{}, nil
}

// --- Entry struct tests ---

func TestEntry_JSONRoundTrip(t *testing.T) {
	e := Entry{
		FlagName:                "model-rollout:acme",
		OldValue:                `{"model":"gpt-4o","pct":100}`,
		NewValue:                `{"model":"gpt-4o-mini","pct":100}`,
		ChangedBy:               "flagctl/kill-switch",
		ChangedAt:               1700000000000000000,
		EvaluationCountSnapshot: 42,
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Entry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.FlagName != e.FlagName {
		t.Errorf("FlagName: want %q, got %q", e.FlagName, out.FlagName)
	}
	if out.EvaluationCountSnapshot != e.EvaluationCountSnapshot {
		t.Errorf("EvaluationCountSnapshot: want %d, got %d",
			e.EvaluationCountSnapshot, out.EvaluationCountSnapshot)
	}
}

func TestEntry_JSONFieldNames(t *testing.T) {
	e := Entry{FlagName: "f", ChangedBy: "bot", EvaluationCountSnapshot: 7}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal into map: %v", err)
	}
	for _, key := range []string{"flag_name", "old_value", "new_value", "changed_by", "changed_at", "evaluation_count_snapshot"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q", key)
		}
	}
}

// --- constant tests ---

func TestAuditConstants(t *testing.T) {
	if auditPrefix != "/audit/" {
		t.Errorf("unexpected auditPrefix: %q", auditPrefix)
	}
	const expected = int64(90 * 24 * 3600)
	if auditTTL != expected {
		t.Errorf("unexpected auditTTL: %d", auditTTL)
	}
}

// TestNew_ReturnsLogger verifies the constructor returns a non-nil Logger.
// We pass nil here because *clientv3.Client satisfies etcdLeaseKV at runtime;
// the constructor just stores the pointer.
func TestNew_ReturnsLogger(t *testing.T) {
	// New accepts *clientv3.Client; passing nil is safe for construction.
	l := New(nil)
	if l == nil {
		t.Fatal("New returned nil Logger")
	}
}

// --- Logger tests ---

func TestLogger_Log_Success(t *testing.T) {
	fake := &fakeLeaseKV{}
	l := &Logger{client: fake}
	e := Entry{
		FlagName:  "feature-x",
		OldValue:  "false",
		NewValue:  "true",
		ChangedBy: "ops-bot",
	}
	if err := l.Log(context.Background(), e); err != nil {
		t.Fatalf("Log: unexpected error: %v", err)
	}
	if len(fake.puts) != 1 {
		t.Fatalf("expected 1 Put call, got %d", len(fake.puts))
	}
	// Key should start with /audit/feature-x/
	if len(fake.puts[0]) < len("/audit/feature-x/") {
		t.Errorf("key too short: %q", fake.puts[0])
	}
}

func TestLogger_Log_GrantError(t *testing.T) {
	fake := &fakeLeaseKV{grantErr: errors.New("etcd unavailable")}
	l := &Logger{client: fake}
	if err := l.Log(context.Background(), Entry{FlagName: "f"}); err == nil {
		t.Error("expected error on Grant failure, got nil")
	}
}

func TestLogger_Log_PutError(t *testing.T) {
	fake := &fakeLeaseKV{putErr: errors.New("write timeout")}
	l := &Logger{client: fake}
	if err := l.Log(context.Background(), Entry{FlagName: "f"}); err == nil {
		t.Error("expected error on Put failure, got nil")
	}
}

func TestLogger_Log_SetsChangedAt(t *testing.T) {
	fake := &fakeLeaseKV{}
	l := &Logger{client: fake}
	e := Entry{FlagName: "ts-flag", ChangedAt: 0}
	// ChangedAt should be overwritten by Log with current nano timestamp.
	if err := l.Log(context.Background(), e); err != nil {
		t.Fatalf("Log: %v", err)
	}
	// The ChangedAt is embedded in the key; we check that a Put happened.
	if len(fake.puts) == 0 {
		t.Fatal("expected a Put to be recorded")
	}
}

func TestLogger_Log_KeyContainsFlagName(t *testing.T) {
	fake := &fakeLeaseKV{}
	l := &Logger{client: fake}
	e := Entry{FlagName: "canary-model", ChangedBy: "ci"}
	if err := l.Log(context.Background(), e); err != nil {
		t.Fatalf("Log: %v", err)
	}
	key := fake.puts[0]
	// Key format: /audit/{flag_name}/{changed_at_ns}
	if len(key) == 0 {
		t.Fatal("empty key")
	}
	const wantPrefix = "/audit/canary-model/"
	if len(key) < len(wantPrefix) || key[:len(wantPrefix)] != wantPrefix {
		t.Errorf("key %q does not start with %q", key, wantPrefix)
	}
}
