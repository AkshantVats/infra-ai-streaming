// SPDX-License-Identifier: MIT
//! Write-ahead log for ingest batches before Kafka produce is acknowledged.

pub mod writer;

pub use writer::{WalEntry, WalWriter};
