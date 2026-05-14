#!/usr/bin/env bash
# Run ingestion crate tests from repo root. Requires: Rust + cmake (for rdkafka).
set -euo pipefail
cd "$(dirname "$0")/.."
exec cargo test -p ingestion "$@"
