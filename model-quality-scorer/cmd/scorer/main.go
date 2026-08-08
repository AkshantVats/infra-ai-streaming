// SPDX-License-Identifier: MIT

// Command scorer is model-quality-scorer's Day 78 vertical slice: it
// reads a JSON Lines file of judge-requests-shaped samples
// ({"tenant_id","task_type","model_id","rubric_version","prompt","response"})
// and runs each one through pkg/consumer.Processor's fixed pipeline —
// resolve the shared rubric template, score against it, and either
// print the scored row or the DLQ reason. "Stub" describes the judge
// Caller wired in here (a deterministic heuristic; see DESIGN.md's
// "Out of scope" note — no live Haiku calls exercised in this
// sandbox) and the printing Writer/DLQ (no live ClickHouse/Kafka
// required to run this), not the batching, rubric resolution, or
// scoring math, all of which run for real against pkg/rubric's
// WeightedScore.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/akshantvats/model-quality-scorer/pkg/consumer"
	"github.com/akshantvats/model-quality-scorer/pkg/dlq"
	"github.com/akshantvats/model-quality-scorer/pkg/judge"
	"github.com/akshantvats/model-quality-scorer/pkg/rubric"
	"github.com/akshantvats/model-quality-scorer/pkg/store"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI and returns the process exit code, kept
// separate from main for testability without exec'ing a built binary
// (same shape as cost-budget-enforcer/cmd/stubgateway/main.go::run).
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scorer", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputPath := fs.String("input", "", "path to a JSON Lines file of judge-requests-shaped samples")
	rubricsDir := fs.String("rubrics-dir", "rubrics", "directory of <task_type>.v<version>.json rubric templates")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *inputPath == "" {
		_, _ = fmt.Fprintln(stderr, "scorer: --input is required")
		return 2
	}

	f, err := os.Open(*inputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "scorer: open input: %v\n", err)
		return 1
	}
	defer func() { _ = f.Close() }()

	w := &printingWriter{out: stdout}
	d := &printingDLQ{out: stderr}
	rubrics := consumer.NewFileRubricStore(*rubricsDir)
	j := &heuristicJudge{}
	p := consumer.NewProcessor(rubrics, j, w, d)

	var lines [][]byte
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, []byte(line))
	}
	if err := scanner.Err(); err != nil {
		_, _ = fmt.Fprintf(stderr, "scorer: read input: %v\n", err)
		return 1
	}

	if err := p.ProcessBatch(context.Background(), lines); err != nil {
		_, _ = fmt.Fprintf(stderr, "scorer: process batch: %v\n", err)
		return 1
	}
	return 0
}

// printingWriter implements store.Writer by printing each scored row —
// a stand-in for a live ClickHouse quality_scores insert.
type printingWriter struct {
	out io.Writer
}

func (w *printingWriter) WriteBatch(_ context.Context, rows []store.ScoredSample) error {
	for _, r := range rows {
		_, _ = fmt.Fprintf(w.out, "tenant=%s task_type=%s model=%s score=%.1f rationale=%q\n",
			r.TenantID, r.TaskType, r.ModelID, r.Score, r.Rationale)
	}
	return nil
}

// printingDLQ implements dlq.Publisher by printing each dead-lettered
// entry — a stand-in for a live judge-requests-dlq publish.
type printingDLQ struct {
	out io.Writer
}

func (d *printingDLQ) Publish(_ context.Context, entry dlq.Entry) error {
	_, _ = fmt.Fprintf(d.out, "dlq reason=%s detail=%q\n", entry.Reason, entry.Detail)
	return nil
}

// heuristicJudge is a deterministic stand-in for judge.HaikuJudge: it
// scores every criterion by response length relative to prompt length
// (longer, non-empty responses score higher, capped at 10) so this
// binary is runnable without an API key, per DESIGN.md's explicit
// deferral of the live judge call to a later day.
type heuristicJudge struct{}

func (heuristicJudge) Score(_ context.Context, r rubric.JudgeRubric, s judge.Sample) (judge.Result, error) {
	if strings.TrimSpace(s.Response) == "" {
		return judge.Result{}, judge.ErrJudgeUnavailable
	}
	scoreVal := float64(len(s.Response)) / float64(len(s.Prompt)+1) * 5
	if scoreVal > 10 {
		scoreVal = 10
	}
	scores := make(map[string]float64, len(r.Criteria))
	for _, c := range r.Criteria {
		scores[c.Name] = scoreVal
	}
	return judge.Result{
		Scores:    scores,
		Rationale: "heuristic stub: scored by response/prompt length ratio, not a real Haiku call",
	}, nil
}
