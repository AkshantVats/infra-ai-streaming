// SPDX-License-Identifier: MIT
// traceforge is the CLI for TraceForge — AI trace analytics for tool-call-analyzer.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "graph":
		runGraph(os.Args[2:])
	case "bottleneck":
		runBottleneck(os.Args[2:])
	case "waterfall":
		runWaterfall(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: traceforge <command> [flags]")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  graph        Build and analyze a tool dependency graph for a trace")
	fmt.Fprintln(os.Stderr, "  bottleneck   Rank spans by exclusive time to find the trace bottleneck")
	fmt.Fprintln(os.Stderr, "  waterfall    Build a Grafana-compatible cost waterfall payload for a trace")
}
