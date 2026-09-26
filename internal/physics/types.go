// Package physics holds the core rectangular-waveguide math: cutoff
// frequencies, propagation state, guide wavelength and evanescent
// attenuation. It has no dependency on storage, validation or HTTP.
package physics

// SpeedOfLight is the exact SI speed of light in vacuum, in m/s.
const SpeedOfLight = 299792458.0

// Mode identifies a TE/TM mode by its integer indices (m, n).
type Mode struct {
	M int
	N int
}

// CrossSection describes a rectangular waveguide cross-section together
// with its (non-magnetic) dielectric filling.
//
//	A:        broad (wide) dimension in meters, strictly greater than B
//	B:        narrow dimension in meters
//	EpsilonR: relative permittivity of the filling, >= 1
type CrossSection struct {
	A        float64
	B        float64
	EpsilonR float64
}

// State is the propagation regime of a mode at a given frequency.
type State string

const (
	// StatePropagating: f is above cutoff, the wave travels with a real
	// guide wavelength.
	StatePropagating State = "propagating"
	// StateEvanescent: f is below cutoff, the wave decays exponentially
	// and only an attenuation constant is meaningful.
	StateEvanescent State = "evanescent"
	// StateCritical: f equals the cutoff frequency (within a tight
	// relative tolerance). The axial wave number is zero: the guide
	// wavelength would be infinite and is therefore NOT reported; the
	// attenuation constant is exactly 0.
	StateCritical State = "critical"
)

// PointResult is the evaluated outcome for one (cross-section, mode,
// frequency) triple. Exactly one of GuideWavelengthM / AttenuationNpPerM
// is populated, depending on State.
type PointResult struct {
	FrequencyHz float64
	CutoffHz    float64
	State       State
	// GuideWavelengthM is set only when State == StatePropagating.
	GuideWavelengthM *float64
	// AttenuationNpPerM is set when State == StateEvanescent, and is
	// exactly 0 when State == StateCritical.
	AttenuationNpPerM *float64
}
