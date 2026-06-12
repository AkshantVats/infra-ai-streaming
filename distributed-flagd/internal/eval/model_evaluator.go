// SPDX-License-Identifier: MIT
package eval

import (
	"context"
	"fmt"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

// ModelEvaluator resolves the active model version for a given tenant and user.
// It looks up the percentage-rollout flag named "model-rollout:{tenantID}" from
// etcd and applies deterministic FNV-1a hashing on "tenantID:userID" to assign
// a sticky variant. If no flag exists for the tenant, it returns defaultModel.
type ModelEvaluator struct {
	store        *etcdstore.Client
	defaultModel string
}

// NewModelEvaluator constructs an evaluator backed by the etcd store.
// defaultModel is returned when no flag is configured for a tenant.
func NewModelEvaluator(store *etcdstore.Client, defaultModel string) *ModelEvaluator {
	return &ModelEvaluator{store: store, defaultModel: defaultModel}
}

// EvalResult holds the resolved model version and variant name.
type EvalResult struct {
	ModelVersion string // fully-qualified model ID, e.g. "gpt-4-turbo-2024-04-09"
	Variant      string // flag variant name, e.g. "model-v2"
	FlagKey      string // flag name looked up
}

// ResolveModelVersion returns the active model version for the given tenant and user.
// The combination "tenantID:userID" is the hash key, ensuring session-level stickiness.
func (e *ModelEvaluator) ResolveModelVersion(ctx context.Context, tenantID, userID string) (EvalResult, error) {
	flagKey := fmt.Sprintf("model-rollout:%s", tenantID)
	fd, err := e.store.GetFlag(ctx, flagKey)
	if err != nil {
		// No flag configured: return the default model.
		return EvalResult{ModelVersion: e.defaultModel, Variant: "default", FlagKey: flagKey}, nil
	}
	if !fd.Enabled || len(fd.Variants) == 0 {
		return EvalResult{ModelVersion: e.defaultModel, Variant: "default", FlagKey: flagKey}, nil
	}

	variants := make([]PercentageVariant, len(fd.Variants))
	for i, v := range fd.Variants {
		variants[i] = PercentageVariant{Value: v.Value, Weight: v.Weight}
	}

	hashKey := tenantID + ":" + userID
	variant := EvaluatePercentage(flagKey, hashKey, variants)
	if variant == "" {
		return EvalResult{ModelVersion: e.defaultModel, Variant: "default", FlagKey: flagKey}, nil
	}

	// The variant value IS the fully-qualified model ID.
	return EvalResult{ModelVersion: variant, Variant: variant, FlagKey: flagKey}, nil
}
