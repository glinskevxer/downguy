package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestReconcileFindsNothingWhenTheyAgree(t *testing.T) {
	got, err := survey.Reconcile(sampleSheet(), sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a matching survey should be clean, got %v", got)
	}
}

func TestReconcileFindsAHeightGap(t *testing.T) {
	s := sampleSheet()
	s.Measurements[0].Height = unit.Tenths(186)
	got, err := survey.Reconcile(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	gaps := survey.OfKind(got, survey.DiscrepancyHeight)
	if len(gaps) != 1 {
		t.Fatalf("gaps = %v", got)
	}
	if gaps[0].Subject != "A3" || gaps[0].Record.Tenths() != 200 || gaps[0].Measured.Tenths() != 186 {
		t.Errorf("gap = %+v", gaps[0])
	}
	worst, ok := survey.WorstHeightGap(got)
	if !ok || worst.Subject != "A3" {
		t.Errorf("worst = %+v ok=%v", worst, ok)
	}
	if worst.String() == "" {
		t.Error("String should say something")
	}
}

func TestReconcileIgnoresSmallDisagreements(t *testing.T) {
	s := sampleSheet()
	s.Measurements[0].Height = unit.Tenths(205)
	got, err := survey.Reconcile(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("half a foot is inside tolerance, got %v", got)
	}
	s.Measurements[0].Height = unit.Tenths(206)
	got, err = survey.Reconcile(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("six tenths is outside it, got %v", got)
	}
}

func TestReconcileFindsAnUnrecordedAttachment(t *testing.T) {
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A9", 178, survey.MethodStick))
	got, err := survey.Reconcile(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	names := survey.Unrecorded(got)
	if len(names) != 1 || names[0] != "A9" {
		t.Errorf("unrecorded = %v", names)
	}
	if got[len(got)-1].Measured.Tenths() != 178 {
		t.Errorf("the unrecorded finding should carry the measured height, got %+v", got)
	}
}

func TestReconcileFindsAMissingAttachment(t *testing.T) {
	s := sampleSheet()
	s.Measurements = s.Measurements[:3]
	got, err := survey.Reconcile(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	missing := survey.OfKind(got, survey.DiscrepancyMissing)
	if len(missing) != 1 || missing[0].Subject != "A2" {
		t.Errorf("missing = %v", got)
	}
	if missing[0].Record.Tenths() != 285 {
		t.Errorf("the missing finding should carry the recorded height, got %+v", missing[0])
	}
}

func TestReconcileFindsAConditionRegrade(t *testing.T) {
	s := sampleSheet()
	s.Condition = plant.ConditionRestorable
	got, err := survey.Reconcile(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	regrades := survey.OfKind(got, survey.DiscrepancyCondition)
	if len(regrades) != 1 {
		t.Fatalf("regrades = %v", got)
	}
	if regrades[0].Subject != "NW-441829" {
		t.Errorf("regrade subject = %q", regrades[0].Subject)
	}
}

func TestReconcileSortsBySubject(t *testing.T) {
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A9", 178, survey.MethodStick))
	s.Measurements = append(s.Measurements, measure("A0", 340, survey.MethodStick))
	s.Measurements[0].Height = unit.Tenths(160)
	got, err := survey.Reconcile(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 3 {
		t.Fatalf("findings = %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Subject < got[i-1].Subject {
			t.Fatalf("findings are not sorted: %v", got)
		}
	}
}

func TestReconcileRejectsAMismatch(t *testing.T) {
	s := sampleSheet()
	s.Pole = "NW-441830"
	if _, err := survey.Reconcile(s, sampleAssembly()); err == nil {
		t.Error("a survey of another pole should be rejected")
	}
	if _, err := survey.Reconcile(sampleSheet(), nil); err == nil {
		t.Error("no assembly should be rejected")
	}
	bad := sampleSheet()
	bad.Crew = ""
	if _, err := survey.Reconcile(bad, sampleAssembly()); err == nil {
		t.Error("an invalid sheet should be rejected")
	}
}

func TestWorstHeightGapOnACleanSurvey(t *testing.T) {
	got, err := survey.Reconcile(sampleSheet(), sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := survey.WorstHeightGap(got); ok {
		t.Error("a clean survey has no worst gap")
	}
	if got := survey.Unrecorded(nil); got != nil {
		t.Errorf("no findings, no unrecorded, got %v", got)
	}
}
