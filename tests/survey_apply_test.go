package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestApplyUsesTheMeasuredHeights(t *testing.T) {
	s := sampleSheet()
	s.Measurements[0].Height = unit.Tenths(186)
	got, err := survey.Apply(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	att, ok := got.Assembly.Find("A3")
	if !ok {
		t.Fatal("A3 should be on the surveyed assembly")
	}
	if att.Height.Tenths() != 186 {
		t.Errorf("A3 sits at %s on the surveyed assembly", att.Height)
	}
	record, ok := sampleAssembly().Find("A3")
	if !ok || record.Height.Tenths() != 200 {
		t.Error("the record itself should not have moved")
	}
	if len(got.Discrepancy) != 1 {
		t.Errorf("discrepancies = %v", got.Discrepancy)
	}
}

func TestApplyKeepsTheHeightOrder(t *testing.T) {
	s := sampleSheet()
	s.Measurements[1].Height = unit.Tenths(150)
	got, err := survey.Apply(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	atts := got.Assembly.Attachments
	for i := 1; i < len(atts); i++ {
		if atts[i].Height > atts[i-1].Height {
			t.Fatalf("the surveyed assembly is out of order: %v", atts)
		}
	}
	if atts[len(atts)-1].ID != "A1" {
		t.Errorf("the primary was measured lowest, so it should sort last, got %s", atts[len(atts)-1].ID)
	}
}

func TestApplyAddsAnUnrecordedFacility(t *testing.T) {
	s := sampleSheet()
	s.Measurements = append(s.Measurements, measure("A9", 178, survey.MethodStick))
	got, err := survey.Apply(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	att, ok := got.Assembly.Find("A9")
	if !ok {
		t.Fatal("A9 should have been added")
	}
	if att.Authorized {
		t.Error("an unrecorded facility is not authorised")
	}
	if att.Owner != survey.UnknownOwner {
		t.Errorf("owner = %q", att.Owner)
	}
	if att.Kind != plant.KindCommCable {
		t.Errorf("kind = %q", att.Kind)
	}
	if len(got.AddedSubject) != 1 || got.AddedSubject[0] != "A9" {
		t.Errorf("added = %v", got.AddedSubject)
	}
	unauth := got.Unauthorized()
	if len(unauth) != 1 || unauth[0].ID != "A9" {
		t.Errorf("unauthorized = %v", unauth)
	}
}

func TestApplyDropsWhatTheCrewCouldNotFind(t *testing.T) {
	s := sampleSheet()
	s.Measurements = s.Measurements[:3]
	got, err := survey.Apply(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Assembly.Find("A2"); ok {
		t.Error("a facility the crew could not find should not be on the surveyed assembly")
	}
	removed := got.Removed()
	if len(removed) != 1 || removed[0] != "A2" {
		t.Errorf("removed = %v", removed)
	}
	if len(got.Assembly.Attachments) != 3 {
		t.Errorf("surveyed attachments = %d", len(got.Assembly.Attachments))
	}
}

func TestApplyCarriesTheFieldConditionGrade(t *testing.T) {
	s := sampleSheet()
	s.Condition = plant.ConditionRestorable
	got, err := survey.Apply(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if got.Assembly.Pole.Condition != plant.ConditionRestorable {
		t.Errorf("the surveyed pole is graded %s", got.Assembly.Pole.Condition)
	}
	if sampleAssembly().Pole.Condition != plant.ConditionSound {
		t.Error("the record itself should not have been regraded")
	}
}

func TestApplyRecordsHowEachHeightWasTaken(t *testing.T) {
	got, err := survey.Apply(sampleSheet(), sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.MethodFor("A1")
	if !ok || m != survey.MethodLaser {
		t.Errorf("A1 method = %q ok=%v", m, ok)
	}
	if _, ok := got.MethodFor("A9"); ok {
		t.Error("A9 was never measured")
	}
	if !got.AllReliable() {
		t.Error("a sheet of stick and laser readings is all reliable")
	}
	s := sampleSheet()
	s.Measurements[2].Method = survey.MethodEstimated
	guessy, err := survey.Apply(s, sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	if guessy.AllReliable() {
		t.Error("one estimated height makes the sheet unreliable")
	}
}

func TestApplySummary(t *testing.T) {
	got, err := survey.Apply(sampleSheet(), sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	want := "NW-441829: 4 facilities surveyed, 0 added, 0 gone, 0 discrepancies"
	if got.Summary() != want {
		t.Errorf("Summary = %q, want %q", got.Summary(), want)
	}
}

func TestApplyRejectsAMismatch(t *testing.T) {
	s := sampleSheet()
	s.Pole = "NW-441830"
	if _, err := survey.Apply(s, sampleAssembly()); err == nil {
		t.Error("a survey of another pole should be rejected")
	}
	if _, err := survey.Apply(sampleSheet(), nil); err == nil {
		t.Error("no assembly should be rejected")
	}
}
