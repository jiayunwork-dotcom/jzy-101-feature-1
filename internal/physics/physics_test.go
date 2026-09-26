package physics

import (
	"math"
	"testing"
)

var wr90 = CrossSection{A: 22.86e-3, B: 10.16e-3, EpsilonR: 1.0}

func TestCutoffKnownValueWR90TE10(t *testing.T) {
	// WR-90 TE10 cutoff: c / (2a) = 6.5572 GHz (handbook value).
	fc := CutoffFrequency(wr90, Mode{M: 1, N: 0})
	want := 6.5572e9
	if math.Abs(fc-want)/want > 1e-4 {
		t.Fatalf("TE10 cutoff = %v Hz, want ~%v Hz", fc, want)
	}
}

// Doubling only the broad dimension must halve the TE10 cutoff exactly.
func TestDoublingBroadDimensionHalvesCutoffExactly(t *testing.T) {
	te10 := Mode{M: 1, N: 0}
	fc1 := CutoffFrequency(wr90, te10)

	doubled := wr90
	doubled.A = 2 * wr90.A
	fc2 := CutoffFrequency(doubled, te10)

	if fc2 != fc1/2 {
		t.Fatalf("fc(2a) = %v, want exactly fc(a)/2 = %v", fc2, fc1/2)
	}
}

// Filling with epsR = 4 instead of vacuum must halve the cutoff exactly.
func TestQuadruplingPermittivityHalvesCutoffExactly(t *testing.T) {
	modes := []Mode{{M: 1, N: 0}, {M: 2, N: 0}, {M: 0, N: 1}, {M: 1, N: 1}, {M: 2, N: 1}}
	filled := wr90
	filled.EpsilonR = 4.0
	for _, m := range modes {
		fcVac := CutoffFrequency(wr90, m)
		fcFilled := CutoffFrequency(filled, m)
		if fcFilled != fcVac/2 {
			t.Fatalf("mode %+v: fc(epsR=4) = %v, want exactly fc(epsR=1)/2 = %v", m, fcFilled, fcVac/2)
		}
	}
}

// A higher mode index must never cut off below the dominant mode.
func TestHigherModesCutoffNotBelowDominant(t *testing.T) {
	fc10 := CutoffFrequency(wr90, Mode{M: 1, N: 0})
	higher := []Mode{
		{M: 2, N: 0}, {M: 0, N: 1}, {M: 1, N: 1},
		{M: 2, N: 1}, {M: 1, N: 2}, {M: 3, N: 2},
	}
	for _, m := range higher {
		if fc := CutoffFrequency(wr90, m); fc < fc10 {
			t.Fatalf("mode %+v cutoff %v Hz is below TE10 cutoff %v Hz", m, fc, fc10)
		}
	}
	// Sanity: TE20 is exactly twice TE10 for the same cross-section.
	if fc20 := CutoffFrequency(wr90, Mode{M: 2, N: 0}); fc20 != 2*fc10 {
		t.Fatalf("TE20 cutoff = %v, want exactly 2*TE10 = %v", fc20, 2*fc10)
	}
}

// At f == fc the state must be critical: not evanescent, and no
// (infinite) guide wavelength is reported; attenuation is exactly zero.
func TestCriticalBoundary(t *testing.T) {
	fc := CutoffFrequency(wr90, Mode{M: 1, N: 0})

	res := EvaluatePoint(wr90, Mode{M: 1, N: 0}, fc)
	if res.State != StateCritical {
		t.Fatalf("state at f == fc: got %q, want %q", res.State, StateCritical)
	}
	if res.GuideWavelengthM != nil {
		t.Fatalf("critical point must not report a guide wavelength, got %v", *res.GuideWavelengthM)
	}
	if res.AttenuationNpPerM == nil || *res.AttenuationNpPerM != 0 {
		t.Fatalf("critical point must report zero attenuation, got %v", res.AttenuationNpPerM)
	}

	// Just above: propagating with a finite guide wavelength.
	above := EvaluatePoint(wr90, Mode{M: 1, N: 0}, fc*(1+1e-9))
	if above.State != StatePropagating {
		t.Fatalf("just above fc: got %q, want %q", above.State, StatePropagating)
	}
	if above.GuideWavelengthM == nil || math.IsInf(*above.GuideWavelengthM, 0) || math.IsNaN(*above.GuideWavelengthM) {
		t.Fatalf("just above fc: guide wavelength must be finite, got %v", above.GuideWavelengthM)
	}

	// Just below: evanescent with a positive attenuation, no wavelength.
	below := EvaluatePoint(wr90, Mode{M: 1, N: 0}, fc*(1-1e-9))
	if below.State != StateEvanescent {
		t.Fatalf("just below fc: got %q, want %q", below.State, StateEvanescent)
	}
	if below.AttenuationNpPerM == nil || *below.AttenuationNpPerM <= 0 {
		t.Fatalf("just below fc: attenuation must be positive, got %v", below.AttenuationNpPerM)
	}
	if below.GuideWavelengthM != nil {
		t.Fatalf("evanescent point must not report a guide wavelength")
	}
}

// Above cutoff the guide wavelength decreases monotonically towards the
// wavelength in the medium.
func TestGuideWavelengthMonotonicTowardsMediumWavelength(t *testing.T) {
	fc := CutoffFrequency(wr90, Mode{M: 1, N: 0})
	multipliers := []float64{1.0001, 1.001, 1.01, 1.1, 1.5, 2, 5, 10, 100, 1e4}

	prev := math.Inf(1)
	for _, k := range multipliers {
		f := fc * k
		lg := GuideWavelength(f, fc, wr90.EpsilonR)
		lm := MediumWavelength(f, wr90.EpsilonR)
		if lg <= lm {
			t.Fatalf("f=%v fc: guide wavelength %v must exceed medium wavelength %v", k, lg, lm)
		}
		if lg >= prev {
			t.Fatalf("f=%v fc: guide wavelength %v not strictly below previous %v", k, lg, prev)
		}
		prev = lg
	}

	// Far above cutoff the guide wavelength converges to the medium
	// wavelength.
	f := fc * 1e4
	lg, lm := GuideWavelength(f, fc, wr90.EpsilonR), MediumWavelength(f, wr90.EpsilonR)
	if rel := (lg - lm) / lm; rel > 1e-6 {
		t.Fatalf("at f=1e4*fc guide wavelength should approach medium wavelength: relative gap %v", rel)
	}
}

// Evanescent attenuation grows as the frequency drops further below
// cutoff.
func TestEvanescentAttenuationIncreasesBelowCutoff(t *testing.T) {
	fc := CutoffFrequency(wr90, Mode{M: 1, N: 0})
	prev := 0.0
	for _, k := range []float64{0.999, 0.9, 0.5, 0.1} {
		alpha := AttenuationNpPerM(fc*k, fc, wr90.EpsilonR)
		if alpha <= prev {
			t.Fatalf("f=%v fc: attenuation %v not above previous %v", k, alpha, prev)
		}
		prev = alpha
	}
}
