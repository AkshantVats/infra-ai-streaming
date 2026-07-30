// SPDX-License-Identifier: MIT
// Package objectstore defines a minimal S3-compatible object storage surface
// used to persist exported trace archives. See DESIGN.md at the repo root
// for the export and retention model built on top of this interface.
package objectstore

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned by Get and Delete when the requested key does not
// exist in the store.
var ErrNotFound = errors.New("objectstore: key not found")

// ObjectMeta describes one stored object, as returned by List.
type ObjectMeta struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// ObjectStore is the minimal S3-compatible surface the exporter needs.
// A MinIO-backed implementation and an in-memory fake both satisfy it —
// tests run against the fake, production runs against MinIO.
type ObjectStore interface {
	// Put writes body (exactly size bytes) to key, replacing any existing
	// object at that key.
	Put(ctx context.Context, key string, body io.Reader, size int64) error

	// Get returns a reader over the object stored at key. The caller must
	// Close it. Returns ErrNotFound if key does not exist.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object at key. Returns ErrNotFound if key does not
	// exist.
	Delete(ctx context.Context, key string) error

	// List returns metadata for every object whose key starts with prefix,
	// in no particular order.
	List(ctx context.Context, prefix string) ([]ObjectMeta, error)
}
