// SPDX-License-Identifier: MIT

// Package rules is the in-memory, concurrency-safe home for each tenant's
// fingerprint.Rules override, mutated only through pkg/admin's PUT
// /tenants/{id}/fingerprint-rules handler. ForTenant's zero-value
// fallback is what makes an un-configured tenant fingerprint exactly as
// every tenant did before Day 76: fingerprint.Rules{}.Apply is a
// documented no-op.
package rules

import (
	"context"
	"fmt"
	"sync"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

// MaxPromptBytesCeiling bounds an operator-set MaxPromptBytes so a typo
// (or a value meant for a different unit, e.g. KB instead of bytes)
// can't push the cap so high it stops meaning anything. 1MB is far above
// any real prompt this stack has seen — the ceiling exists to catch a
// mistake, not to constrain a legitimate use.
const MaxPromptBytesCeiling = 1_000_000

// ErrNegativeMaxPromptBytes is returned by Put when MaxPromptBytes < 0,
// which has no meaning (fingerprint.Rules treats MaxPromptBytes <= 0 as
// "no cap", so a negative value could only ever be a caller mistake).
var ErrNegativeMaxPromptBytes = fmt.Errorf("rules: max_prompt_bytes must be >= 0")

// ErrMaxPromptBytesTooLarge is returned by Put when MaxPromptBytes
// exceeds MaxPromptBytesCeiling.
var ErrMaxPromptBytesTooLarge = fmt.Errorf("rules: max_prompt_bytes exceeds ceiling of %d", MaxPromptBytesCeiling)

// Store holds one fingerprint.Rules value per tenant.
type Store struct {
	mu    sync.RWMutex
	rules map[string]fingerprint.Rules
}

// NewStore constructs an empty Store — every tenant starts at the
// zero-value fingerprint.Rules{} (today's default behavior) until Put is
// called for that tenant.
func NewStore() *Store {
	return &Store{rules: make(map[string]fingerprint.Rules)}
}

// ForTenant implements stack.RulesProvider. It returns tenantID's
// configured Rules, or the zero value if none has ever been set.
func (s *Store) ForTenant(_ context.Context, tenantID string) fingerprint.Rules {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rules[tenantID]
}

// Put replaces tenantID's Rules outright — PUT semantics, a full
// resource replace, not a partial patch like cost-budget-enforcer's
// admin API uses for its own PATCH endpoint. A fingerprint-rules
// resource is three small fields; "send the whole thing" is the simpler
// contract and avoids needing a second pointer-field patch type only
// for this. On validation failure the store is left unchanged.
func (s *Store) Put(tenantID string, r fingerprint.Rules) error {
	if r.MaxPromptBytes < 0 {
		return ErrNegativeMaxPromptBytes
	}
	if r.MaxPromptBytes > MaxPromptBytesCeiling {
		return ErrMaxPromptBytesTooLarge
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[tenantID] = r
	return nil
}
