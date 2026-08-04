// SPDX-License-Identifier: MIT

// Command threshold-sweep validates DESIGN.md §8's shipped 0.92 default
// similarity threshold against a held-out labeled prompt-pair set,
// sweeping candidate thresholds and reporting precision, recall, and
// false-positive rate at each one -- the concrete data DESIGN.md §3's
// "tenants that have validated their own prompt distribution can lower it
// via the per-tenant config" assumes exists somewhere. Its output is
// pasted into BENCHMARKS.md, not read by any other program.
//
// It uses pkg/localsim instead of pkg/embedder because this sandbox's
// OPENAI_API_KEY is at its billing quota limit -- see pkg/localsim's
// package doc for the full explanation and the resulting caveat on these
// numbers.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/akshantvats/semantic-cache-engine/pkg/localsim"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// defaultThresholds mirrors Day 63's plan item: sweep 0.88-0.96.
var defaultThresholds = []float64{0.88, 0.90, 0.92, 0.94, 0.96}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("threshold-sweep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputPath := fs.String("input", "testdata/threshold-sweep-pairs.jsonl", "path to a JSON Lines file of labeled prompt pairs: {\"prompt_a\",\"prompt_b\",\"duplicate\"} per line")
	thresholdsFlag := fs.String("thresholds", "", "comma-separated thresholds to sweep, e.g. 0.60,0.70,0.80 (default: the DESIGN.md-calibrated 0.88,0.90,0.92,0.94,0.96)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	thresholds := defaultThresholds
	if *thresholdsFlag != "" {
		parsed, err := parseThresholds(*thresholdsFlag)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "threshold-sweep: --thresholds: %v\n", err)
			return 2
		}
		thresholds = parsed
	}

	f, err := os.Open(*inputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "threshold-sweep: open %s: %v\n", *inputPath, err)
		return 1
	}
	defer f.Close()

	pairs, err := parsePairs(f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "threshold-sweep: parse %s: %v\n", *inputPath, err)
		return 1
	}
	if len(pairs) == 0 {
		_, _ = fmt.Fprintf(stderr, "threshold-sweep: %s contains no labeled pairs\n", *inputPath)
		return 1
	}

	scored := make([]scoredPair, len(pairs))
	for i, p := range pairs {
		scored[i] = scoredPair{pair: p, similarity: localsim.TokenCosineSimilarity(p.PromptA, p.PromptB)}
	}

	results := Sweep(scored, thresholds)

	_, _ = fmt.Fprintf(stdout, "threshold-sweep: %d labeled pairs (%d duplicate, %d distinct) from %s\n\n",
		len(pairs), countDuplicates(pairs), len(pairs)-countDuplicates(pairs), *inputPath)
	_, _ = fmt.Fprintln(stdout, "| Threshold | Precision | Recall | False Positive Rate | TP | FP | FN | TN |")
	_, _ = fmt.Fprintln(stdout, "|---|---|---|---|---|---|---|---|")
	for _, r := range results {
		_, _ = fmt.Fprintf(stdout, "| %.2f | %s | %s | %s | %d | %d | %d | %d |\n",
			r.Threshold, formatRate(r.Precision, r.PrecisionOK), formatRate(r.Recall, r.RecallOK), formatRate(r.FalsePositiveRate, r.FPROK), r.TP, r.FP, r.FN, r.TN)
	}
	return 0
}

func parseThresholds(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid threshold %q: %w", p, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func countDuplicates(pairs []labeledPair) int {
	n := 0
	for _, p := range pairs {
		if p.Duplicate {
			n++
		}
	}
	return n
}

func formatRate(r float64, ok bool) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", r)
}

// labeledPair is one JSON Lines record in the --input file.
type labeledPair struct {
	PromptA   string `json:"prompt_a"`
	PromptB   string `json:"prompt_b"`
	Duplicate bool   `json:"duplicate"`
}

func parsePairs(r io.Reader) ([]labeledPair, error) {
	var out []labeledPair
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		var p labeledPair
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		if p.PromptA == "" || p.PromptB == "" {
			return nil, fmt.Errorf("line %d: prompt_a and prompt_b are required", lineNum)
		}
		out = append(out, p)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scoredPair pairs a labeled example with its computed similarity, so
// Sweep can be re-run against many thresholds without recomputing
// similarity each time.
type scoredPair struct {
	pair       labeledPair
	similarity float64
}

// SweepResult is one threshold's confusion-matrix counts and the derived
// rates DESIGN.md §4 (false-positive budget) and §3 (per-tenant threshold
// tuning) both need: precision (of predicted duplicates, how many really
// were), recall (of true duplicates, how many were caught), and false
// positive rate (of true distinct pairs, how many were wrongly predicted
// duplicate -- the same quantity DESIGN.md §4 puts a 0.1% budget on).
type SweepResult struct {
	Threshold                            float64
	TP, FP, FN, TN                       int
	Precision, Recall, FalsePositiveRate float64
	PrecisionOK, RecallOK, FPROK         bool
}

// Sweep classifies every scored pair against each threshold (predicted
// duplicate iff similarity >= threshold) and returns one SweepResult per
// threshold, sorted ascending. A rate is reported as not-ok (n/a) when its
// denominator is 0 rather than divided as 0/0, e.g. Precision is n/a for a
// threshold so high that nothing is ever predicted duplicate.
func Sweep(scored []scoredPair, thresholds []float64) []SweepResult {
	out := make([]SweepResult, 0, len(thresholds))
	sorted := append([]float64(nil), thresholds...)
	sort.Float64s(sorted)

	for _, threshold := range sorted {
		var tp, fp, fn, tn int
		for _, s := range scored {
			predicted := s.similarity >= threshold
			switch {
			case predicted && s.pair.Duplicate:
				tp++
			case predicted && !s.pair.Duplicate:
				fp++
			case !predicted && s.pair.Duplicate:
				fn++
			default:
				tn++
			}
		}

		r := SweepResult{Threshold: threshold, TP: tp, FP: fp, FN: fn, TN: tn}
		if tp+fp > 0 {
			r.Precision, r.PrecisionOK = float64(tp)/float64(tp+fp), true
		}
		if tp+fn > 0 {
			r.Recall, r.RecallOK = float64(tp)/float64(tp+fn), true
		}
		if fp+tn > 0 {
			r.FalsePositiveRate, r.FPROK = float64(fp)/float64(fp+tn), true
		}
		out = append(out, r)
	}
	return out
}
