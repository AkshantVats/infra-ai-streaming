# model-quality-scorer — Design Document

| Field | Value |
|-------|-------|
| **Product arc** | RouteIQ (opened Day 60 with `semantic-cache-engine`; `cost-budget-enforcer` joined Day 65, `prompt-fingerprinter` Day 70; `model-quality-scorer` is the arc's fourth module) |
| **Status** | Design-only. No runtime code, migrations, or new Kafka topics yet. |
| **Precedent** | Same shape as `prompt-fingerprinter`'s Day 70 DESIGN.md-first day, itself following `cost-budget-enforcer`'s Day 65 and `semantic-cache-engine`'s Day 60 precedent: the module opens with a written design before any implementation. |

**Document purpose.** Every other RouteIQ module so far answers "was this request served correctly and cheaply" — a cache hit, a budget check, a fingerprint match. None of them answer "was the response actually *good*." `model-quality-scorer` closes that gap: it samples a slice of live traffic, asks a cheap judge model to grade the response against a rubric, and stores the score so a routing decision (which model, which cache tier, which fallback) can eventually be justified by quality, not just by cost and latency. This document records the five decisions needed before any code is written: which model judges, how a rubric is shaped, how sampled requests reach the judge without touching the request's own latency budget, what throughput target the queue is sized against, and what happens when the judge itself fails to answer in time.

---

## 1. Judge model — Claude Haiku, not the model being judged

The judge's own cost and latency are pure overhead on top of the request already served — nothing about grading a response makes the original request faster, cheaper, or more correct, so the grading step has to justify its existence by staying cheap enough that nobody considers turning it off. That rules out judging every request with a model as large as the one that produced the response: a judge call at parity with a frontier model's cost would roughly double spend on every sampled request for a signal that is, by design, only ever sampled rather than universal.

**Haiku is the judge for the same reason `cost-budget-enforcer`'s soft-limit path routes to a cheaper fallback model instead of rejecting outright (Day 65 DESIGN.md §3): the job at hand doesn't need the expensive model's full capability.** Grading against a fixed rubric with numeric, weighted criteria is a narrower task than generating the original response — closer to structured classification than open-ended generation — and a smaller model graded against the same rubric consistently is more valuable here than a larger model graded inconsistently. Consistency matters more than sophistication: two runs of the same rubric against the same response should land on the same score, and a smaller, cheaper model run at a fixed temperature is easier to keep boring in that specific sense than a larger one.

**No judge grades its own output.** A tenant routed to Haiku for their actual inference traffic still gets judged by Haiku — the judge model is fixed per deployment, not selected per tenant's routing target, so "the model that answered" and "the model that grades" are never coupled to the same routing decision by accident.

So: the judge model is chosen for the same reason every other RouteIQ cost decision has been — the cheapest model that can do the specific, narrow job correctly, not the most capable one available.

---

## 2. Rubric schema — one `JudgeRubric` per task type, not one global rubric

"Good" means different things for a summarization response than for a tool-call response than for a structured-extraction response — a rubric that tried to grade all three the same way would either be too vague to score anything (generic "helpfulness") or quietly biased toward whichever task type it was actually written with in mind. `model-quality-scorer` keys its rubric by `task_type`, the same dimension `cost-budget-enforcer`'s budget keys and `prompt-fingerprinter`'s cache keys already scope by tenant:

```go
// JudgeRubric is the versioned, structured grading contract for one
// task_type. Additive: a new criterion or task_type never invalidates a
// score computed under an older rubric version, it is graded and stored
// as that version's contract.
type JudgeRubric struct {
    TaskType string      `json:"task_type"`
    Version  int         `json:"version"`
    Criteria []Criterion `json:"criteria"`
}

type Criterion struct {
    Name        string  `json:"name"`        // e.g. "factual_grounding"
    Weight      float64 `json:"weight"`      // sums to 1.0 across Criteria
    Description string  `json:"description"` // the exact instruction the judge prompt embeds
}
```

**JSON schema, not freeform grading instructions.** The judge prompt embeds each `Criterion.Description` verbatim and asks for a 0–10 score per criterion; the final score is the weighted sum, normalized to 0–100. This is the same contract-first discipline the ingest API's `InferenceEvent` struct already applies to request/response events (`ingestion/src/handlers/event.rs`) — a rubric is a versioned schema a judge prompt is built from, not a paragraph of instructions re-typed slightly differently every time someone tunes it. A structured criteria list means a rubric change is a diffable, reviewable edit to a JSON file, and a score can always be traced back to exactly which weighted criteria produced it.

**Rubrics live one per `task_type`, not one per tenant.** Two tenants both submitting `task_type=summarization` traffic are held to the same quality bar — a tenant-specific rubric would make "quality score" mean something different depending on whose traffic it described, which defeats the point of a comparable metric at all. What *is* tenant-scoped is where the resulting scores get stored and aggregated (§5), not the rubric used to produce them.

So: the rubric is the fixed, reviewable yardstick; only the traffic being measured against it varies.

---

## 3. Async queue topology — sampling never sits in the request path

```mermaid
%%{init: {
  'theme': 'base',
  'themeVariables': {
    'primaryColor': '#1e3a5f',
    'primaryTextColor': '#f0f4f8',
    'primaryBorderColor': '#4a90d9',
    'lineColor': '#4a90d9',
    'secondaryColor': '#0d2137',
    'tertiaryColor': '#0a1a2e',
    'background': 'transparent',
    'nodeBorder': '#4a90d9',
    'clusterBkg': '#0d2137',
    'titleColor': '#f0f4f8',
    'edgeLabelBackground': '#0d2137'
  }
}}%%
flowchart LR
  req["Live request served"] --> sample["Sampling decision"]
  sample --> topic["Kafka judge-requests"]
  topic --> worker["Judge worker pool"]
  worker --> haiku["Haiku scores rubric"]
  haiku --> store["ClickHouse quality_scores"]
  worker -->|"timeout"| dlq["DLQ, judge_unavailable"]
```

**The judge sits entirely outside the request/response cycle.** A request that gets sampled for judging has already been served — cache lookup, budget check, and the model call all completed before `model-quality-scorer` ever sees it. This mirrors why `prompt-fingerprinter`'s exact-match check and `cost-budget-enforcer`'s budget check both run inline while anything slower runs after: judging necessarily costs more than a Redis round trip (it's a full model call), so it belongs on the far side of the response already having gone out, never in front of it.

**Kafka, not Redis, because this is a durable log a consumer group drains at its own pace, not hot-path state a single caller blocks on.** `judge-requests` follows the same partition-by-`tenant_id` strategy the root `DESIGN.md` §3 already establishes for the ingestion topic — one tenant's judge backlog can never head-of-line block another tenant's, and a worker pool scales by adding consumers within the group rather than by any change to how samples are produced.

So: sampling adds a fire-and-forget publish to an already-completed request, and everything after that publish is the judge pipeline's problem, not the gateway's.

---

## 4. Throughput target — 200 samples/hr/tenant, sampling rate adapts to volume

A fixed sampling *percentage* (e.g. "grade 1% of traffic") produces wildly different absolute sample counts across tenants of different sizes — a tenant at 50 req/hr would get essentially no signal, while a tenant at 50,000 req/hr would flood the judge queue for a statistically unnecessary amount of coverage. The target here is instead an **absolute per-tenant rate, 200 samples/hr**, with the sampling percentage computed per tenant from their trailing request volume to hit that number:

```
sample_rate(tenant) = min(1.0, 200 / trailing_1hr_request_count(tenant))
```

A low-volume tenant near or below 200 req/hr samples close to 100% of traffic; a high-volume tenant samples a small fraction, and the fraction recomputes as their volume shifts rather than being hand-tuned per tenant. This is the same instinct as `cost-budget-enforcer`'s sliding-window counter (Day 65 §1): a rate derived from a trailing window, not a static number someone has to remember to revisit.

**200/hr is a coverage floor, not a precision target.** It buys roughly one graded sample every 18 minutes per tenant×task_type pairing at steady volume — enough to catch a routing regression within the same shift it started, not enough to report a tight confidence interval on any single hour. Tightening the interval further is a cost trade a later day can make explicitly; 200/hr is the number this design commits to as the floor every tenant gets regardless of traffic size.

So: the target is stated as an absolute rate precisely so it means the same thing — a comparable amount of signal — for every tenant, independent of how much traffic they send.

---

## 5. Failure modes when the judge times out

A judge call has a fixed timeout (design commitment: 5s, generous relative to Haiku's typical latency but bounded so one slow call can't stall a worker indefinitely). On timeout:

1. **One bounded retry**, not a loop. A single retry absorbs a transient blip; retrying repeatedly on a genuinely down judge would just grow the backlog faster than the queue drains, the same "no unbounded retry loop" discipline this repo already treats as a hard rule elsewhere.
2. **Second timeout → dead-letter, not a fabricated score.** The sample is written to `judge-requests-dlq` tagged `judge_unavailable` and dropped from the request path entirely — a failed grading attempt is not a `0` and not silently skipped; it is a distinct outcome the aggregation layer (§6) has to know how to exclude explicitly, the same way `cost-budget-enforcer`'s Redis-outage design (Day 69 §7) treats "the dependency is down" as its own state rather than approximating it as a pass or a fail.
3. **Circuit breaker on sustained failure.** If the judge's failure rate over a trailing window crosses a threshold (design commitment: >10% of attempts in 5 minutes), the worker pool stops pulling new samples for the affected `task_type` until the judge recovers, rather than letting `judge-requests` grow unbounded behind a judge that is provably not answering. This follows `cost-budget-enforcer`'s fail-open-by-default posture (Day 69 §7): quality sampling pausing briefly costs nothing user-facing, since it sits entirely outside the request path already served.

**A `judge_unavailable` sample must never be averaged in as a passing or failing score.** The whole point of §6's per-tenant×task_type aggregation is that a number reported as "average quality" has to mean every input to it was actually graded — folding a timeout in as an implicit zero would make an outage look like a quality regression, and dropping it silently would hide that coverage for that hour was lower than the 200/hr target promised.

---

## 6. Aggregation — per tenant × task_type, never a single global average

Scores land in a ClickHouse `quality_scores` table, one row per judged sample (`tenant_id`, `task_type`, `model_id`, `rubric_version`, `score`, `rationale`) — `rationale` is the judge's short free-text justification for the score, not just the number, so a low-scoring sample is debuggable after the fact instead of a bare integer nobody can act on. Aggregates are computed at query time by slicing along whichever of those columns the question needs — never pre-collapsed into one running global average at write time. A single blended number across every tenant and every task type would hide exactly the failure this module exists to catch: one tenant's routing regression on one task type, diluted into invisibility by every other tenant's unaffected traffic averaging over it.

This is the same shape as a lesson that shows up outside this codebase entirely: a P99 latency figure computed by averaging pre-merged per-tenant percentiles is not a real P99 for any tenant — it's a number that describes nobody's actual experience, because percentiles (and, here, quality scores) don't merge losslessly across a dimension you might later need to slice by. The fix is identical in both places: keep the raw, dimension-scoped data (per-sample scores here, per-tenant distributions there) and aggregate only at the point where the slicing dimension is already decided, not before.

So: `model-quality-scorer` never produces "the" quality score — only "the score for this tenant, this task type," which is the only version of that number that can actually catch a regression before a tenant reports one.

---

## Out of scope (Day 77)

No runtime code, migrations, or Kafka topics actually created yet — `judge-requests` and `judge-requests-dlq` are named here as the design commitment a future implementation day stands up. No live Haiku calls exercised in this sandbox. No wiring into `cost-budget-enforcer`'s gateway or `semantic-cache-engine`'s lookup path to trigger the sampling decision — that integration point is deferred past this design-only day, the same way `prompt-fingerprinter`'s Day 70 design deferred its own gateway wiring. Implementation lands across three following days: the Kafka consumer, batched judge calls, and ClickHouse persistence this document commits to; 1h/24h rollup normalization with a documented statistical noise floor; and RouteIQ's weighted utility function that finally consumes the rollup alongside cost and latency.
