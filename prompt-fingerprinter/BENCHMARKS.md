# prompt-fingerprinter — Benchmarks

Day 75 of the RouteIQ arc. DESIGN.md §6 committed to shipping the exact-match
cache "with a real hit-rate benchmark and `BENCHMARKS.md`" once Week 2 closed
out — this is that measurement, run against `pkg/stack`'s existing L1/L2
composition (`Stack.Get`, unchanged by this day; only `pkg/stack/bench_test.go`
is new).

**Environment note, honestly stated up front:** no live Redis instance is
exercised here — no Docker daemon in this sandbox, the same constraint Days
56, 64, 65, and 70's DESIGN.md §4 all logged. `MemRedis` (an in-process
`map[string]memEntry` under a mutex, see `pkg/stack/memstore.go`) stands in
for L1. That makes every number below a **lower bound on the fixed overhead
this module's own code adds** (fingerprint computation, span creation, map
access) — it excludes the network round trip a real Redis `GET` would pay,
which is the part DESIGN.md §4 still has to validate against a live instance.
`slowL2` (`pkg/stack/bench_test.go`) simulates the L2 side of that same gap: a
fixed 15ms delay standing in for `semantic-cache-engine`'s embedding-model
call + `pgvector` search, so the L1-vs-L2 comparison below is proportioned
correctly even though neither side is talking to real infrastructure.

Reproduce with:

```
go test ./pkg/stack/ -run 'TestLatencyPercentiles_L1Hit|TestHitRate_DuplicateWorkload' -v -count=1
go test ./pkg/stack/ -bench=BenchmarkStack -benchtime=2000x -run=^$ -count=1
```

## 1. Lookup latency — L1 hit vs semantic-only path

`TestLatencyPercentiles_L1Hit` (`pkg/stack/bench_test.go`) times 5,000
sequential `Stack.Get` calls that resolve at L1, records each call's
wall-clock duration individually (not just `testing.B`'s mean `ns/op` — a
mean can hide a heavy tail a percentile target should catch), and sorts them:

| Percentile | Latency |
|---|---|
| p50 | **7.47µs** |
| p95 | 20.40µs |
| p99 | 52.31µs |

Target from `plan.json`'s Day 75 brief was p50 < 2ms. At 7.47µs, the measured
p50 is roughly **260x under budget** — expected, since this number excludes
the Redis network hop the budget was sized for; it is a ceiling on this
module's own added overhead, not a claim about production latency end to end.

`go test -bench` corroborates the same gap at the `testing.B` mean level,
against the `slowL2` 15ms stand-in for the semantic path:

| Benchmark | ns/op | Meaning |
|---|---|---|
| `BenchmarkStack_Get_L1Hit` | 9,804 ns (~9.8µs) | Fingerprint + MemRedis GET, L2 never called |
| `BenchmarkStack_Get_L2Only` | 17,131,155 ns (~17.1ms) | Every request misses L1, pays the full simulated L2 cost |

An L1 hit costs roughly **0.06% of an L2 round trip** in this simulation —
the entire value proposition DESIGN.md §3 argued for from first principles,
now with a number attached.

## 2. Hit rate on a duplicate-heavy workload

`TestHitRate_DuplicateWorkload` (`pkg/stack/bench_test.go`) generates 4,000
requests against a fixed, seeded RNG (seed `75`, reproducible): 35% of
requests reuse one of 50 "canonical" prompts (simulating retries, replayed
batch jobs, and double-submitted forms — DESIGN.md §"Document purpose"'s
motivating cases), the remainder are genuinely novel prompts that can only
ever miss L1 on first sight. 35% is a deliberately conservative dup rate, not
one picked to flatter the cache — real gateway traffic's retry/replay share
varies by workload and this number is meant as an honest lower bound, not a
best case.

Result:

```
workload=4000 dupRate=0.35 pool=50 -> L1 hits=1306 L2 calls=2694
hitRate=32.6% L2 calls avoided vs semantic-only=1306 (32.6%)
```

**32.6% of the workload resolved at L1** without ever reaching L2 — below
the 35% duplicate rate by construction (every prompt's *first* occurrence
necessarily misses L1, since there is nothing to backfill from yet; only
repeats after that first occurrence can hit). At 15ms/call for the simulated
L2 path, avoiding 1,306 of 4,000 calls is roughly **19.6 seconds of L2 latency
removed from this single workload run** — the concrete form of DESIGN.md §1's
claim that "a duplicate like that deserves a cache hit that costs nothing
more than a Redis `GET`."

## Out of scope

No live Redis GET latency (network round trip, connection pool contention,
Redis server load) — the honest gap every prior sandbox-constrained day in
this repo has logged and this one does not close. No production traffic
shape validation — the 35%/50-prompt-pool workload is a plausible retry/replay
simulation, not traffic captured from a real gateway. Both are natural
follow-ups once RouteIQ's gateway wiring (deferred per DESIGN.md's "Out of
scope") lands against live infrastructure.
