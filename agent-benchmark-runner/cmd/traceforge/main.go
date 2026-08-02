// SPDX-License-Identifier: MIT
// Command traceforge is the CLI entry point for agent-benchmark-runner.
// Its "run" subcommand executes a task YAML against one or two agent
// subprocesses, N times each, and prints a pass-rate summary plus (with
// two agents) a divergence report — see DESIGN.md and
// pkg/subprocess's "Agent Under Test as a Subprocess" doc comment for
// why the agent under test is invoked as an external command rather than
// linked in as a Go package.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akshantvats/agent-benchmark-runner/pkg/compare"
	"github.com/akshantvats/agent-benchmark-runner/pkg/criteria"
	"github.com/akshantvats/agent-benchmark-runner/pkg/lensai"
	"github.com/akshantvats/agent-benchmark-runner/pkg/orchestrator"
	"github.com/akshantvats/agent-benchmark-runner/pkg/report"
	"github.com/akshantvats/agent-benchmark-runner/pkg/subprocess"
	"github.com/akshantvats/agent-benchmark-runner/pkg/task"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI and returns the process exit code. Splitting
// this out of main keeps the subcommand logic testable without exec'ing
// a built binary — the same pattern agent-replay-engine's cmd/traceforge
// uses.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		printUsage(stderr)
		return 2
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	taskPath := fs.String("task", "", "path to a task YAML file")
	agentAName := fs.String("agent-a-name", "agent-a", "name for agent A")
	agentACmd := fs.String("agent-a-cmd", "", "shell command invoked once per repetition for agent A (required)")
	agentBName := fs.String("agent-b-name", "", "name for agent B (optional — enables comparison)")
	agentBCmd := fs.String("agent-b-cmd", "", "shell command invoked once per repetition for agent B (optional)")
	repetitions := fs.Int("repetitions", 10, "repetitions per agent")
	maxParallel := fs.Int("max-parallel", 4, "max concurrent repetitions per agent")
	outDir := fs.String("out", ".", "directory the comparison report is written to")
	lensaiURL := fs.String("lensai-url", "", "LensAI ingest URL for an optional batch-completion dual-write")
	tenantID := fs.String("tenant-id", "", "tenant id (required when --lensai-url is set)")
	lensaiDashboard := fs.String("lensai-dashboard", "", "LensAI dashboard base URL, cross-linked from the landing page (optional, requires --tenant-id)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	if *taskPath == "" || *agentACmd == "" {
		fmt.Fprintln(stderr, "run: --task and --agent-a-cmd are required")
		return 2
	}
	twoAgents := *agentBCmd != ""
	if twoAgents && *agentBName == "" {
		fmt.Fprintln(stderr, "run: --agent-b-name is required when --agent-b-cmd is set")
		return 2
	}
	if *lensaiURL != "" && *tenantID == "" {
		fmt.Fprintln(stderr, "run: --tenant-id is required when --lensai-url is set")
		return 2
	}
	if *lensaiDashboard != "" && *tenantID == "" {
		fmt.Fprintln(stderr, "run: --tenant-id is required when --lensai-dashboard is set")
		return 2
	}

	t, err := task.LoadFile(*taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	ctx := context.Background()
	start := time.Now()

	resultsA, err := orchestrator.Run(ctx,
		orchestrator.Config{Task: t, AgentName: *agentAName, Repetitions: *repetitions, MaxParallel: *maxParallel},
		subprocess.AgentFunc(*agentACmd))
	if err != nil {
		fmt.Fprintf(stderr, "run: agent A batch: %v\n", err)
		return 1
	}
	summaryA := orchestrator.Summarize(resultsA)
	printSummary(stdout, *agentAName, summaryA)
	allPassed := summaryA.Completed > 0 && summaryA.Passed == summaryA.Completed

	if twoAgents {
		resultsB, err := orchestrator.Run(ctx,
			orchestrator.Config{Task: t, AgentName: *agentBName, Repetitions: *repetitions, MaxParallel: *maxParallel},
			subprocess.AgentFunc(*agentBCmd))
		if err != nil {
			fmt.Fprintf(stderr, "run: agent B batch: %v\n", err)
			return 1
		}
		summaryB := orchestrator.Summarize(resultsB)
		printSummary(stdout, *agentBName, summaryB)
		allPassed = allPassed && summaryB.Completed > 0 && summaryB.Passed == summaryB.Completed

		if outcomeA, ok := firstOutcome(resultsA); ok {
			if outcomeB, ok := firstOutcome(resultsB); ok {
				cmpResult := compare.Compare(t,
					compare.AgentRun{AgentName: *agentAName, Outcome: outcomeA},
					compare.AgentRun{AgentName: *agentBName, Outcome: outcomeB})
				rep := report.Build(cmpResult, outcomeA.ToolCallSequence, outcomeB.ToolCallSequence)
				fmt.Fprintf(stdout, "compare: %s\n", rep.Headline)

				lensaiLink := ""
				if *lensaiDashboard != "" {
					lensaiLink = fmt.Sprintf("%s/tenants/%s/traces/%s", strings.TrimRight(*lensaiDashboard, "/"), *tenantID, t.TaskID)
				}
				if err := writeReport(*outDir, t.TaskID, rep, lensaiLink); err != nil {
					fmt.Fprintf(stderr, "run: write report: %v\n", err)
					return 1
				}
			}
		}
	}

	if *lensaiURL != "" {
		w := lensai.New(*lensaiURL)
		batchID := fmt.Sprintf("%s-%d", t.TaskID, start.UnixNano())
		if err := w.Insert(ctx, summaryA, lensai.BatchParams{
			TaskID:      t.TaskID,
			AgentName:   *agentAName,
			TenantID:    *tenantID,
			BatchID:     batchID,
			Duration:    time.Since(start),
			CompletedAt: time.Now(),
		}); err != nil {
			// A failed dual-write does not fail the benchmark run itself —
			// ClickHouse via pkg/store stays the source of truth; LensAI
			// is an additive cross-product view. See pkg/lensai's doc
			// comment.
			fmt.Fprintf(stderr, "run: lensai dual-write: %v\n", err)
		}
	}

	if !allPassed {
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: traceforge run --task <path> --agent-a-cmd <shell-cmd> [flags]")
	fmt.Fprintln(w, "flags:")
	fmt.Fprintln(w, "  --task <path>              task YAML file (required)")
	fmt.Fprintln(w, "  --agent-a-name <name>      name for agent A (default \"agent-a\")")
	fmt.Fprintln(w, "  --agent-a-cmd <cmd>        shell command run once per repetition for agent A (required)")
	fmt.Fprintln(w, "  --agent-b-name <name>      name for agent B (enables comparison)")
	fmt.Fprintln(w, "  --agent-b-cmd <cmd>        shell command for agent B")
	fmt.Fprintln(w, "  --repetitions <n>          repetitions per agent (default 10)")
	fmt.Fprintln(w, "  --max-parallel <n>         max concurrent repetitions (default 4)")
	fmt.Fprintln(w, "  --out <dir>                comparison report directory (default \".\")")
	fmt.Fprintln(w, "  --lensai-url <url>         optional LensAI ingest URL for a batch-completion dual-write")
	fmt.Fprintln(w, "  --tenant-id <id>           required with --lensai-url or --lensai-dashboard")
	fmt.Fprintln(w, "  --lensai-dashboard <url>   optional LensAI dashboard base URL, cross-linked from the landing page")
}

func printSummary(w io.Writer, agentName string, s orchestrator.Summary) {
	fmt.Fprintf(w, "%s: %d/%d passed (%.1f%%, 95%% CI [%.1f%%, %.1f%%])\n",
		agentName, s.Passed, s.Completed, s.PassRate*100, s.CILow*100, s.CIHigh*100)
}

// firstOutcome returns the first completed repetition's outcome, since a
// side-by-side divergence report compares one representative run per
// agent, not the whole batch — the batch's statistical signal already
// lives in the printed Summary.
func firstOutcome(results []orchestrator.RunResult) (criteria.RunOutcome, bool) {
	for _, r := range results {
		if r.Err == nil {
			return r.Outcome, true
		}
	}
	return criteria.RunOutcome{}, false
}

func writeReport(dir, taskID string, rep report.Report, lensaiLink string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	mdFile, err := os.Create(filepath.Join(dir, taskID+"-report.md"))
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	defer mdFile.Close()
	if err := report.RenderMarkdown(mdFile, rep); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	jsonFile, err := os.Create(filepath.Join(dir, taskID+"-report.json"))
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	defer jsonFile.Close()
	if err := report.RenderJSON(jsonFile, rep); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	htmlFile, err := os.Create(filepath.Join(dir, taskID+"-landing.html"))
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	defer htmlFile.Close()
	if err := report.RenderLandingHTML(htmlFile, rep, lensaiLink); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
