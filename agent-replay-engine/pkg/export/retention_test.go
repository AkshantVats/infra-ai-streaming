// SPDX-License-Identifier: MIT
package export

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	"github.com/akshantvats/agent-replay-engine/pkg/objectstore"
)

func TestClassifyBoundaries(t *testing.T) {
	day := 24 * time.Hour

	tests := []struct {
		name string
		age  time.Duration
		want Tier
	}{
		{"29 days is hot", 29 * day, TierHot},
		{"just under 30 days is hot", 30*day - time.Second, TierHot},
		{"30 days is cold", 30 * day, TierCold},
		{"31 days is cold", 31 * day, TierCold},
		{"89 days is cold", 89 * day, TierCold},
		{"just under 90 days is cold", 90*day - time.Second, TierCold},
		{"90 days is expired", 90 * day, TierExpired},
		{"91 days is expired", 91 * day, TierExpired},
		{"zero age is hot", 0, TierHot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.age)
			if got != tt.want {
				t.Errorf("Classify(%v) = %q, want %q", tt.age, got, tt.want)
			}
		})
	}
}

func TestClassifyIsMonotonicAroundBoundaries(t *testing.T) {
	day := 24 * time.Hour
	// The tier of an object must never regress (expired -> cold -> hot) as
	// its age increases.
	rank := map[Tier]int{TierHot: 0, TierCold: 1, TierExpired: 2}

	prev := Classify(0)
	for d := 1; d <= 120; d++ {
		got := Classify(time.Duration(d) * day)
		if rank[got] < rank[prev] {
			t.Fatalf("tier regressed at day %d: %q came after %q", d, got, prev)
		}
		prev = got
	}
}

func TestSweepReturnsOnlyExpiredKeys(t *testing.T) {
	store := objectstore.NewMemoryStore()
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	cases := []struct {
		key string
		age time.Duration
	}{
		{"traces/t/hot-000001-000001.jsonl.zst", 5 * day},
		{"traces/t/cold-000002-000002.jsonl.zst", 45 * day},
		{"traces/t/expired-000003-000003.jsonl.zst", 91 * day},
		{"traces/t/expired-boundary-000004-000004.jsonl.zst", 90 * day},
	}

	for _, c := range cases {
		writtenAt := now.Add(-c.age)
		store.SetClock(func() time.Time { return writtenAt })
		if err := store.Put(ctx, c.key, bytes.NewReader([]byte("x")), 1); err != nil {
			t.Fatalf("Put %s: %v", c.key, err)
		}
	}

	got, err := Sweep(ctx, store, "traces/t/", now)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	sort.Strings(got)

	want := []string{
		"traces/t/expired-000003-000003.jsonl.zst",
		"traces/t/expired-boundary-000004-000004.jsonl.zst",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d expired keys %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expired key %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSweepEmptyStoreReturnsNoKeys(t *testing.T) {
	store := objectstore.NewMemoryStore()

	got, err := Sweep(context.Background(), store, "traces/", time.Now())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d expired keys on empty store, want 0", len(got))
	}
}

func TestSweepRespectsPrefix(t *testing.T) {
	store := objectstore.NewMemoryStore()
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	oldWrite := now.Add(-100 * day)
	store.SetClock(func() time.Time { return oldWrite })
	if err := store.Put(ctx, "traces/other-trace/000001-000001.jsonl.zst", bytes.NewReader([]byte("x")), 1); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := Sweep(ctx, store, "traces/target-trace/", now)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Sweep with non-matching prefix returned %d keys, want 0", len(got))
	}
}
