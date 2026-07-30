// SPDX-License-Identifier: MIT
package objectstore

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryStore is an in-memory ObjectStore, safe for concurrent use. It is
// the ObjectStore implementation used by all tests, and by any caller
// running without a MinIO endpoint configured.
type MemoryStore struct {
	mu      sync.Mutex
	objects map[string]memoryObject
	// now, if set, overrides time.Now for LastModified stamping — used by
	// tests that need deterministic timestamps.
	now func() time.Time
}

type memoryObject struct {
	data         []byte
	lastModified time.Time
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		objects: make(map[string]memoryObject),
		now:     time.Now,
	}
}

// SetClock overrides the function used to stamp LastModified on Put. Tests
// use this to pin object ages deterministically instead of racing wall-clock
// time; production callers never need it.
func (s *MemoryStore) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// Put reads exactly size bytes from body and stores them under key,
// replacing any existing object at that key.
func (s *MemoryStore) Put(_ context.Context, key string, body io.Reader, size int64) error {
	buf := make([]byte, size)
	if size > 0 {
		if _, err := io.ReadFull(body, buf); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = memoryObject{
		data:         buf,
		lastModified: s.now(),
	}
	return nil
}

// Get returns a reader over the object stored at key. Returns ErrNotFound
// if key does not exist.
func (s *MemoryStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	obj, ok := s.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	// Copy the bytes so later mutation of the store (or the returned buffer)
	// can never alias data the caller is still reading.
	cp := make([]byte, len(obj.data))
	copy(cp, obj.data)
	return io.NopCloser(bytes.NewReader(cp)), nil
}

// Delete removes the object at key. Returns ErrNotFound if key does not
// exist.
func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.objects[key]; !ok {
		return ErrNotFound
	}
	delete(s.objects, key)
	return nil
}

// List returns metadata for every object whose key starts with prefix,
// sorted by key for deterministic output.
func (s *MemoryStore) List(_ context.Context, prefix string) ([]ObjectMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []ObjectMeta
	for key, obj := range s.objects {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		out = append(out, ObjectMeta{
			Key:          key,
			Size:         int64(len(obj.data)),
			LastModified: obj.lastModified,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}
