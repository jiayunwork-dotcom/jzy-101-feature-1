package spectrum

import (
	"testing"

	"waveguide/internal/physics"
	"waveguide/internal/validate"
)

var wr90 = physics.CrossSection{A: 22.86e-3, B: 10.16e-3, EpsilonR: 1.0}

// The WR-90 single-mode window must be exactly [fc(TE10), fc(TE20)),
// and the whole X-band (8.2-12.4 GHz) must fall inside it.
func TestWR90RangeBoundsAreExactCutoffs(t *testing.T) {
	rep := Analyze(wr90, 0)

	fc10 := physics.CutoffFrequency(wr90, physics.Mode{M: 1, N: 0})
	fc20 := physics.CutoffFrequency(wr90, physics.Mode{M: 2, N: 0})
	if rep.Range.LowerHz != fc10 {
		t.Fatalf("lower bound = %v, want exactly TE10 cutoff %v", rep.Range.LowerHz, fc10)
	}
	if rep.Range.UpperHz != fc20 {
		t.Fatalf("upper bound = %v, want exactly TE20 cutoff %v", rep.Range.UpperHz, fc20)
	}

	// For WR-90 (a/b ≈ 2.25) the dominant mode is TE10 alone and the
	// next mode is TE20 alone.
	if len(rep.Dominant) != 1 || rep.Dominant[0].Kind != TE || rep.Dominant[0].Mode != (physics.Mode{M: 1, N: 0}) {
		t.Fatalf("dominant = %+v, want exactly TE10", rep.Dominant)
	}
	if len(rep.Next) != 1 || rep.Next[0].Kind != TE || rep.Next[0].Mode != (physics.Mode{M: 2, N: 0}) {
		t.Fatalf("next = %+v, want exactly TE20", rep.Next)
	}

	// The whole X-band lies inside the window; just outside it does not.
	if !rep.Range.Contains(8.2e9) || !rep.Range.Contains(12.4e9) {
		t.Fatalf("X-band must be inside [%v, %v)", rep.Range.LowerHz, rep.Range.UpperHz)
	}
	if rep.Range.Contains(rep.Range.LowerHz * 0.999) {
		t.Fatalf("below the dominant cutoff must be outside the window")
	}
	if rep.Range.Contains(rep.Range.UpperHz) {
		t.Fatalf("the upper bound itself must be excluded (next mode already propagates)")
	}
}

// Doubling only the broad dimension must halve both bounds — and hence
// the window width — exactly.
func TestDoublingBroadDimensionHalvesRangeExactly(t *testing.T) {
	base := Analyze(wr90, 0)

	doubled := wr90
	doubled.A = 2 * wr90.A
	wide := Analyze(doubled, 0)

	if wide.Range.LowerHz != base.Range.LowerHz/2 {
		t.Fatalf("lower(2a) = %v, want exactly %v", wide.Range.LowerHz, base.Range.LowerHz/2)
	}
	if wide.Range.UpperHz != base.Range.UpperHz/2 {
		t.Fatalf("upper(2a) = %v, want exactly %v", wide.Range.UpperHz, base.Range.UpperHz/2)
	}
	if wide.Range.Width() != base.Range.Width()/2 {
		t.Fatalf("width(2a) = %v, want exactly half of %v", wide.Range.Width(), base.Range.Width())
	}
}

// Filling with epsR = 4 instead of vacuum must halve both bounds
// exactly.
func TestPermittivity4HalvesRangeExactly(t *testing.T) {
	base := Analyze(wr90, 0)

	filled := wr90
	filled.EpsilonR = 4
	rep := Analyze(filled, 0)

	if rep.Range.LowerHz != base.Range.LowerHz/2 {
		t.Fatalf("lower(epsR=4) = %v, want exactly %v", rep.Range.LowerHz, base.Range.LowerHz/2)
	}
	if rep.Range.UpperHz != base.Range.UpperHz/2 {
		t.Fatalf("upper(epsR=4) = %v, want exactly %v", rep.Range.UpperHz, base.Range.UpperHz/2)
	}
}

