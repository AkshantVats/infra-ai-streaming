// SPDX-License-Identifier: MIT

// Package enforcer implements DESIGN.md §2's three-threshold decision:
// under 80% of budget passes traffic through unchanged, 80-100% passes
// through and fires a debounced alert webhook (§4), 100-120% routes to
// the tenant's configured fallback model (§3), and over 120% is
// rejected with a Retry-After pointing at the window's reset. The
// weighted-count arithmetic itself lives in pkg/store; this package
// only maps a weighted count to an action.
package enforcer

import (
	"context"
	"fmt"
	"time"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/store"
)

// Action is the decision Check returns for a single request.
type Action int

const (
	// Pass means the tenant is comfortably under budget; forward the
	// request unmodified.
	Pass Action = iota
	// Alert means the tenant has crossed the alert threshold; forward
	// the request unmodified, and fire the webhook if this call won
	// the per-window debounce flag (see Decision.FireWebhook).
	Alert
	// Degrade means the tenant has crossed the soft limit; rewrite the
	// outbound model to Decision.FallbackModel before forwarding.
	Degrade
	// Block means the tenant has crossed the hard limit; reject with
	// 429 and Decision.RetryAfter.
	Block
)

func (a Action) String() string {
	switch a {
	case Pass:
		return "pass"
	case Alert:
		return "alert"
	case Degrade:
		return "degrade"
	case Block:
		return "block"
	default:
		return "unknown"
	}
}

// Decision is the outcome of a budget check for one request.
type Decision struct {
	Action          Action
	FallbackModel   string        // set when Action == Degrade
	RetryAfter      time.Duration // set when Action == Block
	FireWebhook     bool          // set when Action == Alert and this call won the debounce
	PercentConsumed float64       // weighted consumption / budget, for logging/metrics
	WeightedTokens  float64
}

// WebhookSender delivers DESIGN.md §4's alert payload. Implementations
// should be non-blocking or bounded — Check calls Send synchronously
// today but a slow webhook must not be allowed to stall the request
// path it's guarding.
type WebhookSender interface {
	Send(ctx context.Context, payload AlertPayload) error
}

// AlertPayload is DESIGN.md §4's webhook body.
type AlertPayload struct {
	TenantID         string  `json:"tenant_id"`
	WindowStart      int64   `json:"window_start"`
	WindowSeconds    int64   `json:"window_seconds"`
	BudgetTokens     int64   `json:"budget_tokens"`
	ConsumedTokens   float64 `json:"consumed_tokens"`
	PercentConsumed  float64 `json:"percent_consumed"`
	ThresholdCrossed string  `json:"threshold_crossed"`
	Timestamp        int64   `json:"timestamp"`
}

// Enforcer checks tenant spend against configured budgets and decides
// what to do with each request.
type Enforcer struct {
	Store   store.Store
	Webhook WebhookSender
	// Now is the injectable clock; defaults to time.Now if left nil.
	Now func() time.Time
}

func (e *Enforcer) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// Check records tokensEstimate against tenantID's budget and returns
// the resulting decision. If the tenant has no budget configured
// (cfg.BudgetTokens == 0), Check always returns Pass without touching
// the store — an enforcer with no configured limit enforces nothing,
// rather than dividing by zero or blocking everyone by accident.
func (e *Enforcer) Check(ctx context.Context, tenantID string, tokensEstimate int64, cfg config.TenantConfig) (Decision, error) {
	if cfg.BudgetTokens <= 0 {
		return Decision{Action: Pass}, nil
	}

	now := e.now()
	weighted, err := e.Store.CheckAndIncrement(ctx, tenantID, tokensEstimate, cfg.WindowSeconds, now)
	if err != nil {
		return Decision{}, fmt.Errorf("enforcer: check tenant %s: %w", tenantID, err)
	}

	percent := weighted / float64(cfg.BudgetTokens)
	d := Decision{PercentConsumed: percent, WeightedTokens: weighted}

	switch {
	case percent >= cfg.HardThreshold:
		idx := store.WindowIndex(now, cfg.WindowSeconds)
		windowEnd := store.WindowStart(idx, cfg.WindowSeconds).Add(time.Duration(cfg.WindowSeconds) * time.Second)
		d.Action = Block
		d.RetryAfter = windowEnd.Sub(now)
		return d, nil

	case percent >= cfg.SoftThreshold:
		d.Action = Degrade
		d.FallbackModel = cfg.FallbackModel
		return d, nil

	case percent >= cfg.AlertThreshold:
		d.Action = Alert
		fired, err := e.Store.MarkAlerted(ctx, tenantID, cfg.WindowSeconds, now)
		if err != nil {
			return Decision{}, fmt.Errorf("enforcer: mark-alerted tenant %s: %w", tenantID, err)
		}
		d.FireWebhook = fired
		if fired && e.Webhook != nil {
			idx := store.WindowIndex(now, cfg.WindowSeconds)
			payload := AlertPayload{
				TenantID:         tenantID,
				WindowStart:      store.WindowStart(idx, cfg.WindowSeconds).Unix(),
				WindowSeconds:    cfg.WindowSeconds,
				BudgetTokens:     cfg.BudgetTokens,
				ConsumedTokens:   weighted,
				PercentConsumed:  percent,
				ThresholdCrossed: "alert",
				Timestamp:        now.Unix(),
			}
			if err := e.Webhook.Send(ctx, payload); err != nil {
				return Decision{}, fmt.Errorf("enforcer: send alert webhook for tenant %s: %w", tenantID, err)
			}
		}
		return d, nil

	default:
		d.Action = Pass
		return d, nil
	}
}
