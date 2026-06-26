package tests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
)

func poleList(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("NW-%06d", 441000+i)
	}
	return out
}

func TestSweepPolesKeepsTheInputOrder(t *testing.T) {
	t.Parallel()
	poles := poleList(50)
	got, err := clearance.SweepPoles(context.Background(), poles, 8,
		func(ctx context.Context, pole string) (clearance.Findings, error) {
			return clearance.Findings{
				onSubject("NESC-232A", nesc.SeverityCorrect, pole, "A3", 148, 155),
			}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(poles) {
		t.Fatalf("results = %d", len(got))
	}
	for i, r := range got {
		if r.Pole != poles[i] {
			t.Fatalf("result %d is %s, want %s", i, r.Pole, poles[i])
		}
		if r.Failed() || len(r.Findings) != 1 {
			t.Errorf("result = %+v", r)
		}
	}
}

func TestSweepPolesActuallyRunsInParallel(t *testing.T) {
	t.Parallel()
	var running, peak atomic.Int64
	var mu sync.Mutex
	poles := poleList(24)
	_, err := clearance.SweepPoles(context.Background(), poles, 4,
		func(ctx context.Context, pole string) (clearance.Findings, error) {
			now := running.Add(1)
			mu.Lock()
			if now > peak.Load() {
				peak.Store(now)
			}
			mu.Unlock()
			time.Sleep(2 * time.Millisecond)
			running.Add(-1)
			return nil, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if peak.Load() < 2 {
		t.Errorf("nothing ever overlapped, peak was %d", peak.Load())
	}
	if peak.Load() > 4 {
		t.Errorf("more workers ran than were asked for, peak was %d", peak.Load())
	}
}

func TestSweepPolesCarriesFailuresPerPole(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("no survey on file")
	poles := poleList(6)
	got, err := clearance.SweepPoles(context.Background(), poles, 3,
		func(ctx context.Context, pole string) (clearance.Findings, error) {
			if pole == poles[2] {
				return nil, sentinel
			}
			return clearance.Findings{
				onSubject("NESC-232A", nesc.SeverityCorrect, pole, "A3", 148, 155),
			}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	found, failed := clearance.Collect(got)
	if len(found) != 5 {
		t.Errorf("findings = %d", len(found))
	}
	if len(failed) != 1 || !errors.Is(failed[0].Err, sentinel) {
		t.Errorf("failed = %v", failed)
	}
	if clearance.Checked(got) != 5 {
		t.Errorf("checked = %d", clearance.Checked(got))
	}
}

func TestCollectSortsWhatItGathered(t *testing.T) {
	t.Parallel()
	results := []clearance.PoleResult{
		{Pole: "NW-441830", Findings: clearance.Findings{
			onSubject("NESC-232A", nesc.SeverityCorrect, "NW-441830", "A3", 148, 155),
		}},
		{Pole: "NW-441829", Findings: clearance.Findings{
			onSubject("NESC-235C1", nesc.SeverityStop, "NW-441829", "A3", 20, 33),
		}},
	}
	found, failed := clearance.Collect(results)
	if len(failed) != 0 {
		t.Errorf("failed = %v", failed)
	}
	if len(found) != 2 || found[0].RuleID != "NESC-235C1" {
		t.Errorf("the stop finding should sort first, got %v", found)
	}
	empty, none := clearance.Collect(nil)
	if empty != nil || none != nil {
		t.Errorf("collect = %v %v", empty, none)
	}
}

func TestSweepPolesStopsWithTheContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	var seen atomic.Int64
	poles := poleList(200)
	_, err := clearance.SweepPoles(ctx, poles, 2,
		func(ctx context.Context, pole string) (clearance.Findings, error) {
			if seen.Add(1) == 5 {
				cancel()
			}
			return nil, nil
		})
	if err == nil {
		t.Fatal("a cancelled sweep should say so")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v", err)
	}
	if seen.Load() >= int64(len(poles)) {
		t.Errorf("it should have stopped early, ran %d of %d", seen.Load(), len(poles))
	}
}

func TestSweepPolesRejectsBadInput(t *testing.T) {
	t.Parallel()
	if _, err := clearance.SweepPoles(context.Background(), poleList(2), 4, nil); err == nil {
		t.Error("no check should be rejected")
	}
	got, err := clearance.SweepPoles(context.Background(), nil, 4,
		func(ctx context.Context, pole string) (clearance.Findings, error) { return nil, nil })
	if err != nil || got != nil {
		t.Errorf("no poles, no results, got %v %v", got, err)
	}
	// A worker count under one falls back to the default rather than hanging.
	got, err = clearance.SweepPoles(context.Background(), poleList(3), 0,
		func(ctx context.Context, pole string) (clearance.Findings, error) { return nil, nil })
	if err != nil || len(got) != 3 {
		t.Errorf("results = %v %v", got, err)
	}
	if clearance.DefaultWorkers < 1 {
		t.Error("the default worker count should be at least one")
	}
}
