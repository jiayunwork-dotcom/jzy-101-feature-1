// Package spectrum enumerates the TE/TM mode spectrum of a rectangular
// waveguide cross-section and answers cross-mode questions from it:
// which mode is dominant, which mode(s) come next, and over which
// frequency window only the dominant mode propagates (the single-mode
// range). It adds enumeration, ordering and range extraction on top of
// the single-mode cutoff formula of internal/physics — no physics is
// reimplemented here.
package spectrum

import (
	"sort"

	"waveguide/internal/physics"
)

// ModeKind names the two rectangular-waveguide mode families.
type ModeKind string

const (
	// TE (transverse electric) modes exist for every index pair except
	// (0,0).
	TE ModeKind = "TE"
	// TM (transverse magnetic) modes require both indices to be >= 1.
	TM ModeKind = "TM"
)

const (
	// MinMaxIndex is the smallest enumeration bound that still yields
	// the exact single-mode range for any cross-section with a > b.
	//
	// Write K = c/(2·sqrt(epsR)); every cutoff is K·sqrt((m/a)²+(n/b)²).
	// The dominant mode TE10 cuts off at K/a and TE20 at exactly 2·K/a,
	// so the second-lowest cutoff never exceeds 2·K/a. Any mode with
	// m >= 3 cuts off at or above 3·K/a, and any mode with n >= 2 cuts
	// off at or above 2·K/b > 2·K/a (because b < a). The remaining
	// candidates with m <= 2 and n <= 1 — TE01, TE11/TM11, TE21/TM21 —
	// all cut off at or above min(TE20, TE01). Hence the dominant mode
	// and the mode that closes the single-mode range always have
	// indices m <= 2, n <= 1, and enumerating up to MinMaxIndex can
	// never miss them, regardless of the aspect ratio.
	MinMaxIndex = 2

	// DefaultMaxIndex is the enumeration bound used when the caller
	// does not specify a usable one. It sits far above MinMaxIndex, so
	// besides the exact range bounds the reported spectrum also reaches
	// many modes past the range edge — exactly what an engineer
	// inspects when asking "if I push beyond the single-mode range,
	// which mode do I hit next?". For common aspect ratios the modes
	// that decide the range carry very small indices, so 8 is generous
	// rather than wasteful.
	DefaultMaxIndex = 8
)

// Entry is one mode of the spectrum together with its cutoff frequency.
type Entry struct {
	Kind     ModeKind
	Mode     physics.Mode
	CutoffHz float64
}

// Range is the single-mode operating window [LowerHz, UpperHz): below
// LowerHz no mode propagates; from UpperHz on at least two modes
// propagate. LowerHz is inclusive (at exactly the dominant cutoff the
// dominant mode is at its critical onset); UpperHz is exclusive (the
// next mode already propagates there, so the guide is no longer
// single-mode).
type Range struct {
	LowerHz float64
	UpperHz float64
}

// Contains reports whether f lies inside the single-mode window.
func (r Range) Contains(f float64) bool {
	return f >= r.LowerHz && f < r.UpperHz
}

// Width returns the frequency span UpperHz - LowerHz of the window.
func (r Range) Width() float64 { return r.UpperHz - r.LowerHz }

// Report is the full spectrum analysis of one cross-section.
type Report struct {
	// MaxIndex is the enumeration bound actually used.
	MaxIndex int
	// Spectrum lists every enumerated mode, sorted by cutoff frequency
	// in non-decreasing order. Degenerate modes (equal cutoff) all
	// appear, in a deterministic order.
	Spectrum []Entry
	// Dominant holds the mode(s) at the lowest cutoff — for a > b this
	// is always exactly TE10, but it is reported as a list so a
	// degeneracy would never be hidden.
	Dominant []Entry
	// Next holds every mode at the first cutoff strictly above the
	// dominant one. Degenerate partners (e.g. TE20 and TE01 when
	// a = 2b, or any TE_mn/TM_mn pair) appear side by side; none is
	// dropped in favour of another.
	Next []Entry
	// Range is the derived single-mode window.
	Range Range
}

// effectiveMaxIndex maps an unusable bound to the default: anything
// below MinMaxIndex could miss the mode that closes the single-mode
// range, so it is never honoured.
func effectiveMaxIndex(maxIndex int) int {
	if maxIndex < MinMaxIndex {
		return DefaultMaxIndex
	}
	return maxIndex
}

// Enumerate lists every TE mode (indices 0..maxIndex, (0,0) excluded)
// and every TM mode (indices 1..maxIndex) of the cross-section, sorted
// by cutoff frequency in non-decreasing order. Cutoffs come straight
// from physics.CutoffFrequency, so degenerate modes keep bit-identical
// cutoffs whenever their indices make the cutoff expression identical
// (e.g. TE20 vs TE01 for a = 2b, or TE_mn vs TM_mn).
func Enumerate(cs physics.CrossSection, maxIndex int) []Entry {
	maxIndex = effectiveMaxIndex(maxIndex)
	// TE: (maxIndex+1)² - 1 modes; TM: maxIndex² modes.
	entries := make([]Entry, 0, 2*maxIndex*(maxIndex+1))
	for m := 0; m <= maxIndex; m++ {
		for n := 0; n <= maxIndex; n++ {
			if m == 0 && n == 0 {
				continue // (0,0) carries no field in either family
			}
			mode := physics.Mode{M: m, N: n}
			entries = append(entries, Entry{Kind: TE, Mode: mode, CutoffHz: physics.CutoffFrequency(cs, mode)})
			if m >= 1 && n >= 1 {
				entries = append(entries, Entry{Kind: TM, Mode: mode, CutoffHz: physics.CutoffFrequency(cs, mode)})
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.CutoffHz != b.CutoffHz {
			return a.CutoffHz < b.CutoffHz
		}
		// Deterministic order inside a degenerate group: TE before TM,
		// then by m, then by n.
		if a.Kind != b.Kind {
			return a.Kind == TE
		}
		if a.Mode.M != b.Mode.M {
			return a.Mode.M < b.Mode.M
		}
		return a.Mode.N < b.Mode.N
	})
	return entries
}

// Analyze enumerates the spectrum and reads the single-mode range off
// it: the lower bound is the lowest cutoff (the dominant mode), the
// upper bound is the first strictly higher cutoff. Every mode sharing
// either cutoff is reported in Dominant / Next, so a degenerate pair
// closing the range shows up as a pair, not as an arbitrary pick.
func Analyze(cs physics.CrossSection, maxIndex int) Report {
	maxIndex = effectiveMaxIndex(maxIndex)
	entries := Enumerate(cs, maxIndex)

	lower := entries[0].CutoffHz
	upper := lower
	dominant := make([]Entry, 0, 2)
	next := make([]Entry, 0, 2)
	for _, e := range entries {
		if e.CutoffHz == lower {
			dominant = append(dominant, e)
			continue
		}
		if upper == lower {
			upper = e.CutoffHz // first entry strictly above the dominant cutoff
		}
		if e.CutoffHz == upper {
			next = append(next, e)
		}
	}
	// With maxIndex >= MinMaxIndex the enumeration always contains
	// TE20, whose cutoff is strictly above TE10's, so next is never
	// empty and upper > lower always holds here.
	return Report{
		MaxIndex: maxIndex,
		Spectrum: entries,
		Dominant: dominant,
		Next:     next,
		Range:    Range{LowerHz: lower, UpperHz: upper},
	}
}
