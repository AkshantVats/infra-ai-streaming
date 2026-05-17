package redis

import (
	"context"
	"encoding/json"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/akshantvats/infra-ai-streaming/consumer/internal/metrics"
	"github.com/akshantvats/infra-ai-streaming/consumer/internal/model"
)

// OverflowBuffer spills events to a Redis LIST when ClickHouse is degraded.
type OverflowBuffer interface {
	Push(ctx context.Context, events []model.InferenceEvent) error
	PopN(ctx context.Context, n int) ([]model.InferenceEvent, error)
	Depth(ctx context.Context) (int64, error)
}

// ListOverflow implements OverflowBuffer with LPUSH / RPOP.
type ListOverflow struct {
	client *goredis.Client
	key    string
	m      *metrics.M
}

// NewListOverflow connects to Redis and returns an overflow buffer.
func NewListOverflow(ctx context.Context, redisURL, key string, m *metrics.M) (*ListOverflow, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	o := &ListOverflow{client: client, key: key, m: m}
	_ = o.refreshDepth(ctx)
	return o, nil
}

// Close releases the Redis client.
func (o *ListOverflow) Close() error {
	return o.client.Close()
}

// Push serializes events and LPUSHes them onto the overflow list.
func (o *ListOverflow) Push(ctx context.Context, events []model.InferenceEvent) error {
	if len(events) == 0 {
		return nil
	}
	pipe := o.client.Pipeline()
	for _, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal overflow event: %w", err)
		}
		pipe.LPush(ctx, o.key, b)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis lpush: %w", err)
	}
	return o.refreshDepth(ctx)
}

// PopN removes up to n events from the tail (FIFO drain).
func (o *ListOverflow) PopN(ctx context.Context, n int) ([]model.InferenceEvent, error) {
	if n <= 0 {
		return nil, nil
	}
	out := make([]model.InferenceEvent, 0, n)
	for i := 0; i < n; i++ {
		val, err := o.client.RPop(ctx, o.key).Result()
		if err == goredis.Nil {
			break
		}
		if err != nil {
			return out, fmt.Errorf("redis rpop: %w", err)
		}
		var e model.InferenceEvent
		if err := json.Unmarshal([]byte(val), &e); err != nil {
			return out, fmt.Errorf("unmarshal overflow event: %w", err)
		}
		out = append(out, e)
	}
	_ = o.refreshDepth(ctx)
	return out, nil
}

// Depth returns LLEN of the overflow list.
func (o *ListOverflow) Depth(ctx context.Context) (int64, error) {
	n, err := o.client.LLen(ctx, o.key).Result()
	if err != nil {
		return 0, err
	}
	o.m.RedisOverflowDepth.Set(float64(n))
	return n, nil
}

func (o *ListOverflow) refreshDepth(ctx context.Context) error {
	_, err := o.Depth(ctx)
	return err
}
