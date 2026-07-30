// SPDX-License-Identifier: MIT
package export

import (
	"context"
	"fmt"
	"time"

	"github.com/akshantvats/agent-replay-engine/pkg/objectstore"
)

// Tier names a retention stage an exported object falls into based on age.
type Tier string

const (
	TierHot     Tier = "hot"     // 0-30 days: fast path, uncompressed listing expected
	TierCold    Tier = "cold"    // 30-90 days: compressed, infrequent access
	TierExpired Tier = "expired" // >90 days: eligible for deletion
)

// hotCeiling is the age at which an object moves from hot to cold.
const hotCeiling = 30 * 24 * time.Hour

// coldCeiling is the age at which an object moves from cold to expired.
const coldCeiling = 90 * 24 * time.Hour

// Classify returns the retention tier for an object given its age.
// Boundaries are inclusive on the lower bound: exactly 30 days is cold, and
// exactly 90 days is expired.
func Classify(age time.Duration) Tier {
	switch {
	case age < hotCeiling:
		return TierHot
	case age < coldCeiling:
		return TierCold
	default:
		return TierExpired
	}
}

// Sweep scans store under prefix and returns the keys classified
// TierExpired as of now. Callers decide whether to delete them — Sweep does
// not delete, per the "no silent data loss" rule for a debug-oriented
// system.
func Sweep(ctx context.Context, store objectstore.ObjectStore, prefix string, now time.Time) ([]string, error) {
	objs, err := store.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("export: sweep list %s: %w", prefix, err)
	}

	var expired []string
	for _, obj := range objs {
		age := now.Sub(obj.LastModified)
		if Classify(age) == TierExpired {
			expired = append(expired, obj.Key)
		}
	}
	return expired, nil
}
