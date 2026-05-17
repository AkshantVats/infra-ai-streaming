package clickhouse

import (
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
