# Statistical noise floor for quality rollups

`pkg/rollup.Query` averages `normalized_score` per `(window, model_id, task_type)` bucket. An
average on its own says nothing about how much to trust it — a bucket built from 4 samples and a
bucket built from 400 samples can report the identical mean while meaning very different things.
This doc defines the floor below which a bucket's mean should not be trusted the same way a full
bucket's is, and what a downstream consumer should do about it.

## The floor

`pkg/rollup.MinSamplesForConfidence = 30`. This is the standard rule of thumb from the Central
Limit Theorem: with roughly 30 or more samples, the sampling distribution of the mean is
reasonably approximated as normal regardless of the shape of the underlying per-sample score
distribution, which is what makes a standard-error-based confidence statement meaningful in the
first place. Below 30 samples, that approximation gets shaky and the reported standard error
understates how uncertain the mean actually is.

`pkg/rollup.LowConfidence(sampleCount int) bool` returns `sampleCount < 30`.

## Standard error

`pkg/rollup.StandardError(stddev, sampleCount float64) float64` computes `stddev / sqrt(n)` — the
standard error of the mean for a bucket. A 1h bucket at steady-state traffic (recall DESIGN.md
§4's 200 samples/hr/tenant target) comfortably clears 30 samples per tenant; a `model_id×task_type`
slice within that hour, or an hour early in a tenant's ramp-up, is exactly the case this floor is
meant to catch.

## What a low-confidence bucket means for a caller

A rollup consumer — today, the Grafana panel on `dashboards/traceforge-lensai-cross-product.json`;
Day 80's RouteIQ weighted utility function, per Day 78's AI Learning post — should not treat a
`low_confidence` bucket's mean as equally trustworthy input:

- **Grafana panel:** the low-confidence threshold is rendered as a visual flag (`sample_count < 30`)
  on the 1h rollup table, so a thin hour is visibly distinguishable from a full one rather than
  looking identical to it.
- **A weighted-utility consumer (future work):** prefer the wider `Window24h` rollup over a thin
  `Window1h` one for a `model_id×task_type` pairing that hasn't cleared the floor yet, rather than
  routing traffic on a mean with an unstated, possibly-large error bar. This mirrors
  `cost-budget-enforcer`'s fail-open-by-default posture — an under-sampled signal should degrade
  gracefully to a coarser, better-supported one, not silently masquerade as full-confidence data.

## Why this isn't solved by a bigger window instead

Widening every window to guarantee 30+ samples defeats the point of having an hourly rollup at
all — it would just be the 24h rollup with extra steps whenever traffic is thin. The floor is
meant to be visible and per-bucket, not "solved" by never reporting a thin bucket in the first
place: a caller that knows a bucket is under-sampled can make an informed choice (fall back to a
wider window, wait for more data) that a caller kept blind to it cannot.