// The spectrum must be non-decreasing in cutoff, its first cutoff must
// be the range lower bound, and the first strictly higher cutoff must
// be the range upper bound — for any cross-section shape.
func TestSpectrumOrderingAndRangeConsistency(t *testing.T) {
	sections := map[string]physics.CrossSection{
		"WR-90":       wr90,
		"near square": {A: 12e-3, B: 10e-3, EpsilonR: 1},
		"a = 2b":      {A: 2 * 10.16e-3, B: 10.16e-3, EpsilonR: 1},
		"dielectric":  {A: 22.86e-3, B: 10.16e-3, EpsilonR: 4},
	}
	for name, cs := range sections {
		t.Run(name, func(t *testing.T) {
			rep := Analyze(cs, 0)

			for i := 1; i < len(rep.Spectrum); i++ {
				if rep.Spectrum[i].CutoffHz < rep.Spectrum[i-1].CutoffHz {
					t.Fatalf("spectrum decreases at %d: %v < %v",
						i, rep.Spectrum[i].CutoffHz, rep.Spectrum[i-1].CutoffHz)
				}
			}
			if rep.Spectrum[0].CutoffHz != rep.Range.LowerHz {
				t.Fatalf("first cutoff %v != lower bound %v", rep.Spectrum[0].CutoffHz, rep.Range.LowerHz)
			}
			upper, found := 0.0, false
			for _, e := range rep.Spectrum {
				if e.CutoffHz > rep.Range.LowerHz {
					upper, found = e.CutoffHz, true
					break
				}
			}
			if !found || upper != rep.Range.UpperHz {
				t.Fatalf("first strictly higher cutoff = %v (found %v), want upper bound %v",
					upper, found, rep.Range.UpperHz)
			}
			for _, d := range rep.Dominant {
				if d.CutoffHz != rep.Range.LowerHz {
					t.Fatalf("dominant mode %+v not at the lower bound %v", d, rep.Range.LowerHz)
				}
			}
			for _, n := range rep.Next {
				if n.CutoffHz != rep.Range.UpperHz {
					t.Fatalf("next mode %+v not at the upper bound %v", n, rep.Range.UpperHz)
				}
			}
		})
	}
}

