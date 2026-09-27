package api

import "waveguide/internal/physics"

// ---------- requests ----------

type profileRequest struct {
	Name                 string  `json:"name"`
	BroadDimensionM      float64 `json:"broad_dimension_m"`
	NarrowDimensionM     float64 `json:"narrow_dimension_m"`
	RelativePermittivity float64 `json:"relative_permittivity"`
}

type evaluateRequest struct {
	ModeM         int       `json:"mode_m"`
	ModeN         int       `json:"mode_n"`
	FrequenciesHz []float64 `json:"frequencies_hz"`
}

type adHocRequest struct {
	BroadDimensionM      float64   `json:"broad_dimension_m"`
	NarrowDimensionM     float64   `json:"narrow_dimension_m"`
	RelativePermittivity float64   `json:"relative_permittivity"`
	ModeM                int       `json:"mode_m"`
	ModeN                int       `json:"mode_n"`
	FrequenciesHz        []float64 `json:"frequencies_hz"`
}

// adHocSpectrumRequest is a cross-section submitted for whole-family
// spectrum analysis without naming a mode or any frequencies.
type adHocSpectrumRequest struct {
	BroadDimensionM      float64 `json:"broad_dimension_m"`
	NarrowDimensionM     float64 `json:"narrow_dimension_m"`
	RelativePermittivity float64 `json:"relative_permittivity"`
}

// ---------- responses ----------

type profileDTO struct {
	Name                 string  `json:"name"`
	BroadDimensionM      float64 `json:"broad_dimension_m"`
	NarrowDimensionM     float64 `json:"narrow_dimension_m"`
	RelativePermittivity float64 `json:"relative_permittivity"`
}

type profileListDTO struct {
	Count    int          `json:"count"`
	Profiles []profileDTO `json:"profiles"`
}

type modeDTO struct {
	M int `json:"m"`
	N int `json:"n"`
}

type pointDTO struct {
	FrequencyHz       float64       `json:"frequency_hz"`
	State             physics.State `json:"state,omitempty"`
	GuideWavelengthM  *float64      `json:"guide_wavelength_m,omitempty"`
	AttenuationNpPerM *float64      `json:"attenuation_np_per_m,omitempty"`
	Error             *errorBody    `json:"error,omitempty"`
}

type evaluateResponse struct {
	Profile           *string    `json:"profile,omitempty"`
	Mode              modeDTO    `json:"mode"`
	CutoffFrequencyHz float64    `json:"cutoff_frequency_hz"`
	Results           []pointDTO `json:"results"`
}

// ---------- mode spectrum ----------

type spectralModeDTO struct {
	Kind              string  `json:"kind"`
	M                 int     `json:"m"`
	N                 int     `json:"n"`
	CutoffFrequencyHz float64 `json:"cutoff_frequency_hz"`
}

type singleModeBandDTO struct {
	LowerFrequencyHz float64           `json:"lower_frequency_hz"`
	UpperFrequencyHz float64           `json:"upper_frequency_hz"`
	LowerInclusive   bool              `json:"lower_inclusive"`
	UpperInclusive   bool              `json:"upper_inclusive"`
	BandwidthHz      float64           `json:"bandwidth_hz"`
	DominantModes    []spectralModeDTO `json:"dominant_modes"`
	NextModes        []spectralModeDTO `json:"next_modes"`
}

type spectrumResponse struct {
	Profile        *string           `json:"profile,omitempty"`
	SingleModeBand singleModeBandDTO `json:"single_mode_band"`
	Enumeration    spectrumWindowDTO `json:"enumeration"`
	Spectrum       []spectralModeDTO `json:"spectrum"`
}

type spectrumWindowDTO struct {
	MaxModeIndex int `json:"max_mode_index"`
	ModeCount    int `json:"mode_count"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}
