// SPDX-License-Identifier: MIT
// Package export writes recorded eventlog.EventLogs to object storage as
// compressed, checksummed archives. See DESIGN.md at the repo root for the
// export key layout and retention model.
package export

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
	"github.com/akshantvats/agent-replay-engine/pkg/objectstore"
)

// ErrEmptyLog is returned by Export when passed an EventLog with no events —
// there is no trace_id or seq_num range to build a key from.
var ErrEmptyLog = errors.New("export: cannot export an empty event log")

// ErrChecksumMismatch is returned by Verify when the SHA-256 of the
// downloaded object does not match its sidecar checksum file.
var ErrChecksumMismatch = errors.New("export: checksum mismatch")

// sha256Suffix is appended to an exported object's key to name its
// companion checksum file.
const sha256Suffix = ".sha256"

// Exporter compresses an EventLog with zstd and writes it plus a checksum
// sidecar to an ObjectStore under a deterministic trace-scoped key.
type Exporter struct {
	Store objectstore.ObjectStore
}

// Export writes log as traces/{trace_id}/{first_seq:06d}-{last_seq:06d}.jsonl.zst
// and a sibling .sha256 file containing the hex SHA-256 of the compressed
// bytes. log must be non-empty and every event must share the same TraceID.
// Returns the object key written for the compressed archive (not the
// checksum sidecar).
func (e *Exporter) Export(ctx context.Context, log eventlog.EventLog) (string, error) {
	if len(log) == 0 {
		return "", ErrEmptyLog
	}

	traceID, firstSeq, lastSeq, err := traceRange(log)
	if err != nil {
		return "", err
	}

	var jsonl bytes.Buffer
	if err := log.WriteJSONL(&jsonl); err != nil {
		return "", fmt.Errorf("export: serialize jsonl: %w", err)
	}

	compressed, err := compressZstd(jsonl.Bytes())
	if err != nil {
		return "", fmt.Errorf("export: compress: %w", err)
	}

	sum := sha256.Sum256(compressed)
	checksum := hex.EncodeToString(sum[:])

	key := objectKey(traceID, firstSeq, lastSeq)
	if err := e.Store.Put(ctx, key, bytes.NewReader(compressed), int64(len(compressed))); err != nil {
		return "", fmt.Errorf("export: put %s: %w", key, err)
	}

	checksumKey := key + sha256Suffix
	checksumBytes := []byte(checksum)
	if err := e.Store.Put(ctx, checksumKey, bytes.NewReader(checksumBytes), int64(len(checksumBytes))); err != nil {
		return "", fmt.Errorf("export: put %s: %w", checksumKey, err)
	}

	return key, nil
}

// Verify re-downloads the object at key and its checksum sidecar and
// confirms SHA-256(object bytes) == sidecar contents. Returns
// ErrChecksumMismatch (wrapped with the two checksums) if corrupted.
func (e *Exporter) Verify(ctx context.Context, key string) error {
	obj, err := e.Store.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("export: get %s: %w", key, err)
	}
	defer func() { _ = obj.Close() }()

	data, err := io.ReadAll(obj)
	if err != nil {
		return fmt.Errorf("export: read %s: %w", key, err)
	}

	checksumKey := key + sha256Suffix
	sidecar, err := e.Store.Get(ctx, checksumKey)
	if err != nil {
		return fmt.Errorf("export: get %s: %w", checksumKey, err)
	}
	defer func() { _ = sidecar.Close() }()

	wantBytes, err := io.ReadAll(sidecar)
	if err != nil {
		return fmt.Errorf("export: read %s: %w", checksumKey, err)
	}

	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	want := string(wantBytes)

	if got != want {
		return fmt.Errorf("%w: key=%s got=%s want=%s", ErrChecksumMismatch, key, got, want)
	}
	return nil
}

// traceRange returns the shared TraceID and the min/max SeqNum across log.
// Returns an error if log contains more than one distinct TraceID.
func traceRange(log eventlog.EventLog) (traceID string, firstSeq, lastSeq int64, err error) {
	traceID = log[0].TraceID
	firstSeq, lastSeq = log[0].SeqNum, log[0].SeqNum

	for _, ev := range log[1:] {
		if ev.TraceID != traceID {
			return "", 0, 0, fmt.Errorf("export: mixed trace_id in log: %q and %q", traceID, ev.TraceID)
		}
		if ev.SeqNum < firstSeq {
			firstSeq = ev.SeqNum
		}
		if ev.SeqNum > lastSeq {
			lastSeq = ev.SeqNum
		}
	}
	return traceID, firstSeq, lastSeq, nil
}

// objectKey builds the deterministic export key for a trace's seq_num range.
func objectKey(traceID string, firstSeq, lastSeq int64) string {
	return fmt.Sprintf("traces/%s/%06d-%06d.jsonl.zst", traceID, firstSeq, lastSeq)
}

// compressZstd returns data compressed with zstd at the default level.
func compressZstd(data []byte) ([]byte, error) {
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, err
	}
	out := enc.EncodeAll(data, nil)
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out, nil
}
