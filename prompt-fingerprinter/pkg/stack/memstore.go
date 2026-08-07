// SPDX-License-Identifier: MIT

package stack

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

// memEntry is a stored value plus the wall-clock time it expires at.
type memEntry struct {
	value    string
	expireAt time.Time
}

// MemRedis is an in-memory RedisClient for tests — no Docker daemon in
// this sandbox, the same constraint every prior Redis-dependent day in
// this repo has logged. ErrOnGet, if set, makes every Get return that
// error instead of consulting the map, so tests can exercise Stack's
// fail-open path without a real connection failure.
//
// Set takes a TTL, mirroring a real Redis client's SETEX, and Get
// checks that TTL against Clock before returning a value — an expired
// entry reads back as a miss, the same way a real Redis instance would
// have already evicted the key. This is what makes the collision drill
// in collision_test.go possible: it advances the Clock past the TTL
// and confirms the stack no longer serves the stale entry, without a
// real 30-day wait.
type MemRedis struct {
	mu    sync.Mutex
	data  map[string]memEntry
	clock Clock

	ErrOnGet error
}

// NewMemRedis constructs a MemRedis backed by the real wall clock —
// the right default for every existing test, none of which cares
// about expiry.
func NewMemRedis() *MemRedis {
	return NewMemRedisWithClock(RealClock{})
}

// NewMemRedisWithClock constructs a MemRedis backed by clock, so a
// test can inject a FakeClock and control expiry deterministically.
func NewMemRedisWithClock(clock Clock) *MemRedis {
	return &MemRedis{data: make(map[string]memEntry), clock: clock}
}

func (m *MemRedis) Get(_ context.Context, key string) (string, bool, error) {
	if m.ErrOnGet != nil {
		return "", false, m.ErrOnGet
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok {
		return "", false, nil
	}
	if !e.expireAt.IsZero() && !m.clock.Now().Before(e.expireAt) {
		// Expired: a real Redis instance would already have evicted
		// this key via its own TTL, so a lazily-expired read here
		// reports the same "not found" a live GET would.
		delete(m.data, key)
		return "", false, nil
	}
	return e.value, true, nil
}

// Set stores value at key with the given ttl. A zero ttl means "never
// expires" (matches go-redis's Set semantics for a zero expiration),
// which no production call site in this package uses — Stack.Get
// always backfills with HardTTL — but keeping the zero-value meaning
// explicit avoids a silent behavior change for any future caller that
// omits it.
func (m *MemRedis) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := memEntry{value: value}
	if ttl > 0 {
		e.expireAt = m.clock.Now().Add(ttl)
	}
	m.data[key] = e
	return nil
}

// Contains reports whether key is present and unexpired, for tests
// asserting an L2 hit was backfilled into L1.
func (m *MemRedis) Contains(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok {
		return false
	}
	if !e.expireAt.IsZero() && !m.clock.Now().Before(e.expireAt) {
		return false
	}
	return true
}

// MemL2 is a fixed-response fake L2Store. Responses is keyed by
// tenantID; ErrTenant, if it matches the requested tenant, makes Get
// return an error instead of a lookup result.
type MemL2 struct {
	Responses map[string]string
	ErrTenant string
}

func (m *MemL2) Get(_ context.Context, tenantID string, _ fingerprint.PromptRequest) (string, bool, error) {
	if m.ErrTenant != "" && tenantID == m.ErrTenant {
		return "", false, fmt.Errorf("memL2: simulated failure for tenant %s", tenantID)
	}
	resp, ok := m.Responses[tenantID]
	return resp, ok, nil
}

// MemMetrics counts each outcome per tenant, for tests to assert on.
type MemMetrics struct {
	mu     sync.Mutex
	L1Hits map[string]int
	L2Hits map[string]int
	Misses map[string]int
}

func NewMemMetrics() *MemMetrics {
	return &MemMetrics{
		L1Hits: make(map[string]int),
		L2Hits: make(map[string]int),
		Misses: make(map[string]int),
	}
}

func (m *MemMetrics) IncL1Hit(_ context.Context, tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L1Hits[tenantID]++
}

func (m *MemMetrics) IncL2Hit(_ context.Context, tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L2Hits[tenantID]++
}

func (m *MemMetrics) IncMiss(_ context.Context, tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Misses[tenantID]++
}
