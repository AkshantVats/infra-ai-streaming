// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestParsePairsHappyPath(t *testing.T) {
	input := `{"prompt_a":"a","prompt_b":"b","duplicate":true}
{"prompt_a":"c","prompt_b":"d","duplicate":false}
`
	got, err := parsePairs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parsePairs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(got))
	}
	if !got[0].Duplicate || got[1].Duplicate {
		t.Fatalf("unexpected duplicate flags: %+v", got)
	}
}

func TestParsePairsRejectsMissingFields(t *testing.T) {
	if _, err := parsePairs(strings.NewReader(`{"prompt_a":"a","duplicate":true}`)); err == nil {
		t.Fatal("expected an error for a record missing prompt_b")
	}
}

func TestSweepPerfectSeparation(t *testing.T) {
	// Two pairs: one clearly duplicate (similarity 1.0), one clearly
	// distinct (similarity 0.0). Any threshold strictly between them
	// should classify both correctly.
	scored := []scoredPair{
		{pair: labeledPair{Duplicate: true}, similarity: 1.0},
		{pair: labeledPair{Duplicate: false}, similarity: 0.0},
	}
	results := Sweep(scored, []float64{0.5})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.TP != 1 || r.TN != 1 || r.FP != 0 || r.FN != 0 {
		t.Fatalf("confusion matrix = %+v, want TP=1 TN=1 FP=0 FN=0", r)
	}
	if !r.PrecisionOK || r.Precision != 1.0 {
		t.Errorf("Precision = %v (ok=%v), want 1.0", r.Precision, r.PrecisionOK)
	}
	if !r.RecallOK || r.Recall != 1.0 {
		t.Errorf("Recall = %v (ok=%v), want 1.0", r.Recall, r.RecallOK)
	}
	if !r.FPROK || r.FalsePositiveRate != 0 {
		t.Errorf("FalsePositiveRate = %v (ok=%v), want 0", r.FalsePositiveRate, r.FPROK)
	}
}

func TestSweepHigherThresholdIncreasesPrecisionDecreasesRecall(t *testing.T) {
	scored := []scoredPair{
		{pair: labeledPair{Duplicate: true}, similarity: 0.95},
		{pair: labeledPair{Duplicate: true}, similarity: 0.89},
		{pair: labeledPair{Duplicate: false}, similarity: 0.90},
	}
	results := Sweep(scored, []float64{0.88, 0.94})
	low, high := results[0], results[1]

	if !low.PrecisionOK || !high.PrecisionOK {
		t.Fatal("expected both thresholds to have a defined precision")
	}
	if high.Precision < low.Precision {
		t.Errorf("higher threshold precision %.3f should be >= lower threshold precision %.3f", high.Precision, low.Precision)
	}
	if high.Recall > low.Recall {
		t.Errorf("higher threshold recall %.3f should be <= lower threshold recall %.3f", high.Recall, low.Recall)
	}
}

func TestSweepReportsNAWhenDenominatorIsZero(t *testing.T) {
	// Threshold so high nothing is predicted duplicate: TP+FP = 0, so
	// Precision must be reported not-ok rather than a fabricated 0/0.
	scored := []scoredPair{
		{pair: labeledPair{Duplicate: true}, similarity: 0.5},
	}
	results := Sweep(scored, []float64{0.99})
	r := results[0]
	if r.PrecisionOK {
		t.Errorf("Precision should be not-ok when nothing is predicted duplicate, got %v", r.Precision)
	}
}

func TestParseThresholdsHappyPath(t *testing.T) {
	got, err := parseThresholds("0.60, 0.70,0.80")
	if err != nil {
		t.Fatalf("parseThresholds: %v", err)
	}
	want := []float64{0.60, 0.70, 0.80}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseThresholdsRejectsInvalidNumber(t *testing.T) {
	if _, err := parseThresholds("0.5,not-a-number"); err == nil {
		t.Fatal("expected an error for a non-numeric threshold")
	}
}

func TestSweepSortsThresholdsAscending(t *testing.T) {
	results := Sweep([]scoredPair{{pair: labeledPair{Duplicate: true}, similarity: 1.0}}, []float64{0.96, 0.88, 0.92})
	for i := 1; i < len(results); i++ {
		if results[i].Threshold < results[i-1].Threshold {
			t.Fatalf("results not sorted ascending: %+v", results)
		}
	}
}
