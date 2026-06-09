// SPDX-License-Identifier: MIT
package controller

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/akshantvats/distributed-flagd/internal/etcdstore"
)

const (
	crdGroup    = "flagd.lensai.io"
	crdVersion  = "v1alpha1"
	crdResource = "flagdefinitions"
)

// WatchEvent is one event from the Kubernetes watch stream.
type WatchEvent struct {
	Type   string         `json:"type"` // ADDED, MODIFIED, DELETED
	Object FlagDefinition `json:"object"`
}

// FlagDefinition is the Kubernetes custom resource.
type FlagDefinition struct {
	Metadata ObjectMeta `json:"metadata"`
	Spec     FlagSpec   `json:"spec"`
}

// ObjectMeta is the subset of k8s ObjectMeta we need.
type ObjectMeta struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	ResourceVersion string            `json:"resourceVersion"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

// FlagSpec mirrors etcdstore.FlagData in Kubernetes spec form.
type FlagSpec struct {
	FlagKey  string        `json:"flagKey"`
	Enabled  bool          `json:"enabled"`
	Value    string        `json:"value,omitempty"`
	Variants []VariantSpec `json:"variants,omitempty"`
}

// VariantSpec is one percentage-rollout bucket.
type VariantSpec struct {
	Value  string `json:"value"`
	Weight int    `json:"weight"`
}

// Store is the interface for syncing flags to the backing store.
type Store interface {
	SetFlag(ctx context.Context, fd *etcdstore.FlagData) error
	DeleteFlag(ctx context.Context, name string) error
}

// Reconciler watches FlagDefinition CRs and syncs them to etcd.
type Reconciler struct {
	k8sURL    string
	namespace string
	token     string
	store     Store
	client    *http.Client
}

// New returns a Reconciler that watches the given namespace.
func New(k8sURL, namespace, token string, store Store) *Reconciler {
	return &Reconciler{
		k8sURL:    k8sURL,
		namespace: namespace,
		token:     token,
		store:     store,
		client:    &http.Client{Timeout: 0}, // streaming — no timeout
	}
}

// Run starts the watch loop. It blocks until ctx is cancelled, retrying on errors with backoff.
func (r *Reconciler) Run(ctx context.Context) error {
	backoff := 2 * time.Second
	for {
		if err := r.watch(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("controller: watch error: %v — retrying in %s", err, backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 2 * time.Second
	}
}

// watch opens a long-polling HTTP watch on the FlagDefinition CRD and processes events.
func (r *Reconciler) watch(ctx context.Context) error {
	url := fmt.Sprintf("%s/apis/%s/%s/namespaces/%s/%s?watch=true",
		r.k8sURL, crdGroup, crdVersion, r.namespace, crdResource)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("k8s watch returned %d: %s", resp.StatusCode, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event WatchEvent
		if err := json.Unmarshal(line, &event); err != nil {
			log.Printf("controller: unmarshal watch event: %v", err)
			continue
		}
		if err := r.reconcile(ctx, event); err != nil {
			log.Printf("controller: reconcile %s/%s: %v",
				event.Type, event.Object.Metadata.Name, err)
		}
	}
	return scanner.Err()
}

// reconcile applies one watch event to etcd.
func (r *Reconciler) reconcile(ctx context.Context, event WatchEvent) error {
	fd := specToFlagData(event.Object)
	switch event.Type {
	case "ADDED", "MODIFIED":
		return r.store.SetFlag(ctx, fd)
	case "DELETED":
		return r.store.DeleteFlag(ctx, fd.Name)
	default:
		return nil
	}
}

// specToFlagData converts a FlagDefinition CR into the etcdstore representation.
func specToFlagData(cr FlagDefinition) *etcdstore.FlagData {
	fd := &etcdstore.FlagData{
		Name:    cr.Spec.FlagKey,
		Value:   cr.Spec.Value,
		Enabled: cr.Spec.Enabled,
	}
	for _, v := range cr.Spec.Variants {
		fd.Variants = append(fd.Variants, etcdstore.VariantData{
			Value:  v.Value,
			Weight: v.Weight,
		})
	}
	return fd
}
