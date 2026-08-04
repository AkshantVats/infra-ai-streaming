// SPDX-License-Identifier: MIT

// Package config loads per-tenant budget configuration: the token
// budget itself, the window size, the fallback model DESIGN.md §3
// requires an explicit (never-guessed) mapping for, and the alert
// webhook URL §4 posts to. Shape mirrors semantic-cache-engine's
// pkg/config — a JSON file with a "default" entry and a "tenants"
// override map — so this repo's tenant-config convention stays one
// convention, not one per module.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Default thresholds, as fractions of a tenant's budget. DESIGN.md §2
// fixes these at 80/100/120 and explains why the 20-point gap between
// soft and hard exists (room for fallback-routed traffic to run before
// the hard stop actually engages).
const (
	DefaultAlertThreshold = 0.8
	DefaultSoftThreshold  = 1.0
	DefaultHardThreshold  = 1.2
)

// DefaultWindowSeconds is one day — cost-budget-enforcer's windows are
// calendar-day budgets, not the request-rate windows ingestion's
// rate_limit package uses.
const DefaultWindowSeconds = int64(86400)

// TenantConfig is one tenant's budget configuration.
type TenantConfig struct {
	BudgetTokens    int64   `json:"budget_tokens"`
	WindowSeconds   int64   `json:"window_seconds"`
	FallbackModel   string  `json:"fallback_model"`
	AlertWebhookURL string  `json:"alert_webhook_url"`
	AlertThreshold  float64 `json:"alert_threshold"`
	SoftThreshold   float64 `json:"soft_threshold"`
	HardThreshold   float64 `json:"hard_threshold"`
}

// Config is the parsed tenant budget config file.
type Config struct {
	Default TenantConfig            `json:"default"`
	Tenants map[string]TenantConfig `json:"tenants"`
}

// Load reads and parses a tenant budget config file at path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	cfg.Default = applyDefaults(cfg.Default)
	for id, tc := range cfg.Tenants {
		cfg.Tenants[id] = applyDefaults(tc)
	}
	return cfg, nil
}

func applyDefaults(tc TenantConfig) TenantConfig {
	if tc.WindowSeconds == 0 {
		tc.WindowSeconds = DefaultWindowSeconds
	}
	if tc.AlertThreshold == 0 {
		tc.AlertThreshold = DefaultAlertThreshold
	}
	if tc.SoftThreshold == 0 {
		tc.SoftThreshold = DefaultSoftThreshold
	}
	if tc.HardThreshold == 0 {
		tc.HardThreshold = DefaultHardThreshold
	}
	return tc
}

// ForTenant returns tenantID's config: its own entry if one is
// configured, else the file's default, else a zero-value TenantConfig
// with package defaults applied and BudgetTokens left at 0 — callers
// must treat a zero BudgetTokens as "no budget configured" (unlimited
// or reject, depending on policy) rather than "budget of zero tokens",
// since Load never invents a nonzero budget.
func (c Config) ForTenant(tenantID string) TenantConfig {
	if tc, ok := c.Tenants[tenantID]; ok {
		return tc
	}
	if c.Default.BudgetTokens != 0 {
		return c.Default
	}
	return applyDefaults(TenantConfig{})
}
