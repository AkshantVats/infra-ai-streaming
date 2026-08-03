-- SPDX-License-Identifier: MIT
-- semantic_cache_entries: one row per (tenant_id, prompt_hash), matching
-- DESIGN.md §2. hnsw over ivfflat: this table starts empty per tenant and
-- grows continuously, and hnsw builds incrementally with no training-set
-- requirement -- see DESIGN.md §2 for the full "why hnsw" writeup.
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS semantic_cache_entries (
    tenant_id      TEXT        NOT NULL,
    prompt_hash    TEXT        NOT NULL,   -- sha256 of normalized prompt text, see pkg/prompthash
    embedding      vector(1536) NOT NULL,
    response       TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_hit_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, prompt_hash)
);

CREATE INDEX IF NOT EXISTS semantic_cache_embedding_idx
    ON semantic_cache_entries
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
