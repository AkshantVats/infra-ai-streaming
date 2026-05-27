//! Per-tenant rate-limit configuration loaded from a JSON file.
//!
//! Schema: `deploy/tenant-limits.example.json`. Unknown tenants fall back to
//! the `"default"` entry (or env-level `RATE_LIMIT_DEFAULT_RPS` /
//! `RATE_LIMIT_BURST_MULTIPLIER` if the file is absent).

use std::collections::HashMap;
use std::path::Path;

use serde::Deserialize;

/// Limits for a single tenant (or the default).
#[derive(Debug, Clone, Copy, Deserialize, PartialEq)]
pub struct TenantLimit {
    pub max_events_per_sec: u32,
    pub burst_multiplier: f32,
}

/// Root schema of the tenant-limits JSON file.
#[derive(Debug, Clone, Deserialize)]
struct TenantLimitsFile {
    default: TenantLimit,
    #[serde(default)]
    tenants: HashMap<String, TenantLimit>,
}

/// In-memory tenant limit resolver. Immutable after construction.
#[derive(Debug, Clone)]
pub struct TenantLimitsConfig {
    default: TenantLimit,
    tenants: HashMap<String, TenantLimit>,
}

impl TenantLimitsConfig {
    /// Build from env-level defaults only (no file).
    pub fn from_defaults(default_rps: u32, burst_multiplier: f32) -> Self {
        Self {
            default: TenantLimit {
                max_events_per_sec: default_rps,
                burst_multiplier,
            },
            tenants: HashMap::new(),
        }
    }

    /// Load from a JSON file; validation errors are returned (not swallowed).
    pub fn from_file(path: &Path) -> anyhow::Result<Self> {
        let raw = std::fs::read_to_string(path)
            .map_err(|e| anyhow::anyhow!("read tenant limits file {}: {e}", path.display()))?;
        let parsed: TenantLimitsFile = serde_json::from_str(&raw)
            .map_err(|e| anyhow::anyhow!("parse tenant limits file {}: {e}", path.display()))?;

        for (id, limit) in &parsed.tenants {
            anyhow::ensure!(
                limit.max_events_per_sec > 0,
                "tenant {id}: max_events_per_sec must be > 0"
            );
            anyhow::ensure!(
                limit.burst_multiplier >= 1.0,
                "tenant {id}: burst_multiplier must be >= 1.0"
            );
        }
        anyhow::ensure!(
            parsed.default.max_events_per_sec > 0,
            "default: max_events_per_sec must be > 0"
        );
        anyhow::ensure!(
            parsed.default.burst_multiplier >= 1.0,
            "default: burst_multiplier must be >= 1.0"
        );

        Ok(Self {
            default: parsed.default,
            tenants: parsed.tenants,
        })
    }

    /// Resolve limits for a tenant, falling back to the default.
    pub fn resolve(&self, tenant_id: &str) -> TenantLimit {
        self.tenants.get(tenant_id).copied().unwrap_or(self.default)
    }

    pub fn default_limit(&self) -> TenantLimit {
        self.default
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    #[test]
    fn from_defaults_resolves_unknown_tenant() {
        let cfg = TenantLimitsConfig::from_defaults(500, 2.0);
        let limit = cfg.resolve("unknown-tenant");
        assert_eq!(limit.max_events_per_sec, 500);
        assert!((limit.burst_multiplier - 2.0).abs() < f32::EPSILON);
    }

    #[test]
    fn from_file_resolves_known_tenant() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("limits.json");
        let mut f = std::fs::File::create(&path).unwrap();
        f.write_all(
            br#"{
              "default": {"max_events_per_sec": 1000, "burst_multiplier": 2.0},
              "tenants": {
                "acme": {"max_events_per_sec": 50, "burst_multiplier": 1.5}
              }
            }"#,
        )
        .unwrap();
        let cfg = TenantLimitsConfig::from_file(&path).unwrap();
        let acme = cfg.resolve("acme");
        assert_eq!(acme.max_events_per_sec, 50);
        assert!((acme.burst_multiplier - 1.5).abs() < f32::EPSILON);

        let unknown = cfg.resolve("other");
        assert_eq!(unknown.max_events_per_sec, 1000);
    }

    #[test]
    fn from_file_rejects_zero_rps() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("limits.json");
        std::fs::write(
            &path,
            r#"{"default":{"max_events_per_sec":0,"burst_multiplier":1.0},"tenants":{}}"#,
        )
        .unwrap();
        let err = TenantLimitsConfig::from_file(&path).unwrap_err();
        assert!(err.to_string().contains("max_events_per_sec must be > 0"));
    }

    #[test]
    fn from_file_rejects_burst_below_one() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("limits.json");
        std::fs::write(
            &path,
            r#"{"default":{"max_events_per_sec":100,"burst_multiplier":0.5},"tenants":{}}"#,
        )
        .unwrap();
        let err = TenantLimitsConfig::from_file(&path).unwrap_err();
        assert!(err.to_string().contains("burst_multiplier must be >= 1.0"));
    }

    #[test]
    fn from_file_rejects_missing_file() {
        let err = TenantLimitsConfig::from_file(Path::new("/nonexistent/limits.json")).unwrap_err();
        assert!(err.to_string().contains("read tenant limits file"));
    }

    #[test]
    fn from_file_rejects_malformed_json() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("limits.json");
        std::fs::write(&path, "not json").unwrap();
        let err = TenantLimitsConfig::from_file(&path).unwrap_err();
        assert!(err.to_string().contains("parse tenant limits file"));
    }
}
