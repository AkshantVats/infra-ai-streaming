// SPDX-License-Identifier: MIT

package stack

import "time"

// Clock is the time source MemRedis uses to decide whether a key has
// expired. Production code has no need for anything but wall-clock
// time, but the collision drill in collision_test.go needs to advance
// past a 30-day TTL without a real 30-day sleep, so the source is an
// interface rather than a direct time.Now() call.
type Clock interface {
	Now() time.Time
}

// RealClock is the production Clock, backed by time.Now().
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// FakeClock is a settable Clock for tests. The zero value is not
// usable — construct with NewFakeClock so Now() never returns the
// zero time.Time, which would make every TTL comparison degenerate.
type FakeClock struct {
	now time.Time
}

func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

func (c *FakeClock) Now() time.Time { return c.now }

// Advance moves the fake clock forward by d.
func (c *FakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }
