// SPDX-License-Identifier: MIT
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	auditPrefix = "/audit/"
	auditTTL    = int64(90 * 24 * 3600) // 90 days in seconds
)

// Entry is one audit log record for a flag mutation.
// All six fields are required; zero values are not valid entries.
type Entry struct {
	FlagName                string `json:"flag_name"`
	OldValue                string `json:"old_value"`
	NewValue                string `json:"new_value"`
	ChangedBy               string `json:"changed_by"`
	ChangedAt               int64  `json:"changed_at"`
	EvaluationCountSnapshot int64  `json:"evaluation_count_snapshot"`
}

// Logger writes audit entries to etcd with a 90-day TTL via etcd lease.
type Logger struct {
	client *clientv3.Client
}

// New constructs an audit Logger.
func New(c *clientv3.Client) *Logger {
	return &Logger{client: c}
}

// Log persists an audit entry. The key includes unix nanoseconds so entries
// are ordered and unique within /audit/{flag_name}/.
func (l *Logger) Log(ctx context.Context, e Entry) error {
	e.ChangedAt = time.Now().UnixNano()
	key := fmt.Sprintf("%s%s/%d", auditPrefix, e.FlagName, e.ChangedAt)
	val, err := json.Marshal(e)
	if err != nil {
		return err
	}
	lease, err := l.client.Grant(ctx, auditTTL)
	if err != nil {
		return err
	}
	_, err = l.client.Put(ctx, key, string(val), clientv3.WithLease(lease.ID))
	return err
}
