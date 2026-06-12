// SPDX-License-Identifier: MIT

// Package sdk is a typed Go HTTP client for the distributed-flagd REST API.
// It is generated from api/openapi.yaml and kept in sync with the server.
//
// Usage:
//
//	c := sdk.New("http://flagd:8080")
//	resp, _ := c.Evaluate(ctx, "acme-corp", "user-42")
//	// resp.ResolvedModelID → write to inference_events.resolved_model_id
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is a typed HTTP client for the distributed-flagd REST API.
type Client struct {
	base string
	hc   *http.Client
}

// New returns a Client pointed at addr (e.g. "http://flagd:8080").
func New(addr string) *Client {
	return &Client{
		base: addr,
		hc:   &http.Client{Timeout: 2 * time.Second},
	}
}

// FlagData mirrors the server FlagData struct.
type FlagData struct {
	Name     string        `json:"name"`
	Value    string        `json:"value"`
	Enabled  bool          `json:"enabled"`
	Variants []VariantData `json:"variants,omitempty"`
}

// VariantData is one percentage-rollout bucket.
type VariantData struct {
	Value  string `json:"value"`
	Weight int    `json:"weight"`
}

// FlagRequest is the body for Create and Update.
type FlagRequest struct {
	Name      string        `json:"name"`
	Value     string        `json:"value"`
	Enabled   bool          `json:"enabled"`
	Variants  []VariantData `json:"variants,omitempty"`
	ChangedBy string        `json:"changed_by,omitempty"`
	Reason    string        `json:"reason,omitempty"`
}

// EvalResponse is the resolved model and variant assignment.
type EvalResponse struct {
	ResolvedModelID string `json:"resolved_model_id"`
	Variant         string `json:"variant"`
	FlagKey         string `json:"flag_key"`
}

// FlagList is returned by ListFlags.
type FlagList struct {
	Flags []*FlagData `json:"flags"`
	Count int         `json:"count"`
}

// Evaluate resolves the active model version for tenant+user.
// ResolvedModelID must be written to inference_events.resolved_model_id
// for cost attribution in ClickHouse.
func (c *Client) Evaluate(ctx context.Context, tenantID, userID string) (*EvalResponse, error) {
	body := map[string]string{"tenant_id": tenantID, "user_id": userID}
	var resp EvalResponse
	if err := c.postExpect(ctx, "/evaluate", body, &resp, http.StatusOK); err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}
	return &resp, nil
}

// GetFlag fetches a single flag by name.
func (c *Client) GetFlag(ctx context.Context, name string) (*FlagData, error) {
	var fd FlagData
	if err := c.get(ctx, "/flags/"+name, &fd); err != nil {
		return nil, fmt.Errorf("get flag %q: %w", name, err)
	}
	return &fd, nil
}

// ListFlags returns all flags stored in etcd.
func (c *Client) ListFlags(ctx context.Context) (*FlagList, error) {
	var list FlagList
	if err := c.get(ctx, "/flags", &list); err != nil {
		return nil, fmt.Errorf("list flags: %w", err)
	}
	return &list, nil
}

// CreateFlag creates a new flag and returns the stored value.
func (c *Client) CreateFlag(ctx context.Context, req FlagRequest) (*FlagData, error) {
	var fd FlagData
	if err := c.postExpect(ctx, "/flags", req, &fd, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("create flag: %w", err)
	}
	return &fd, nil
}

// UpdateFlag overwrites an existing flag.
func (c *Client) UpdateFlag(ctx context.Context, name string, req FlagRequest) (*FlagData, error) {
	var fd FlagData
	if err := c.put(ctx, "/flags/"+name, req, &fd); err != nil {
		return nil, fmt.Errorf("update flag %q: %w", name, err)
	}
	return &fd, nil
}

// DeleteFlag removes a flag by name.
func (c *Client) DeleteFlag(ctx context.Context, name string) error {
	if err := c.del(ctx, "/flags/"+name); err != nil {
		return fmt.Errorf("delete flag %q: %w", name, err)
	}
	return nil
}

// Healthz returns nil when the server is reachable and healthy.
func (c *Client) Healthz(ctx context.Context) error {
	return c.get(ctx, "/healthz", nil)
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out, http.StatusOK)
}

func (c *Client) postExpect(ctx context.Context, path string, in, out interface{}, expect int) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out, expect)
}

func (c *Client) put(ctx context.Context, path string, in, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out, http.StatusOK)
}

func (c *Client) del(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil, http.StatusNoContent)
}

func (c *Client) do(req *http.Request, out interface{}, expectStatus int) error {
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectStatus {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, e.Error)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
