// SPDX-License-Identifier: MIT

package config

import "sync"

// TenantConfigPatch is a partial update to a tenant's budget config.
// Every field is a pointer so LiveStore.Patch can tell "not sent" (nil)
// apart from "sent as its zero value" (e.g. a caller deliberately
// setting fallback_model back to "") — the same distinction a plain
// TenantConfig can't make, since json.Unmarshal into TenantConfig
// leaves an omitted field indistinguishable from an explicit zero.
type TenantConfigPatch struct {
	BudgetTokens    *int64   `json:"budget_tokens"`
	WindowSeconds   *int64   `json:"window_seconds"`
	FallbackModel   *string  `json:"fallback_model"`
	AlertWebhookURL *string  `json:"alert_webhook_url"`
	AlertThreshold  *float64 `json:"alert_threshold"`
	SoftThreshold   *float64 `json:"soft_threshold"`
	HardThreshold   *float64 `json:"hard_threshold"`
}

// apply returns base with every non-nil field of p overlaid on top.
func (p TenantConfigPatch) apply(base TenantConfig) TenantConfig {
	if p.BudgetTokens != nil {
		base.BudgetTokens = *p.BudgetTokens
	}
	if p.WindowSeconds != nil {
		base.WindowSeconds = *p.WindowSeconds
	}
	if p.FallbackModel != nil {
		base.FallbackModel = *p.FallbackModel
	}
	if p.AlertWebhookURL != nil {
		base.AlertWebhookURL = *p.AlertWebhookURL
	}
	if p.AlertThreshold != nil {
		base.AlertThreshold = *p.AlertThreshold
	}
	if p.SoftThreshold != nil {
		base.SoftThreshold = *p.SoftThreshold
	}
	if p.HardThreshold != nil {
		base.HardThreshold = *p.HardThreshold
	}
	return base
}

// LiveStore wraps a Config loaded once at startup (Load) with a mutex,
// so an Admin API handler can mutate one tenant's budget in place while
// enforcer.Enforcer instances elsewhere in the same process keep
// reading — the gap Day 65's DESIGN.md and Day 66's file-based Config
// both left open: a budget change previously required a process
// restart to pick up an edited JSON file.
type LiveStore struct {
	mu  sync.RWMutex
	cfg Config
}

// NewLiveStore wraps cfg (typically the result of Load) for concurrent
// read/patch access.
func NewLiveStore(cfg Config) *LiveStore {
	return &LiveStore{cfg: cfg}
}

// ForTenant returns tenantID's current config, same semantics as
// Config.ForTenant.
func (s *LiveStore) ForTenant(tenantID string) TenantConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.ForTenant(tenantID)
}

// Patch applies patch on top of tenantID's current config (its own
// tenant entry if one exists, else the file default) and, if the
// result passes Validate, commits it as tenantID's new tenant-specific
// entry. On validation failure the store is left unchanged and after
// is the zero value — callers must check err, not just compare before
// and after.
func (s *LiveStore) Patch(tenantID string, patch TenantConfigPatch) (before, after TenantConfig, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	before = s.cfg.ForTenant(tenantID)
	candidate := patch.apply(before)
	if err := candidate.Validate(); err != nil {
		return before, TenantConfig{}, err
	}

	if s.cfg.Tenants == nil {
		s.cfg.Tenants = make(map[string]TenantConfig)
	}
	s.cfg.Tenants[tenantID] = candidate
	return before, candidate, nil
}

// Set overwrites tenantID's entry outright, bypassing patch semantics.
// It exists for callers that already hold a validated TenantConfig and
// need to restore one — e.g. rolling a Patch back when the audit log
// it must be paired with fails to publish.
func (s *LiveStore) Set(tenantID string, tc TenantConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.Tenants == nil {
		s.cfg.Tenants = make(map[string]TenantConfig)
	}
	s.cfg.Tenants[tenantID] = tc
}
