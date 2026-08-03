#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Applies the pgvector DDL. Usage: PGVECTOR_DSN=postgres://... bash schema/002_apply.sh
set -euo pipefail

PGVECTOR_DSN="${PGVECTOR_DSN:?set PGVECTOR_DSN, e.g. postgres://user:pass@localhost:5432/lensai}"

echo "Applying schema/001_semantic_cache_entries.sql ..."
psql "$PGVECTOR_DSN" -v ON_ERROR_STOP=1 -f schema/001_semantic_cache_entries.sql
echo "All DDL applied."
