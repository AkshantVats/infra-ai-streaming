#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Applies all ClickHouse DDL files in order.
# Usage: CLICKHOUSE_URL=http://localhost:8123 bash schema/002_apply.sh
set -euo pipefail

CLICKHOUSE_URL="${CLICKHOUSE_URL:-http://localhost:8123}"

for f in schema/001_benchmark_runs.sql; do
    echo "Applying $f ..."
    curl -s --fail -X POST "$CLICKHOUSE_URL/" --data-binary "@$f"
    echo "  OK: $f"
done
echo "All DDL applied."
