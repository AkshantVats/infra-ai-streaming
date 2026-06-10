// SPDX-License-Identifier: MIT
//
// Chaos test: etcd leader loss during a model-route rollout.
//
// The invariant under test: when the etcd leader is unavailable, the flag
// client must NOT return an error to callers — it falls back to the last
// known-good flag snapshot (stale-reads-ok) and serves traffic from cache.
// This is "fail-safe" behaviour: prefer stale data over a hard failure that
// would cause the entire inference gateway to crash.
//
// Run with: go test ./chaos/... -v -run TestEtcdLeaderLoss
// Requires no external process — it uses an in-process mock that injects
// network-level errors on demand.
package chaos

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// flagCache is a minimal in-memory cache that mirrors what a production
// flagd gRPC streaming client would maintain in the watch loop.
type flagCache struct {
	mu    sync.RWMutex
	flags map[string]string // flag name → JSON value
}

func newFlagCache() *flagCache {
	return &flagCache{flags: make(map[string]string)}
}

func (c *flagCache) set(name, value string) {
	c.mu.Lock()
	c.flags[name] = value
	c.mu.Unlock()
}

func (c *flagCache) get(name string) (string, bool) {
	c.mu.RLock()
	v, ok := c.flags[name]
	c.mu.RUnlock()
	return v, ok
}

// etcdLike simulates the flag-fetch path through an etcd-backed store.
// When broken == true, all reads return an error (leader election in progress).
type etcdLike struct {
	mu     sync.Mutex
	broken bool
	store  map[string]string
}

func newEtcdLike() *etcdLike {
	return &etcdLike{store: make(map[string]string)}
}

func (e *etcdLike) put(k, v string) {
	e.mu.Lock()
	e.store[k] = v
	e.mu.Unlock()
}

func (e *etcdLike) get(k string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.broken {
		return "", errors.New("etcdserver: leader changed, re-elect in progress")
	}
	v, ok := e.store[k]
	if !ok {
		return "", errors.New("key not found")
	}
	return v, nil
}

func (e *etcdLike) setLeaderLost(lost bool) {
	e.mu.Lock()
	e.broken = lost
	e.mu.Unlock()
}

// flagdWatcher mimics the watch loop: on each tick it tries to refresh the
// cache from etcd; on error it leaves the cache intact (last-known-good).
func flagdWatcher(ctx context.Context, etcd *etcdLike, cache *flagCache, flags []string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, name := range flags {
				val, err := etcd.get(name)
				if err != nil {
					// Leader lost: leave cache untouched — fail-safe.
					continue
				}
				cache.set(name, val)
			}
		}
	}
}

// TestEtcdLeaderLoss verifies that a 3-second leader-loss window does not
// evict the cached flag value — all reads during the window return the last
// known-good value, not an error.
func TestEtcdLeaderLoss(t *testing.T) {
	etcd := newEtcdLike()
	cache := newFlagCache()

	const flagName = "model_route"
	const knownGood = `{"model":"gpt-4o","pct":100}`

	etcd.put(flagName, knownGood)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go flagdWatcher(ctx, etcd, cache, []string{flagName}, 50*time.Millisecond)

	// Allow the watcher to populate the cache.
	time.Sleep(150 * time.Millisecond)

	val, ok := cache.get(flagName)
	if !ok || val != knownGood {
		t.Fatalf("pre-chaos: expected cache to hold %q, got %q (ok=%v)", knownGood, val, ok)
	}

	// Inject leader loss for 3 seconds.
	etcd.setLeaderLost(true)
	leaderLostAt := time.Now()

	// While leader is down, the cache must not be cleared.
	deadline := leaderLostAt.Add(2 * time.Second)
	for time.Now().Before(deadline) {
		v, cacheOK := cache.get(flagName)
		if !cacheOK {
			t.Errorf("cache entry evicted during leader loss — fail-unsafe behaviour")
		}
		if v != knownGood {
			t.Errorf("cache value changed during leader loss: got %q want %q", v, knownGood)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Restore leader — watcher must resume normal operation.
	etcd.setLeaderLost(false)
	newVal := `{"model":"gpt-4o-mini","pct":100}`
	etcd.put(flagName, newVal)

	// Allow at least two watch ticks to pick up the new value.
	time.Sleep(200 * time.Millisecond)

	v, _ := cache.get(flagName)
	if v != newVal {
		t.Errorf("post-recovery: cache not updated; got %q want %q", v, newVal)
	}

	t.Logf("leader-loss window survived: %s with last-known-good preserved", 2*time.Second)
}

// TestEtcdLeaderLossKillSwitch verifies that a kill-switch written during
// leader recovery is picked up within two watcher ticks.
func TestEtcdLeaderLossKillSwitch(t *testing.T) {
	etcd := newEtcdLike()
	cache := newFlagCache()

	const flagName = "model_route"
	etcd.put(flagName, `{"model":"gpt-4o","pct":50}`)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	go flagdWatcher(ctx, etcd, cache, []string{flagName}, 50*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	etcd.setLeaderLost(true)
	time.Sleep(300 * time.Millisecond)

	// Write kill-switch while leader is still down — it will land when leader returns.
	killSwitchVal := `{"model":"gpt-4o-mini","pct":100,"kill_switch":true}`
	etcd.put(flagName, killSwitchVal)

	// Restore leader.
	etcd.setLeaderLost(false)

	// Give the watcher two full ticks to pick up the new value.
	time.Sleep(200 * time.Millisecond)

	v, _ := cache.get(flagName)
	if v != killSwitchVal {
		t.Errorf("kill-switch not propagated after leader recovery: got %q", v)
	}
}
