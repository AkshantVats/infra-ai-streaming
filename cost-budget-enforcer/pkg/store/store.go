// SPDX-License-Identifier: MIT

// Package store implements the sliding-window token budget counter
// DESIGN.md §1 specifies: two fixed buckets per tenant (current,
// previous) and a weighted-average approximation of true usage,
// updated atomically via a Lua script so concurrent requests for the
// same tenant can't race past each other into an overspend (§1's "the
// same race a naive read-modify-write would leave open").
//
// This implementation refines DESIGN.md's Lua script in one respect:
// instead of persisting a window_start timestamp seeded by whichever
// request happens to be first after expiry (which lets window
// boundaries drift), window membership is computed as a deterministic
// function of wall-clock time — window_index = floor(now / window_seconds)
// — so a window_seconds of 86400 always rolls over exactly at UTC
// midnight (the Unix epoch is itself a UTC midnight), never a few
// seconds late because of when traffic happened to arrive.
package store

import (
	"context"
	"time"
)

// Store is the tenant budget counter. Implementations must make
// CheckAndIncrement atomic per tenant: two concurrent calls for the
// same tenant must not both observe headroom that only exists once.
type Store interface {
	// CheckAndIncrement records tokens spent by tenantID at time now,
	// rolling the window over first if now has crossed into a new
	// window_seconds-sized window, and returns the weighted total
	// (DESIGN.md §1's current + previous*(1-elapsed_fraction)) after
	// this call's tokens are included.
	CheckAndIncrement(ctx context.Context, tenantID string, tokens int64, windowSeconds int64, now time.Time) (weighted float64, err error)

	// MarkAlerted implements DESIGN.md §4's debounce: it returns true
	// the first time it's called for a given (tenantID, window) pair
	// and false on every subsequent call within the same window, so
	// callers can use the return value to decide whether to actually
	// fire the alert webhook.
	MarkAlerted(ctx context.Context, tenantID string, windowSeconds int64, now time.Time) (fired bool, err error)
}

// WindowIndex returns the index of the window containing now, given a
// window size in seconds. Two timestamps are in the same window iff
// WindowIndex returns the same value for both.
func WindowIndex(now time.Time, windowSeconds int64) int64 {
	return now.Unix() / windowSeconds
}

// WindowStart returns the UTC instant a window begins, given the
// window's index.
func WindowStart(windowIndex, windowSeconds int64) time.Time {
	return time.Unix(windowIndex*windowSeconds, 0).UTC()
}
