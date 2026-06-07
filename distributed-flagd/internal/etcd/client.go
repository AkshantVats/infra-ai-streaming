// SPDX-License-Identifier: MIT
package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	flagPrefix  = "/flags/"
	auditPrefix = "/audit/"
)

// FlagData is the JSON-encoded value stored at /flags/{name} in etcd.
type FlagData struct {
	Name     string        `json:"name"`
	Value    string        `json:"value"`
	Enabled  bool          `json:"enabled"`
	Variants []VariantData `json:"variants,omitempty"`
}

// VariantData is one percentage-rollout bucket serialised alongside the flag.
type VariantData struct {
	Value  string `json:"value"`
	Weight int    `json:"weight"`
}

// Client wraps etcd KV and Watch operations scoped to the /flags/ prefix.
type Client struct {
	kv      clientv3.KV
	watcher clientv3.Watcher
}

// NewClient constructs an etcd Client from a connected etcd client.
func NewClient(c *clientv3.Client) *Client {
	return &Client{kv: c, watcher: c}
}

// GetFlag fetches a single flag by name. Returns error if not found.
func (c *Client) GetFlag(ctx context.Context, name string) (*FlagData, error) {
	resp, err := c.kv.Get(ctx, flagPrefix+name)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("flag not found: %s", name)
	}
	var fd FlagData
	if err := json.Unmarshal(resp.Kvs[0].Value, &fd); err != nil {
		return nil, err
	}
	return &fd, nil
}

// SetFlag writes a flag to etcd. Callers must also call audit.Logger.Log.
func (c *Client) SetFlag(ctx context.Context, fd *FlagData) error {
	val, err := json.Marshal(fd)
	if err != nil {
		return err
	}
	_, err = c.kv.Put(ctx, flagPrefix+fd.Name, string(val))
	return err
}

// ListFlags returns all flags under the /flags/ prefix.
func (c *Client) ListFlags(ctx context.Context) ([]*FlagData, error) {
	resp, err := c.kv.Get(ctx, flagPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]*FlagData, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var fd FlagData
		if err := json.Unmarshal(kv.Value, &fd); err != nil {
			continue
		}
		out = append(out, &fd)
	}
	return out, nil
}

// WatchFlags returns a channel of etcd watch events on /flags/ prefix.
func (c *Client) WatchFlags(ctx context.Context) clientv3.WatchChan {
	return c.watcher.Watch(ctx, flagPrefix, clientv3.WithPrefix())
}

// FlagNameFromKey strips the /flags/ prefix from an etcd key.
func FlagNameFromKey(key string) string {
	return strings.TrimPrefix(key, flagPrefix)
}
