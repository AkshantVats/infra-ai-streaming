// SPDX-License-Identifier: MIT

package config

import (
	"sync"
	"testing"
)

func testDefault() TenantConfig {
	return TenantConfig{
		BudgetTokens:   1_000_000,
		WindowSeconds:  86400,
		FallbackModel:  "gpt-4o-mini",
		AlertThreshold: 0.8,
		SoftThreshold:  1.0,
		HardThreshold:  1.2,
	}
}

func TestLiveStorePatchMergesPartialFields(t *testing.T) {
	store := NewLiveStore(Config{Default: testDefault()})

	newBudget := int64(5_000_000)
	before, after, err := store.Patch("acme", TenantConfigPatch{BudgetTokens: &newBudget})
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if before.BudgetTokens != 1_000_000 {
		t.Fatalf("before.BudgetTokens = %d, want 1000000", before.BudgetTokens)
	}
	if after.BudgetTokens != 5_000_000 {
		t.Fatalf("after.BudgetTokens = %d, want 5000000", after.BudgetTokens)
	}
	if after.FallbackModel != "gpt-4o-mini" {
		t.Fatalf("after.FallbackModel = %q, want carried over from default", after.FallbackModel)
	}
	if got := store.ForTenant("acme").BudgetTokens; got != 5_000_000 {
		t.Fatalf("ForTenant after Patch = %d, want 5000000", got)
	}
}

func TestLiveStorePatchRejectsInvalidResultAndDoesNotCommit(t *testing.T) {
	store := NewLiveStore(Config{Default: testDefault()})

	badHard := 0.5 // below soft (1.0)
	_, _, err := store.Patch("acme", TenantConfigPatch{HardThreshold: &badHard})
	if err == nil {
		t.Fatalf("Patch with hard < soft: want error, got nil")
	}

	if got := store.ForTenant("acme").HardThreshold; got != 1.2 {
		t.Fatalf("HardThreshold after rejected patch = %.2f, want unchanged 1.2", got)
	}
}

func TestLiveStorePatchOnSecondCallBuildsOnFirst(t *testing.T) {
	store := NewLiveStore(Config{Default: testDefault()})

	budget1 := int64(2_000_000)
	if _, _, err := store.Patch("acme", TenantConfigPatch{BudgetTokens: &budget1}); err != nil {
		t.Fatalf("first Patch: %v", err)
	}
	model := "claude-haiku"
	_, after, err := store.Patch("acme", TenantConfigPatch{FallbackModel: &model})
	if err != nil {
		t.Fatalf("second Patch: %v", err)
	}
	if after.BudgetTokens != 2_000_000 {
		t.Fatalf("BudgetTokens after second patch = %d, want first patch's value retained", after.BudgetTokens)
	}
	if after.FallbackModel != "claude-haiku" {
		t.Fatalf("FallbackModel = %q, want claude-haiku", after.FallbackModel)
	}
}

func TestLiveStoreSetOverwritesOutright(t *testing.T) {
	store := NewLiveStore(Config{Default: testDefault()})
	restored := testDefault()
	restored.BudgetTokens = 42

	store.Set("acme", restored)
	if got := store.ForTenant("acme").BudgetTokens; got != 42 {
		t.Fatalf("BudgetTokens after Set = %d, want 42", got)
	}
}

func TestLiveStoreConcurrentPatchesDoNotRace(t *testing.T) {
	store := NewLiveStore(Config{Default: testDefault()})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int64) {
			defer wg.Done()
			budget := 1_000_000 + n
			_, _, _ = store.Patch("acme", TenantConfigPatch{BudgetTokens: &budget})
		}(int64(i))
	}
	wg.Wait()

	// No assertion on final value (last-writer-wins is expected under
	// concurrent patches to the same tenant) — this test's job is to
	// run clean under -race, not to pin a winner.
	_ = store.ForTenant("acme")
}

func TestValidateRejectsNonPositiveBudget(t *testing.T) {
	tc := testDefault()
	tc.BudgetTokens = 0
	if err := tc.Validate(); err == nil {
		t.Fatalf("Validate with zero budget: want error, got nil")
	}
}

func TestValidateRejectsOutOfOrderThresholds(t *testing.T) {
	tc := testDefault()
	tc.AlertThreshold = 1.1 // above soft (1.0)
	if err := tc.Validate(); err == nil {
		t.Fatalf("Validate with alert > soft: want error, got nil")
	}
}

func TestValidateAcceptsWellFormedConfig(t *testing.T) {
	if err := testDefault().Validate(); err != nil {
		t.Fatalf("Validate on well-formed config: %v", err)
	}
}
