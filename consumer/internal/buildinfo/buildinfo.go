// SPDX-License-Identifier: MIT
// Package buildinfo holds linker-injected build metadata.
package buildinfo

// Set at link time via -ldflags; defaults suit local `go run`.
var (
	Version   = "dev"
	GitSHA    = "unknown"
	BuildTime = "unknown"
)
