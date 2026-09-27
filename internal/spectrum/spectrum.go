// Package spectrum enumerates the whole TE/TM mode family supported by a
// rectangular waveguide cross-section, orders the resulting cutoff
// spectrum and derives the cross-section's single-mode operating band.
//
// It is deliberately a separate module from the point-by-point physics
// in internal/physics and from the HTTP layer: it reuses
// physics.CutoffFrequency (so there is still exactly one cutoff
// formula) but contains no per-frequency evaluation, no storage and no
// request handling. Callers must validate the cross-section with
// internal/validate before calling anything here, exactly as with the
// physics package.
package spectrum

import (
	"sort"

	"waveguide/internal/physics"
)

// Kind is the polarization family of a waveguide mode.
type Kind string

const (
	// KindTE is a transverse-electric mode. TE modes exist for every
	// (m, n) except (0, 0): at least one index must be positive.
	KindTE Kind = "TE"
	// KindTM is a transverse-magnetic mode. TM modes exist only when
	// both indices are positive (m >= 1 and n >= 1).
	KindTM Kind = "TM"
)

// SpectralMode is one member of the enumerated mode family together
// with its cutoff frequency.
type SpectralMode struct {
	Kind     Kind
	M        int
	N        int
	CutoffHz float64
}

// MaxModeIndex is the inclusive upper bound of the enumeration box:
// every TE mode with 0 <= m,n <= MaxModeIndex (excluding (0,0)) and
// every TM mode with 1 <= m,n <= MaxModeIndex is generated.
//
// Why this bound can never miss the mode that sets the upper edge of
// the single-mode band. For a valid cross-section a > b the cutoff
// wave number is kc = sqrt((m/a)^2 + (n/b)^2). The smallest possible
// value is 1/a, realized uniquely by TE10. The second-smallest value
// in the *unbounded* mode family is
//
//	min(2/a, 1/b)
//
// because:
//   - TE20 contributes 2/a and TE01 contributes 1/b;
//   - every TM mode has m,n >= 1, hence kc >= sqrt(1/a^2+1/b^2) > 1/b;
//   - any mode with m >= 3 has kc >= 3/a > 2/a;
//   - any mode with n >= 2 has kc >= 2/b > 1/b.
//
// So the second cutoff is realized only by TE20, by TE01, or by both
// at once when a = 2b — all with indices at most 2. Any mode outside
// a box of half-width 2 cuts off strictly higher. MaxModeIndex >= 2
// therefore makes the single-mode band exact for EVERY aspect ratio,
// including extreme ones; no physical mode capable of setting the
// upper edge is omitted. The larger value below merely widens the
// returned spectrum window, so engineers can inspect which higher
// modes intervene immediately above the band. For common waveguide
// aspect ratios (roughly 1 < a/b <= 2.5) the first several dozen modes
// all carry small indices and lie inside this window.
const MaxModeIndex = 20

