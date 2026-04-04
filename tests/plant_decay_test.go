package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
)

func agedPole(species plant.Species, installed int) plant.Pole {
	p := samplePole()
	p.Species = species
	p.InstalledIn = installed
	return p
}

func TestServiceLifeCoversEverySpecies(t *testing.T) {
	t.Parallel()
	for _, species := range []plant.Species{
		plant.SouthernPine, plant.DouglasFir, plant.WesternLarch, plant.LodgepolePine,
		plant.RedPine, plant.JackPine, plant.WesternRedCedar, plant.PonderosaPine,
	} {
		years, ok := plant.ServiceLife(species)
		if !ok {
			t.Errorf("%s has no planning life", species)
			continue
		}
		if years < 30 || years > 80 {
			t.Errorf("%s has a planning life of %d years", species, years)
		}
	}
	if _, ok := plant.ServiceLife(plant.Species("BAL")); ok {
		t.Error("an untabulated species has no planning life")
	}
	cedar, _ := plant.ServiceLife(plant.WesternRedCedar)
	jack, _ := plant.ServiceLife(plant.JackPine)
	if cedar <= jack {
		t.Error("cedar outlasts jack pine")
	}
}

func TestStagesAreOrdered(t *testing.T) {
	t.Parallel()
	stages := plant.Stages()
	if len(stages) != 5 || stages[0] != plant.StageYoung || stages[4] != plant.StageUnknown {
		t.Fatalf("stages = %v", stages)
	}
	for i := 1; i < len(stages); i++ {
		if !stages[i].LaterThan(stages[i-1]) {
			t.Errorf("%s should sort after %s", stages[i], stages[i-1])
		}
	}
	if plant.StageYoung.LaterThan(plant.StageOverdue) {
		t.Error("young does not sort after overdue")
	}
	if !plant.KnownStage(plant.StageAgeing) || plant.KnownStage(plant.LifeStage("elderly")) {
		t.Error("known stage check")
	}
}

func TestStageOfAPole(t *testing.T) {
	t.Parallel()
	// Southern pine plans on forty five years.
	cases := []struct {
		installed int
		want      plant.LifeStage
	}{
		{2020, plant.StageYoung},
		{2004, plant.StageYoung},
		{2003, plant.StageMature},
		{1991, plant.StageMature},
		{1990, plant.StageAgeing},
		{1981, plant.StageAgeing},
		{1980, plant.StageOverdue},
		{1955, plant.StageOverdue},
	}
	for _, c := range cases {
		got := plant.StageOf(agedPole(plant.SouthernPine, c.installed), 2026)
		if got != c.want {
			age := 2026 - c.installed
			t.Errorf("a %d year old southern pine is %q, want %q", age, got, c.want)
		}
	}
}

func TestStageOfWhatCannotBeJudged(t *testing.T) {
	t.Parallel()
	undated := agedPole(plant.SouthernPine, 0)
	if got := plant.StageOf(undated, 2026); got != plant.StageUnknown {
		t.Errorf("a pole with no install year = %q", got)
	}
	untabulated := agedPole(plant.Species("BAL"), 1990)
	if got := plant.StageOf(untabulated, 2026); got != plant.StageUnknown {
		t.Errorf("an untabulated species = %q", got)
	}
	if got := plant.StageOf(agedPole(plant.SouthernPine, 1990), 0); got != plant.StageUnknown {
		t.Errorf("no reference year = %q", got)
	}
	future := agedPole(plant.SouthernPine, 2030)
	if got := plant.StageOf(future, 2026); got != plant.StageUnknown {
		t.Errorf("a pole set in the future = %q", got)
	}
}

func TestInspectionIntervalShortensWithAge(t *testing.T) {
	t.Parallel()
	previous := 99
	for _, stage := range []plant.LifeStage{plant.StageYoung, plant.StageMature, plant.StageAgeing, plant.StageOverdue} {
		years, ok := plant.InspectionInterval(stage)
		if !ok {
			t.Errorf("%s has no interval", stage)
			continue
		}
		if years >= previous {
			t.Errorf("%s at %d years is not shorter than the %d before it", stage, years, previous)
		}
		previous = years
	}
	if _, ok := plant.InspectionInterval(plant.StageUnknown); !ok {
		t.Error("an unknown stage still gets inspected")
	}
	if _, ok := plant.InspectionInterval(plant.LifeStage("elderly")); ok {
		t.Error("an unknown stage has no interval")
	}
}

func TestDueForInspection(t *testing.T) {
	t.Parallel()
	young := agedPole(plant.SouthernPine, 2020)
	due, err := plant.DueForInspection(young, 2020, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if due {
		t.Error("six years into a twelve year cycle is not due")
	}
	due, err = plant.DueForInspection(young, 2014, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if !due {
		t.Error("twelve years into a twelve year cycle is due")
	}

	old := agedPole(plant.SouthernPine, 1970)
	due, err = plant.DueForInspection(old, 2022, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if !due {
		t.Error("an overdue pole is on a four year cycle")
	}

	never, err := plant.DueForInspection(young, 0, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if !never {
		t.Error("a pole nobody has inspected is always due")
	}

	if _, err := plant.DueForInspection(young, 2020, 0); err == nil {
		t.Error("no reference year should be rejected")
	}
	if _, err := plant.DueForInspection(young, 2030, 2026); err == nil {
		t.Error("an inspection in the future should be rejected")
	}
}

func TestExpectedConditionWorsensWithAge(t *testing.T) {
	t.Parallel()
	cases := map[int]plant.Condition{
		2020: plant.ConditionSound,
		2003: plant.ConditionSound,
		1985: plant.ConditionServiceabe,
		1970: plant.ConditionRestorable,
	}
	for installed, want := range cases {
		got := plant.ExpectedCondition(agedPole(plant.SouthernPine, installed), 2026)
		if got != want {
			t.Errorf("a pole from %d is expected %q, want %q", installed, got, want)
		}
	}
	if got := plant.ExpectedCondition(agedPole(plant.SouthernPine, 0), 2026); got != plant.ConditionUnknown {
		t.Errorf("an undated pole = %q", got)
	}
}

func TestGradeIsOptimistic(t *testing.T) {
	t.Parallel()
	old := agedPole(plant.SouthernPine, 1970)
	old.Condition = plant.ConditionSound
	if !plant.GradeIsOptimistic(old, 2026) {
		t.Error("a fifty six year old pole graded sound is the list an inspection programme works from")
	}
	old.Condition = plant.ConditionRestorable
	if plant.GradeIsOptimistic(old, 2026) {
		t.Error("a grade that matches the age is not optimistic")
	}
	old.Condition = plant.ConditionReject
	if plant.GradeIsOptimistic(old, 2026) {
		t.Error("a worse grade than the age suggests is not optimistic")
	}
	young := agedPole(plant.SouthernPine, 2020)
	young.Condition = plant.ConditionSound
	if plant.GradeIsOptimistic(young, 2026) {
		t.Error("a young pole graded sound is what it should be")
	}
	undated := agedPole(plant.SouthernPine, 0)
	undated.Condition = plant.ConditionSound
	if plant.GradeIsOptimistic(undated, 2026) {
		t.Error("with no expected grade there is nothing to compare against")
	}
	unknownGrade := old
	unknownGrade.Condition = plant.Condition("mint")
	if plant.GradeIsOptimistic(unknownGrade, 2026) {
		t.Error("an ungraded pole is not optimistic, it is unrecorded")
	}
}
