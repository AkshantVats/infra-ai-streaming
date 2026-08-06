# cost-budget-enforcer — Benchmarks

**Status:** Go micro-benchmarks (`go test -bench`) against a real, in-process `miniredis` instance — same Lua-eval engine `pkg/store.RedisStore` sends `EVAL` through in production, but a local loopback connection with no network hop. No Docker daemon is available in this sandbox (`docker ps` fails with `dial unix /var/run/docker.sock: connect: no such file or directory`), so there is no live-Redis-over-network number here. **Read "What this does and doesn't measure" before treating any number below as a production SLA.**

Run on: this build's sandbox environment, `go1.25.0`, 4 vCPU (`Intel(R) Xeon(R) Processor @ 2.10GHz`), 2026-08-06.

---

## Why this benchmark exists

Day 69's chaos work (`pkg/middleware`, `pkg/gateway`) adds `config.TenantConfig.FailClosed`: an opt-in policy that rejects requests with `503` instead of forwarding them unmetered when the budget `Store` is unreachable (see DESIGN.md's "Redis outage policy" section below). Every enforcement path — pass, degrade, fail-closed — costs something on the request's critical path, and DESIGN.md's existing "fail open by default" reasoning ("a missed budget check for one request is a bounded, self-correcting cost") is a *risk* argument, not a *latency* one. This benchmark supplies the latency side: what does `pkg/middleware.Wrap` actually add to a request in each mode.

## Methodology

`go test ./pkg/middleware/... -bench . -benchtime=3s -benchmem -count=3`, each benchmark driving `Middleware.Wrap` directly via `httptest.NewRecorder()` — no real HTTP listener, no real network socket for the *server* side. The Redis side of `BenchmarkWrapPass` and `BenchmarkWrapDegrade` is a live TCP connection to `miniredis` on `127.0.0.1` (loopback, not a Unix socket, not mocked at the `Store` interface) so the Lua `EVAL` round trip is real. `BenchmarkWrapFailClosedStoreDown` closes `miniredis` before the benchmark loop starts and disables both of go-redis's retry layers (`MaxRetries: -1` and `DialerRetries: 1`) so the number reflects one failed dial attempt, not five attempts of default backoff — see "What this does and doesn't measure" for why that choice matters.

## Results

| Benchmark | Path exercised | ns/op (3 runs) | µs/op | allocs/op | B/op |
|---|---|---|---|---|---|
| `BenchmarkWrapPass` | Under budget, `Action=Pass`, Redis reachable | 234,500 / 260,989 / 260,159 | **~235–261 µs** | 849 | ~225 KB |
| `BenchmarkWrapDegrade` | Over soft threshold, `Action=Degrade`, JSON body rewrite, Redis reachable | 246,165 / 226,551 / 249,214 | **~227–249 µs** | 872 | ~227 KB |
| `BenchmarkWrapFailClosedStoreDown` | Redis unreachable, `FailClosed=true`, `503` short-circuit | 6,158 / 5,883 / 5,654 | **~5.7–6.2 µs** | 40 | ~1.6 KB |

## Interpretation

**Pass and Degrade cost almost the same.** `rewriteModel`'s JSON decode/re-encode of the request body (Degrade's extra step) doesn't show up as a distinguishable cost against run-to-run noise — both are dominated by the same thing: one `EVAL` round trip to Redis running `pkg/store`'s sliding-window Lua script. The middleware's own logic (threshold comparison, header writes) is not what a caller is paying for here; the Redis call is.

**Fail-closed-on-store-down is ~40× cheaper than a successful check, not more expensive.** This is the opposite of what "chaos" suggests intuitively — rejecting a request costs less than serving one, because there's no Lua script to run and no response body to marshal past a fixed JSON string. The real cost of choosing `FailClosed` isn't per-request CPU time; it's the availability DESIGN.md already discusses (a rejected request instead of a forwarded one). This benchmark's job was to confirm that trade isn't *also* hiding a latency trap, and it doesn't.

**The 40 allocs / 1.6 KB on the fail-closed path is almost entirely the one failed dial attempt** (`net.Dial`'s error path, the pool's error-wrapping) plus the fixed `503` body — there is no Lua script, no JSON unmarshal of a config struct per tenant lookup output, none of Pass/Degrade's ~225 KB of Redis client buffers and JSON scratch space.

## What this does and doesn't measure

| Question | Does this benchmark answer it? |
|---|---|
| What does `pkg/middleware.Wrap` add per request when Redis is healthy? | **Yes, for a loopback `miniredis`** — real `EVAL` round trip, real TCP connection, just not a network hop or a production-sized Redis instance under concurrent load from other tenants. |
| Does `FailClosed` make failing requests slower than passing ones? | **No — measured the opposite.** A fail-closed rejection is faster than a successful check, on this sandbox's loopback Redis. |
| Is ~250 µs/request the real production p50/p99 against a networked Redis (e.g. AWS ElastiCache, cross-AZ)? | **No.** Add real network RTT (typically 0.5–2 ms same-AZ, more cross-AZ) — this number isolates the middleware's own overhead plus loopback Redis, not a production topology. |
| Does `DialerRetries: 1` in the fail-closed benchmark reflect production behavior? | **No, deliberately not** — production leaves go-redis's default retry policy (`MaxRetries: 3`, `DialerRetries: 5` at 100 ms backoff) in place, because that policy is what makes a real, transient blip recoverable without every request layer knowing about it. This benchmark disables both specifically to isolate the middleware's own overhead from retry-backoff sleep time; a request that hits `FailClosed` in production pays those retries' wall-clock time *before* landing on the numbers in this table. |

## Reproduce

```bash
cd cost-budget-enforcer
go test ./pkg/middleware/... -run '^$' -bench . -benchtime=3s -benchmem -count=3
```
