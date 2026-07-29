#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Applies all ClickHouse DDL files in order.
# Usage: CLICKHOUSE_URL=http://localhost:8123 bash schema/005_apply.sh
set -euo pipefail

CLICKHOUSE_URL="${CLICKHOUSE_URL:-http://localhost:8123}"

for f in schema/001_tool_calls.sql \
          schema/002_tool_stats_1m_mv.sql \
          schema/003_tool_cost_rollup_mv.sql \
          schema/004_tool_duration_alert_mv.sql; do
    echo "Applying $f ..."
    curl -s --fail -X POST "$CLICKHOUSE_URL/" --data-binary "@$f"
    echo "  OK: $f"
done
echo "All DDL applied."
