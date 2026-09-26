package batch

import (
	"testing"

	"waveguide/internal/physics"
)

var wr90 = physics.CrossSection{A: 22.86e-3, B: 10.16e-3, EpsilonR: 1}

func TestEvaluateIsolatesBadFrequencies(t *testing.T) {
	freqs := []float64{10e9, -5, 5e9, 0, 12e9}
	out := Evaluate(wr90, physics.Mode{M: 1, N: 0}, freqs, 4)

	if len(out) != len(freqs) {
		t.Fatalf("got %d outcomes for %d frequencies", len(out), len(freqs))
	}
	// Valid points produce results in their own slots.
	for _, i := range []int{0, 2, 4} {
		if out[i].Err != nil || out[i].Result == nil {
			t.Fatalf("slot %d: want result, got %+v", i, out[i])
		}
		if out[i].Result.FrequencyHz != freqs[i] {
			t.Fatalf("slot %d: frequency mismatch %v vs %v", i, out[i].Result.FrequencyHz, freqs[i])
		}
	}
	// Invalid points fail only their own slot.
	for _, i := range []int{1, 3} {
		if out[i].Err == nil || out[i].Result != nil {
			t.Fatalf("slot %d: want error, got %+v", i, out[i])
		}
	}
	// States: 10 GHz and 12 GHz propagate (WR-90 TE10 fc ~6.56 GHz),
	// 5 GHz is evanescent.
	if out[0].Result.State != physics.StatePropagating || out[4].Result.State != physics.StatePropagating {
		t.Fatalf("10/12 GHz must propagate: %+v %+v", out[0].Result, out[4].Result)
	}
	if out[2].Result.State != physics.StateEvanescent {
		t.Fatalf("5 GHz must be evanescent: %+v", out[2].Result)
	}
}

func TestEvaluateEmpty(t *testing.T) {
	if out := Evaluate(wr90, physics.Mode{M: 1, N: 0}, nil, 4); len(out) != 0 {
		t.Fatalf("empty input must yield empty output, got %d", len(out))
	}
}
