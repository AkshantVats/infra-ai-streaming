// SPDX-License-Identifier: MIT
// Command traceforge is the CLI entry point for agent-replay-engine.
// Its first subcommand, replay, replays a recorded event log through
// pkg/replay, optionally halting after a chosen number of steps via
// --stop-at-step — see DESIGN.md's Replay Algorithm section.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/akshantvats/agent-replay-engine/pkg/eventlog"
	"github.com/akshantvats/agent-replay-engine/pkg/mocker"
	"github.com/akshantvats/agent-replay-engine/pkg/replay"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI and returns the process exit code. Splitting this
// out of main keeps the subcommand logic testable without exec'ing a
// built binary.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: traceforge <command> [flags]")
		fmt.Fprintln(stderr, "commands:")
		fmt.Fprintln(stderr, "  replay --log <path> --trace-id <id> [--stop-at-step N]")
		return 2
	}

	switch args[0] {
	case "replay":
		return runReplay(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "traceforge: unknown command %q\n", args[0])
		return 2
	}
}

func runReplay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	logPath := fs.String("log", "", "path to a recorded event log (JSON Lines)")
	traceID := fs.String("trace-id", "", "trace_id of the run to replay")
	stopAtStep := fs.Int("stop-at-step", 0, "halt after this many tool-call steps (0 = run to completion)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *logPath == "" || *traceID == "" {
		fmt.Fprintln(stderr, "traceforge replay: --log and --trace-id are required")
		fs.Usage()
		return 2
	}

	f, err := os.Open(*logPath)
	if err != nil {
		fmt.Fprintf(stderr, "traceforge replay: %v\n", err)
		return 1
	}
	defer f.Close()

	full, err := eventlog.ReadJSONL(f)
	if err != nil {
		fmt.Fprintf(stderr, "traceforge replay: reading log: %v\n", err)
		return 1
	}

	log := full.FilterByTraceID(*traceID)
	if len(log) == 0 {
		fmt.Fprintf(stderr, "traceforge replay: no events found for trace_id=%q in %s\n", *traceID, *logPath)
		return 1
	}
	if err := log.Validate(); err != nil {
		fmt.Fprintf(stderr, "traceforge replay: invalid log: %v\n", err)
		return 1
	}

	m, err := mocker.LoadFromLog(log)
	if err != nil {
		fmt.Fprintf(stderr, "traceforge replay: %v\n", err)
		return 1
	}

	result := replay.Run(log, m, *stopAtStep)

	fmt.Fprintf(stdout, "trace_id: %s\n", *traceID)
	fmt.Fprintf(stdout, "steps run: %d\n", result.StepsRun)

	if result.Err != nil {
		fmt.Fprintf(stderr, "traceforge replay: %v\n", result.Err)
		return 1
	}
	if result.StoppedEarly {
		fmt.Fprintf(stdout, "stopped early: halted at --stop-at-step=%d, %d recorded step(s) remain unreplayed\n",
			*stopAtStep, len(log.AllOfKind(eventlog.KindToolCall))-result.StepsRun)
		return 0
	}

	fmt.Fprintf(stdout, "output: %s\n", result.Output)
	return 0
}
