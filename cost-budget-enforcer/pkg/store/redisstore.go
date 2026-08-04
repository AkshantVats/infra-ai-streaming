// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// checkAndIncrementScript is DESIGN.md §1's Lua script, refined to use
// an explicit window_index (see package doc) instead of a stored
// window_start timestamp: rollover is a comparison of the caller's
// current window index against the stored one, not an elapsed-seconds
// check against a boundary that could itself have drifted.
//
// KEYS[1] = "budget:{tenant_id}"
// ARGV[1] = tokens_this_call
// ARGV[2] = window_seconds
// ARGV[3] = now (unix seconds)
const checkAndIncrementScript = `
local window_seconds = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local window_index = math.floor(now / window_seconds)

local stored_index_raw = redis.call('HGET', KEYS[1], 'window_index')
local current = tonumber(redis.call('HGET', KEYS[1], 'current') or 0)
local previous = tonumber(redis.call('HGET', KEYS[1], 'previous') or 0)
local stored_index = stored_index_raw and tonumber(stored_index_raw) or window_index

if window_index > stored_index then
  if window_index == stored_index + 1 then
    previous = current
  else
    previous = 0
  end
  current = 0
  stored_index = window_index
end

local window_start = stored_index * window_seconds
local elapsed = now - window_start
local weighted = current + previous * (1 - (elapsed / window_seconds))

current = current + tonumber(ARGV[1])
redis.call('HSET', KEYS[1], 'window_index', stored_index, 'current', current, 'previous', previous)
redis.call('EXPIRE', KEYS[1], window_seconds * 2)

return tostring(weighted + tonumber(ARGV[1]))
`

// RedisStore is the production Store backend: DESIGN.md's Redis hash
// per tenant (budget:{tenant_id}) plus a SETNX-based alert flag
// (budget:{tenant_id}:alerted:{window_index}), both evaluated through
// go-redis against any redis.Cmdable — a *redis.Client for production,
// or a miniredis-backed one in tests, since miniredis runs the same
// Lua interpreter EVAL goes through.
type RedisStore struct {
	rdb goredis.Cmdable
}

// NewRedisStore wraps an existing redis client. The caller owns the
// client's lifecycle (Ping/Close) — this mirrors how consumer's
// ListOverflow takes a pre-dialed client rather than a URL, keeping
// connection policy (TLS, pool size, retries) out of this package.
func NewRedisStore(rdb goredis.Cmdable) *RedisStore {
	return &RedisStore{rdb: rdb}
}

func (s *RedisStore) CheckAndIncrement(ctx context.Context, tenantID string, tokens int64, windowSeconds int64, now time.Time) (float64, error) {
	key := "budget:" + tenantID
	res, err := s.rdb.Eval(ctx, checkAndIncrementScript, []string{key}, tokens, windowSeconds, now.Unix()).Result()
	if err != nil {
		return 0, fmt.Errorf("store: check-and-increment %s: %w", tenantID, err)
	}
	str, ok := res.(string)
	if !ok {
		return 0, fmt.Errorf("store: unexpected script result type %T for %s", res, tenantID)
	}
	var weighted float64
	if _, err := fmt.Sscanf(str, "%g", &weighted); err != nil {
		return 0, fmt.Errorf("store: parse weighted count %q for %s: %w", str, tenantID, err)
	}
	return weighted, nil
}

func (s *RedisStore) MarkAlerted(ctx context.Context, tenantID string, windowSeconds int64, now time.Time) (bool, error) {
	idx := WindowIndex(now, windowSeconds)
	key := fmt.Sprintf("budget:%s:alerted:%d", tenantID, idx)
	ok, err := s.rdb.SetNX(ctx, key, 1, time.Duration(windowSeconds)*time.Second).Result()
	if err != nil {
		return false, fmt.Errorf("store: mark-alerted %s: %w", tenantID, err)
	}
	return ok, nil
}
