// SPDX-License-Identifier: MIT
package server

import (
	"sync"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

// registry manages a set of per-stream channels for fan-out delivery.
// A single background etcd watcher feeds all open streams via broadcast.
type registry struct {
	mu      sync.RWMutex
	streams map[string]chan *etcdstore.FlagData
}

func newRegistry() *registry {
	return &registry{streams: make(map[string]chan *etcdstore.FlagData)}
}

// subscribe returns a buffered channel that will receive flag mutations.
func (r *registry) subscribe(id string) chan *etcdstore.FlagData {
	ch := make(chan *etcdstore.FlagData, 64)
	r.mu.Lock()
	r.streams[id] = ch
	r.mu.Unlock()
	return ch
}

// unsubscribe closes and removes the channel for id.
func (r *registry) unsubscribe(id string) {
	r.mu.Lock()
	if ch, ok := r.streams[id]; ok {
		close(ch)
		delete(r.streams, id)
	}
	r.mu.Unlock()
}

// broadcast sends fd to all subscribers. Slow clients are dropped (non-blocking).
func (r *registry) broadcast(fd *etcdstore.FlagData) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ch := range r.streams {
		select {
		case ch <- fd:
		default:
		}
	}
}
