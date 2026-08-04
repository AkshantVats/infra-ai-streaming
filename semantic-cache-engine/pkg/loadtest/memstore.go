// SPDX-License-Identifier: MIT

package loadtest

import (
	"context"
	"sync"
	"time"

	"github.com/akshantvats/semantic-cache-engine/pkg/cachestore"
)

// MemStore is an in-memory cachestore.Reader that simulates a fixed
// round-trip Latency per call before returning. It stands in for a live
// Postgres+pgvector instance when one isn't available (this sandbox has
// no Docker daemon -- see DESIGN.md §10), the same "fake with a
// documented, honest limitation" shape pkg/localsim uses for
// cmd/threshold-sweep when no live embedding API is reachable.
//
// Latency is a flat simulated number, not a measurement of any real
// hnsw query -- it does not reproduce index-size effects, disk I/O, or
// the per-tenant skew a real production cache would see. Numbers
// produced against MemStore describe the load-test harness's own
// correctness, not semantic-cache-engine's real production latency.
type MemStore struct {
	mu      sync.Mutex
	nearest map[string]cachestore.Match // key: tenantID
	Latency time.Duration
}

// NewMemStore returns a MemStore that sleeps latency before every call.
func NewMemStore(latency time.Duration) *MemStore {
	return &MemStore{
		nearest: make(map[string]cachestore.Match),
		Latency: latency,
	}
}

// Seed registers the single nearest-candidate match FindNearest returns
// for tenantID, mirroring lookup_test.go's fakeStore shape: one fixed
// candidate per tenant is enough to exercise the read path's latency
// without modeling a full index.
func (m *MemStore) Seed(tenantID string, match cachestore.Match) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nearest[tenantID] = match
}

// FindExact always reports a miss -- Day 64's harness only drives
// FindNearest (see loadtest.go's doc comment for why), so MemStore
// implements FindExact only to satisfy cachestore.Reader.
func (m *MemStore) FindExact(ctx context.Context, _, _ string) (cachestore.Match, bool, error) {
	sleep(ctx, m.Latency)
	return cachestore.Match{}, false, nil
}

// FindNearest sleeps Latency to simulate a round trip, then returns
// whatever was registered via Seed for tenantID, or a miss if nothing
// was seeded.
func (m *MemStore) FindNearest(ctx context.Context, tenantID string, _ []float32) (cachestore.Match, bool, error) {
	sleep(ctx, m.Latency)
	m.mu.Lock()
	match, ok := m.nearest[tenantID]
	m.mu.Unlock()
	return match, ok, nil
}

// sleep simulates a round trip while still honoring context
// cancellation, so a Run call bounded by a timed-out ctx doesn't block
// past its deadline waiting on MemStore.
func sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
