// SPDX-License-Identifier: MIT
//
// Integration tests for distributed-flagd against a real etcd instance.
// Set FLAGD_TEST_ETCD (default: localhost:2379) to override the endpoint.
//
// Run locally:
//
//	docker run -d --rm -p 2379:2379 \
//	  -e ALLOW_NONE_AUTHENTICATION=yes bitnami/etcd:3.5
//	FLAGD_INTEGRATION=1 go test -v ./tests/integration/...
package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

func etcdClient(t *testing.T) *clientv3.Client {
	t.Helper()
	endpoint := os.Getenv("FLAGD_TEST_ETCD")
	if endpoint == "" {
		endpoint = "localhost:2379"
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect etcd %s: %v", endpoint, err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	return cli
}

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("FLAGD_INTEGRATION") == "" {
		t.Skip("set FLAGD_INTEGRATION=1 to run")
	}
}

// TestFlagCRUD exercises Create → Get → Update → List → Delete against real etcd.
func TestFlagCRUD(t *testing.T) {
	skipUnlessIntegration(t)
	cli := etcdClient(t)
	store := etcdstore.NewClient(cli)
	ctx := context.Background()

	flag := &etcdstore.FlagData{
		Name:    "integration-test-flag",
		Value:   "gpt-4o-mini",
		Enabled: true,
		Variants: []etcdstore.VariantData{
			{Value: "gpt-4o-mini", Weight: 90},
			{Value: "gpt-4o", Weight: 10},
		},
	}

	// Create
	if err := store.SetFlag(ctx, flag); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteFlag(context.Background(), flag.Name) })

	// Get
	got, err := store.GetFlag(ctx, flag.Name)
	if err != nil {
		t.Fatalf("GetFlag: %v", err)
	}
	if got.Value != flag.Value {
		t.Errorf("value: want %s, got %s", flag.Value, got.Value)
	}
	if len(got.Variants) != 2 {
		t.Errorf("variants: want 2, got %d", len(got.Variants))
	}

	// Update
	flag.Value = "gpt-4o"
	if err := store.SetFlag(ctx, flag); err != nil {
		t.Fatalf("SetFlag (update): %v", err)
	}
	updated, err := store.GetFlag(ctx, flag.Name)
	if err != nil {
		t.Fatalf("GetFlag (after update): %v", err)
	}
	if updated.Value != "gpt-4o" {
		t.Errorf("updated value: want gpt-4o, got %s", updated.Value)
	}

	// List — flag must appear
	flags, err := store.ListFlags(ctx)
	if err != nil {
		t.Fatalf("ListFlags: %v", err)
	}
	found := false
	for _, f := range flags {
		if f.Name == flag.Name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("flag %q missing from list of %d flags", flag.Name, len(flags))
	}

	// Delete
	if err := store.DeleteFlag(ctx, flag.Name); err != nil {
		t.Fatalf("DeleteFlag: %v", err)
	}
	_, err = store.GetFlag(ctx, flag.Name)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

// TestWatchFlags verifies Watch delivers an event when a flag changes in etcd.
func TestWatchFlags(t *testing.T) {
	skipUnlessIntegration(t)
	cli := etcdClient(t)
	store := etcdstore.NewClient(cli)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	watchCh := store.WatchFlags(ctx)

	flag := &etcdstore.FlagData{Name: "integration-watch-flag", Value: "gpt-4o-mini", Enabled: true}
	t.Cleanup(func() { _ = store.DeleteFlag(context.Background(), flag.Name) })

	if err := store.SetFlag(ctx, flag); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}

	select {
	case event, ok := <-watchCh:
		if !ok {
			t.Fatal("watch channel closed unexpectedly")
		}
		if event.Err() != nil {
			t.Fatalf("watch error: %v", event.Err())
		}
		if len(event.Events) == 0 {
			t.Fatal("expected at least one watch event")
		}
		key := string(event.Events[0].Kv.Key)
		if key != "/flags/"+flag.Name {
			t.Errorf("want /flags/%s, got %s", flag.Name, key)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for watch event")
	}
}

// TestSampleFlags seeds the realistic flag set used in a fresh LensAI deployment.
func TestSampleFlags(t *testing.T) {
	skipUnlessIntegration(t)
	cli := etcdClient(t)
	store := etcdstore.NewClient(cli)
	ctx := context.Background()

	samples := []*etcdstore.FlagData{
		{
			Name:    "model-version",
			Value:   "gpt-4o-mini",
			Enabled: true,
			Variants: []etcdstore.VariantData{
				{Value: "gpt-4o-mini", Weight: 90},
				{Value: "gpt-4o", Weight: 10},
			},
		},
		{
			Name:    "retrieval-strategy",
			Value:   "prompt",
			Enabled: true,
			Variants: []etcdstore.VariantData{
				{Value: "prompt", Weight: 50},
				{Value: "rag", Weight: 50},
			},
		},
		{
			Name:    "otel-export-enabled",
			Value:   "false",
			Enabled: true,
		},
	}

	for _, f := range samples {
		if err := store.SetFlag(ctx, f); err != nil {
			t.Fatalf("seed %s: %v", f.Name, err)
		}
		name := f.Name
		t.Cleanup(func() { _ = store.DeleteFlag(context.Background(), name) })
	}

	all, err := store.ListFlags(ctx)
	if err != nil {
		t.Fatalf("ListFlags: %v", err)
	}
	byName := map[string]*etcdstore.FlagData{}
	for _, f := range all {
		byName[f.Name] = f
	}

	for _, want := range samples {
		got, ok := byName[want.Name]
		if !ok {
			t.Errorf("flag %q not found after seed", want.Name)
			continue
		}
		if got.Value != want.Value {
			t.Errorf("%s: want value %s, got %s", want.Name, want.Value, got.Value)
		}
		if got.Enabled != want.Enabled {
			t.Errorf("%s: want enabled %v, got %v", want.Name, want.Enabled, got.Enabled)
		}
	}
}
