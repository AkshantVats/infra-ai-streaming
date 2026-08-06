// SPDX-License-Identifier: MIT

package stack

import (
	"context"
	"fmt"
	"sync"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

// MemRedis is an in-memory RedisClient for tests — no Docker daemon in
// this sandbox, the same constraint every prior Redis-dependent day in
// this repo has logged. ErrOnGet, if set, makes every Get return that
// error instead of consulting the map, so tests can exercise Stack's
// fail-open path without a real connection failure.
type MemRedis struct {
	mu       sync.Mutex
	data     map[string]string
	ErrOnGet error
}

func NewMemRedis() *MemRedis {
	return &MemRedis{data: make(map[string]string)}
}

func (m *MemRedis) Get(_ context.Context, key string) (string, bool, error) {
	if m.ErrOnGet != nil {
		return "", false, m.ErrOnGet
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	return v, ok, nil
}

func (m *MemRedis) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

// Contains reports whether key is present, for tests asserting an L2
// hit was backfilled into L1.
func (m *MemRedis) Contains(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[key]
	return ok
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
