// SPDX-License-Identifier: MIT
package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

// mockStore records SetFlag / DeleteFlag calls.
type mockStore struct {
	mu      sync.Mutex
	flags   map[string]*etcdstore.FlagData
	deleted []string
}

func newMockStore() *mockStore {
	return &mockStore{flags: make(map[string]*etcdstore.FlagData)}
}

func (m *mockStore) SetFlag(_ context.Context, fd *etcdstore.FlagData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *fd
	m.flags[fd.Name] = &cp
	return nil
}

func (m *mockStore) DeleteFlag(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.flags, name)
	m.deleted = append(m.deleted, name)
	return nil
}

func makeEvent(typ, flagKey string, enabled bool, variants []VariantSpec) []byte {
	cr := FlagDefinition{
		Metadata: ObjectMeta{Name: "fd-" + flagKey, Namespace: "default"},
		Spec:     FlagSpec{FlagKey: flagKey, Enabled: enabled, Variants: variants},
	}
	b, _ := json.Marshal(WatchEvent{Type: typ, Object: cr})
	return append(b, '\n')
}

func TestReconcileAdded(t *testing.T) {
	store := newMockStore()
	events := makeEvent("ADDED", "model-rollout:tenant-a", true, []VariantSpec{
		{Value: "gpt-4o-mini", Weight: 90},
		{Value: "gpt-4o", Weight: 10},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(events)
		// Handler returns → connection closes → scanner hits EOF → watch returns
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	// watch returns when server closes the connection (EOF)
	if err := rec.watch(context.Background()); err != nil {
		t.Fatalf("watch returned unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	fd, ok := store.flags["model-rollout:tenant-a"]
	if !ok {
		t.Fatal("expected flag model-rollout:tenant-a to be synced to store")
	}
	if len(fd.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(fd.Variants))
	}
	if fd.Variants[1].Value != "gpt-4o" || fd.Variants[1].Weight != 10 {
		t.Errorf("unexpected second variant: %+v", fd.Variants[1])
	}
}

func TestReconcileModified(t *testing.T) {
	store := newMockStore()
	// First ADDED at 10%, then MODIFIED to 50%
	data := append(
		makeEvent("ADDED", "model-rollout:tenant-b", true, []VariantSpec{
			{Value: "gpt-4o-mini", Weight: 90},
			{Value: "gpt-4o", Weight: 10},
		}),
		makeEvent("MODIFIED", "model-rollout:tenant-b", true, []VariantSpec{
			{Value: "gpt-4o-mini", Weight: 50},
			{Value: "gpt-4o", Weight: 50},
		})...,
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	if err := rec.watch(context.Background()); err != nil {
		t.Fatalf("watch returned unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	fd := store.flags["model-rollout:tenant-b"]
	if fd == nil {
		t.Fatal("expected flag to be present")
	}
	if fd.Variants[1].Weight != 50 {
		t.Errorf("expected MODIFIED to set gpt-4o weight=50, got %d", fd.Variants[1].Weight)
	}
}

func TestReconcileDeleted(t *testing.T) {
	store := newMockStore()
	_ = store.SetFlag(context.Background(), &etcdstore.FlagData{Name: "model-rollout:tenant-c", Enabled: true})

	events := makeEvent("DELETED", "model-rollout:tenant-c", true, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(events)
	}))
	defer srv.Close()

	rec := New(srv.URL, "default", "", store)
	if err := rec.watch(context.Background()); err != nil {
		t.Fatalf("watch returned unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.flags["model-rollout:tenant-c"]; ok {
		t.Error("expected flag to be deleted from store")
	}
	if len(store.deleted) == 0 || store.deleted[0] != "model-rollout:tenant-c" {
		t.Errorf("expected deleted list to contain tenant-c, got %v", store.deleted)
	}
}

func TestSpecToFlagData(t *testing.T) {
	cr := FlagDefinition{
		Spec: FlagSpec{
			FlagKey: "model-rollout:acme",
			Enabled: true,
			Variants: []VariantSpec{
				{Value: "gpt-4o-mini", Weight: 50},
				{Value: "gpt-4o", Weight: 50},
			},
		},
	}
	fd := specToFlagData(cr)
	if fd.Name != "model-rollout:acme" {
		t.Errorf("unexpected name: %s", fd.Name)
	}
	if !fd.Enabled {
		t.Error("expected enabled=true")
	}
	if len(fd.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(fd.Variants))
	}
	if fd.Variants[0].Value != "gpt-4o-mini" || fd.Variants[0].Weight != 50 {
		t.Errorf("unexpected first variant: %+v", fd.Variants[0])
	}
}

func TestWatchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"reason":"Forbidden"}`))
	}))
	defer srv.Close()

	store := newMockStore()
	rec := New(srv.URL, "default", "", store)
	err := rec.watch(context.Background())
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
}
