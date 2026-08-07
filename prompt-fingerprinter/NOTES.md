# prompt-fingerprinter — project-blog bullets (for Day 87)

Working notes captured on Day 75 for the Day 87 project blog ("Fallback
Chains — Sub-10ms Streaming Proxy, Three-Product Integration, and RouteIQ
Week-4 Architecture"), which rolls up the RouteIQ arc's Week 2-3 modules.
Not for publication as-is — bullets to pull from when that post is drafted.

- **What it is.** The third RouteIQ module (after `semantic-cache-engine`,
  Day 60; `cost-budget-enforcer`, Day 65): a cheap exact-match cache tier that
  sits in front of the semantic/embedding lookup, so a byte-identical retry,
  replayed batch job, or double-submitted request never has to pay an
  embedding-model round trip to get an answer that was already computed.
- **The one-line pitch.** Normalize → SHA-256 → tenant-scoped Redis key →
  `Stack.Get` tries L1 first, falls through to L2 on a miss, backfills L1 on
  an L2 hit so the next identical prompt resolves at L1 too.
- **The number that matters.** Day 75's benchmark: on a workload with a
  conservative 35% duplicate rate, 32.6% of requests resolved at L1 without
  ever reaching the semantic path — roughly 260x under the p50 <2ms latency
  budget (measured p50: 7.47µs, MemRedis-backed; real number pending a live
  Redis instance). Full methodology and the "why MemRedis, not live Redis"
  caveat are in `BENCHMARKS.md`.
- **The design decision worth explaining in prose.** Why SHA-256 over a
  faster non-cryptographic hash (xxhash, murmur3) when the hash itself isn't
  the bottleneck: a collision here means serving one tenant's cached response
  for a different prompt, and the module already reuses the `prompt_hash`
  column `semantic-cache-engine` reserved on Day 60 without ever populating
  it — this is the module that finally gives that column a reader.
  (DESIGN.md §2 has the full argument.)
- **The failure-mode story.** The Day 73 collision drill: since a real
  SHA-256 collision can't be manufactured in a test, the drill seeds
  `MemRedis` directly at the key a collision *would* produce and shows what
  the stack does about it — tenant-scoped keys contain the blast radius to
  one tenant, and the 30-day `HardTTL` bounds how long a colliding entry can
  serve a wrong answer. Good material for "designing for a failure mode you
  can't actually trigger."
- **Where it sits in the 3-product topology.** `prompt-fingerprinter` is
  pure Go, standard-library-adjacent (only the OTel SDK as a real
  dependency), and depends on `semantic-cache-engine` only through the
  `L2Store` interface — no shared `go.work`, no direct import. That's the
  shape Day 87's `fallback-chain` integration doc will need to describe for
  all three products: independently testable modules wired together at a
  gateway layer, not a monolith.
- **Open thread into Week 3.** Day 76 adds the admin
  `PUT /tenants/{id}/fingerprint-rules` endpoint and starts emitting
  `cache_hit_type=exact` to the LensAI Kafka topic — the point where this
  module's `cache_hit_exact` source value (DESIGN.md §4) actually starts
  showing up on a dashboard instead of just being reserved.
