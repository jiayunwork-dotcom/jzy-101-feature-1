package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"waveguide/internal/batch"
	"waveguide/internal/physics"
	"waveguide/internal/profile"
	"waveguide/internal/validate"
)

type handler struct {
	store *profile.Store
}

// ---------- helpers ----------

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

func writeValidationError(c *gin.Context, verr *validate.Error) {
	writeError(c, http.StatusBadRequest, verr.Code, verr.Message)
}

func writeStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, profile.ErrNotFound):
		writeError(c, http.StatusNotFound, "PROFILE_NOT_FOUND", err.Error())
	case errors.Is(err, profile.ErrConflict):
		writeError(c, http.StatusConflict, "PROFILE_CONFLICT", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
	}
}

func toDTO(p profile.Profile) profileDTO {
	return profileDTO{
		Name:                 p.Name,
		BroadDimensionM:      p.A,
		NarrowDimensionM:     p.B,
		RelativePermittivity: p.EpsilonR,
	}
}

// ---------- profile management ----------

func (h *handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *handler) createProfile(c *gin.Context) {
	var req profileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "MALFORMED_JSON", "request body is not valid JSON: "+err.Error())
		return
	}
	if verr := validate.ProfileName(req.Name); verr != nil {
		writeValidationError(c, verr)
		return
	}
	if verr := validate.CrossSection(req.BroadDimensionM, req.NarrowDimensionM, req.RelativePermittivity); verr != nil {
		writeValidationError(c, verr)
		return
	}
	p := profile.Profile{
		Name: req.Name,
		CrossSection: physics.CrossSection{
			A:        req.BroadDimensionM,
			B:        req.NarrowDimensionM,
			EpsilonR: req.RelativePermittivity,
		},
	}
	if err := h.store.Add(p); err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toDTO(p))
}

func (h *handler) listProfiles(c *gin.Context) {
	profiles := h.store.List()
	dtos := make([]profileDTO, 0, len(profiles))
	for _, p := range profiles {
		dtos = append(dtos, toDTO(p))
	}
	c.JSON(http.StatusOK, profileListDTO{Count: len(dtos), Profiles: dtos})
}

func (h *handler) getProfile(c *gin.Context) {
	p, err := h.store.Get(c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, toDTO(p))
}

func (h *handler) deleteProfile(c *gin.Context) {
	if err := h.store.Delete(c.Param("name")); err != nil {
		writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------- evaluation ----------

func (h *handler) evaluateProfile(c *gin.Context) {
	p, err := h.store.Get(c.Param("name"))
	if err != nil {
		writeStoreError(c, err)
		return
	}
	var req evaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "MALFORMED_JSON", "request body is not valid JSON: "+err.Error())
		return
	}
	h.runEvaluation(c, &p.Name, p.CrossSection, req.ModeM, req.ModeN, req.FrequenciesHz)
}

func (h *handler) evaluateAdHoc(c *gin.Context) {
	var req adHocRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "MALFORMED_JSON", "request body is not valid JSON: "+err.Error())
		return
	}
	if verr := validate.CrossSection(req.BroadDimensionM, req.NarrowDimensionM, req.RelativePermittivity); verr != nil {
		writeValidationError(c, verr)
		return
	}
	cs := physics.CrossSection{
		A:        req.BroadDimensionM,
		B:        req.NarrowDimensionM,
		EpsilonR: req.RelativePermittivity,
	}
	h.runEvaluation(c, nil, cs, req.ModeM, req.ModeN, req.FrequenciesHz)
}

// runEvaluation is the single computation path shared by the
// profile-based and the ad-hoc endpoints: validate, schedule the batch,
// shape the response.
func (h *handler) runEvaluation(c *gin.Context, profileName *string, cs physics.CrossSection, modeM, modeN int, freqs []float64) {
	if verr := validate.ModeIndices(modeM, modeN); verr != nil {
		writeValidationError(c, verr)
		return
	}
	if verr := validate.FrequencyBatch(len(freqs)); verr != nil {
		writeValidationError(c, verr)
		return
	}
	mode := physics.Mode{M: modeM, N: modeN}
	outcomes := batch.Evaluate(cs, mode, freqs, batch.DefaultMaxWorkers)

	resp := evaluateResponse{
		Profile:           profileName,
		Mode:              modeDTO{M: modeM, N: modeN},
		CutoffFrequencyHz: physics.CutoffFrequency(cs, mode),
		Results:           make([]pointDTO, 0, len(outcomes)),
	}
	for i, oc := range outcomes {
		switch {
		case oc.Err != nil:
			resp.Results = append(resp.Results, pointDTO{
				FrequencyHz: freqs[i],
				Error:       &errorBody{Code: oc.Err.Code, Message: oc.Err.Message},
			})
		default:
			r := oc.Result
			resp.Results = append(resp.Results, pointDTO{
				FrequencyHz:       r.FrequencyHz,
				State:             r.State,
				GuideWavelengthM:  r.GuideWavelengthM,
				AttenuationNpPerM: r.AttenuationNpPerM,
			})
		}
	}
	c.JSON(http.StatusOK, resp)
}
