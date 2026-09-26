// Package api exposes the waveguide service over HTTP using Gin. It is
// a thin layer: all physics lives in internal/physics, all storage in
// internal/profile, all checking in internal/validate, and batch
// scheduling in internal/batch.
package api

import (
	"github.com/gin-gonic/gin"

	"waveguide/internal/profile"
)

// NewServer builds the Gin engine with all routes wired to the given
// profile store. The engine keeps no per-request state of its own, so
// concurrent clients never leak state into each other's requests.
func NewServer(store *profile.Store) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	h := &handler{store: store}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/healthz", h.health)

		v1.POST("/profiles", h.createProfile)
		v1.GET("/profiles", h.listProfiles)
		v1.GET("/profiles/:name", h.getProfile)
		v1.DELETE("/profiles/:name", h.deleteProfile)
		v1.POST("/profiles/:name/evaluate", h.evaluateProfile)

		v1.POST("/evaluate", h.evaluateAdHoc)
	}
	return r
}
