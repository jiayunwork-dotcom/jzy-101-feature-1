package physics

// EvaluatePoint is the single entry point that turns a validated
// cross-section, mode and frequency into a PointResult. Both the
// profile-based and the ad-hoc query paths funnel through it, so there
// is exactly one implementation of the physics.
func EvaluatePoint(cs CrossSection, mode Mode, f float64) PointResult {
	fc := CutoffFrequency(cs, mode)
	res := PointResult{
		FrequencyHz: f,
		CutoffHz:    fc,
		State:       StateAt(f, fc),
	}
	switch res.State {
	case StatePropagating:
		lg := GuideWavelength(f, fc, cs.EpsilonR)
		res.GuideWavelengthM = &lg
	case StateEvanescent:
		alpha := AttenuationNpPerM(f, fc, cs.EpsilonR)
		res.AttenuationNpPerM = &alpha
	case StateCritical:
		// Critical point: axial wave number is zero. No real guide
		// wavelength exists (it would diverge), attenuation is zero.
		zero := 0.0
		res.AttenuationNpPerM = &zero
	}
	return res
}
