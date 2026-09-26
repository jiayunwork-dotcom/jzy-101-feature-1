package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"waveguide/internal/physics"
	"waveguide/internal/profile"
)

func newTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := profile.NewStore()
	profile.SeedBuiltin(store)
	return NewServer(store)
}

func doJSON(t *testing.T, srv http.Handler, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var parsed map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("response is not JSON: %v\nbody: %s", err, rec.Body.String())
		}
	}
	return rec, parsed
}

func TestSampleProfileIsSeededAndSingleMode(t *testing.T) {
	srv := newTestServer(t)

	rec, body := doJSON(t, srv, http.MethodGet, "/api/v1/profiles", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d", rec.Code)
	}
	if body["count"].(float64) < 1 {
		t.Fatalf("expected at least the builtin profile, got %v", body)
	}

	// WR-90, TE10 at 10 GHz (X-band): must propagate.
	rec, body = doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/evaluate", map[string]any{
		"mode_m": 1, "mode_n": 0, "frequencies_hz": []float64{10e9},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("evaluate: status %d body %v", rec.Code, body)
	}
	results := body["results"].([]any)
	pt := results[0].(map[string]any)
	if pt["state"] != string(physics.StatePropagating) {
		t.Fatalf("TE10 at 10 GHz must propagate, got %v", pt)
	}
	if pt["guide_wavelength_m"] == nil {
		t.Fatalf("propagating point must carry a guide wavelength: %v", pt)
	}

	// Same frequency, TE20 (fc ~13.1 GHz): must be evanescent, i.e. the
	// band is single-mode for TE10.
	_, body = doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/evaluate", map[string]any{
		"mode_m": 2, "mode_n": 0, "frequencies_hz": []float64{10e9},
	})
	pt = body["results"].([]any)[0].(map[string]any)
	if pt["state"] != string(physics.StateEvanescent) {
		t.Fatalf("TE20 at 10 GHz must be evanescent, got %v", pt)
	}
	if pt["attenuation_np_per_m"] == nil || pt["guide_wavelength_m"] != nil {
		t.Fatalf("evanescent point must carry attenuation only: %v", pt)
	}
}

func TestCreateDuplicateDeleteProfile(t *testing.T) {
	srv := newTestServer(t)
	newProfile := map[string]any{
		"name": "WR-62", "broad_dimension_m": 15.7988e-3,
		"narrow_dimension_m": 7.8994e-3, "relative_permittivity": 1.0,
	}

	rec, _ := doJSON(t, srv, http.MethodPost, "/api/v1/profiles", newProfile)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d", rec.Code)
	}

	// Duplicate name must conflict, not overwrite.
	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles", newProfile)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate create: want 409, got %d (%v)", rec.Code, body)
	}
	if body["error"].(map[string]any)["code"] != "PROFILE_CONFLICT" {
		t.Fatalf("duplicate create: want PROFILE_CONFLICT, got %v", body)
	}

	// Delete works once, then reports not found.
	rec, _ = doJSON(t, srv, http.MethodDelete, "/api/v1/profiles/WR-62", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d", rec.Code)
	}
	rec, body = doJSON(t, srv, http.MethodDelete, "/api/v1/profiles/WR-62", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete: want 404, got %d (%v)", rec.Code, body)
	}
	if body["error"].(map[string]any)["code"] != "PROFILE_NOT_FOUND" {
		t.Fatalf("second delete: want PROFILE_NOT_FOUND, got %v", body)
	}
}

func TestUnknownProfileEvaluateNotFound(t *testing.T) {
	srv := newTestServer(t)
	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/NOPE/evaluate", map[string]any{
		"mode_m": 1, "mode_n": 0, "frequencies_hz": []float64{10e9},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%v)", rec.Code, body)
	}
}

