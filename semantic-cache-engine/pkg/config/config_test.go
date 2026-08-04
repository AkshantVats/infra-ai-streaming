// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadAndThresholdWithTenantOverride(t *testing.T) {
	path := writeConfig(t, `{
		"default": {"similarity_threshold": 0.92},
		"tenants": {
			"tenant-a": {"similarity_threshold": 0.97}
		}
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := cfg.Threshold("tenant-a"); got != 0.97 {
		t.Errorf("Threshold(tenant-a) = %v, want 0.97", got)
	}
	if got := cfg.Threshold("tenant-b"); got != 0.92 {
		t.Errorf("Threshold(tenant-b) = %v, want 0.92 (default)", got)
	}
}

func TestThresholdFallsBackToDefaultThreshold(t *testing.T) {
	var cfg Config // zero value: no file loaded
	if got := cfg.Threshold("any-tenant"); got != DefaultThreshold {
		t.Errorf("Threshold on zero Config = %v, want DefaultThreshold %v", got, DefaultThreshold)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Fatal("Load: expected error for missing file, got nil")
	}
}
