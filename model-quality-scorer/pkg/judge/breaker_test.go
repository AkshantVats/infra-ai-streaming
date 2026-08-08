// SPDX-License-Identifier: MIT

package judge

import (
	"testing"
	"time"
)

func TestBreaker_allowsBelowMinAttempts(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	b := NewBreaker(clock)
	for i := 0; i < minAttempts-1; i++ {
		b.Record("summarization", false)
	}
	if !b.Allow("summarization") {
		t.Fatal("expected Allow to stay true below minAttempts, even at 100% failure")
	}
}

func TestBreaker_opensAboveThreshold(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	b := NewBreaker(clock)
	// 5 attempts, 2 failures = 40% > 10% threshold.
	b.Record("summarization", true)
	b.Record("summarization", true)
	b.Record("summarization", true)
	b.Record("summarization", false)
	b.Record("summarization", false)
	if b.Allow("summarization") {
		t.Fatal("expected breaker to open above 10% failure rate")
	}
}

func TestBreaker_staysClosedBelowThreshold(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	b := NewBreaker(clock)
	for i := 0; i < 19; i++ {
		b.Record("summarization", true)
	}
	b.Record("summarization", false) // 1/20 = 5% < 10%.
	if !b.Allow("summarization") {
		t.Fatal("expected breaker to stay closed below 10% failure rate")
	}
}

func TestBreaker_pruneOldAttemptsOutsideWindow(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	b := NewBreaker(clock)
	for i := 0; i < 10; i++ {
		b.Record("summarization", false)
	}
	if b.Allow("summarization") {
		t.Fatal("sanity check: breaker should be open before advancing clock")
	}
	clock.Advance(window + time.Second)
	if !b.Allow("summarization") {
		t.Fatal("expected old failures to age out of the trailing window")
	}
}

func TestBreaker_taskTypesAreIndependent(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	b := NewBreaker(clock)
	for i := 0; i < 10; i++ {
		b.Record("summarization", false)
	}
	if b.Allow("summarization") {
		t.Fatal("expected summarization breaker to be open")
	}
	if !b.Allow("extraction") {
		t.Fatal("expected an unrelated task_type's breaker to remain closed")
	}
}
