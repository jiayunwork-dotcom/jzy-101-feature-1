package api

import (
	"waveguide/internal/physics"
	"waveguide/internal/spectrum"
)

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

// spectrumRequest is the ad-hoc spectrum query: a cross-section plus an
// optional enumeration bound (nil means the service default).
type spectrumRequest struct {
	BroadDimensionM      float64 `json:"broad_dimension_m"`
	NarrowDimensionM     float64 `json:"narrow_dimension_m"`
	RelativePermittivity float64 `json:"relative_permittivity"`
	MaxIndex             *int    `json:"max_index,omitempty"`
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

// ---------- spectrum responses ----------

type spectrumEntryDTO struct {
	Kind     spectrum.ModeKind `json:"kind"`
	M        int               `json:"m"`
	N        int               `json:"n"`
	CutoffHz float64           `json:"cutoff_hz"`
}

// singleModeRangeDTO is the window [lower_hz, upper_hz): the lower
// bound is inclusive, the upper bound exclusive. The flags are constant
// and exist so the semantics travel with the payload.
type singleModeRangeDTO struct {
	LowerHz        float64 `json:"lower_hz"`
	UpperHz        float64 `json:"upper_hz"`
	LowerInclusive bool    `json:"lower_inclusive"`
	UpperInclusive bool    `json:"upper_inclusive"`
}

type spectrumResponse struct {
	Profile              *string            `json:"profile,omitempty"`
	BroadDimensionM      float64            `json:"broad_dimension_m"`
	NarrowDimensionM     float64            `json:"narrow_dimension_m"`
	RelativePermittivity float64            `json:"relative_permittivity"`
	MaxIndex             int                `json:"max_index"`
	SingleModeRange      singleModeRangeDTO `json:"single_mode_range"`
	DominantModes        []spectrumEntryDTO `json:"dominant_modes"`
	NextModes            []spectrumEntryDTO `json:"next_modes"`
	Spectrum             []spectrumEntryDTO `json:"spectrum"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}
