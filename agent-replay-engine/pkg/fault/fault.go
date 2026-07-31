// SPDX-License-Identifier: MIT
// Package fault defines the synthetic tool-call failures that can be
// injected into a replay run. Recorded runs only ever contain the tool
// responses that actually happened — usually success. Fault injection lets
// a replay force a specific step to fail instead, so the error path an
// agent would take on a timeout, a 500, or a stale cache hit can be
// exercised without needing that failure to have been recorded live. See
// DESIGN.md's Fault Injection section.
package fault

import (
	"errors"
	"fmt"
)

// Kind identifies which synthetic failure to inject at a tool-call step.
type Kind string

const (
	// KindTimeout simulates the tool call never returning in time.
	KindTimeout Kind = "timeout"

	// KindHTTP500 simulates the tool's backing service returning a server
	// error for this call.
	KindHTTP500 Kind = "http_500"

	// KindStaleCache simulates a cache layer in front of the tool serving
	// an out-of-date response instead of failing outright. It is modeled
	// as an injectable fault (rather than a response substitution) because
	// the interesting behavior to verify is the same as the other two
	// kinds: does the caller's error/staleness handling path trigger.
	KindStaleCache Kind = "stale_cache"
)

// ErrTimeout, ErrHTTP500, and ErrStaleCache are the sentinel errors returned
// for each injected Kind. Callers use errors.Is against these to assert
// which fault fired.
var (
	ErrTimeout    = errors.New("fault: injected timeout")
	ErrHTTP500    = errors.New("fault: injected http 500")
	ErrStaleCache = errors.New("fault: injected stale cache response")
)

// Valid reports whether k is one of the known fault kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindTimeout, KindHTTP500, KindStaleCache:
		return true
	default:
		return false
	}
}

// Err returns the sentinel error for k. Callers should only call this after
// confirming Valid() — an unknown Kind returns a non-sentinel error that
// errors.Is will never match.
func (k Kind) Err() error {
	switch k {
	case KindTimeout:
		return ErrTimeout
	case KindHTTP500:
		return ErrHTTP500
	case KindStaleCache:
		return ErrStaleCache
	default:
		return fmt.Errorf("fault: unknown kind %q", string(k))
	}
}
