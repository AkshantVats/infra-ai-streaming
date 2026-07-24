// SPDX-License-Identifier: MIT
//! Compile-time build metadata (see `build.rs`).

/// Crate version from `Cargo.toml`.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Short git SHA (or `unknown` when not built from a git checkout).
pub const GIT_SHA: &str = env!("GIT_SHA");

/// UTC build timestamp (RFC3339).
pub const BUILD_TIME: &str = env!("BUILD_TIME");
