# semantic-cache-engine

`semantic-cache-engine` is RouteIQ's caching layer: it caches LLM responses keyed by embedding similarity instead of exact prompt match, so near-duplicate prompts (same intent, different wording) can still hit cache, and it reports its hits into LensAI's existing inference-event pipeline (`source='cache_hit'`) instead of a separate metrics table. Full design — embedding pipeline, pgvector schema, per-tenant similarity threshold, false-positive budget, LensAI integration, and TTL/decay policy — is in [`DESIGN.md`](DESIGN.md).

**Status: embedding worker implemented (Day 61). Similarity lookup, LensAI dual-write, and TTL decay are still design-only — see `DESIGN.md` §5–§6.**

## Quickstart

```bash
go test ./...
```

Run the pgvector-touching tests (skipped by default) against a local instance with the `vector` extension installed:

```bash
PGVECTOR_DSN="postgres://user:pass@localhost:5432/lensai" go test -tags=integration ./pkg/cachestore/...
```

Apply the schema first:

```bash
PGVECTOR_DSN="postgres://user:pass@localhost:5432/lensai" bash schema/002_apply.sh
```

## Embedding worker (`cmd/embedworker`)

Reads pending prompts from a JSON Lines file, embeds them via OpenAI's `text-embedding-3-small` (`pkg/embedder`) in batches of 32 (`pkg/worker.BatchSize`, per DESIGN.md's Day 61 plan item), and upserts them into `semantic_cache_entries` (`pkg/cachestore`). Idempotent on `prompt_hash` (`pkg/prompthash`): re-running the same input, or two inputs that only differ in incidental whitespace/case, writes each distinct prompt exactly once.

```bash
export OPENAI_API_KEY=sk-...
export PGVECTOR_DSN="postgres://user:pass@localhost:5432/lensai"
go run ./cmd/embedworker --input pending-prompts.jsonl
```

`pending-prompts.jsonl` is one JSON object per line: `{"tenant_id": "...", "prompt": "...", "response": "..."}`.

## Packages

| Package | Responsibility |
|---|---|
| [`pkg/prompthash`](pkg/prompthash/prompthash.go) | Normalizes and sha256-hashes prompt text — the exact-dup fast path and idempotency key (DESIGN.md §2) |
| [`pkg/embedder`](pkg/embedder/embedder.go) | `Embedder` interface + `OpenAIEmbedder`, DESIGN.md §1's "pluggable interface, not a hard dependency" |
| [`pkg/cachestore`](pkg/cachestore/cachestore.go) | `Writer` interface + `PostgresWriter`, upserts into pgvector's `semantic_cache_entries` (DESIGN.md §2) |
| [`pkg/worker`](pkg/worker/worker.go) | Batches prompts into groups of 32, dedups by `prompt_hash`, wires embedder → store |
| [`cmd/embedworker`](cmd/embedworker/main.go) | CLI entry point |