func TestValidationErrorsPrecedeComputation(t *testing.T) {
	srv := newTestServer(t)
	cases := []struct {
		name string
		path string
		body map[string]any
		code string
	}{
		{"broad not greater than narrow", "/api/v1/profiles", map[string]any{
			"name": "BAD", "broad_dimension_m": 5e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0}, "INVALID_DIMENSION"},
		{"negative dimension", "/api/v1/profiles", map[string]any{
			"name": "BAD", "broad_dimension_m": -1, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0}, "INVALID_DIMENSION"},
		{"permittivity below vacuum", "/api/v1/profiles", map[string]any{
			"name": "BAD", "broad_dimension_m": 20e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 0.5}, "INVALID_PERMITTIVITY"},
		{"zero-zero mode", "/api/v1/profiles/WR-90/evaluate", map[string]any{
			"mode_m": 0, "mode_n": 0, "frequencies_hz": []float64{10e9}}, "INVALID_MODE"},
		{"empty frequency list", "/api/v1/profiles/WR-90/evaluate", map[string]any{
			"mode_m": 1, "mode_n": 0, "frequencies_hz": []float64{}}, "EMPTY_FREQUENCY_LIST"},
		{"ad-hoc bad geometry", "/api/v1/evaluate", map[string]any{
			"broad_dimension_m": 10e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0, "mode_m": 1, "mode_n": 0,
			"frequencies_hz": []float64{10e9}}, "INVALID_DIMENSION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, body := doJSON(t, srv, http.MethodPost, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d (%v)", rec.Code, body)
			}
			errObj, ok := body["error"].(map[string]any)
			if !ok || errObj["code"] != tc.code || errObj["message"] == "" {
				t.Fatalf("want error code %s with message, got %v", tc.code, body)
			}
		})
	}
}

func TestBatchEvaluationPartialFailure(t *testing.T) {
	srv := newTestServer(t)
	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/evaluate", map[string]any{
		"mode_m": 1, "mode_n": 0,
		"frequencies_hz": []float64{10e9, -1, 5e9},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("batch: status %d (%v)", rec.Code, body)
	}
	results := body["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("want 3 results, got %v", results)
	}
	if results[0].(map[string]any)["state"] != string(physics.StatePropagating) {
		t.Fatalf("first point must propagate: %v", results[0])
	}
	bad := results[1].(map[string]any)
	if bad["error"] == nil || bad["error"].(map[string]any)["code"] != "INVALID_FREQUENCY" {
		t.Fatalf("second point must carry INVALID_FREQUENCY: %v", bad)
	}
	if results[2].(map[string]any)["state"] != string(physics.StateEvanescent) {
		t.Fatalf("third point must be evanescent: %v", results[2])
	}
}

func TestCriticalFrequencyHandling(t *testing.T) {
	srv := newTestServer(t)

	// Ask for the cutoff, then evaluate exactly at it.
	_, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/evaluate", map[string]any{
		"mode_m": 1, "mode_n": 0, "frequencies_hz": []float64{10e9},
	})
	fc := body["cutoff_frequency_hz"].(float64)

	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/evaluate", map[string]any{
		"mode_m": 1, "mode_n": 0, "frequencies_hz": []float64{fc},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("critical evaluate: status %d", rec.Code)
	}
	pt := body["results"].([]any)[0].(map[string]any)
	if pt["state"] != string(physics.StateCritical) {
		t.Fatalf("at fc the state must be critical, got %v", pt)
	}
	if _, hasLG := pt["guide_wavelength_m"]; hasLG {
		t.Fatalf("critical point must not report a guide wavelength: %v", pt)
	}
	if alpha, ok := pt["attenuation_np_per_m"].(float64); !ok || alpha != 0 {
		t.Fatalf("critical point must report zero attenuation: %v", pt)
	}
}

