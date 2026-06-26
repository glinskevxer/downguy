package clearance

import (
	"context"
	"errors"
	"sort"
	"sync"
)

// A system wide sweep is sixty thousand independent checks against a database
// that reads happily from several connections at once. Running them one at a
// time leaves seven cores idle for the whole of a job that takes minutes.
//
// What this does not do is share anything between the workers. Each one loads
// its own pole, runs its own rules and hands back its own findings; the only
// shared state is the channel and the results slice, and the results slice is
// written under a lock. That is deliberate: a sweep that is fast and subtly
// wrong is worse than a slow one.

// PoleResult is what a sweep found on one pole.
type PoleResult struct {
	Pole     string
	Findings Findings
	Err      error
}

// Failed reports whether the pole could not be checked.
func (r PoleResult) Failed() bool { return r.Err != nil }

// CheckFunc is the work one pole takes. It is a function rather than a context
// so that the caller decides what loading a pole means, which is the part that
// touches the database.
type CheckFunc func(ctx context.Context, pole string) (Findings, error)

// DefaultWorkers is how many poles are checked at once when nothing else says.
const DefaultWorkers = 4

// SweepPoles runs a check across a list of poles and returns one result each,
// in the order the poles were given rather than the order they finished. A
// sweep whose output moves between runs is a sweep nobody can diff.
func SweepPoles(ctx context.Context, poles []string, workers int, check CheckFunc) ([]PoleResult, error) {
	if check == nil {
		return nil, errors.New("clearance: no check to run")
	}
	if workers < 1 {
		workers = DefaultWorkers
	}
	if len(poles) == 0 {
		return nil, nil
	}
	if workers > len(poles) {
		workers = len(poles)
	}

	results := make([]PoleResult, len(poles))
	jobs := make(chan int)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				pole := poles[index]
				select {
				case <-ctx.Done():
					results[index] = PoleResult{Pole: pole, Err: ctx.Err()}
					continue
				default:
				}
				found, err := check(ctx, pole)
				results[index] = PoleResult{Pole: pole, Findings: found, Err: err}
			}
		}()
	}

	for i := range poles {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return results, ctx.Err()
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	return results, nil
}

// Collect flattens a set of results into one sorted finding list and the poles
// that could not be checked.
func Collect(results []PoleResult) (Findings, []PoleResult) {
	var found Findings
	var failed []PoleResult
	for _, r := range results {
		if r.Failed() {
			failed = append(failed, r)
			continue
		}
		found = append(found, r.Findings...)
	}
	found.Sort()
	sort.SliceStable(failed, func(i, j int) bool { return failed[i].Pole < failed[j].Pole })
	return found, failed
}

// Checked returns how many poles were actually checked.
func Checked(results []PoleResult) int {
	count := 0
	for _, r := range results {
		if !r.Failed() {
			count++
		}
	}
	return count
}
