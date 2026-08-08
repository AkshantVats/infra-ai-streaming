// SPDX-License-Identifier: MIT

package judge

import (
	"sync"
	"time"
)

// window and failureRateThreshold are DESIGN.md §5's stated commitments:
// if more than 10% of judge attempts in a trailing 5 minutes fail, the
// worker pool stops pulling new samples for the affected task_type
// until the judge recovers.
const (
	window               = 5 * time.Minute
	failureRateThreshold = 0.10
	// minAttempts guards against a handful of attempts near process
	// start looking like a 100% failure rate off a sample of one — the
	// threshold only applies once there's enough volume for a rate to
	// mean anything.
	minAttempts = 5
)

type attempt struct {
	at      time.Time
	success bool
}

// Breaker tracks judge call outcomes per task_type over a trailing
// window and reports whether new samples for that task_type should be
// pulled from the queue, per DESIGN.md §5's circuit-breaker failure
// mode. Unlike a consecutive-failure breaker, this measures a rate over
// wall-clock time, because DESIGN.md's stated threshold ("10% of
// attempts in 5 minutes") is a rate, not a streak.
type Breaker struct {
	mu       sync.Mutex
	clock    Clock
	attempts map[string][]attempt
}

// NewBreaker constructs a Breaker using clock as its time source.
func NewBreaker(clock Clock) *Breaker {
	return &Breaker{
		clock:    clock,
		attempts: make(map[string][]attempt),
	}
}

// Allow reports whether a new sample for taskType should be pulled from
// judge-requests right now.
func (b *Breaker) Allow(taskType string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	events := b.prune(taskType)
	if len(events) < minAttempts {
		return true
	}
	return b.failureRate(events) <= failureRateThreshold
}

// Record logs one judge attempt's outcome for taskType.
func (b *Breaker) Record(taskType string, success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	events := b.prune(taskType)
	b.attempts[taskType] = append(events, attempt{at: b.clock.Now(), success: success})
}

// prune must be called with b.mu held. It drops attempts older than
// window and returns the surviving slice for taskType.
func (b *Breaker) prune(taskType string) []attempt {
	cutoff := b.clock.Now().Add(-window)
	events := b.attempts[taskType]
	kept := events[:0:0]
	for _, e := range events {
		if e.at.After(cutoff) {
			kept = append(kept, e)
		}
	}
	b.attempts[taskType] = kept
	return kept
}

func (b *Breaker) failureRate(events []attempt) float64 {
	var failures int
	for _, e := range events {
		if !e.success {
			failures++
		}
	}
	return float64(failures) / float64(len(events))
}
