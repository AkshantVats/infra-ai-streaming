# semantic-cache-engine

`semantic-cache-engine` is RouteIQ's caching layer: it caches LLM responses keyed by embedding similarity instead of exact prompt match, so near-duplicate prompts (same intent, different wording) can still hit cache, and it reports its hits into LensAI's existing inference-event pipeline (`source='cache_hit'`) instead of a separate metrics table. Full design — embedding pipeline, pgvector schema, per-tenant similarity threshold, false-positive budget, LensAI integration, and TTL/decay policy — is in [`DESIGN.md`](DESIGN.md).

**Status: design-only, no runtime code yet.**
