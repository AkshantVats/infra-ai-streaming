package clickhouse

import (
	"sync"
	"time"
)

// BreakerState is the circuit breaker phase.
type BreakerState int

const (
	BreakerClosed BreakerState = iota
	BreakerOpen
	BreakerHalfOpen
)

func (s BreakerState) String() string {
	switch s {
	case BreakerOpen:
		return "open"
	case BreakerHalfOpen:
		return "halfopen"
	default:
		return "closed"
	}
}

// CircuitBreaker opens after failureThreshold consecutive failures; half-open after resetTimeout.
type CircuitBreaker struct {
	mu                sync.Mutex
	state             BreakerState
	failureCount      int
	failureThreshold  int
	resetTimeout      time.Duration
	lastOpened        time.Time
	halfOpenSuccesses int
}

// NewCircuitBreaker builds a breaker with the given thresholds.
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            BreakerClosed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

// State returns the current breaker state (may transition open → half-open).
func (cb *CircuitBreaker) State() BreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maybeHalfOpen()
	return cb.state
}

// AllowInsert reports whether a ClickHouse insert should be attempted.
func (cb *CircuitBreaker) AllowInsert() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maybeHalfOpen()
	return cb.state == BreakerClosed || cb.state == BreakerHalfOpen
}

func (cb *CircuitBreaker) maybeHalfOpen() {
	if cb.state == BreakerOpen && time.Since(cb.lastOpened) >= cb.resetTimeout {
		cb.state = BreakerHalfOpen
		cb.halfOpenSuccesses = 0
	}
}

// RecordSuccess records a successful insert.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == BreakerHalfOpen {
		cb.state = BreakerClosed
	}
	cb.failureCount = 0
	cb.state = BreakerClosed
}

// RecordFailure records a failed insert.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.state == BreakerHalfOpen {
		cb.state = BreakerOpen
		cb.lastOpened = time.Now()
		return
	}
	if cb.failureCount >= cb.failureThreshold {
		cb.state = BreakerOpen
		cb.lastOpened = time.Now()
	}
}
