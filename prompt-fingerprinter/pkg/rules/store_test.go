// SPDX-License-Identifier: MIT

package rules

import (
	"context"
	"sync"
	"testing"

	"github.com/akshantvats/prompt-fingerprinter/pkg/fingerprint"
)

func TestForTenant_UnconfiguredReturnsZeroValue(t *testing.T) {
	s := NewStore()
	got := s.ForTenant(context.Background(), "tenant-never-configured")
	if got != (fingerprint.Rules{}) {
		t.Errorf("unconfigured tenant: got %+v, want zero value", got)
	}
}

func TestPut_ThenForTenant_RoundTrips(t *testing.T) {
	s := NewStore()
	want := fingerprint.Rules{StripPunctuation: true, Lowercase: true, MaxPromptBytes: 4096}
	if err := s.Put("tenant-a", want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got := s.ForTenant(context.Background(), "tenant-a")
	if got != want {
		t.Errorf("ForTenant after Put: got %+v, want %+v", got, want)
	}
}

func TestPut_IsFullReplaceNotPatch(t *testing.T) {
	s := NewStore()
	if err := s.Put("tenant-a", fingerprint.Rules{StripPunctuation: true, MaxPromptBytes: 100}); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	// A second Put with only Lowercase set must NOT merge with the first
	// call's StripPunctuation/MaxPromptBytes — PUT replaces wholesale.
	if err := s.Put("tenant-a", fingerprint.Rules{Lowercase: true}); err != nil {
		t.Fatalf("second Put: %v", err)
	}
	got := s.ForTenant(context.Background(), "tenant-a")
	want := fingerprint.Rules{Lowercase: true}
	if got != want {
		t.Errorf("second Put should fully replace: got %+v, want %+v", got, want)
	}
}

func TestPut_RejectsNegativeMaxPromptBytes(t *testing.T) {
	s := NewStore()
	err := s.Put("tenant-a", fingerprint.Rules{MaxPromptBytes: -1})
	if err != ErrNegativeMaxPromptBytes {
		t.Errorf("got err %v, want ErrNegativeMaxPromptBytes", err)
	}
}

func TestPut_RejectsMaxPromptBytesAboveCeiling(t *testing.T) {
	s := NewStore()
	err := s.Put("tenant-a", fingerprint.Rules{MaxPromptBytes: MaxPromptBytesCeiling + 1})
	if err != ErrMaxPromptBytesTooLarge {
		t.Errorf("got err %v, want ErrMaxPromptBytesTooLarge", err)
	}
}

func TestPut_ValidationFailureLeavesStoreUnchanged(t *testing.T) {
	s := NewStore()
	original := fingerprint.Rules{Lowercase: true}
	if err := s.Put("tenant-a", original); err != nil {
		t.Fatalf("seed Put: %v", err)
	}
	if err := s.Put("tenant-a", fingerprint.Rules{MaxPromptBytes: -1}); err == nil {
		t.Fatal("expected validation error, got nil")
	}
	got := s.ForTenant(context.Background(), "tenant-a")
	if got != original {
		t.Errorf("failed Put must not mutate store: got %+v, want unchanged %+v", got, original)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = s.Put("tenant-a", fingerprint.Rules{MaxPromptBytes: i})
		}(i)
		go func() {
			defer wg.Done()
			_ = s.ForTenant(context.Background(), "tenant-a")
		}()
	}
	wg.Wait()
}
