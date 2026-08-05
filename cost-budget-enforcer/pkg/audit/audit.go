// SPDX-License-Identifier: MIT

// Package audit publishes a record of every live budget change the
// Admin API (pkg/admin) applies through config.LiveStore. Day 65's
// DESIGN.md deferred this — "no new Kafka topics yet" — because there
// was nothing to audit until Day 67 added a way to change a tenant's
// budget without a restart. A change that can happen from any admin's
// terminal with no deploy and no code review needs an append-only trail
// of who changed what and when at least as much as the deploy-gated
// config file it replaces for live edits.
package audit

import "context"

// Topic is the Kafka topic budget-change events are published to.
const Topic = "cost_budget_audit_log"

// BudgetChangeEvent is one admin-initiated change to a tenant's budget
// config. Before and After are TenantConfig values encoded as
// map[string]any (rather than typed config.TenantConfig) so this
// package has no import dependency on pkg/config — audit is a leaf
// package other packages publish into, not one that reaches back into
// the thing it's auditing.
type BudgetChangeEvent struct {
	TenantID  string         `json:"tenant_id"`
	Actor     string         `json:"actor"`
	Before    map[string]any `json:"before"`
	After     map[string]any `json:"after"`
	Timestamp int64          `json:"timestamp"`
}

// Publisher delivers a BudgetChangeEvent to the audit trail. Unlike
// enforcer.WebhookSender — which middleware.go deliberately fails open
// on, because losing one webhook costs one late alert — pkg/admin
// treats a Publish error as fatal to the request: an admin budget
// change with no corresponding audit record can't be reconstructed
// after the fact, so the change itself must not be allowed to stand.
type Publisher interface {
	Publish(ctx context.Context, event BudgetChangeEvent) error
}