// With a = 2b the TE20 and TE01 cutoffs coincide. Both modes must
// appear in the spectrum and in the Next list, and their shared cutoff
// must be the upper bound of the single-mode range.
func TestDegeneratePairSharesUpperBound(t *testing.T) {
	b := 10.16e-3
	cs := physics.CrossSection{A: 2 * b, B: b, EpsilonR: 1}

	fc20 := physics.CutoffFrequency(cs, physics.Mode{M: 2, N: 0})
	fc01 := physics.CutoffFrequency(cs, physics.Mode{M: 0, N: 1})
	if fc20 != fc01 {
		t.Fatalf("test construction broken: TE20 cutoff %v != TE01 cutoff %v", fc20, fc01)
	}

	rep := Analyze(cs, 0)
	if rep.Range.UpperHz != fc20 {
		t.Fatalf("upper bound = %v, want the shared cutoff %v", rep.Range.UpperHz, fc20)
	}
	if len(rep.Dominant) != 1 || rep.Dominant[0].Mode != (physics.Mode{M: 1, N: 0}) {
		t.Fatalf("dominant = %+v, want TE10 alone", rep.Dominant)
	}

	// Both degenerate modes must be reported as the next modes — not
	// one arbitrarily picked, the other dropped.
	if len(rep.Next) != 2 {
		t.Fatalf("next = %+v, want the degenerate pair TE20 + TE01", rep.Next)
	}
	seen := map[physics.Mode]bool{}
	for _, e := range rep.Next {
		if e.Kind != TE {
			t.Fatalf("next mode %+v: want TE family", e)
		}
		seen[e.Mode] = true
	}
	if !seen[physics.Mode{M: 2, N: 0}] || !seen[physics.Mode{M: 0, N: 1}] {
		t.Fatalf("next = %+v, want TE(2,0) and TE(0,1)", rep.Next)
	}

	// And both must appear in the spectrum at that shared cutoff.
	count := 0
	for _, e := range rep.Spectrum {
		if e.CutoffHz == fc20 && (e.Mode == physics.Mode{M: 2, N: 0} || e.Mode == physics.Mode{M: 0, N: 1}) {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("spectrum must list both degenerate modes at %v, found %d", fc20, count)
	}
}

// TE_mn and TM_mn share the cutoff formula, so every TM-capable index
// pair must show up as a degenerate TE/TM pair in the spectrum.
func TestTETMPairsAreDegenerate(t *testing.T) {
	rep := Analyze(wr90, 0)
	want := physics.CutoffFrequency(wr90, physics.Mode{M: 1, N: 1})

	var te, tm *Entry
	for i := range rep.Spectrum {
		e := &rep.Spectrum[i]
		if e.Mode == (physics.Mode{M: 1, N: 1}) {
			switch e.Kind {
			case TE:
				te = e
			case TM:
				tm = e
			}
		}
	}
	if te == nil || tm == nil {
		t.Fatalf("TE11 and TM11 must both appear in the spectrum")
	}
	if te.CutoffHz != want || tm.CutoffHz != want {
		t.Fatalf("TE11/TM11 cutoffs %v / %v, want the shared %v", te.CutoffHz, tm.CutoffHz, want)
	}
}

// The single-mode range must not depend on how deep the enumeration
// goes: any bound >= MinMaxIndex yields the identical window.
func TestRangeIndependentOfEnumerationBound(t *testing.T) {
	sections := []physics.CrossSection{
		wr90,
		{A: 12e-3, B: 10e-3, EpsilonR: 1}, // a/b = 1.2: TE01 is the second mode
		{A: 2 * 10.16e-3, B: 10.16e-3, EpsilonR: 1}, // degenerate second step
		{A: 50e-3, B: 10e-3, EpsilonR: 2.55},        // a/b = 5, dielectric filled
	}
	for _, cs := range sections {
		want := Analyze(cs, MinMaxIndex).Range
		for _, k := range []int{3, 4, DefaultMaxIndex, 16} {
			if got := Analyze(cs, k).Range; got != want {
				t.Fatalf("cs %+v maxIndex %d: range %+v, want %+v", cs, k, got, want)
			}
		}
	}
}

// Across aspect ratios the upper bound is always min(fc(TE20),
// fc(TE01)) and the lower bound is always fc(TE10) — the modes that
// decide the window carry indices m <= 2, n <= 1.
func TestRangeBoundsAcrossAspectRatios(t *testing.T) {
	b := 10e-3
	for _, ratio := range []float64{1.01, 1.2, 1.5, 2, 2.25, 3, 5, 10} {
		cs := physics.CrossSection{A: ratio * b, B: b, EpsilonR: 1}
		rep := Analyze(cs, 0)

		fc10 := physics.CutoffFrequency(cs, physics.Mode{M: 1, N: 0})
		fc20 := physics.CutoffFrequency(cs, physics.Mode{M: 2, N: 0})
		fc01 := physics.CutoffFrequency(cs, physics.Mode{M: 0, N: 1})

		if rep.Range.LowerHz != fc10 {
			t.Fatalf("a/b=%v: lower = %v, want TE10 cutoff %v", ratio, rep.Range.LowerHz, fc10)
		}
		if want := min(fc20, fc01); rep.Range.UpperHz != want {
			t.Fatalf("a/b=%v: upper = %v, want min(TE20,TE01) = %v", ratio, rep.Range.UpperHz, want)
		}
	}
}

// An unusable enumeration bound falls back to the default, and the
// spectrum size matches the TE/TM mode count exactly.
func TestMaxIndexFallbackAndSpectrumSize(t *testing.T) {
	def := Analyze(wr90, 0)
	if def.MaxIndex != DefaultMaxIndex {
		t.Fatalf("maxIndex 0 must fall back to %d, got %d", DefaultMaxIndex, def.MaxIndex)
	}
	if got, want := len(def.Spectrum), 2*DefaultMaxIndex*(DefaultMaxIndex+1); got != want {
		t.Fatalf("spectrum size = %d, want %d", got, want)
	}

	neg := Analyze(wr90, -5)
	if neg.MaxIndex != DefaultMaxIndex || len(neg.Spectrum) != len(def.Spectrum) {
		t.Fatalf("negative maxIndex must fall back to the default, got %d", neg.MaxIndex)
	}

	small := Analyze(wr90, MinMaxIndex)
	if got, want := len(small.Spectrum), 2*MinMaxIndex*(MinMaxIndex+1); got != want {
		t.Fatalf("spectrum size at MinMaxIndex = %d, want %d", got, want)
	}
}

// The API-layer acceptance window must start at the algorithmic floor,
// otherwise an accepted max_index could silently corrupt the range.
func TestEnumerationFloorMatchesValidation(t *testing.T) {
	if validate.MinSpectrumMaxIndex != MinMaxIndex {
		t.Fatalf("validate.MinSpectrumMaxIndex = %d, spectrum.MinMaxIndex = %d: must agree",
			validate.MinSpectrumMaxIndex, MinMaxIndex)
	}
}
