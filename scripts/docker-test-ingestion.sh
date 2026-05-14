#!/usr/bin/env bash
# Run ingestion tests in Docker with capped parallelism (avoids OOM / exit 137 on laptops).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
exec docker run --rm \
  -v "${ROOT}:/workspace" \
  -w /workspace \
  -e CARGO_BUILD_JOBS="${CARGO_BUILD_JOBS:-2}" \
  -e CMAKE_BUILD_PARALLEL_LEVEL="${CMAKE_BUILD_PARALLEL_LEVEL:-2}" \
  rust:1.86-bookworm \
  bash -c 'export PATH="/usr/local/cargo/bin:$PATH" \
    && apt-get update -qq \
    && apt-get install -y -qq cmake pkg-config libssl-dev libsasl2-dev libzstd-dev libcurl4-openssl-dev >/dev/null \
    && cargo test -p ingestion --verbose'
