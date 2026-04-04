package plant

import (
	"fmt"
	"sort"
)

// Poles rot from the groundline up, and how fast depends on what they are and
// what preservative went into them. None of this replaces an inspection: what
// it is for is deciding which poles to inspect next, out of sixty thousand,
// with a budget for four thousand.

// ServiceLife is the age in years at which half the poles of a species are
// expected to have been replaced. It is a planning figure, not a promise about
// any one pole.
var serviceLife = map[Species]int{
	SouthernPine:    45,
	DouglasFir:      50,
	WesternLarch:    50,
	LodgepolePine:   40,
	RedPine:         40,
	JackPine:        38,
	WesternRedCedar: 60,
	PonderosaPine:   42,
}

// ServiceLife returns the planning life for a species and reports whether the
// species is one the table covers.
func ServiceLife(s Species) (int, bool) {
	years, ok := serviceLife[s]
	return years, ok
}

// LifeStage is where a pole is in its life, as a planning judgement rather
// than an inspection finding.
type LifeStage string

// The stages, from a pole nobody has to think about to one that should already
// be on a list.
const (
	StageYoung   LifeStage = "young"
	StageMature  LifeStage = "mature"
	StageAgeing  LifeStage = "ageing"
	StageOverdue LifeStage = "overdue"
	StageUnknown LifeStage = "unknown"
)

var stageOrder = map[LifeStage]int{
	StageYoung:   0,
	StageMature:  1,
	StageAgeing:  2,
	StageOverdue: 3,
	StageUnknown: 4,
}

// KnownStage reports whether the stage is one the planner handles.
func KnownStage(s LifeStage) bool {
	_, ok := stageOrder[s]
	return ok
}

// LaterThan reports whether s is further through a pole's life than other. An
// unknown age sorts last, because a pole nobody has a date for is a records
// problem rather than a wood problem.
func (s LifeStage) LaterThan(other LifeStage) bool {
	return stageOrder[s] > stageOrder[other]
}

// Stages returns the stages in order.
func Stages() []LifeStage {
	out := make([]LifeStage, 0, len(stageOrder))
	for s := range stageOrder {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return stageOrder[out[i]] < stageOrder[out[j]] })
	return out
}

// StageOf returns where a pole is in its life as of a year. A pole is young
// under half its planning life, mature to four fifths of it, ageing to the
// whole of it, and overdue after.
func StageOf(p Pole, asOfYear int) LifeStage {
	if p.InstalledIn == 0 || asOfYear <= 0 {
		return StageUnknown
	}
	life, ok := ServiceLife(p.Species)
	if !ok {
		return StageUnknown
	}
	age := p.Age(asOfYear)
	if age < 0 {
		return StageUnknown
	}
	switch {
	case age*2 < life:
		return StageYoung
	case age*5 < life*4:
		return StageMature
	case age <= life:
		return StageAgeing
	default:
		return StageOverdue
	}
}

// InspectionInterval returns how many years this utility leaves between
// groundline inspections of a pole at a stage.
func InspectionInterval(s LifeStage) (int, bool) {
	switch s {
	case StageYoung:
		return 12, true
	case StageMature:
		return 10, true
	case StageAgeing:
		return 6, true
	case StageOverdue:
		return 4, true
	case StageUnknown:
		return 6, true
	}
	return 0, false
}

// DueForInspection reports whether a pole is due, given the year it was last
// inspected. A pole with no inspection on record is always due, which is the
// answer that gets it looked at.
func DueForInspection(p Pole, lastInspected, asOfYear int) (bool, error) {
	if asOfYear <= 0 {
		return false, fmt.Errorf("%w: %s cannot be judged as of year %d", ErrBadPole, p.ID, asOfYear)
	}
	stage := StageOf(p, asOfYear)
	interval, ok := InspectionInterval(stage)
	if !ok {
		return false, fmt.Errorf("%w: %s is at stage %q", ErrBadPole, p.ID, stage)
	}
	if lastInspected <= 0 {
		return true, nil
	}
	if lastInspected > asOfYear {
		return false, fmt.Errorf("%w: %s was inspected in %d, which is after %d",
			ErrBadPole, p.ID, lastInspected, asOfYear)
	}
	return asOfYear-lastInspected >= interval, nil
}

// ExpectedCondition returns the condition grade a pole of this age would be
// assumed to hold if nobody has been out to it. It is what the loading
// calculation should use in place of a real grade, and it is deliberately
// pessimistic: assuming a fifty year old pole is sound because nobody wrote
// down otherwise is how an application gets approved onto a pole that then
// fails.
func ExpectedCondition(p Pole, asOfYear int) Condition {
	switch StageOf(p, asOfYear) {
	case StageYoung:
		return ConditionSound
	case StageMature:
		return ConditionSound
	case StageAgeing:
		return ConditionServiceabe
	case StageOverdue:
		return ConditionRestorable
	default:
		return ConditionUnknown
	}
}

// GradeIsOptimistic reports whether a pole's recorded condition is better than
// its age suggests, which is the list an inspection programme works from.
func GradeIsOptimistic(p Pole, asOfYear int) bool {
	expected := ExpectedCondition(p, asOfYear)
	// An unknown expected grade is a pole whose age nobody knows, which is a
	// records problem rather than a wood one. Calling it optimistic would put
	// every undated pole on the inspection list ahead of poles with a real age
	// and a real reason to be there.
	if expected == ConditionUnknown || !KnownCondition(p.Condition) {
		return false
	}
	return expected.WorseThan(p.Condition)
}
