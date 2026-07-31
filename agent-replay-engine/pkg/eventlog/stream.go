// SPDX-License-Identifier: MIT
package eventlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Scanner reads a JSON Lines event log one AgentEvent at a time, in the
// style of bufio.Scanner. Unlike ReadJSONL, it never buffers the whole
// log in memory or sorts it — the caller sees events in file order and
// pays only for one line at a time. Use Scanner when the log may be
// larger than comfortably fits in memory; use ReadJSONL when you need
// SeqNum-sorted random access to the whole log.
//
// Scanner trusts the recorder's append-only ordering guarantee instead
// of re-sorting; a log written out of SeqNum order will be read out of
// order too.
type Scanner struct {
	sc  *bufio.Scanner
	cur AgentEvent
	err error
}

// NewScanner returns a Scanner over r. It grows the line buffer the same
// way ReadJSONL does, so a single large tool-response payload doesn't
// error out with bufio.ErrTooLong.
func NewScanner(r io.Reader) *Scanner {
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 16*1024*1024)
	return &Scanner{sc: sc}
}

// Scan advances the Scanner to the next non-blank line and reports
// whether an event is available. Call Event to retrieve it. Scan
// returns false at EOF or on the first malformed line; call Err to
// distinguish the two.
func (s *Scanner) Scan() bool {
	for s.sc.Scan() {
		line := s.sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev AgentEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			s.err = fmt.Errorf("eventlog: scan: %w", err)
			return false
		}
		s.cur = ev
		return true
	}
	if err := s.sc.Err(); err != nil {
		s.err = fmt.Errorf("eventlog: scan: %w", err)
	}
	return false
}

// Event returns the event most recently produced by Scan.
func (s *Scanner) Event() AgentEvent {
	return s.cur
}

// Err returns the first non-EOF error encountered by Scan, if any.
func (s *Scanner) Err() error {
	return s.err
}
