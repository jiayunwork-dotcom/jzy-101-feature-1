// Package validate performs all input checking for the service. Every
// rule is evaluated before any physics computation starts, and each
// failure carries a machine-readable code plus a specific reason.
package validate

import (
	"fmt"
	"math"
	"regexp"
)

// Error is a validation failure with a stable machine-readable code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// MaxFrequenciesPerBatch bounds a single batch evaluation request.
const MaxFrequenciesPerBatch = 256

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ProfileName checks a profile name: 1-64 chars, alphanumeric plus
// '.', '_' and '-', starting with an alphanumeric character.
func ProfileName(name string) *Error {
	if name == "" {
		return newError("INVALID_NAME", "profile name must not be empty")
	}
	if !namePattern.MatchString(name) {
		return newError("INVALID_NAME",
			"profile name %q is invalid: use 1-64 characters from [A-Za-z0-9._-], starting with a letter or digit", name)
	}
	return nil
}

// CrossSection checks the geometric and dielectric constraints:
// both dimensions finite and positive, broad strictly greater than
// narrow, and relative permittivity finite and >= 1 (never below
// vacuum).
func CrossSection(a, b, epsR float64) *Error {
	if math.IsNaN(a) || math.IsInf(a, 0) || math.IsNaN(b) || math.IsInf(b, 0) {
		return newError("INVALID_DIMENSION", "broad and narrow dimensions must be finite numbers, got a=%v b=%v", a, b)
	}
	if a <= 0 {
		return newError("INVALID_DIMENSION", "broad dimension a must be positive, got %v m", a)
	}
	if b <= 0 {
		return newError("INVALID_DIMENSION", "narrow dimension b must be positive, got %v m", b)
	}
	if a <= b {
		return newError("INVALID_DIMENSION",
			"broad dimension a (%v m) must be strictly greater than narrow dimension b (%v m)", a, b)
	}
	if math.IsNaN(epsR) || math.IsInf(epsR, 0) {
		return newError("INVALID_PERMITTIVITY", "relative permittivity must be a finite number, got %v", epsR)
	}
	if epsR < 1 {
		return newError("INVALID_PERMITTIVITY",
			"relative permittivity must be >= 1 (not below vacuum), got %v", epsR)
	}
	return nil
}

// ModeIndices checks the mode indices: both non-negative, and not both
// zero — the (0,0) combination carries no electromagnetic field and is
// rejected as an illegal mode.
func ModeIndices(m, n int) *Error {
	if m < 0 || n < 0 {
		return newError("INVALID_MODE", "mode indices must be non-negative, got m=%d n=%d", m, n)
	}
	if m == 0 && n == 0 {
		return newError("INVALID_MODE", "mode (0,0) does not exist physically: at least one index must be positive")
	}
	return nil
}

// Frequency checks a single operating frequency: finite and positive.
func Frequency(f float64) *Error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return newError("INVALID_FREQUENCY", "frequency must be a finite number, got %v", f)
	}
	if f <= 0 {
		return newError("INVALID_FREQUENCY", "frequency must be positive, got %v Hz", f)
	}
	return nil
}

// FrequencyBatch checks the size of a batch evaluation request.
func FrequencyBatch(count int) *Error {
	if count == 0 {
		return newError("EMPTY_FREQUENCY_LIST", "at least one frequency is required")
	}
	if count > MaxFrequenciesPerBatch {
		return newError("TOO_MANY_FREQUENCIES",
			"at most %d frequencies per batch, got %d", MaxFrequenciesPerBatch, count)
	}
	return nil
}
