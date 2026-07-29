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

// etcdLeaseKV is the subset of *clientv3.Client used by Logger.
// Defining it as an interface makes Logger testable without a live etcd cluster.
type etcdLeaseKV interface {
	Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error)
	Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error)
}

// Logger writes audit entries to etcd with a 90-day TTL via etcd lease.
type Logger struct {
	client etcdLeaseKV
}

// New constructs an audit Logger backed by an etcd client.
// c must not be nil.
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
