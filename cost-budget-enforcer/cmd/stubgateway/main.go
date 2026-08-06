// SPDX-License-Identifier: MIT

// Command stubgateway is RouteIQ's Day 68 vertical slice: it reads a JSON
// Lines file of {"tenant_id","model","prompt"} requests and runs each one
// through pkg/gateway.Gateway's fixed order — budget check, then cache
// lookup, then model call — printing the outcome and, when --lensai-url is
// set, dual-writing it to LensAI's cost_usd stream. "Stub" describes the
// cache and model clients wired in here (in-process fakes; see DESIGN.md
// §6's "Out of scope" note), not the ordering or the LensAI wiring, both of
// which run for real.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/akshantvats/cost-budget-enforcer/pkg/config"
	"github.com/akshantvats/cost-budget-enforcer/pkg/enforcer"
	"github.com/akshantvats/cost-budget-enforcer/pkg/gateway"
	"github.com/akshantvats/cost-budget-enforcer/pkg/lensai"
	"github.com/akshantvats/cost-budget-enforcer/pkg/store"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI and returns the process exit code, kept separate
// from main for testability without exec'ing a built binary (same shape
// as semantic-cache-engine/cmd/cachelookup/main.go::run).
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stubgateway", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputPath := fs.String("input", "", "path to a JSON Lines file of requests: {\"tenant_id\",\"model\",\"prompt\"} per line")
	budgetTokens := fs.Int64("budget-tokens", 1000, "per-tenant token budget for this demo run")
	lensaiURL := fs.String("lensai-url", "", "LensAI ingest URL; when empty, outcomes print but are not dual-written")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *inputPath == "" {
		_, _ = fmt.Fprintln(stderr, "stubgateway: --input is required")
		return 2
	}

	f, err := os.Open(*inputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stubgateway: open input: %v\n", err)
		return 1
	}
	defer func() { _ = f.Close() }()

	mr, err := miniredis.Run()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stubgateway: start in-process redis: %v\n", err)
		return 1
	}
	defer mr.Close()
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	g := &gateway.Gateway{
		Enforcer: &enforcer.Enforcer{Store: store.NewRedisStore(rdb)},
		Config: func(string) config.TenantConfig {
			return config.TenantConfig{
				BudgetTokens:   *budgetTokens,
				WindowSeconds:  config.DefaultWindowSeconds,
				FallbackModel:  "gpt-4o-mini",
				AlertThreshold: config.DefaultAlertThreshold,
				SoftThreshold:  config.DefaultSoftThreshold,
				HardThreshold:  config.DefaultHardThreshold,
			}
		},
		Tokens: estimateTokens,
		Cache:  alwaysMissCache{},
		Model:  priceTableModel{},
	}
	if *lensaiURL != "" {
		g.Events = lensai.New(*lensaiURL)
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var req struct {
			TenantID string `json:"tenant_id"`
			Model    string `json:"model"`
			Prompt   string `json:"prompt"`
		}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_, _ = fmt.Fprintf(stderr, "stubgateway: skipping malformed line: %v\n", err)
			continue
		}

		result, err := g.Handle(context.Background(), req.TenantID, req.Model, req.Prompt)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "stubgateway: tenant=%s: %v\n", req.TenantID, err)
			continue
		}

		switch {
		case result.StoreUnavailable:
			_, _ = fmt.Fprintf(stdout, "tenant=%s action=fail_closed_503 cost_usd=0\n", req.TenantID)
		case result.Blocked:
			_, _ = fmt.Fprintf(stdout, "tenant=%s action=block retry_after=%s cost_usd=0\n", req.TenantID, result.RetryAfter)
		case result.CacheHit:
			_, _ = fmt.Fprintf(stdout, "tenant=%s action=cache_hit model=%s cost_usd=0\n", req.TenantID, result.ModelUsed)
		default:
			_, _ = fmt.Fprintf(stdout, "tenant=%s action=inference model=%s degraded=%v tokens=%d cost_usd=%.4f\n",
				req.TenantID, result.ModelUsed, result.Degraded, result.TokensUsed, result.CostUSD)
		}
		if result.EmitErr != nil {
			_, _ = fmt.Fprintf(stderr, "stubgateway: tenant=%s: lensai emit failed: %v\n", req.TenantID, result.EmitErr)
		}
	}
	if err := scanner.Err(); err != nil {
		_, _ = fmt.Fprintf(stderr, "stubgateway: read input: %v\n", err)
		return 1
	}
	return 0
}

// estimateTokens is a length heuristic (roughly 4 bytes per token), the
// same shape pkg/middleware.TokenEstimator implementations are documented
// to use — the estimate need not be exact, only cheap and monotonic in
// prompt length.
func estimateTokens(_ string, prompt string) int64 {
	return int64(len(prompt))/4 + 1
}

// alwaysMissCache is the stub CacheClient DESIGN.md §6 calls out as future
// work: a real semantic cache lookup would live behind this same interface
// (semantic-cache-engine/pkg/lookup, once the two modules share one, per
// the "Out of scope" note), but proving the fixed order — budget before
// cache before model — does not require a real cache backing it yet.
type alwaysMissCache struct{}

func (alwaysMissCache) Lookup(_ context.Context, _, _ string) (gateway.CacheResult, error) {
	return gateway.CacheResult{Hit: false}, nil
}

// priceTableModel is the stub ModelClient: a fixed price per model name,
// standing in for a real provider call. The prices differ by model
// specifically so a degraded request (routed to the cheaper fallback)
// visibly costs less in the printed output and in the LensAI event it
// emits — that visible difference is the whole point of wiring real
// cost_usd instead of a flat placeholder.
type priceTableModel struct{}

var modelPricePerToken = map[string]float64{
	"gpt-4o":      0.00003,
	"gpt-4o-mini": 0.000005,
}

func (priceTableModel) Call(_ context.Context, tenantID, model, prompt string) (gateway.ModelResult, error) {
	tokens := estimateTokens(tenantID, prompt)
	price, ok := modelPricePerToken[model]
	if !ok {
		price = modelPricePerToken["gpt-4o-mini"]
	}
	sum := sha256.Sum256([]byte(prompt))
	return gateway.ModelResult{
		Response:   "stub-response-" + hex.EncodeToString(sum[:4]),
		TokensUsed: tokens,
		CostUSD:    float64(tokens) * price,
		Latency:    50 * time.Millisecond,
	}, nil
}
