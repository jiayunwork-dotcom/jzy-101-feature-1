package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func modeCutoffOverHTTP(t *testing.T, srv http.Handler, a, b, eps float64, m, n int) float64 {
	t.Helper()
	_, body := doJSON(t, srv, http.MethodPost, "/api/v1/evaluate", map[string]any{
		"broad_dimension_m": a, "narrow_dimension_m": b,
		"relative_permittivity": eps, "mode_m": m, "mode_n": n,
		"frequencies_hz": []float64{1e9},
	})
	return body["cutoff_frequency_hz"].(float64)
}

func nonDecreasingCutoffs(t *testing.T, spec []any) {
	t.Helper()
	prev := 0.0
	for i, raw := range spec {
		fc := raw.(map[string]any)["cutoff_frequency_hz"].(float64)
		if i > 0 && fc < prev {
			t.Fatalf("spectrum not non-decreasing at %d: %v after %v", i, fc, prev)
		}
		prev = fc
	}
}

// Named profile path: WR-90 band edges must equal the TE10/TE20
// cutoffs returned by the existing point endpoint exactly, and the
// whole X-band must lie inside [lower, upper).
func TestProfileSpectrumWR90(t *testing.T) {
	srv := newTestServer(t)

	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/spectrum", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %v", rec.Code, body)
	}
	if body["profile"] != "WR-90" {
		t.Fatalf("profile name not echoed: %v", body["profile"])
	}

	band := body["single_mode_band"].(map[string]any)
	fc10 := modeCutoffOverHTTP(t, srv, 22.86e-3, 10.16e-3, 1.0, 1, 0)
	fc20 := modeCutoffOverHTTP(t, srv, 22.86e-3, 10.16e-3, 1.0, 2, 0)
	if band["lower_frequency_hz"] != fc10 {
		t.Fatalf("lower %v != TE10 cutoff %v", band["lower_frequency_hz"], fc10)
	}
	if band["upper_frequency_hz"] != fc20 {
		t.Fatalf("upper %v != TE20 cutoff %v", band["upper_frequency_hz"], fc20)
	}
	if band["lower_inclusive"] != true || band["upper_inclusive"] != false {
		t.Fatalf("band endpoints must be [lower, upper), got %+v", band)
	}
	if band["bandwidth_hz"].(float64) != fc20-fc10 {
		t.Fatalf("bandwidth %v, want %v", band["bandwidth_hz"], fc20-fc10)
	}
	if 8.2e9 < band["lower_frequency_hz"].(float64) ||
		12.4e9 >= band["upper_frequency_hz"].(float64) {
		t.Fatalf("X-band must lie inside [%v, %v)", band["lower_frequency_hz"], band["upper_frequency_hz"])
	}

	dom := band["dominant_modes"].([]any)
	nxt := band["next_modes"].([]any)
	if len(dom) != 1 || dom[0].(map[string]any)["m"].(float64) != 1 ||
		dom[0].(map[string]any)["n"].(float64) != 0 {
		t.Fatalf("dominant group must be [TE10], got %v", dom)
	}
	if len(nxt) != 1 || nxt[0].(map[string]any)["m"].(float64) != 2 ||
		nxt[0].(map[string]any)["n"].(float64) != 0 {
		t.Fatalf("next group must be [TE20], got %v", nxt)
	}

	spec := body["spectrum"].([]any)
	nonDecreasingCutoffs(t, spec)
	if spec[0].(map[string]any)["cutoff_frequency_hz"] != band["lower_frequency_hz"] {
		t.Fatal("spectrum first cutoff must equal the band lower edge")
	}
	i := 1
	for spec[i].(map[string]any)["cutoff_frequency_hz"] == band["lower_frequency_hz"] {
		i++
	}
	if spec[i].(map[string]any)["cutoff_frequency_hz"] != band["upper_frequency_hz"] {
		t.Fatal("first strictly higher spectrum cutoff must equal the band upper edge")
	}

	enum := body["enumeration"].(map[string]any)
	if enum["mode_count"].(float64) != float64(len(spec)) ||
		enum["max_mode_index"].(float64) < 2 {
		t.Fatalf("enumeration window malformed: %v (spectrum len %d)", enum, len(spec))
	}
}

// The ad-hoc spectrum path and the named-profile path must reuse the
// same core and agree bit for bit on identical cross-sections.
func TestAdHocSpectrumMatchesProfile(t *testing.T) {
	srv := newTestServer(t)

	_, prof := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/WR-90/spectrum", nil)
	_, adhoc := doJSON(t, srv, http.MethodPost, "/api/v1/spectrum", map[string]any{
		"broad_dimension_m": 22.86e-3, "narrow_dimension_m": 10.16e-3,
		"relative_permittivity": 1.0,
	})

	pb := prof["single_mode_band"].(map[string]any)
	ab := adhoc["single_mode_band"].(map[string]any)
	if pb["lower_frequency_hz"] != ab["lower_frequency_hz"] ||
		pb["upper_frequency_hz"] != ab["upper_frequency_hz"] {
		t.Fatalf("band mismatch: profile %+v vs ad-hoc %+v", pb, ab)
	}
	ps := prof["spectrum"].([]any)
	as := adhoc["spectrum"].([]any)
	if len(ps) != len(as) {
		t.Fatalf("spectrum length mismatch: %d vs %d", len(ps), len(as))
	}
	for i := range ps {
		if !reflect.DeepEqual(ps[i], as[i]) {
			t.Fatalf("spectrum entry %d differs: profile %v vs ad-hoc %v", i, ps[i], as[i])
		}
	}
	if _, echoed := adhoc["profile"]; echoed {
		t.Fatalf("ad-hoc response must not carry a profile name, got %v", adhoc["profile"])
	}
}

