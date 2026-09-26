package physics

import "math"

// criticalRelTol is the relative tolerance around the cutoff frequency
// within which an operating point is classified as StateCritical. It is
// far below any physically meaningful frequency resolution (~6.5 mHz at
// 6.5 GHz) and exists so that a frequency copied verbatim from a
// previously reported cutoff value is recognized as exactly critical.
const criticalRelTol = 1e-12

// CutoffFrequency returns the cutoff frequency in Hz of the given mode:
//
//	fc_mn = c / (2 * sqrt(epsR)) * sqrt((m/a)^2 + (n/b)^2)
//
// The filling is assumed non-magnetic (muR = 1).
func CutoffFrequency(cs CrossSection, mode Mode) float64 {
	kc := math.Hypot(float64(mode.M)/cs.A, float64(mode.N)/cs.B)
	return SpeedOfLight / (2 * math.Sqrt(cs.EpsilonR)) * kc
}

// StateAt classifies the propagation regime of an operating frequency f
// relative to a cutoff frequency fc.
func StateAt(f, fc float64) State {
	switch {
	case f > fc*(1+criticalRelTol):
		return StatePropagating
	case f < fc*(1-criticalRelTol):
		return StateEvanescent
	default:
		return StateCritical
	}
}
