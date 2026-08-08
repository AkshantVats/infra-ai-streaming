// SPDX-License-Identifier: MIT

package judge

import "time"

// Clock is the time source the breaker's trailing window uses. Production
// code has no need for anything but wall-clock time, but the breaker's
// 5-minute window can't be exercised in a unit test with a real 5-minute
// sleep, so the source is an interface — same pattern as
// prompt-fingerprinter/pkg/stack.Clock.
type Clock interface {
	Now() time.Time
}

// RealClock is the production Clock, backed by time.Now().
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// FakeClock is a settable Clock for tests.
type FakeClock struct {
	now time.Time
}

func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

func (c *FakeClock) Now() time.Time { return c.now }

func (c *FakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }
