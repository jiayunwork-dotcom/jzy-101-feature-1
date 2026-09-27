package spectrum

import (
	"math"
	"sort"
	"testing"

	"waveguide/internal/physics"
)

var wr90 = physics.CrossSection{A: 22.86e-3, B: 10.16e-3, EpsilonR: 1.0}

// The built-in X-band sample: the single-mode band's lower edge must
// equal the dominant TE10 cutoff exactly, its upper edge the second
// mode (TE20) cutoff exactly, and the whole X-band must lie inside.
func TestWR90SingleModeBand(t *testing.T) {
	a := Analyze(wr90)

	fc10 := physics.CutoffFrequency(wr90, physics.Mode{M: 1, N: 0})
	fc20 := physics.CutoffFrequency(wr90, physics.Mode{M: 2, N: 0})
	fc01 := physics.CutoffFrequency(wr90, physics.Mode{M: 0, N: 1})

	if a.Band.LowerHz != fc10 {
		t.Fatalf("lower edge = %v, want TE10 cutoff %v exactly", a.Band.LowerHz, fc10)
	}
	if a.Band.UpperHz != fc20 {
		t.Fatalf("upper edge = %v, want TE20 cutoff %v exactly", a.Band.UpperHz, fc20)
	}
	if fc20 >= fc01 {
		t.Fatalf("test setup: for WR-90 TE20 must beat TE01, got %v vs %v", fc20, fc01)
	}
	if len(a.Band.Dominant) != 1 || a.Band.Dominant[0].Kind != KindTE ||
		a.Band.Dominant[0].M != 1 || a.Band.Dominant[0].N != 0 {
		t.Fatalf("dominant group = %+v, want [TE10]", a.Band.Dominant)
	}
	if len(a.Band.Next) != 1 || a.Band.Next[0].Kind != KindTE ||
		a.Band.Next[0].M != 2 || a.Band.Next[0].N != 0 {
		t.Fatalf("next group = %+v, want [TE20]", a.Band.Next)
	}

	// Whole X-band (8.2-12.4 GHz) is inside [lower, upper).
	if 8.2e9 < a.Band.LowerHz {
		t.Fatalf("lower X-band edge %v below band onset %v", 8.2e9, a.Band.LowerHz)
	}
	if 12.4e9 >= a.Band.UpperHz {
		t.Fatalf("upper X-band edge %v not below band end %v", 12.4e9, a.Band.UpperHz)
	}
}

// Doubling only the broad dimension must halve BOTH band edges and the
// band width exactly.
func TestDoublingBroadDimensionHalvesBandExactly(t *testing.T) {
	base := Analyze(wr90).Band

	doubled := wr90
	doubled.A = 2 * wr90.A
	got := Analyze(doubled).Band

	if got.LowerHz != base.LowerHz/2 {
		t.Fatalf("lower(2a) = %v, want exactly %v", got.LowerHz, base.LowerHz/2)
	}
	if got.UpperHz != base.UpperHz/2 {
		t.Fatalf("upper(2a) = %v, want exactly %v", got.UpperHz, base.UpperHz/2)
	}
	if got.Width() != base.Width()/2 {
		t.Fatalf("width(2a) = %v, want exactly %v", got.Width(), base.Width()/2)
	}
}

// Filling with epsR = 4 instead of vacuum must halve BOTH band edges
// and the band width exactly.
func TestQuadruplingPermittivityHalvesBandExactly(t *testing.T) {
	base := Analyze(wr90).Band

	filled := wr90
	filled.EpsilonR = 4.0
	got := Analyze(filled).Band

	if got.LowerHz != base.LowerHz/2 {
		t.Fatalf("lower(epsR=4) = %v, want exactly %v", got.LowerHz, base.LowerHz/2)
	}
	if got.UpperHz != base.UpperHz/2 {
		t.Fatalf("upper(epsR=4) = %v, want exactly %v", got.UpperHz, base.UpperHz/2)
	}
	if got.Width() != base.Width()/2 {
		t.Fatalf("width(epsR=4) = %v, want exactly %v", got.Width(), base.Width()/2)
	}
}

// The spectrum must be non-decreasing by cutoff; its first cutoff must
// equal the band's lower edge, and the first STRICTLY higher cutoff
// must equal the band's upper edge.
func TestSpectrumOrderingAndBandEdges(t *testing.T) {
	a := Analyze(wr90)
	spec := a.Spectrum

	if len(spec) == 0 {
		t.Fatal("spectrum is empty")
	}
	for i := 1; i < len(spec); i++ {
		if spec[i].CutoffHz < spec[i-1].CutoffHz {
			t.Fatalf("spectrum not non-decreasing at %d: %v then %v",
				i, spec[i-1].CutoffHz, spec[i].CutoffHz)
		}
	}
	if spec[0].CutoffHz != a.Band.LowerHz {
		t.Fatalf("spectrum first cutoff %v != band lower %v", spec[0].CutoffHz, a.Band.LowerHz)
	}
	i := 1
	for i < len(spec) && spec[i].CutoffHz == spec[0].CutoffHz {
		i++
	}
	if i >= len(spec) {
		t.Fatal("no strictly higher cutoff found in spectrum")
	}
	if spec[i].CutoffHz != a.Band.UpperHz {
		t.Fatalf("first strictly higher cutoff %v != band upper %v", spec[i].CutoffHz, a.Band.UpperHz)
	}
}

// At a = 2b the TE20 and TE01 cutoffs coincide. Both modes must appear
// in the spectrum, and their shared cutoff must be the band's upper
// edge — neither may be dropped.
func TestDegenerateUpperEdgeKeepsBothModes(t *testing.T) {
	cs := physics.CrossSection{A: 0.02, B: 0.01, EpsilonR: 1.0}
	a := Analyze(cs)

	fc20 := physics.CutoffFrequency(cs, physics.Mode{M: 2, N: 0})
	fc01 := physics.CutoffFrequency(cs, physics.Mode{M: 0, N: 1})
	if fc20 != fc01 {
		t.Fatalf("at a=2b the cutoffs must coincide bit-for-bit, got %v and %v", fc20, fc01)
	}
	if a.Band.UpperHz != fc20 {
		t.Fatalf("upper edge = %v, want shared cutoff %v", a.Band.UpperHz, fc20)
	}
	if len(a.Band.Next) != 2 {
		t.Fatalf("degenerate next group must contain both modes, got %+v", a.Band.Next)
	}
	kinds := map[[2]int]bool{}
	for _, m := range a.Band.Next {
		if m.CutoffHz != a.Band.UpperHz {
			t.Fatalf("group member %+v cutoff %v != upper %v", m, m.CutoffHz, a.Band.UpperHz)
		}
		kinds[[2]int{m.M, m.N}] = true
	}
	if !kinds[[2]int{2, 0}] || !kinds[[2]int{0, 1}] {
		t.Fatalf("next group %+v must contain TE20 and TE01", a.Band.Next)
	}

	// The shared cutoff also appears as two adjacent entries in the
	// full spectrum; their tie-break order is a presentation detail,
	// so assert membership and adjacency rather than a fixed order.
	var hits []SpectralMode
	for _, m := range a.Spectrum {
		if m.CutoffHz == a.Band.UpperHz {
			hits = append(hits, m)
		}
	}
	if len(hits) != 2 {
		t.Fatalf("spectrum must carry exactly two modes at the shared cutoff, got %+v", hits)
	}
	indices := map[[2]int]bool{}
	for _, m := range hits {
		if m.Kind != KindTE {
			t.Fatalf("both degenerate modes must be TE, got %+v", hits)
		}
		indices[[2]int{m.M, m.N}] = true
	}
	if !indices[[2]int{2, 0}] || !indices[[2]int{0, 1}] {
		t.Fatalf("spectrum entries at shared cutoff must be TE20 and TE01, got %+v", hits)
	}
}

// For a narrow broad-wall ratio the second mode is TE01 instead of
// TE20; the band edge must follow.
func TestSecondModeCanBeTE01(t *testing.T) {
	cs := physics.CrossSection{A: 11e-3, B: 10e-3, EpsilonR: 1.0}
	a := Analyze(cs)

	fc20 := physics.CutoffFrequency(cs, physics.Mode{M: 2, N: 0})
	fc01 := physics.CutoffFrequency(cs, physics.Mode{M: 0, N: 1})
	if !(fc01 < fc20) {
		t.Fatalf("test setup: TE01 %v must precede TE20 %v", fc01, fc20)
	}
	if a.Band.UpperHz != fc01 {
		t.Fatalf("upper edge = %v, want TE01 cutoff %v", a.Band.UpperHz, fc01)
	}
	if len(a.Band.Next) != 1 || a.Band.Next[0].M != 0 || a.Band.Next[0].N != 1 {
		t.Fatalf("next group = %+v, want [TE01]", a.Band.Next)
	}
}

// TE_mn and TM_mn with both indices positive share the same cutoff and
// must both be present in the spectrum as a degenerate pair; (0,*) and
// (*,0) carry no TM member, and (0,0) must be absent entirely.
func TestTETMDegeneracyAndExistenceRules(t *testing.T) {
	spec := Enumerate(wr90)

	present := map[struct {
		kind Kind
		m, n int
	}]bool{}
	for _, m := range spec {
		present[struct {
			kind Kind
			m, n int
		}{m.Kind, m.M, m.N}] = true
	}
	for _, k := range []struct {
		kind Kind
		m, n int
	}{
		{KindTE, 1, 1}, {KindTM, 1, 1},
		{KindTE, 3, 2}, {KindTM, 3, 2},
		{KindTE, 0, 1}, // TE with a zero index exists ...
		{KindTE, 1, 0},
	} {
		if !present[struct {
			kind Kind
			m, n int
		}{k.kind, k.m, k.n}] {
			t.Fatalf("mode %s%d%d missing from spectrum", k.kind, k.m, k.n)
		}
	}
	for _, k := range []struct {
		kind Kind
		m, n int
	}{
		{KindTM, 0, 1}, {KindTM, 1, 0}, {KindTM, 0, 0},
		{KindTE, 0, 0},
	} {
		if present[struct {
			kind Kind
			m, n int
		}{k.kind, k.m, k.n}] {
			t.Fatalf("illegal mode %s%d%d must not be enumerated", k.kind, k.m, k.n)
		}
	}

	// TE11/TM11 coincide and sit next to each other.
	fc11 := physics.CutoffFrequency(wr90, physics.Mode{M: 1, N: 1})
	var pair []SpectralMode
	for _, m := range spec {
		if m.CutoffHz == fc11 && m.M == 1 && m.N == 1 {
			pair = append(pair, m)
		}
	}
	if len(pair) != 2 || pair[0].Kind != KindTE || pair[1].Kind != KindTM {
		t.Fatalf("TE11/TM11 degenerate pair malformed: %+v", pair)
	}
}

// The enumeration box of width MaxModeIndex must give exactly the same
// band edges as a vastly wider brute-force enumeration, across aspect
// ratios spanning the switch between TE20 and TE01 as second mode, the
// degeneracy point, and extreme/square ratios. This is the direct,
// automatable check that the index bound cannot miss the mode that
// fixes the upper edge.
func TestBoundNeverMissesBandEdge(t *testing.T) {
	ratios := []float64{1.01, 1.2, 1.5, 1.8, 2.0, 2.0000000001, 2.25, 2.5, 3.5, 10, 1000}
	const brute = 200

	for _, r := range ratios {
		cs := physics.CrossSection{A: r * 0.01, B: 0.01, EpsilonR: 2.25}
		got := Analyze(cs).Band
		want := bruteForceBand(cs, brute)

		if got.LowerHz != want.LowerHz {
			t.Fatalf("ratio %v: lower %v, brute-force %v", r, got.LowerHz, want.LowerHz)
		}
		if got.UpperHz != want.UpperHz {
			t.Fatalf("ratio %v: upper %v, brute-force %v", r, got.UpperHz, want.UpperHz)
		}
		if len(got.Next) != len(want.Next) {
			t.Fatalf("ratio %v: next group size %d, brute-force %d", r, len(got.Next), len(want.Next))
		}
	}
}

