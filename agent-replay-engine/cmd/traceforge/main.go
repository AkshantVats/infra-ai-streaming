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

	"github.com/akshantvats/agent-replay-engine/pkg/diff"
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
		fmt.Fprintln(stderr, "  diff --log <path> --trace-a <id> --trace-b <id>")
		return 2
	}

	switch args[0] {
	case "replay":
		return runReplay(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
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

func runDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	logPath := fs.String("log", "", "path to a recorded event log (JSON Lines) containing both traces")
	traceA := fs.String("trace-a", "", "trace_id of the first trace")
	traceB := fs.String("trace-b", "", "trace_id of the second trace")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *logPath == "" || *traceA == "" || *traceB == "" {
		fmt.Fprintln(stderr, "traceforge diff: --log, --trace-a and --trace-b are required")
		fs.Usage()
		return 2
	}

	f, err := os.Open(*logPath)
	if err != nil {
		fmt.Fprintf(stderr, "traceforge diff: %v\n", err)
		return 1
	}
	defer f.Close()

	full, err := eventlog.ReadJSONL(f)
	if err != nil {
		fmt.Fprintf(stderr, "traceforge diff: reading log: %v\n", err)
		return 1
	}

	logA := full.FilterByTraceID(*traceA)
	logB := full.FilterByTraceID(*traceB)
	if len(logA) == 0 {
		fmt.Fprintf(stderr, "traceforge diff: no events found for trace_id=%q (--trace-a) in %s\n", *traceA, *logPath)
		return 1
	}
	if len(logB) == 0 {
		fmt.Fprintf(stderr, "traceforge diff: no events found for trace_id=%q (--trace-b) in %s\n", *traceB, *logPath)
		return 1
	}

	result := diff.Compare(logA, logB)

	fmt.Fprintf(stdout, "trace_a: %s (%d tool calls)\n", *traceA, result.StepsTotalA)
	fmt.Fprintf(stdout, "trace_b: %s (%d tool calls)\n", *traceB, result.StepsTotalB)

	if !result.Found() {
		fmt.Fprintf(stdout, "no divergence — %d shared step(s) matched\n", result.StepsCompared)
		return 0
	}

	d := result.Divergence
	fmt.Fprintf(stdout, "first divergence at step %d (%d matching step(s) before it):\n", d.StepIndex, result.StepsCompared)
	fmt.Fprintf(stdout, "  reason: %s\n", d.Reason)
	fmt.Fprintf(stdout, "  trace_a: span_id=%s tool_name=%s\n", valueOr(d.SpanIDA, "<none>"), valueOr(d.ToolNameA, "<none>"))
	fmt.Fprintf(stdout, "  trace_b: span_id=%s tool_name=%s\n", valueOr(d.SpanIDB, "<none>"), valueOr(d.ToolNameB, "<none>"))
	return 0
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
