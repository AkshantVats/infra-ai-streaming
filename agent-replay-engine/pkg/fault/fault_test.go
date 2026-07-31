// SPDX-License-Identifier: MIT
package fault

import (
	"errors"
	"testing"
)

func TestKindValid(t *testing.T) {
	cases := []struct {
		kind Kind
		want bool
	}{
		{KindTimeout, true},
		{KindHTTP500, true},
		{KindStaleCache, true},
		{Kind("bogus"), false},
		{Kind(""), false},
	}
	for _, c := range cases {
		if got := c.kind.Valid(); got != c.want {
			t.Errorf("Kind(%q).Valid() = %v, want %v", c.kind, got, c.want)
		}
	}
}

func TestKindErrSentinels(t *testing.T) {
	cases := []struct {
		kind Kind
		want error
	}{
		{KindTimeout, ErrTimeout},
		{KindHTTP500, ErrHTTP500},
		{KindStaleCache, ErrStaleCache},
	}
	for _, c := range cases {
		if got := c.kind.Err(); !errors.Is(got, c.want) {
			t.Errorf("Kind(%q).Err() = %v, want %v", c.kind, got, c.want)
		}
	}
}

func TestKindErrUnknownKindIsNotASentinel(t *testing.T) {
	err := Kind("bogus").Err()
	if errors.Is(err, ErrTimeout) || errors.Is(err, ErrHTTP500) || errors.Is(err, ErrStaleCache) {
		t.Errorf("Kind(bogus).Err() = %v, unexpectedly matched a known sentinel", err)
	}
	if err == nil {
		t.Error("Kind(bogus).Err() = nil, want a non-nil error")
	}
}
