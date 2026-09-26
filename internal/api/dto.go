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

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}
