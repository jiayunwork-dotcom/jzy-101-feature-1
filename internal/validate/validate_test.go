package validate

import "testing"

func TestCrossSectionRules(t *testing.T) {
	cases := []struct {
		name      string
		a, b, eps float64
		wantCode  string // empty means valid
	}{
		{"valid", 22.86e-3, 10.16e-3, 1.0, ""},
		{"valid dielectric", 22.86e-3, 10.16e-3, 2.55, ""},
		{"zero broad", 0, 10e-3, 1, "INVALID_DIMENSION"},
		{"negative broad", -1, 10e-3, 1, "INVALID_DIMENSION"},
		{"zero narrow", 20e-3, 0, 1, "INVALID_DIMENSION"},
		{"negative narrow", 20e-3, -1, 1, "INVALID_DIMENSION"},
		{"broad equals narrow", 10e-3, 10e-3, 1, "INVALID_DIMENSION"},
		{"broad below narrow", 5e-3, 10e-3, 1, "INVALID_DIMENSION"},
		{"permittivity below vacuum", 20e-3, 10e-3, 0.5, "INVALID_PERMITTIVITY"},
		{"permittivity zero", 20e-3, 10e-3, 0, "INVALID_PERMITTIVITY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CrossSection(tc.a, tc.b, tc.eps)
			if tc.wantCode == "" && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("expected %s, got nil", tc.wantCode)
				}
				if err.Code != tc.wantCode {
					t.Fatalf("expected code %s, got %s", tc.wantCode, err.Code)
				}
			}
		})
	}
}

func TestModeIndicesRules(t *testing.T) {
	if err := ModeIndices(0, 0); err == nil || err.Code != "INVALID_MODE" {
		t.Fatalf("mode (0,0) must be rejected, got %v", err)
	}
	if err := ModeIndices(-1, 0); err == nil || err.Code != "INVALID_MODE" {
		t.Fatalf("negative index must be rejected, got %v", err)
	}
	for _, mn := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {4, 3}} {
		if err := ModeIndices(mn[0], mn[1]); err != nil {
			t.Fatalf("mode %v must be accepted, got %v", mn, err)
		}
	}
}

func TestFrequencyRules(t *testing.T) {
	if err := Frequency(10e9); err != nil {
		t.Fatalf("positive frequency must be accepted, got %v", err)
	}
	for _, f := range []float64{0, -1, -1e9} {
		if err := Frequency(f); err == nil || err.Code != "INVALID_FREQUENCY" {
			t.Fatalf("frequency %v must be rejected, got %v", f, err)
		}
	}
}

func TestProfileNameRules(t *testing.T) {
	if err := ProfileName("WR-90"); err != nil {
		t.Fatalf("WR-90 must be accepted, got %v", err)
	}
	if err := ProfileName(""); err == nil || err.Code != "INVALID_NAME" {
		t.Fatalf("empty name must be rejected, got %v", err)
	}
	if err := ProfileName("bad name!"); err == nil || err.Code != "INVALID_NAME" {
		t.Fatalf("name with spaces must be rejected, got %v", err)
	}
}

func TestFrequencyBatchRules(t *testing.T) {
	if err := FrequencyBatch(0); err == nil || err.Code != "EMPTY_FREQUENCY_LIST" {
		t.Fatalf("empty batch must be rejected, got %v", err)
	}
	if err := FrequencyBatch(MaxFrequenciesPerBatch + 1); err == nil || err.Code != "TOO_MANY_FREQUENCIES" {
		t.Fatalf("oversized batch must be rejected, got %v", err)
	}
	if err := FrequencyBatch(3); err != nil {
		t.Fatalf("small batch must be accepted, got %v", err)
	}
}
