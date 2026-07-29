// SPDX-License-Identifier: MIT
package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

func TestListOverflowPushPopFIFO(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	ctx := context.Background()
	buf, err := NewListOverflow(ctx, "redis://"+mr.Addr()+"/0", "test:overflow", metrics.NewWithRegistry(prometheus.NewRegistry()))
	if err != nil {
		t.Fatalf("NewListOverflow: %v", err)
	}
	defer buf.Close()

	events := []model.InferenceEvent{
		{TenantID: "a", ModelID: "m1", TimestampUnixMs: 1, LatencyMs: 10, CostUSD: 0.1},
		{TenantID: "b", ModelID: "m2", TimestampUnixMs: 2, LatencyMs: 20, CostUSD: 0.2},
	}
	if err := buf.Push(ctx, events); err != nil {
		t.Fatalf("Push: %v", err)
	}

	depth, err := buf.Depth(ctx)
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if depth != 2 {
		t.Fatalf("depth = %d, want 2", depth)
	}

	got, err := buf.PopN(ctx, 1)
	if err != nil {
		t.Fatalf("PopN: %v", err)
	}
	if len(got) != 1 || got[0].TenantID != "a" {
		t.Fatalf("first pop = %+v, want tenant a", got)
	}

	got, err = buf.PopN(ctx, 5)
	if err != nil {
		t.Fatalf("PopN rest: %v", err)
	}
	if len(got) != 1 || got[0].TenantID != "b" {
		t.Fatalf("second pop = %+v, want tenant b", got)
	}

	depth, err = buf.Depth(ctx)
	if err != nil {
		t.Fatalf("Depth after drain: %v", err)
	}
	if depth != 0 {
		t.Fatalf("depth after drain = %d, want 0", depth)
	}
}

func TestListOverflowPushEmptyNoOp(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	buf, err := NewListOverflow(ctx, "redis://"+mr.Addr()+"/0", "test:empty", metrics.NewWithRegistry(prometheus.NewRegistry()))
	if err != nil {
		t.Fatalf("NewListOverflow: %v", err)
	}
	defer buf.Close()

	if err := buf.Push(ctx, nil); err != nil {
		t.Fatalf("Push nil: %v", err)
	}
	if err := buf.Push(ctx, []model.InferenceEvent{}); err != nil {
		t.Fatalf("Push empty: %v", err)
	}
	depth, err := buf.Depth(ctx)
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if depth != 0 {
		t.Fatalf("depth = %d, want 0", depth)
	}
}

// TestNewListOverflow_BadURL verifies that a malformed Redis URL returns an error.
func TestNewListOverflow_BadURL_ErrorPath(t *testing.T) {
	ctx := context.Background()
	_, err := NewListOverflow(ctx, "not-a-valid-url://??", "key", metrics.NewWithRegistry(prometheus.NewRegistry()))
	if err == nil {
		t.Fatal("expected error for invalid redis URL, got nil")
	}
}

// TestNewListOverflow_UnreachableHost verifies that a Ping failure returns an error.
func TestNewListOverflow_UnreachableHost_ErrorPath(t *testing.T) {
	ctx := context.Background()
	// Port 1 is typically unreachable on loopback.
	_, err := NewListOverflow(ctx, "redis://127.0.0.1:1/0", "key", metrics.NewWithRegistry(prometheus.NewRegistry()))
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

// TestListOverflow_PopN_ZeroOrNeg verifies that PopN with n<=0 is a no-op.
func TestListOverflow_PopN_ZeroOrNeg_EdgeCase(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	ctx := context.Background()
	buf, err := NewListOverflow(ctx, "redis://"+mr.Addr()+"/0", "test:popn", metrics.NewWithRegistry(prometheus.NewRegistry()))
	if err != nil {
		t.Fatalf("NewListOverflow: %v", err)
	}
	defer buf.Close()

	got, err := buf.PopN(ctx, 0)
	if err != nil {
		t.Fatalf("PopN(0): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("PopN(0) should return empty slice, got %d items", len(got))
	}

	got, err = buf.PopN(ctx, -5)
	if err != nil {
		t.Fatalf("PopN(-5): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("PopN(-5) should return empty slice, got %d items", len(got))
	}
}

// TestListOverflow_Depth_ErrorPath verifies Depth returns an error when the server is closed.
func TestListOverflow_Depth_ErrorPath(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	buf, err := NewListOverflow(ctx, "redis://"+mr.Addr()+"/0", "test:depth-err", metrics.NewWithRegistry(prometheus.NewRegistry()))
	if err != nil {
		t.Fatalf("NewListOverflow: %v", err)
	}

	// Close the miniredis server to force subsequent commands to fail.
	mr.Close()

	_, err = buf.Depth(context.Background())
	if err == nil {
		t.Fatal("expected error from Depth after server closed, got nil")
	}
	_ = buf.Close()
}

// TestListOverflow_Push_ErrorAfterServerClose verifies Push returns an error when server is down.
func TestListOverflow_Push_ErrorPath(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	buf, err := NewListOverflow(ctx, "redis://"+mr.Addr()+"/0", "test:push-err", metrics.NewWithRegistry(prometheus.NewRegistry()))
	if err != nil {
		t.Fatalf("NewListOverflow: %v", err)
	}
	defer buf.Close()

	mr.Close() // bring server down

	events := []model.InferenceEvent{
		{TenantID: "x", ModelID: "m1"},
	}
	if err := buf.Push(context.Background(), events); err == nil {
		t.Fatal("expected Push error after server closed, got nil")
	}
}
