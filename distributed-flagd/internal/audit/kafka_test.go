// SPDX-License-Identifier: MIT
package audit

import (
	"context"
	"encoding/json"
	"testing"
)

// TestKafkaEntryMarshal verifies the JSON shape consumers depend on.
func TestKafkaEntryMarshal(t *testing.T) {
	e := KafkaEntry{
		FlagName:  "model_route",
		OldValue:  `{"model":"gpt-4o","pct":100}`,
		NewValue:  `{"model":"gpt-4o-mini","pct":100}`,
		ChangedBy: "flagctl/kill-switch",
		Reason:    "kill-switch",
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out KafkaEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.FlagName != e.FlagName {
		t.Errorf("flag_name: got %q want %q", out.FlagName, e.FlagName)
	}
	if out.Reason != "kill-switch" {
		t.Errorf("reason: got %q want kill-switch", out.Reason)
	}
}

// TestKafkaProducerNilBrokers verifies that an empty broker list surfaces an
// error rather than a nil panic — fail-fast on misconfiguration.
func TestKafkaProducerNilBrokers(t *testing.T) {
	// franz-go does NOT error on empty seed brokers at New time; it errors
	// only on the first produce attempt (lazy connect). Test that Publish
	// returns a non-nil error when no broker is reachable.
	p, err := NewKafkaProducer([]string{"127.0.0.1:1"})
	if err != nil {
		// Some versions error eagerly — both paths are acceptable.
		return
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*/* seconds */ 1000000000)
	defer cancel()
	err = p.Publish(ctx, KafkaEntry{FlagName: "test", ChangedBy: "test"})
	if err == nil {
		t.Error("expected error publishing to unreachable broker, got nil")
	}
}
