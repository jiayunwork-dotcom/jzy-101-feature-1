package api

import (
	"fmt"
	"net/http"
	"testing"
)

// The WR-90 single-mode window over HTTP: lower bound exactly the TE10
// cutoff reported by point evaluation, upper bound exactly the TE20
// cutoff (twice TE10), the whole X-band inside, and a spectrum whose
// ordering yields the same bounds.
func TestProfileSpectrumWR90(t *testing.T) {
	srv := newTestServer(t)

	rec, body := doJSON(t, srv, http.MethodGet, "/api/v1/profiles/WR-90/spectrum", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("spectrum: status %d body %v", rec.Code, body)
	}
	rng := body["single_mode_range"].(map[string]any)
	lower := rng["lower_hz"].(float64)
	upper := rng["upper_hz"].(float64)
	if rng["lower_inclusive"] != true || rng["upper_inclusive"] != false {
		t.Fatalf("window must be [lower, upper): %v", rng)
	}

	// The lower bound must be bit-identical to the TE10 cutoff the
	// point-evaluation endpoint reports for the same profile.
	_, evalBody := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/evaluate", map[string]any{
		"mode_m": 1, "mode_n": 0, "frequencies_hz": []float64{10e9},
	})
	if fc := evalBody["cutoff_frequency_hz"].(float64); fc != lower {
		t.Fatalf("lower bound %v != TE10 cutoff %v from evaluate", lower, fc)
	}
	// For WR-90 the second mode is TE20, whose cutoff is exactly twice
	// the TE10 cutoff.
	if upper != 2*lower {
		t.Fatalf("upper bound %v, want exactly 2*lower = %v", upper, 2*lower)
	}

	// The whole X-band (8.2-12.4 GHz) lies inside the window.
	if !(8.2e9 >= lower && 12.4e9 < upper) {
		t.Fatalf("X-band must lie inside [%v, %v)", lower, upper)
	}

	// Dominant is TE10 alone, next is TE20 alone at the upper bound.
	dom := body["dominant_modes"].([]any)
	if len(dom) != 1 {
		t.Fatalf("dominant_modes = %v, want exactly TE10", dom)
	}
	d0 := dom[0].(map[string]any)
	if d0["kind"] != "TE" || d0["m"] != 1.0 || d0["n"] != 0.0 || d0["cutoff_hz"].(float64) != lower {
		t.Fatalf("dominant mode = %v, want TE10 at the lower bound", d0)
	}
	next := body["next_modes"].([]any)
	if len(next) != 1 {
		t.Fatalf("next_modes = %v, want exactly TE20", next)
	}
	n0 := next[0].(map[string]any)
	if n0["kind"] != "TE" || n0["m"] != 2.0 || n0["n"] != 0.0 || n0["cutoff_hz"].(float64) != upper {
		t.Fatalf("next mode = %v, want TE20 at the upper bound", n0)
	}

	// The spectrum itself is non-decreasing, starts at the lower bound,
	// and its first strictly higher cutoff is the upper bound.
	spec := body["spectrum"].([]any)
	if len(spec) == 0 {
		t.Fatalf("spectrum must not be empty")
	}
	if spec[0].(map[string]any)["cutoff_hz"].(float64) != lower {
		t.Fatalf("first spectrum cutoff must be the lower bound: %v", spec[0])
	}
	prev, firstHigher := 0.0, 0.0
	for i, raw := range spec {
		fc := raw.(map[string]any)["cutoff_hz"].(float64)
		if i > 0 && fc < prev {
			t.Fatalf("spectrum decreases at %d: %v < %v", i, fc, prev)
		}
		prev = fc
		if fc > lower && firstHigher == 0 {
			firstHigher = fc
		}
	}
	if firstHigher != upper {
		t.Fatalf("first strictly higher cutoff %v != upper bound %v", firstHigher, upper)
	}
}