// Enumerate generates every TE/TM mode inside the MaxModeIndex box for
// the given (validated) cross-section and returns them sorted by
// cutoff frequency in non-decreasing order. Equal cutoffs — genuine
// degeneracies such as TE20/TE01 at a = 2b or any TE_mn/TM_mn pair
// with m,n >= 1 — are kept as separate, adjacent entries. Ties are
// ordered deterministically (TE before TM, then m, then n) so the
// spectrum is stable across calls; callers grouping degenerate modes
// must compare cutoff values, never assume an index-based order.
func Enumerate(cs physics.CrossSection) []SpectralMode {
	teCount := (MaxModeIndex+1)*(MaxModeIndex+1) - 1
	tmCount := MaxModeIndex * MaxModeIndex
	out := make([]SpectralMode, 0, teCount+tmCount)

	for m := 0; m <= MaxModeIndex; m++ {
		for n := 0; n <= MaxModeIndex; n++ {
			// TE: both indices non-negative, (0,0) excluded.
			if m != 0 || n != 0 {
				out = append(out, newMode(KindTE, cs, m, n))
			}
			// TM: both indices strictly positive. Its cutoff equals
			// the TE mode with the same indices; both stay in the
			// spectrum as distinct degenerate members.
			if m >= 1 && n >= 1 {
				out = append(out, newMode(KindTM, cs, m, n))
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		switch {
		case out[i].CutoffHz != out[j].CutoffHz:
			return out[i].CutoffHz < out[j].CutoffHz
		case out[i].Kind != out[j].Kind:
			return out[i].Kind == KindTE // TE sorts before TM
		case out[i].M != out[j].M:
			return out[i].M < out[j].M
		default:
			return out[i].N < out[j].N
		}
	})
	return out
}

func newMode(kind Kind, cs physics.CrossSection, m, n int) SpectralMode {
	return SpectralMode{
		Kind:     kind,
		M:        m,
		N:        n,
		CutoffHz: physics.CutoffFrequency(cs, physics.Mode{M: m, N: n}),
	}
}

// Band is the single-mode operating interval of a cross-section.
//
// Frequencies below LowerHz propagate no mode at all; frequencies in
// [LowerHz, UpperHz) propagate exactly the dominant mode (LowerHz is
// the critical onset and is included); at or above UpperHz at least
// two modes propagate together. UpperHz equals the shared cutoff of
// every mode in Next when that edge is degenerate.
type Band struct {
	LowerHz float64
	UpperHz float64
	// Dominant is the degenerate group at LowerHz. For every valid
	// a > b cross-section it contains exactly one mode (TE10).
	Dominant []SpectralMode
	// Next is the degenerate group at UpperHz: one mode normally
	// (TE20 or TE01), two when their cutoffs coincide (a = 2b).
	Next []SpectralMode
}

// Width returns UpperHz - LowerHz.
func (b Band) Width() float64 { return b.UpperHz - b.LowerHz }

// Analysis bundles the sorted mode spectrum with the band derived
// from it.
type Analysis struct {
	Band Band
	// Spectrum is the full enumerated family sorted by non-decreasing
	// cutoff frequency.
	Spectrum     []SpectralMode
	MaxModeIndex int
}

// Analyze enumerates the mode family of cs and derives its single-mode
// band. The cs must have passed validate.CrossSection.
func Analyze(cs physics.CrossSection) Analysis {
	spectrum := Enumerate(cs)
	return Analysis{
		Band:         BandFromSpectrum(spectrum),
		Spectrum:     spectrum,
		MaxModeIndex: MaxModeIndex,
	}
}

// BandFromSpectrum derives the single-mode band from a spectrum sorted
// by non-decreasing cutoff frequency (as produced by Enumerate). The
// lower edge is the first (lowest) cutoff; the upper edge is the first
// cutoff strictly above it, and every mode sharing that cutoff is
// returned as the degenerate Next group.
func BandFromSpectrum(spectrum []SpectralMode) Band {
	dominantEnd := groupEnd(spectrum, 0)
	dominant := copyGroup(spectrum[:dominantEnd])

	nextStart := dominantEnd
	nextEnd := groupEnd(spectrum, nextStart)
	next := copyGroup(spectrum[nextStart:nextEnd])

	return Band{
		LowerHz:  spectrum[0].CutoffHz,
		UpperHz:  spectrum[nextStart].CutoffHz,
		Dominant: dominant,
		Next:     next,
	}
}

// groupEnd returns the index just past the run of modes in spec that
// share the cutoff at spec[start].
func groupEnd(spec []SpectralMode, start int) int {
	end := start + 1
	for end < len(spec) && spec[end].CutoffHz == spec[start].CutoffHz {
		end++
	}
	return end
}

func copyGroup(in []SpectralMode) []SpectralMode {
	out := make([]SpectralMode, len(in))
	copy(out, in)
	return out
}