// HTTP-level scaling invariants: doubling a halves both edges and the
// bandwidth exactly; epsR = 4 does the same.
func TestSpectrumScalingOverHTTP(t *testing.T) {
	srv := newTestServer(t)
	spectrumOf := func(a, eps float64) map[string]any {
		_, body := doJSON(t, srv, http.MethodPost, "/api/v1/spectrum", map[string]any{
			"broad_dimension_m": a, "narrow_dimension_m": 10.16e-3,
			"relative_permittivity": eps,
		})
		return body["single_mode_band"].(map[string]any)
	}

	base := spectrumOf(22.86e-3, 1.0)
	doubled := spectrumOf(2*22.86e-3, 1.0)
	if doubled["lower_frequency_hz"] != base["lower_frequency_hz"].(float64)/2 ||
		doubled["upper_frequency_hz"] != base["upper_frequency_hz"].(float64)/2 ||
		doubled["bandwidth_hz"] != base["bandwidth_hz"].(float64)/2 {
		t.Fatalf("doubling a must halve band and width: base %+v got %+v", base, doubled)
	}
	filled := spectrumOf(22.86e-3, 4.0)
	if filled["lower_frequency_hz"] != base["lower_frequency_hz"].(float64)/2 ||
		filled["upper_frequency_hz"] != base["upper_frequency_hz"].(float64)/2 ||
		filled["bandwidth_hz"] != base["bandwidth_hz"].(float64)/2 {
		t.Fatalf("epsR=4 must halve band and width: base %+v got %+v", base, filled)
	}
}

// At a = 2b the TE20/TE01 degeneracy must be surfaced: both appear in
// next_modes and in the spectrum, sharing the band upper edge.
func TestSpectrumDegeneracyOverHTTP(t *testing.T) {
	srv := newTestServer(t)
	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/spectrum", map[string]any{
		"broad_dimension_m": 0.02, "narrow_dimension_m": 0.01,
		"relative_permittivity": 1.0,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %v", rec.Code, body)
	}
	band := body["single_mode_band"].(map[string]any)
	fc20 := modeCutoffOverHTTP(t, srv, 0.02, 0.01, 1.0, 2, 0)
	fc01 := modeCutoffOverHTTP(t, srv, 0.02, 0.01, 1.0, 0, 1)
	if fc20 != fc01 {
		t.Fatalf("TE20/TE01 cutoffs must coincide, got %v and %v", fc20, fc01)
	}
	if band["upper_frequency_hz"] != fc20 {
		t.Fatalf("upper %v must equal shared cutoff %v", band["upper_frequency_hz"], fc20)
	}
	nxt := band["next_modes"].([]any)
	if len(nxt) != 2 {
		t.Fatalf("next_modes must hold both degenerate modes, got %v", nxt)
	}
	seen := map[[2]int]bool{}
	for _, raw := range nxt {
		m := raw.(map[string]any)
		if m["cutoff_frequency_hz"] != fc20 {
			t.Fatalf("degenerate member %v not at shared cutoff", m)
		}
		seen[[2]int{int(m["m"].(float64)), int(m["n"].(float64))}] = true
	}
	if !seen[[2]int{2, 0}] || !seen[[2]int{0, 1}] {
		t.Fatalf("next_modes must be TE20 and TE01, got %v", nxt)
	}

	spec := body["spectrum"].([]any)
	nonDecreasingCutoffs(t, spec)
	var atEdge []any
	for _, raw := range spec {
		if raw.(map[string]any)["cutoff_frequency_hz"] == fc20 {
			atEdge = append(atEdge, raw)
		}
	}
	if len(atEdge) != 2 {
		t.Fatalf("spectrum must list both modes at the shared cutoff, got %v", atEdge)
	}
}

// Validation runs before enumeration on both paths; unknown profiles
// are 404.
func TestSpectrumValidationAndNotFound(t *testing.T) {
	srv := newTestServer(t)

	rec, body := doJSON(t, srv, http.MethodPost, "/api/v1/profiles/NOPE/spectrum", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%v)", rec.Code, body)
	}

	for _, tc := range []struct {
		name string
		body map[string]any
		code string
	}{
		{"a equals b", map[string]any{
			"broad_dimension_m": 10e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0}, "INVALID_DIMENSION"},
		{"a below b", map[string]any{
			"broad_dimension_m": 5e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 1.0}, "INVALID_DIMENSION"},
		{"eps below vacuum", map[string]any{
			"broad_dimension_m": 20e-3, "narrow_dimension_m": 10e-3,
			"relative_permittivity": 0.9}, "INVALID_PERMITTIVITY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, b := doJSON(t, srv, http.MethodPost, "/api/v1/spectrum", tc.body)
			if r.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d (%v)", r.Code, b)
			}
			if b["error"].(map[string]any)["code"] != tc.code {
				t.Fatalf("want %s, got %v", tc.code, b)
			}
		})
	}

	// Genuinely malformed JSON is rejected before validation.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spectrum",
		bytes.NewBufferString("{not json"))
	req.Header.Set("Content-Type", "application/json")
	recMalformed := httptest.NewRecorder()
	srv.ServeHTTP(recMalformed, req)
	if recMalformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: want 400, got %d", recMalformed.Code)
	}
	var parsed map[string]any
	if err := json.Unmarshal(recMalformed.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if parsed["error"].(map[string]any)["code"] != "MALFORMED_JSON" {
		t.Fatalf("malformed body: want MALFORMED_JSON, got %v", parsed)
	}
}
