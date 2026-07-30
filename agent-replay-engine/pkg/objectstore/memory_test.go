// SPDX-License-Identifier: MIT
package objectstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestMemoryStorePutGetRoundTrip(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	want := []byte("hello object store")

	if err := s.Put(ctx, "a/b.txt", bytes.NewReader(want), int64(len(want))); err != nil {
		t.Fatalf("Put: %v", err)
	}

	r, err := s.Get(ctx, "a/b.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = r.Close() }()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMemoryStoreGetNotFound(t *testing.T) {
	s := NewMemoryStore()

	_, err := s.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get missing key: got %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreDeleteRoundTrip(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	body := []byte("to be deleted")

	if err := s.Put(ctx, "gone", bytes.NewReader(body), int64(len(body))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.Delete(ctx, "gone"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get(ctx, "gone")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreDeleteNotFound(t *testing.T) {
	s := NewMemoryStore()

	err := s.Delete(context.Background(), "never-existed")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete missing key: got %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreListPrefixFilter(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	keys := []string{
		"traces/t1/000001-000002.jsonl.zst",
		"traces/t1/000001-000002.jsonl.zst.sha256",
		"traces/t2/000001-000005.jsonl.zst",
		"other/file.txt",
	}
	for _, k := range keys {
		body := []byte("x")
		if err := s.Put(ctx, k, bytes.NewReader(body), int64(len(body))); err != nil {
			t.Fatalf("Put %s: %v", k, err)
		}
	}

	got, err := s.List(ctx, "traces/t1/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d objects under traces/t1/, want 2", len(got))
	}
	wantKeys := map[string]bool{
		"traces/t1/000001-000002.jsonl.zst":        true,
		"traces/t1/000001-000002.jsonl.zst.sha256": true,
	}
	for _, meta := range got {
		if !wantKeys[meta.Key] {
			t.Errorf("unexpected key in List result: %s", meta.Key)
		}
		if meta.Size != 1 {
			t.Errorf("key %s: got size %d, want 1", meta.Key, meta.Size)
		}
	}
}

func TestMemoryStoreListEmptyPrefixMatchesAll(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	for _, k := range []string{"a", "b", "c"} {
		if err := s.Put(ctx, k, bytes.NewReader([]byte("v")), 1); err != nil {
			t.Fatalf("Put %s: %v", k, err)
		}
	}

	got, err := s.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d objects, want 3", len(got))
	}
}

func TestMemoryStoreSetClockStampsLastModified(t *testing.T) {
	s := NewMemoryStore()
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return fixed })

	ctx := context.Background()
	if err := s.Put(ctx, "k", bytes.NewReader([]byte("v")), 1); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := s.List(ctx, "k")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d objects, want 1", len(got))
	}
	if !got[0].LastModified.Equal(fixed) {
		t.Errorf("got LastModified %v, want %v", got[0].LastModified, fixed)
	}
}

func TestMemoryStoreConcurrentPutGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "concurrent"
			body := []byte{byte(n)}
			if err := s.Put(ctx, key, bytes.NewReader(body), 1); err != nil {
				t.Errorf("Put: %v", err)
			}
			if _, err := s.List(ctx, ""); err != nil {
				t.Errorf("List: %v", err)
			}
		}(i)
	}
	wg.Wait()

	r, err := s.Get(ctx, "concurrent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = r.Close() }()
	if _, err := io.ReadAll(r); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
}
