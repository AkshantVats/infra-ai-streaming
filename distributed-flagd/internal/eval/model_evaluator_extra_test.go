// SPDX-License-Identifier: MIT
package eval

import (
	"context"
	"errors"
	"testing"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

// fakeFlagStore is a test double for FlagStore.
type fakeFlagStore struct {
	flags map[string]*etcdstore.FlagData
	err   error
}

func (f *fakeFlagStore) GetFlag(_ context.Context, name string) (*etcdstore.FlagData, error) {
	if f.err != nil {
		return nil, f.err
	}
	fd, ok := f.flags[name]
	if !ok {
		return nil, errors.New("flag not found: " + name)
	}
	return fd, nil
}

// TestNewModelEvaluator verifies the constructor wires up correctly.
func TestNewModelEvaluator(t *testing.T) {
	store := &fakeFlagStore{}
	e := NewModelEvaluator(store, "gpt-4o-mini")
	if e == nil {
		t.Fatal("NewModelEvaluator returned nil")
	}
	if e.defaultModel != "gpt-4o-mini" {
		t.Errorf("defaultModel: want gpt-4o-mini, got %q", e.defaultModel)
	}
}

// TestResolveModelVersion_FlagNotFound returns the defaultModel when etcd has no flag.
func TestResolveModelVersion_FlagNotFound(t *testing.T) {
	store := &fakeFlagStore{flags: map[string]*etcdstore.FlagData{}}
	e := NewModelEvaluator(store, "gpt-4o-mini")

	result, err := e.ResolveModelVersion(context.Background(), "acme", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModelVersion != "gpt-4o-mini" {
		t.Errorf("want gpt-4o-mini, got %q", result.ModelVersion)
	}
	if result.Variant != "default" {
		t.Errorf("want default variant, got %q", result.Variant)
	}
	if result.FlagKey != "model-rollout:acme" {
		t.Errorf("want model-rollout:acme, got %q", result.FlagKey)
	}
}

// TestResolveModelVersion_StoreError also returns the defaultModel on store error.
func TestResolveModelVersion_StoreError(t *testing.T) {
	store := &fakeFlagStore{err: errors.New("etcd unreachable")}
	e := NewModelEvaluator(store, "fallback-model")

	result, err := e.ResolveModelVersion(context.Background(), "tenant-x", "u-99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModelVersion != "fallback-model" {
		t.Errorf("want fallback-model, got %q", result.ModelVersion)
	}
	if result.Variant != "default" {
		t.Errorf("want default, got %q", result.Variant)
	}
}

// TestResolveModelVersion_FlagDisabled returns default when flag is disabled.
func TestResolveModelVersion_FlagDisabled(t *testing.T) {
	store := &fakeFlagStore{flags: map[string]*etcdstore.FlagData{
		"model-rollout:acme": {
			Name:    "model-rollout:acme",
			Enabled: false,
			Variants: []etcdstore.VariantData{
				{Value: "gpt-4o", Weight: 100},
			},
		},
	}}
	e := NewModelEvaluator(store, "default-model")

	result, err := e.ResolveModelVersion(context.Background(), "acme", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModelVersion != "default-model" {
		t.Errorf("want default-model for disabled flag, got %q", result.ModelVersion)
	}
}

// TestResolveModelVersion_FlagNoVariants returns default when variant list is empty.
func TestResolveModelVersion_FlagNoVariants(t *testing.T) {
	store := &fakeFlagStore{flags: map[string]*etcdstore.FlagData{
		"model-rollout:corp": {
			Name:     "model-rollout:corp",
			Enabled:  true,
			Variants: []etcdstore.VariantData{},
		},
	}}
	e := NewModelEvaluator(store, "gpt-3.5-turbo")

	result, err := e.ResolveModelVersion(context.Background(), "corp", "u-0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModelVersion != "gpt-3.5-turbo" {
		t.Errorf("want gpt-3.5-turbo for empty variants, got %q", result.ModelVersion)
	}
}

// TestResolveModelVersion_HappyPath verifies the correct variant is resolved.
func TestResolveModelVersion_HappyPath(t *testing.T) {
	// With 100% weight on one variant the result must always be that variant.
	store := &fakeFlagStore{flags: map[string]*etcdstore.FlagData{
		"model-rollout:startup": {
			Name:    "model-rollout:startup",
			Enabled: true,
			Variants: []etcdstore.VariantData{
				{Value: "gpt-4o-mini", Weight: 100},
			},
		},
	}}
	e := NewModelEvaluator(store, "default")

	result, err := e.ResolveModelVersion(context.Background(), "startup", "u-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModelVersion != "gpt-4o-mini" {
		t.Errorf("want gpt-4o-mini, got %q", result.ModelVersion)
	}
	if result.Variant != "gpt-4o-mini" {
		t.Errorf("want variant gpt-4o-mini, got %q", result.Variant)
	}
	if result.FlagKey != "model-rollout:startup" {
		t.Errorf("want flag key model-rollout:startup, got %q", result.FlagKey)
	}
}

// TestResolveModelVersion_Deterministic same tenant+user always maps to the same variant.
func TestResolveModelVersion_Deterministic(t *testing.T) {
	store := &fakeFlagStore{flags: map[string]*etcdstore.FlagData{
		"model-rollout:repeatable": {
			Name:    "model-rollout:repeatable",
			Enabled: true,
			Variants: []etcdstore.VariantData{
				{Value: "model-a", Weight: 50},
				{Value: "model-b", Weight: 50},
			},
		},
	}}
	e := NewModelEvaluator(store, "default")

	first, _ := e.ResolveModelVersion(context.Background(), "repeatable", "user-sticky")
	for i := 0; i < 20; i++ {
		got, _ := e.ResolveModelVersion(context.Background(), "repeatable", "user-sticky")
		if got.ModelVersion != first.ModelVersion {
			t.Fatalf("non-deterministic: iteration %d got %q, first was %q",
				i, got.ModelVersion, first.ModelVersion)
		}
	}
}

// TestResolveModelVersion_ZeroWeightFallback verifies fallback to last variant
// when all weights are 0.
func TestResolveModelVersion_ZeroWeightFallback(t *testing.T) {
	store := &fakeFlagStore{flags: map[string]*etcdstore.FlagData{
		"model-rollout:edge": {
			Name:    "model-rollout:edge",
			Enabled: true,
			Variants: []etcdstore.VariantData{
				{Value: "model-first", Weight: 0},
				{Value: "model-last", Weight: 0},
			},
		},
	}}
	e := NewModelEvaluator(store, "default")
	result, err := e.ResolveModelVersion(context.Background(), "edge", "anyone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// EvaluatePercentage falls back to last when all weights are zero.
	if result.ModelVersion != "model-last" {
		t.Errorf("want model-last (fallback), got %q", result.ModelVersion)
	}
}
