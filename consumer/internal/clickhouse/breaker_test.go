// SPDX-License-Identifier: MIT
package clickhouse

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	if cb.State() != BreakerClosed {
		t.Fatalf("state = %v, want closed after 4 failures", cb.State())
	}
	cb.RecordFailure()
	if cb.State() != BreakerOpen {
		t.Fatalf("state = %v, want open after 5 failures", cb.State())
	}
	if cb.AllowInsert() {
		t.Fatal("AllowInsert should be false when open")
	}
}

func TestCircuitBreakerHalfOpenAndClosesOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	if cb.State() != BreakerOpen {
		t.Fatalf("state = %v, want open", cb.State())
	}
	time.Sleep(15 * time.Millisecond)
	if cb.State() != BreakerHalfOpen {
		t.Fatalf("state = %v, want halfopen", cb.State())
	}
	cb.RecordSuccess()
	if cb.State() != BreakerClosed {
		t.Fatalf("state = %v, want closed", cb.State())
	}
}

// TestCircuitBreakerHalfOpenFailureReopens verifies that a failure while in
// HalfOpen state transitions back to Open.
func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	if cb.State() != BreakerHalfOpen {
		t.Fatalf("expected HalfOpen, got %v", cb.State())
	}
	cb.RecordFailure()
	if cb.State() != BreakerOpen {
		t.Fatalf("expected Open after HalfOpen failure, got %v", cb.State())
	}
	if cb.AllowInsert() {
		t.Fatal("AllowInsert must be false when Open")
	}
}

// TestCircuitBreakerSuccessInClosedResetsCount verifies that RecordSuccess
// while closed keeps the breaker closed.
func TestCircuitBreakerSuccessInClosedResetsCount(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets failure count
	cb.RecordFailure()
	cb.RecordFailure()
	// Only 2 failures since last success — still below threshold of 3.
	if cb.State() != BreakerClosed {
		t.Fatalf("state = %v, want closed after reset", cb.State())
	}
}

// TestCircuitBreakerAllowInsertHalfOpen verifies that AllowInsert returns true
// when the breaker is in HalfOpen state.
func TestCircuitBreakerAllowInsertHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	if !cb.AllowInsert() {
		t.Fatal("AllowInsert should be true in HalfOpen")
	}
}

// TestBreakerStateString verifies the String() method for all three states.
func TestBreakerStateString(t *testing.T) {
	tests := []struct {
		state BreakerState
		want  string
	}{
		{BreakerClosed, "closed"},
		{BreakerOpen, "open"},
		{BreakerHalfOpen, "halfopen"},
	}
	for _, tc := range tests {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("BreakerState(%d).String() = %q, want %q", tc.state, got, tc.want)
		}
	}
}

// TestCircuitBreakerConcurrentAccess verifies that the breaker does not race
// when RecordFailure, RecordSuccess, State, and AllowInsert are called from
// multiple goroutines simultaneously.
func TestCircuitBreakerConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(5, 10*time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%3 == 0 {
				cb.RecordFailure()
			} else if n%3 == 1 {
				cb.RecordSuccess()
			} else {
				_ = cb.AllowInsert()
				_ = cb.State()
			}
		}(i)
	}
	wg.Wait()
	// Just verify no panic/race; state is non-deterministic.
}