// The ad-hoc path and the profile path must run the same core logic and
// agree bit for bit on identical inputs.
func TestAdHocMatchesProfileEvaluation(t *testing.T) {
	srv := newTestServer(t)
	freqs := []float64{8e9, 10e9, 12e9}

	_, profBody := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/evaluate", map[string]any{
		"mode_m": 1, "mode_n": 0, "frequencies_hz": freqs,
	})
	_, adhocBody := doJSON(t, srv, http.MethodPost, "/api/v1/evaluate", map[string]any{
		"broad_dimension_m": 22.86e-3, "narrow_dimension_m": 10.16e-3,
		"relative_permittivity": 1.0, "mode_m": 1, "mode_n": 0,
		"frequencies_hz": freqs,
	})

	if profBody["cutoff_frequency_hz"] != adhocBody["cutoff_frequency_hz"] {
		t.Fatalf("cutoff mismatch: profile %v vs ad-hoc %v",
			profBody["cutoff_frequency_hz"], adhocBody["cutoff_frequency_hz"])
	}
	pr := profBody["results"].([]any)
	ar := adhocBody["results"].([]any)
	for i := range pr {
		plg := pr[i].(map[string]any)["guide_wavelength_m"]
		alg := ar[i].(map[string]any)["guide_wavelength_m"]
		if plg != alg {
			t.Fatalf("point %d: profile λg %v vs ad-hoc λg %v", i, plg, alg)
		}
	}
}

// Physics invariants through the HTTP layer: doubling a halves the TE10
// cutoff exactly; epsR=4 halves it exactly.
func TestScalingInvariantsOverHTTP(t *testing.T) {
	srv := newTestServer(t)
	eval := func(a, eps float64) float64 {
		_, body := doJSON(t, srv, http.MethodPost, "/api/v1/evaluate", map[string]any{
			"broad_dimension_m": a, "narrow_dimension_m": 10.16e-3,
			"relative_permittivity": eps, "mode_m": 1, "mode_n": 0,
			"frequencies_hz": []float64{1e9},
		})
		return body["cutoff_frequency_hz"].(float64)
	}

	base := eval(22.86e-3, 1.0)
	if got := eval(2*22.86e-3, 1.0); got != base/2 {
		t.Fatalf("doubling a: cutoff %v, want exactly %v", got, base/2)
	}
	if got := eval(22.86e-3, 4.0); got != base/2 {
		t.Fatalf("epsR=4: cutoff %v, want exactly %v", got, base/2)
	}
}

// Concurrent clients must not leak request state into each other.
func TestConcurrentClients(t *testing.T) {
	srv := newTestServer(t)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("WG-%d", i)
			rec, _ := doJSON(t, srv, http.MethodPost, "/api/v1/profiles", map[string]any{
				"name": name, "broad_dimension_m": 20e-3 + float64(i)*1e-4,
				"narrow_dimension_m": 10e-3, "relative_permittivity": 1.0,
			})
			if rec.Code != http.StatusCreated {
				t.Errorf("create %s: status %d", name, rec.Code)
				return
			}
			rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/"+name+"/evaluate", map[string]any{
				"mode_m": 1, "mode_n": 0, "frequencies_hz": []float64{9e9, 11e9},
			})
			if rec.Code != http.StatusOK || len(body["results"].([]any)) != 2 {
				t.Errorf("evaluate %s: status %d body %v", name, rec.Code, body)
			}
			// Ad-hoc query with the same dimensions must agree on cutoff.
			_, adhoc := doJSON(t, srv, http.MethodPost, "/api/v1/evaluate", map[string]any{
				"broad_dimension_m": 20e-3 + float64(i)*1e-4, "narrow_dimension_m": 10e-3,
				"relative_permittivity": 1.0, "mode_m": 1, "mode_n": 0,
				"frequencies_hz": []float64{9e9},
			})
			if math.Abs(body["cutoff_frequency_hz"].(float64)-adhoc["cutoff_frequency_hz"].(float64)) != 0 {
				t.Errorf("client %d: profile/ad-hoc cutoff mismatch", i)
			}
		}(i)
	}
	wg.Wait()
}
