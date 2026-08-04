// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "budgets.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeConfig(t, `{"default": {"budget_tokens": 1000000, "fallback_model": "gpt-4o-mini"}}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Default.WindowSeconds != DefaultWindowSeconds {
		t.Fatalf("WindowSeconds = %d, want default %d", cfg.Default.WindowSeconds, DefaultWindowSeconds)
	}
	if cfg.Default.AlertThreshold != DefaultAlertThreshold || cfg.Default.SoftThreshold != DefaultSoftThreshold || cfg.Default.HardThreshold != DefaultHardThreshold {
		t.Fatalf("thresholds = %.2f/%.2f/%.2f, want defaults %.2f/%.2f/%.2f",
			cfg.Default.AlertThreshold, cfg.Default.SoftThreshold, cfg.Default.HardThreshold,
			DefaultAlertThreshold, DefaultSoftThreshold, DefaultHardThreshold)
	}
}

func TestForTenantPrefersOverride(t *testing.T) {
	path := writeConfig(t, `{
		"default": {"budget_tokens": 1000000, "fallback_model": "gpt-4o-mini"},
		"tenants": {"acme": {"budget_tokens": 5000000, "fallback_model": "claude-haiku"}}
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	acme := cfg.ForTenant("acme")
	if acme.BudgetTokens != 5000000 || acme.FallbackModel != "claude-haiku" {
		t.Fatalf("acme config = %+v, want tenant override", acme)
	}

	other := cfg.ForTenant("unknown-tenant")
	if other.BudgetTokens != 1000000 || other.FallbackModel != "gpt-4o-mini" {
		t.Fatalf("unknown-tenant config = %+v, want default", other)
	}
}

func TestForTenantWithNoConfigAppliesPackageDefaults(t *testing.T) {
	var cfg Config
	tc := cfg.ForTenant("nobody")
	if tc.BudgetTokens != 0 {
		t.Fatalf("BudgetTokens = %d, want 0 (unconfigured)", tc.BudgetTokens)
	}
	if tc.WindowSeconds != DefaultWindowSeconds {
		t.Fatalf("WindowSeconds = %d, want default %d even when unconfigured", tc.WindowSeconds, DefaultWindowSeconds)
	}
}

func TestLoadRejectsMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("Load of missing file: want error, got nil")
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	path := writeConfig(t, `{not valid json`)
	if _, err := Load(path); err == nil {
		t.Fatalf("Load of invalid JSON: want error, got nil")
	}
}
