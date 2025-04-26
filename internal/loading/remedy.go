package loading

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
)

// Replacement is the pole that would carry the load, worked out by searching
// the stock list rather than by a rule of thumb. The search matters because
// "go one class up" is wrong about as often as it is right: a taller pole
// carries more wind on itself and can come out worse than the one it replaced.
type Replacement struct {
	Length      int
	Class       plant.Class
	Species     plant.Species
	Utilisation float64
	Found       bool
	Reason      string
}

// stockLengths is what the yard carries, shortest first. A replacement is
// never shorter than the pole it replaces, because the attachments have to go
// back on at their own heights.
var stockLengths = []int{30, 35, 40, 45, 50, 55, 60, 65, 70}

// StockLengths returns the lengths the yard carries.
func StockLengths() []int {
	out := make([]int, len(stockLengths))
	copy(out, stockLengths)
	return out
}

// FindReplacement searches the stock list for the cheapest pole that carries
// the load, where cheapest means the shortest length and then the weakest
// class that passes. The search keeps the species, because a utility does not
// change what it buys to solve one pole.
func FindReplacement(p plant.Pole, wires []Wire, d nesc.District, g nesc.Grade) (Replacement, error) {
	if err := p.Validate(); err != nil {
		return Replacement{}, err
	}
	if _, err := nesc.LoadingFor(d); err != nil {
		return Replacement{}, err
	}
	if _, err := nesc.FactorsFor(g); err != nil {
		return Replacement{}, err
	}
	classes := plant.Classes()
	existingRank, ok := plant.Rank(p.Class)
	if !ok {
		return Replacement{}, fmt.Errorf("%w: %s is class %s", ErrBadLoad, p.ID, p.Class)
	}
	for _, length := range stockLengths {
		if length < p.Length {
			continue
		}
		for i := existingRank; i >= 0; i-- {
			// A replacement is never a weaker class than what is coming out.
			// The arithmetic will happily say a class 5 carries the load, but
			// nobody swaps a class 4 for a class 5 and calls it make ready.
			candidate := p
			candidate.Length = length
			candidate.Class = classes[i]
			candidate.Condition = plant.ConditionNew
			candidate.SetDepth = plant.StandardSetDepth(length)
			if _, err := plant.ButtCircumference(candidate.Class, candidate.Length); err != nil {
				continue
			}
			a, err := Analyse(candidate, wires, d, g)
			if err != nil {
				continue
			}
			if a.Passes() {
				return Replacement{
					Length: length, Class: classes[i], Species: p.Species,
					Utilisation: a.Utilisation, Found: true,
					Reason: fmt.Sprintf("a %d foot class %s carries the load at %d%%",
						length, classes[i], a.UtilisationPercent()),
				}, nil
			}
		}
	}
	return Replacement{
		Found:  false,
		Reason: "nothing in the stock list carries this load, so the span needs redesigning",
	}, nil
}

// SameSize reports whether the replacement is the pole that is already there,
// which means the existing pole would pass if it were sound and the real
// remedy is its condition rather than its size.
func (r Replacement) SameSize(p plant.Pole) bool {
	return r.Found && r.Length == p.Length && r.Class == p.Class
}

// String prints the replacement the way a work order calls for it.
func (r Replacement) String() string {
	if !r.Found {
		return "no stock pole carries this load"
	}
	return fmt.Sprintf("%d/%s %s", r.Length, r.Class, r.Species)
}

// NeedsGuying reports whether a pole that fails its groundline check would
// pass with the unbalanced pull taken off it, which is the test that
// distinguishes a guying job from a pole replacement.
func NeedsGuying(p plant.Pole, wires []Wire, pulls []Pull, d nesc.District, g nesc.Grade) (bool, error) {
	a, err := Analyse(p, wires, d, g)
	if err != nil {
		return false, err
	}
	if a.Passes() {
		return false, nil
	}
	moment := MomentOf(pulls)
	if moment <= 0 {
		return false, nil
	}
	f, err := nesc.FactorsFor(g)
	if err != nil {
		return false, err
	}
	relieved := a.FactoredMoment - moment*f.Longitudinal
	return relieved <= a.FactoredCapacty, nil
}
