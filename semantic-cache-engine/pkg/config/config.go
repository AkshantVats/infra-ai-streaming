// SPDX-License-Identifier: MIT

// Package config loads the per-tenant similarity threshold DESIGN.md §3
// specifies: a JSON file with a "default" entry and an optional
// "tenants" map of overrides, the same two-level shape
// ingestion/src/rate_limit/tenant_limits.rs already uses for
// TENANT_LIMITS_PATH (see deploy/tenant-limits.example.json), so this
// module's tenant config follows a pattern this repo has already
// established rather than inventing a new one.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// DefaultThreshold is the cosine similarity a lookup requires to count as
// a hit when no config file is loaded and no per-tenant override exists.
// DESIGN.md §3 set 0.94 as the conservative design-time estimate; this
// implementation day's plan item fixes the shipped default at 0.92,
// still on the safe side of "loose semantic matching" but slightly more
// permissive now that §5's cache_hit event gives every hit a trace back
// to the entry it matched, making a wrong hit observable after the fact.
const DefaultThreshold = 0.92

// TenantConfig is one tenant's (or "default"'s) threshold entry.
type TenantConfig struct {
	SimilarityThreshold float64 `json:"similarity_threshold"`
}

// Config is the parsed tenant threshold config file.
type Config struct {
	Default TenantConfig            `json:"default"`
	Tenants map[string]TenantConfig `json:"tenants"`
}

// Load reads and parses a tenant threshold config file at path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if cfg.Default.SimilarityThreshold == 0 {
		cfg.Default.SimilarityThreshold = DefaultThreshold
	}
	return cfg, nil
}

// Threshold returns the similarity threshold a lookup for tenantID must
// meet: the tenant's own override if one is configured, else the
// config's default, else DefaultThreshold. A zero-value Config (no file
// loaded, e.g. CACHE_CONFIG_PATH unset) falls all the way through to
// DefaultThreshold, so callers can treat config loading as optional.
func (c Config) Threshold(tenantID string) float64 {
	if tc, ok := c.Tenants[tenantID]; ok {
		return tc.SimilarityThreshold
	}
	if c.Default.SimilarityThreshold != 0 {
		return c.Default.SimilarityThreshold
	}
	return DefaultThreshold
}
