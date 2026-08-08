// SPDX-License-Identifier: MIT

package consumer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Reader is the subset of *kafka.Reader Run depends on, so tests can
// exercise the batching/flush loop without a live Kafka broker.
type Reader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
}

// BatchSize and FlushInterval bound how many judge-requests messages
// accumulate before ProcessBatch runs: whichever limit is hit first
// triggers a flush, so a quiet topic still drains within FlushInterval
// instead of samples sitting unscored waiting for a batch to fill. Vars
// rather than consts so tests can shrink FlushInterval instead of
// sleeping for the production default.
var (
	BatchSize     = 25
	FlushInterval = 2 * time.Second
)

// Run reads from r, batches up to BatchSize messages or FlushInterval
// (whichever comes first), and hands each batch to p.ProcessBatch.
// Offsets are committed only after a batch's ProcessBatch call returns
// without error, so a flush failure leaves those messages uncommitted
// and eligible for redelivery — the same replay-on-failure posture the
// root ingestion consumer uses for ClickHouse write failures.
func Run(ctx context.Context, r Reader, p *Processor) error {
	var batch []kafka.Message
	timer := time.NewTimer(FlushInterval)
	defer timer.Stop()

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		raw := make([][]byte, len(batch))
		for i, m := range batch {
			raw[i] = m.Value
		}
		if err := p.ProcessBatch(ctx, raw); err != nil {
			return err
		}
		if err := r.CommitMessages(ctx, batch...); err != nil {
			return fmt.Errorf("consumer: commit %d messages: %w", len(batch), err)
		}
		batch = batch[:0]
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			if err := flush(); err != nil {
				return err
			}
			return ctx.Err()
		case <-timer.C:
			if err := flush(); err != nil {
				return err
			}
			timer.Reset(FlushInterval)
		default:
			fetchCtx, cancel := context.WithTimeout(ctx, FlushInterval)
			msg, err := r.FetchMessage(fetchCtx)
			cancel()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				if err := flush(); err != nil {
					return err
				}
				return fmt.Errorf("consumer: fetch message: %w", err)
			}
			batch = append(batch, msg)
			if len(batch) >= BatchSize {
				if err := flush(); err != nil {
					return err
				}
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(FlushInterval)
			}
		}
	}
}
