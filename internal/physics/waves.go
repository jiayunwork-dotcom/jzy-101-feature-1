package physics

import "math"

// MediumWavelength returns the free-space wavelength in the filling
// medium, in meters: lambda = c / (f * sqrt(epsR)).
func MediumWavelength(f, epsR float64) float64 {
	return SpeedOfLight / (f * math.Sqrt(epsR))
}

// GuideWavelength returns the guide wavelength in meters of a
// propagating mode:
//
//	lambda_g = lambda_medium / sqrt(1 - (fc/f)^2)
//
// Callers must ensure f > fc; as f approaches fc from above the result
// diverges, which is exactly why the critical state never reports one.
func GuideWavelength(f, fc, epsR float64) float64 {
	r := fc / f
	return MediumWavelength(f, epsR) / math.Sqrt(1-r*r)
}

// AttenuationNpPerM returns the evanescent attenuation constant in
// nepers per meter:
//
//	alpha = (2*pi*f*sqrt(epsR)/c) * sqrt((fc/f)^2 - 1)
//
// Callers must ensure f <= fc; at f == fc the value is exactly 0.
func AttenuationNpPerM(f, fc, epsR float64) float64 {
	r := fc / f
	d := r*r - 1
	if d <= 0 {
		return 0
	}
	k := 2 * math.Pi * f * math.Sqrt(epsR) / SpeedOfLight
	return k * math.Sqrt(d)
}