// bruteForceBand recomputes the band with an independent, much larger
// enumeration box, used only to validate the production bound.
func bruteForceBand(cs physics.CrossSection, limit int) Band {
	type flat struct {
		kind   Kind
		m, n   int
		cutoff float64
	}
	var all []flat
	for m := 0; m <= limit; m++ {
		for n := 0; n <= limit; n++ {
			fc := physics.CutoffFrequency(cs, physics.Mode{M: m, N: n})
			if m != 0 || n != 0 {
				all = append(all, flat{KindTE, m, n, fc})
			}
			if m >= 1 && n >= 1 {
				all = append(all, flat{KindTM, m, n, fc})
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		switch {
		case all[i].cutoff != all[j].cutoff:
			return all[i].cutoff < all[j].cutoff
		case all[i].kind != all[j].kind:
			return all[i].kind == KindTE
		case all[i].m != all[j].m:
			return all[i].m < all[j].m
		default:
			return all[i].n < all[j].n
		}
	})

	b := Band{LowerHz: all[0].cutoff}
	j := 1
	for all[j].cutoff == all[0].cutoff {
		j++
	}
	b.UpperHz = all[j].cutoff
	k := j
	for k < len(all) && all[k].cutoff == all[j].cutoff {
		b.Next = append(b.Next, SpectralMode{
			Kind: all[k].kind, M: all[k].m, N: all[k].n, CutoffHz: all[k].cutoff,
		})
		k++
	}
	return b
}

// BandFromSpectrum must derive the band straight from any correctly
// sorted spectrum, and the groups it returns must not alias the input
// slice (safe for callers to retain after the backing array changes).
func TestBandFromSpectrumAndGroupCopies(t *testing.T) {
	a := Analyze(wr90)
	rebuilt := BandFromSpectrum(a.Spectrum)
	if rebuilt.LowerHz != a.Band.LowerHz || rebuilt.UpperHz != a.Band.UpperHz {
		t.Fatalf("BandFromSpectrum edges %v/%v disagree with Analyze %v/%v",
			rebuilt.LowerHz, rebuilt.UpperHz, a.Band.LowerHz, a.Band.UpperHz)
	}

	original := rebuilt.Dominant[0]
	a.Spectrum[0].M = 99
	a.Spectrum[0].N = 99
	if rebuilt.Dominant[0] != original {
		t.Fatal("dominant group aliases the spectrum slice")
	}
}

// Sanity: every reported cutoff is positive and the number of entries
// matches the TE/TM existence rules for the box.
func TestEnumerationCompleteness(t *testing.T) {
	spec := Enumerate(wr90)
	te := (MaxModeIndex+1)*(MaxModeIndex+1) - 1
	tm := MaxModeIndex * MaxModeIndex
	if len(spec) != te+tm {
		t.Fatalf("enumerated %d modes, want %d TE + %d TM = %d", len(spec), te, tm, te+tm)
	}
	for _, m := range spec {
		if math.IsNaN(m.CutoffHz) || m.CutoffHz <= 0 {
			t.Fatalf("mode %+v has invalid cutoff %v", m, m.CutoffHz)
		}
		if m.M > MaxModeIndex || m.N > MaxModeIndex {
			t.Fatalf("mode %+v outside the enumeration box", m)
		}
	}
}
