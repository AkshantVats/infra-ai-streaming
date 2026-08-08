// SPDX-License-Identifier: MIT

// Package decision implements RouteIQ's weighted-utility routing
// decision (DESIGN.md §8): the first RouteIQ module in the arc that
// actually picks a winner among candidate models, rather than only
// measuring or gating one input to that choice. It joins quality (a
// Day 79 pkg/rollup.Row), cost, and latency into one scalar via a
// tenant-overridable weighted sum — linear scalarization, the same
// technique a cost-aware load balancer uses to rank backends on more
// than one axis at once.
package decision

import (
	"fmt"
	"math"

	"github.com/akshantvats/model-quality-scorer/pkg/rollup"
)

// RoutingWeights scales each of the three objectives before they are
// summed into one utility value. Quality is rewarded; cost and latency
// are penalized (see Utility). Weights are tenant-overridable — see
// WeightsForTenant — because how much latency should matter relative to
// cost is a legitimate per-tenant preference, not a global constant.
type RoutingWeights struct {
	WQuality float64
	WCost    float64
	WLatency float64
}

// DefaultWeights is the fallback RoutingWeights a tenant gets when it
// has not set an override: all three objectives weighted equally. This
// is a starting point, not a claim that equal weighting is correct for
// every tenant — WeightsForTenant is how a tenant moves off it.
var DefaultWeights = RoutingWeights{WQuality: 1.0, WCost: 1.0, WLatency: 1.0}

// Validate reports why w is not usable, or nil if it is. A negative
// weight would invert an objective's meaning (rewarding higher cost);
// an all-zero weight set can never distinguish any two candidates by
// utility, which is a config bug — every Decide call would fall straight
// through to the deterministic tiebreak for every decision, silently
// discarding the entire point of weighting quality, cost, and latency in
// the first place.
func (w RoutingWeights) Validate() error {
	if w.WQuality < 0 || w.WCost < 0 || w.WLatency < 0 {
		return fmt.Errorf("decision: weights must be non-negative, got %+v", w)
	}
	if w.WQuality == 0 && w.WCost == 0 && w.WLatency == 0 {
		return fmt.Errorf("decision: weights cannot all be zero")
	}
	return nil
}

// WeightsForTenant looks up tenant's override in overrides, falling back
// to DefaultWeights when absent. This is the same "override map keyed by
// tenant_id, default when absent" shape cost-budget-enforcer's per-tenant
// budget config and prompt-fingerprinter's per-tenant cache config
// already use — RouteIQ's fourth module reuses the arc's established
// config pattern rather than inventing a fifth one.
func WeightsForTenant(tenant string, overrides map[string]RoutingWeights) RoutingWeights {
	if w, ok := overrides[tenant]; ok {
		return w
	}
	return DefaultWeights
}

// Candidate is one model RouteIQ could route a request to, scored on the
// three axes Utility combines.
type Candidate struct {
	ModelID string

	// Quality is 0-1, expected to be a rollup.Row's AvgNormalizedScore —
	// see FromRollupRow. It inherits that package's [0,1] range contract
	// rather than defining a second one.
	Quality float64

	// CostPerCall and LatencyP99Ms are externally supplied signals
	// (per-model pricing, observed p99 latency) this package does not
	// itself measure — see FromRollupRow's doc comment.
	CostPerCall  float64
	LatencyP99Ms float64

	// LowConfidence mirrors rollup.Row.LowConfidence(): true when the
	// rollup bucket Quality was computed from sat below NOISE-FLOOR.md's
	// 30-sample statistical floor. Decide still ranks a low-confidence
	// candidate normally — a thin bucket is still the best information
	// available — but Decision.LowConfidence surfaces it so a caller
	// knows the winning score wasn't fully earned yet.
	LowConfidence bool
}

