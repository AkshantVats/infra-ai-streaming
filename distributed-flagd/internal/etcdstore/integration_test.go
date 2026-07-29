// SPDX-License-Identifier: MIT

//go:build integration

package etcdstore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func etcdEndpoints(t *testing.T) []string {
	t.Helper()
	v := os.Getenv("ETCD_ENDPOINTS")
	if v == "" {
		t.Skip("set ETCD_ENDPOINTS to run etcd integration tests")
	}
	eps := strings.Split(v, ",")
	for i := range eps {
		eps[i] = strings.TrimSpace(eps[i])
	}
	return eps
}

func newEtcdClient(t *testing.T) *clientv3.Client {
	t.Helper()
	endpoints := etcdEndpoints(t)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to etcd: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	return cli
}

// TestPutAndGetFlag verifies that a flag written via SetFlag can be read back with GetFlag.
func TestPutAndGetFlag(t *testing.T) {
	cli := newEtcdClient(t)
	c := NewClient(cli)
	ctx := context.Background()

	fd := &FlagData{
		Name:    "integ-test-flag",
		Value:   "enabled",
		Enabled: true,
	}

	if err := c.SetFlag(ctx, fd); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	t.Cleanup(func() {
		_, _ = cli.Delete(context.Background(), flagPrefix+fd.Name)
	})

	got, err := c.GetFlag(ctx, fd.Name)
	if err != nil {
		t.Fatalf("GetFlag: %v", err)
	}
	if got.Name != fd.Name {
		t.Errorf("Name = %q, want %q", got.Name, fd.Name)
	}
	if got.Value != fd.Value {
		t.Errorf("Value = %q, want %q", got.Value, fd.Value)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
}

// TestGetFlagNotFound verifies that GetFlag returns a "not found" error for missing flags.
func TestGetFlagNotFound(t *testing.T) {
	cli := newEtcdClient(t)
	c := NewClient(cli)
	ctx := context.Background()

	_, err := c.GetFlag(ctx, "definitely-does-not-exist-xyz-abc-123")
	if err == nil {
		t.Fatal("expected error for missing flag, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q does not contain 'not found'", err.Error())
	}
}

// TestListFlags verifies ListFlags returns written flags.
func TestListFlags(t *testing.T) {
	cli := newEtcdClient(t)
	c := NewClient(cli)
	ctx := context.Background()

	names := []string{"list-flag-a", "list-flag-b"}
	for _, name := range names {
		fd := &FlagData{Name: name, Value: "v", Enabled: true}
		if err := c.SetFlag(ctx, fd); err != nil {
			t.Fatalf("SetFlag %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			_, _ = cli.Delete(context.Background(), flagPrefix+name)
		}
	})

	flags, err := c.ListFlags(ctx)
	if err != nil {
		t.Fatalf("ListFlags: %v", err)
	}

	found := map[string]bool{}
	for _, f := range flags {
		found[f.Name] = true
	}
	for _, name := range names {
		if !found[name] {
			t.Errorf("ListFlags: missing flag %q", name)
		}
	}
}

// TestDeleteFlag verifies that a flag can be deleted and is no longer retrievable.
func TestDeleteFlag(t *testing.T) {
	cli := newEtcdClient(t)
	c := NewClient(cli)
	ctx := context.Background()

	fd := &FlagData{Name: "delete-me-flag", Value: "x", Enabled: false}
	if err := c.SetFlag(ctx, fd); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}

	if err := c.DeleteFlag(ctx, fd.Name); err != nil {
		t.Fatalf("DeleteFlag: %v", err)
	}

	_, err := c.GetFlag(ctx, fd.Name)
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}
}

// TestDeleteFlagNotFound verifies that deleting a non-existent flag returns an error.
func TestDeleteFlagNotFound(t *testing.T) {
	cli := newEtcdClient(t)
	c := NewClient(cli)
	ctx := context.Background()

	err := c.DeleteFlag(ctx, "never-existed-flag-xyz")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q does not contain 'not found'", err.Error())
	}
}

// TestWatchFlags verifies that WatchFlags emits a PUT event after SetFlag.
func TestWatchFlags(t *testing.T) {
	cli := newEtcdClient(t)
	c := NewClient(cli)

	watchCtx, watchCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer watchCancel()

	ch := c.WatchFlags(watchCtx)

	flagName := "watch-test-flag"
	fd := &FlagData{Name: flagName, Value: "watched", Enabled: true}
	if err := c.SetFlag(context.Background(), fd); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	t.Cleanup(func() {
		_, _ = cli.Delete(context.Background(), flagPrefix+flagName)
	})

	select {
	case resp, ok := <-ch:
		if !ok {
			t.Fatal("watch channel closed unexpectedly")
		}
		if resp.Err() != nil {
			t.Fatalf("watch error: %v", resp.Err())
		}
		if len(resp.Events) == 0 {
			t.Fatal("received watch response with no events")
		}
		ev := resp.Events[0]
		gotKey := FlagNameFromKey(string(ev.Kv.Key))
		if gotKey != flagName {
			t.Errorf("watch event key = %q, want %q", gotKey, flagName)
		}
	case <-watchCtx.Done():
		t.Fatal("timed out waiting for watch event")
	}
}

// TestFlagNameFromKey verifies the key-stripping helper.
func TestFlagNameFromKey(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"/flags/my-flag", "my-flag"},
		{"/flags/", ""},
		{"/flags/a/b", "a/b"},
	}
	for _, tc := range cases {
		got := FlagNameFromKey(tc.key)
		if got != tc.want {
			t.Errorf("FlagNameFromKey(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

// TestFlagWithVariants verifies that percentage-rollout variants round-trip correctly.
func TestFlagWithVariants(t *testing.T) {
	cli := newEtcdClient(t)
	c := NewClient(cli)
	ctx := context.Background()

	fd := &FlagData{
		Name:    "variant-flag",
		Value:   "",
		Enabled: true,
		Variants: []VariantData{
			{Value: "model-v1", Weight: 90},
			{Value: "model-v2", Weight: 10},
		},
	}
	if err := c.SetFlag(ctx, fd); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	t.Cleanup(func() {
		_, _ = cli.Delete(context.Background(), flagPrefix+fd.Name)
	})

	got, err := c.GetFlag(ctx, fd.Name)
	if err != nil {
		t.Fatalf("GetFlag: %v", err)
	}
	if len(got.Variants) != 2 {
		t.Fatalf("Variants len = %d, want 2", len(got.Variants))
	}
	if got.Variants[0].Value != "model-v1" || got.Variants[0].Weight != 90 {
		t.Errorf("Variants[0] = %+v, want {model-v1 90}", got.Variants[0])
	}
	if got.Variants[1].Value != "model-v2" || got.Variants[1].Weight != 10 {
		t.Errorf("Variants[1] = %+v, want {model-v2 10}", got.Variants[1])
	}
}
