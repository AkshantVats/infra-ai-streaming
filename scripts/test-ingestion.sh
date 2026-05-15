#!/usr/bin/env bash
# Run ingestion crate tests from repo root. Requires: Rust + cmake (for rdkafka).
# On macOS, rdkafka-sys C++ build needs libc++ headers from the CLT SDK (see docs/dev-macos.md).
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "$(uname -s)" == "Darwin" ]]; then
  export SDKROOT="${SDKROOT:-/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk}"
  if [[ -d "${SDKROOT}/usr/include/c++/v1" ]]; then
    export CXXFLAGS="${CXXFLAGS:--isystem ${SDKROOT}/usr/include/c++/v1}"
  fi
fi

exec cargo test -p ingestion "$@"
