package buildinfo

import "testing"

func TestBuildInfoDefaultsNonEmpty(t *testing.T) {
	for _, v := range []string{Version, GitSHA, BuildTime} {
		if v == "" {
			t.Fatal("build metadata must not be empty")
		}
	}
}
