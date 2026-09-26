// Package batch schedules the evaluation of many frequency points
// against one cross-section and mode. Each point is computed
// independently: a bad frequency fails its own slot only and never
// affects the other points of the batch.
package batch

import (
	"sync"

	"waveguide/internal/physics"
	"waveguide/internal/validate"
)

// DefaultMaxWorkers caps the number of points evaluated concurrently.
const DefaultMaxWorkers = 8

// Outcome is the per-frequency result of a batch evaluation: exactly
// one of Result / Err is set.
type Outcome struct {
	Result *physics.PointResult
	Err    *validate.Error
}

// Evaluate computes every frequency in freqs against the same
// cross-section and mode, preserving input order in the returned slice.
// Work is spread over a bounded pool of goroutines; all inputs are
// passed by value and each outcome is written to its own slot, so
// concurrent evaluations share no mutable state.
func Evaluate(cs physics.CrossSection, mode physics.Mode, freqs []float64, maxWorkers int) []Outcome {
	out := make([]Outcome, len(freqs))
	if len(freqs) == 0 {
		return out
	}
	if maxWorkers < 1 {
		maxWorkers = DefaultMaxWorkers
	}
	if maxWorkers > len(freqs) {
		maxWorkers = len(freqs)
	}
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	for i, f := range freqs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, f float64) {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = evaluateOne(cs, mode, f)
		}(i, f)
	}
	wg.Wait()
	return out
}

// evaluateOne validates a single frequency and, if valid, runs it
// through the shared physics core.
func evaluateOne(cs physics.CrossSection, mode physics.Mode, f float64) Outcome {
	if verr := validate.Frequency(f); verr != nil {
		return Outcome{Err: verr}
	}
	res := physics.EvaluatePoint(cs, mode, f)
	return Outcome{Result: &res}
}
