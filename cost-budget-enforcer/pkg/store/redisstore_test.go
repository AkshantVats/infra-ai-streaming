// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) *RedisStore {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewRedisStore(rdb)
}

// utcMidnight returns the UTC midnight that begins the given calendar
// day, so tests can reason in terms of "this day's window" the same
// way DESIGN.md's daily budget windows do.
func utcMidnight(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

const dayWindowSeconds = int64(86400)

func TestCheckAndIncrementAccumulatesWithinOneWindow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := utcMidnight(2026, 8, 5).Add(2 * time.Hour)

	weighted, err := s.CheckAndIncrement(ctx, "tenant-a", 100, dayWindowSeconds, now)
	if err != nil {
		t.Fatalf("CheckAndIncrement: %v", err)
	}
	if weighted != 100 {
		t.Fatalf("first call weighted = %v, want 100 (empty previous bucket)", weighted)
	}

	weighted, err = s.CheckAndIncrement(ctx, "tenant-a", 50, dayWindowSeconds, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("CheckAndIncrement: %v", err)
	}
	if weighted != 150 {
		t.Fatalf("second call weighted = %v, want 150 (100+50, same window)", weighted)
	}
}

// TestWindowRollsOverAtUTCMidnight verifies that a request one second
// before midnight and a request one second after midnight land in
// different windows — the counter resets exactly on the UTC day
// boundary, not on some offset derived from when the tenant's first
// request happened to arrive (the drift DESIGN.md's stored
// window_start would otherwise allow).
func TestWindowRollsOverAtUTCMidnight(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	beforeMidnight := utcMidnight(2026, 8, 5).Add(-1 * time.Second)
	afterMidnight := utcMidnight(2026, 8, 5).Add(1 * time.Second)

	if _, err := s.CheckAndIncrement(ctx, "tenant-b", 900, dayWindowSeconds, beforeMidnight); err != nil {
		t.Fatalf("CheckAndIncrement before midnight: %v", err)
	}

	// Immediately after rollover, elapsed_fraction of the new window is
	// ~0, so the full previous-window spend still weighs in at ~100%
	// (DESIGN.md §1's approximation) — this is the window's burst
	// allowance, exercised for its own sake in TestBurstAllowanceDecaysAcrossWindow.
	weighted, err := s.CheckAndIncrement(ctx, "tenant-b", 0, dayWindowSeconds, afterMidnight)
	if err != nil {
		t.Fatalf("CheckAndIncrement after midnight: %v", err)
	}
	if weighted < 899 || weighted > 900 {
		t.Fatalf("weighted just after midnight = %v, want ~900 (previous bucket barely decayed)", weighted)
	}

	// A full day later (two windows past the original), the previous
	// bucket must be treated as stale, not carried forward again.
	twoDaysLater := utcMidnight(2026, 8, 7)
	weighted, err = s.CheckAndIncrement(ctx, "tenant-b", 10, dayWindowSeconds, twoDaysLater)
	if err != nil {
		t.Fatalf("CheckAndIncrement two windows later: %v", err)
	}
	if weighted != 10 {
		t.Fatalf("weighted after a skipped window = %v, want 10 (stale previous bucket dropped)", weighted)
	}
}

// TestBurstAllowanceDecaysAcrossWindow spends the full budget right at
// the end of a window, then checks the weighted total at three points
// into the next window: the allowance from the prior window's spend
// should be near-full immediately after rollover and linearly decay to
// zero by the time the new window is half over, per DESIGN.md §1's
// weighted = current + previous*(1 - elapsed/window_seconds).
func TestBurstAllowanceDecaysAcrossWindow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	windowStart := utcMidnight(2026, 8, 5)
	spendAtEnd := windowStart.Add(time.Duration(dayWindowSeconds-1) * time.Second)

	if _, err := s.CheckAndIncrement(ctx, "tenant-c", 1000, dayWindowSeconds, spendAtEnd); err != nil {
		t.Fatalf("CheckAndIncrement at window end: %v", err)
	}

	nextWindowStart := windowStart.Add(24 * time.Hour)

	justAfter, err := s.CheckAndIncrement(ctx, "tenant-c", 0, dayWindowSeconds, nextWindowStart.Add(time.Second))
	if err != nil {
		t.Fatalf("CheckAndIncrement just after rollover: %v", err)
	}
	if justAfter < 990 {
		t.Fatalf("weighted just after rollover = %v, want close to 1000 (near-full burst allowance)", justAfter)
	}

	halfway, err := s.CheckAndIncrement(ctx, "tenant-c", 0, dayWindowSeconds, nextWindowStart.Add(12*time.Hour))
	if err != nil {
		t.Fatalf("CheckAndIncrement halfway through window: %v", err)
	}
	if halfway > 520 || halfway < 480 {
		t.Fatalf("weighted halfway through window = %v, want ~500 (half-decayed)", halfway)
	}
	if halfway >= justAfter {
		t.Fatalf("weighted did not decay: just-after=%v halfway=%v", justAfter, halfway)
	}

	nearEnd, err := s.CheckAndIncrement(ctx, "tenant-c", 0, dayWindowSeconds, nextWindowStart.Add(time.Duration(dayWindowSeconds-1)*time.Second))
	if err != nil {
		t.Fatalf("CheckAndIncrement near end of window: %v", err)
	}
	if nearEnd > 5 {
		t.Fatalf("weighted near end of window = %v, want ~0 (fully decayed)", nearEnd)
	}
}

func TestMarkAlertedDebouncesWithinWindow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := utcMidnight(2026, 8, 5).Add(time.Hour)

	first, err := s.MarkAlerted(ctx, "tenant-d", dayWindowSeconds, now)
	if err != nil {
		t.Fatalf("MarkAlerted first: %v", err)
	}
	if !first {
		t.Fatalf("first MarkAlerted call returned false, want true (should win the flag)")
	}

	second, err := s.MarkAlerted(ctx, "tenant-d", dayWindowSeconds, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("MarkAlerted second: %v", err)
	}
	if second {
		t.Fatalf("second MarkAlerted call in same window returned true, want false (debounced)")
	}

	nextWindow, err := s.MarkAlerted(ctx, "tenant-d", dayWindowSeconds, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("MarkAlerted next window: %v", err)
	}
	if !nextWindow {
		t.Fatalf("MarkAlerted in the next window returned false, want true (fresh flag)")
	}
}

func TestConcurrentCheckAndIncrementDoesNotRace(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := utcMidnight(2026, 8, 5).Add(time.Hour)

	const goroutines = 20
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := s.CheckAndIncrement(ctx, "tenant-race", 10, dayWindowSeconds, now)
			errs <- err
		}()
	}
	for i := 0; i < goroutines; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent CheckAndIncrement: %v", err)
		}
	}

	// A final call sees every prior increment reflected — none were
	// lost to the read-then-write race DESIGN.md §1 calls out.
	weighted, err := s.CheckAndIncrement(ctx, "tenant-race", 0, dayWindowSeconds, now)
	if err != nil {
		t.Fatalf("final CheckAndIncrement: %v", err)
	}
	if weighted != goroutines*10 {
		t.Fatalf("weighted after %d concurrent +10s = %v, want %d", goroutines, weighted, goroutines*10)
	}
}