// The ad-hoc path and the profile path must run the same spectrum core
// and agree bit for bit on identical cross-sections.
func TestAdHocSpectrumMatchesProfileSpectrum(t *testing.T) {
	srv := newTestServer(t)

	_, prof := doJSON(t, srv, http.MethodGet, "/api/v1/profiles/WR-90/spectrum", nil)
	rec, adhoc := doJSON(t, srv, http.MethodPost, "/api/v1/spectrum", map[string]any{
		"broad_dimension_m": 22.86e-3, "narrow_dimension_m": 10.16e-3,
		"relative_permittivity": 1.0,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("ad-hoc spectrum: status %d body %v", rec.Code, adhoc)
	}
	if _, hasProfile := adhoc["profile"]; hasProfile {
		t.Fatalf("ad-hoc response must not carry a profile name: %v", adhoc["profile"])
	}

	pr := prof["single_mode_range"].(map[string]any)
	ar := adhoc["single_mode_range"].(map[string]any)
	if pr["lower_hz"] != ar["lower_hz"] || pr["upper_hz"] != ar["upper_hz"] {
		t.Fatalf("range mismatch: profile %v vs ad-hoc %v", pr, ar)
	}

	ps := prof["spectrum"].([]any)
	as := adhoc["spectrum"].([]any)
	if len(ps) != len(as) {
		t.Fatalf("spectrum length mismatch: %d vs %d", len(ps), len(as))
	}
	for i := range ps {
		pe, ae := ps[i].(map[string]any), as[i].(map[string]any)
		if pe["kind"] != ae["kind"] || pe["m"] != ae["m"] || pe["n"] != ae["n"] || pe["cutoff_hz"] != ae["cutoff_hz"] {
			t.Fatalf("spectrum entry %d mismatch: %v vs %v", i, pe, ae)
		}
	}
}

// Invalid cross-sections and enumeration bounds must be rejected before
// any mode is enumerated; an unknown profile is a 404.
func TestSpectrumValidation(t *testing.T) {
	srv := newTestServer(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   map[string]any
		status int
		code   string
	}{
		{"broad not greater than narrow", "POST", "/api/v1/spectrum", map[string]any{
			"broad_dimension_m": 10e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0}, http.StatusBadRequest, "INVALID_DIMENSION"},
		{"negative dimension", "POST", "/api/v1/spectrum", map[string]any{
			"broad_dimension_m": -1, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0}, http.StatusBadRequest, "INVALID_DIMENSION"},
		{"permittivity below vacuum", "POST", "/api/v1/spectrum", map[string]any{
			"broad_dimension_m": 20e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 0.5}, http.StatusBadRequest, "INVALID_PERMITTIVITY"},
		{"max_index below the enumeration floor", "POST", "/api/v1/spectrum", map[string]any{
			"broad_dimension_m": 20e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0, "max_index": 1}, http.StatusBadRequest, "INVALID_MAX_INDEX"},
		{"max_index above the cap", "POST", "/api/v1/spectrum", map[string]any{
			"broad_dimension_m": 20e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0, "max_index": 100}, http.StatusBadRequest, "INVALID_MAX_INDEX"},
		{"max_index not an integer", "GET", "/api/v1/profiles/WR-90/spectrum?max_index=lots", nil,
			http.StatusBadRequest, "INVALID_MAX_INDEX"},
		{"max_index too small", "GET", "/api/v1/profiles/WR-90/spectrum?max_index=1", nil,
			http.StatusBadRequest, "INVALID_MAX_INDEX"},
		{"unknown profile", "GET", "/api/v1/profiles/NOPE/spectrum", nil,
			http.StatusNotFound, "PROFILE_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, body := doJSON(t, srv, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("want status %d, got %d (%v)", tc.status, rec.Code, body)
			}
			errObj, ok := body["error"].(map[string]any)
			if !ok || errObj["code"] != tc.code || errObj["message"] == "" {
				t.Fatalf("want error code %s with message, got %v", tc.code, body)
			}
		})
	}
}

// With a = 2b the TE20 and TE01 cutoffs coincide: both must appear in
// next_modes and their shared cutoff must be the upper bound.
func TestSpectrumDegeneratePairOverHTTP(t *testing.T) {
	srv := newTestServer(t)
	b := 10.16e-3

	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/spectrum", map[string]any{
		"broad_dimension_m": 2 * b, "narrow_dimension_m": b,
		"relative_permittivity": 1.0,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("spectrum: status %d body %v", rec.Code, body)
	}
	upper := body["single_mode_range"].(map[string]any)["upper_hz"].(float64)

	next := body["next_modes"].([]any)
	if len(next) != 2 {
		t.Fatalf("next_modes = %v, want the degenerate pair TE20 + TE01", next)
	}
	seen := map[string]bool{}
	for _, raw := range next {
		e := raw.(map[string]any)
		if e["cutoff_hz"].(float64) != upper {
			t.Fatalf("degenerate mode %v not at the upper bound %v", e, upper)
		}
		seen[fmt.Sprintf("%v-%v-%v", e["kind"], e["m"], e["n"])] = true
	}
	if !seen["TE-2-0"] || !seen["TE-0-1"] {
		t.Fatalf("next_modes = %v, want TE(2,0) and TE(0,1)", next)
	}

	// Both must also appear in the spectrum at that shared cutoff.
	count := 0
	for _, raw := range body["spectrum"].([]any) {
		e := raw.(map[string]any)
		if e["cutoff_hz"].(float64) != upper || e["kind"] != "TE" {
			continue
		}
		if (e["m"] == 2.0 && e["n"] == 0.0) || (e["m"] == 0.0 && e["n"] == 1.0) {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("spectrum must list both degenerate modes at the upper bound, found %d", count)
	}
}

// The enumeration bound is controllable: max_index sizes the spectrum
// but never moves the single-mode window.
func TestSpectrumMaxIndexControlsSpectrumNotRange(t *testing.T) {
	srv := newTestServer(t)

	_, def := doJSON(t, srv, http.MethodGet, "/api/v1/profiles/WR-90/spectrum", nil)
	defRange := def["single_mode_range"].(map[string]any)

	rec, body := doJSON(t, srv, http.MethodGet, "/api/v1/profiles/WR-90/spectrum?max_index=4", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("spectrum: status %d body %v", rec.Code, body)
	}
	if body["max_index"].(float64) != 4 {
		t.Fatalf("max_index = %v, want 4", body["max_index"])
	}
	// TE modes: (4+1)²-1 = 24; TM modes: 4² = 16; total 40.
	if got := len(body["spectrum"].([]any)); got != 40 {
		t.Fatalf("spectrum length = %d, want 40", got)
	}
	rng := body["single_mode_range"].(map[string]any)
	if rng["lower_hz"] != defRange["lower_hz"] || rng["upper_hz"] != defRange["upper_hz"] {
		t.Fatalf("max_index must not move the window: %v vs %v", rng, defRange)
	}

	// The ad-hoc path accepts the same bound.
	rec, body = doJSON(t, srv, http.MethodPost, "/api/v1/spectrum", map[string]any{
		"broad_dimension_m": 22.86e-3, "narrow_dimension_m": 10.16e-3,
		"relative_permittivity": 1.0, "max_index": 4,
	})
	if rec.Code != http.StatusOK || len(body["spectrum"].([]any)) != 40 {
		t.Fatalf("ad-hoc max_index=4: status %d, spectrum %v", rec.Code, body["spectrum"])
	}
}
