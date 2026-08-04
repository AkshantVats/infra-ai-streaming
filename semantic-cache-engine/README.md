# semantic-cache-engine

`semantic-cache-engine` is RouteIQ's caching layer: it caches LLM responses keyed by embedding similarity instead of exact prompt match, so near-duplicate prompts (same intent, different wording) can still hit cache, and it reports its hits into LensAI's existing inference-event pipeline (`source='cache_hit'`) instead of a separate metrics table. Full design — embedding pipeline, pgvector schema, per-tenant similarity threshold, false-positive budget, LensAI integration, and TTL/decay policy — is in [`DESIGN.md`](DESIGN.md).

**Status: embedding worker (Day 61) and cache lookup path (Day 62) implemented. TTL decay is still design-only — see `DESIGN.md` §6–§7.**

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

## Cache lookup (`cmd/cachelookup`)

Reads queries from a JSON Lines file and runs each through DESIGN.md §1's lookup path (`pkg/lookup`): the exact-dup fast path first (`pkg/cachestore.Reader.FindExact`), then a pgvector nearest-neighbor search (`FindNearest`) against the tenant's configured similarity threshold (`pkg/config`, default `0.92`, per-tenant override). A hit emits a `source=cache_hit` event to LensAI (`pkg/lensai`, §5); a miss means the caller should pass the prompt through to inference.

```bash
export OPENAI_API_KEY=sk-...
export PGVECTOR_DSN="postgres://user:pass@localhost:5432/lensai"
export LENSAI_INGEST_URL="http://localhost:8080/ingest"   # optional -- omit to skip cache_hit emission
export CACHE_CONFIG_PATH="deploy/semantic-cache-tenant-thresholds.example.json"  # optional -- omit to use the 0.92 default for every tenant
go run ./cmd/cachelookup --input queries.jsonl
```

`queries.jsonl` is one JSON object per line: `{"tenant_id": "...", "prompt": "..."}`.

## Packages

| Package | Responsibility |
|---|---|
| [`pkg/prompthash`](pkg/prompthash/prompthash.go) | Normalizes and sha256-hashes prompt text — the exact-dup fast path and idempotency key (DESIGN.md §2) |
| [`pkg/embedder`](pkg/embedder/embedder.go) | `Embedder` interface + `OpenAIEmbedder`, DESIGN.md §1's "pluggable interface, not a hard dependency" |
| [`pkg/cachestore`](pkg/cachestore/cachestore.go) | `Writer` + `Reader` interfaces, `PostgresWriter` upserts and queries pgvector's `semantic_cache_entries` (DESIGN.md §2) |
| [`pkg/worker`](pkg/worker/worker.go) | Batches prompts into groups of 32, dedups by `prompt_hash`, wires embedder → store |
| [`pkg/config`](pkg/config/config.go) | Per-tenant similarity threshold config, default `0.92` (DESIGN.md §3) |
| [`pkg/lensai`](pkg/lensai/writer.go) | Dual-writes `source=cache_hit` events to LensAI's `/ingest` (DESIGN.md §5) |
| [`pkg/lookup`](pkg/lookup/lookup.go) | Orchestrates exact-dup fast path → semantic search → threshold check → event emission (DESIGN.md §1) |
| [`cmd/embedworker`](cmd/embedworker/main.go) | CLI entry point for the embedding worker |
| [`cmd/cachelookup`](cmd/cachelookup/main.go) | CLI entry point for the cache lookup path |