// Validate reports why c is not usable, or nil if it is.
func (c Candidate) Validate() error {
	if c.ModelID == "" {
		return fmt.Errorf("decision: candidate model_id is empty")
	}
	if c.Quality < 0 || c.Quality > 1 {
		return fmt.Errorf("decision: candidate %q quality %v out of range [0,1]", c.ModelID, c.Quality)
	}
	if c.CostPerCall < 0 {
		return fmt.Errorf("decision: candidate %q cost_per_call %v is negative", c.ModelID, c.CostPerCall)
	}
	if c.LatencyP99Ms < 0 {
		return fmt.Errorf("decision: candidate %q latency_p99_ms %v is negative", c.ModelID, c.LatencyP99Ms)
	}
	return nil
}

// FromRollupRow builds a Candidate from a Day 79 rollup.Row (the quality
// signal) plus externally-supplied cost and latency — the literal "cost
// and latency signals join quality in one function" DESIGN.md §8 commits
// this day to. costPerCall and latencyP99Ms are not model-quality-scorer's
// own data: a future day sources real per-model pricing and
// cost-budget-enforcer/OTel latency observability instead of caller-
// supplied values.
func FromRollupRow(r rollup.Row, costPerCall, latencyP99Ms float64) Candidate {
	return Candidate{
		ModelID:       r.ModelID,
		Quality:       r.AvgNormalizedScore,
		CostPerCall:   costPerCall,
		LatencyP99Ms:  latencyP99Ms,
		LowConfidence: r.LowConfidence(),
	}
}

// Utility is RouteIQ's scalarization step: a weighted sum reducing three
// incomparable units (a quality fraction, a dollar amount, a millisecond
// count) into one scalar a caller can rank candidates by. Quality is
// rewarded; cost and latency are penalized — the same shape a cost-aware
// load balancer uses to score backends on free capacity minus $/request,
// applied here to a model-routing choice instead.
func Utility(c Candidate, w RoutingWeights) float64 {
	return w.WQuality*c.Quality - w.WCost*c.CostPerCall - w.WLatency*c.LatencyP99Ms
}

// tieEpsilon is how close two candidates' utilities have to be before
// Decide treats them as tied rather than picking the strictly higher
// one. Scalarizing three objectives into one number can produce genuine
// near-ties that don't reflect a real preference between two candidates
// — this is the threshold below which Decide stops trusting the raw
// utility difference and falls through to the deterministic tiebreak.
const tieEpsilon = 1e-6

// Decision is the result of a Decide call.
type Decision struct {
	Winner Candidate

	// LowConfidence mirrors Winner.LowConfidence: whether the decision
	// was made using a quality signal that sat below the statistical
	// noise floor.
	LowConfidence bool
}

// Decide picks the candidate with the highest Utility under w. When two
// or more candidates land within tieEpsilon of the top utility, Decide
// prefers the one with the lower CostPerCall — candidates that are, for
// practical purposes, equally good right now should be distinguished by
// a deterministic, externally meaningful signal (cost), not by a
// rounding artifact in the utility computation. If CostPerCall also ties
// exactly, Decide breaks by ModelID lexicographic order so the same
// input always produces the same output.
func Decide(candidates []Candidate, w RoutingWeights) (Decision, error) {
	if len(candidates) == 0 {
		return Decision{}, fmt.Errorf("decision: no candidates to decide among")
	}
	if err := w.Validate(); err != nil {
		return Decision{}, err
	}
	for _, c := range candidates {
		if err := c.Validate(); err != nil {
			return Decision{}, err
		}
	}

	maxUtility := math.Inf(-1)
	for _, c := range candidates {
		if u := Utility(c, w); u > maxUtility {
			maxUtility = u
		}
	}

	var tied []Candidate
	for _, c := range candidates {
		if maxUtility-Utility(c, w) <= tieEpsilon {
			tied = append(tied, c)
		}
	}

	winner := tied[0]
	for _, c := range tied[1:] {
		switch {
		case c.CostPerCall < winner.CostPerCall:
			winner = c
		case c.CostPerCall == winner.CostPerCall && c.ModelID < winner.ModelID:
			winner = c
		}
	}

	return Decision{Winner: winner, LowConfidence: winner.LowConfidence}, nil
}
