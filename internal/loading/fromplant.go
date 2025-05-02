package loading

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/sagten"
)

// Assignment says which catalogued conductor an attachment actually is. The
// plant record carries what a facility is for and who owns it; it does not
// carry the wire, because the same cable company runs three different sizes
// through one town. The survey supplies this.
type Assignment map[string]string

// BuildWires turns a pole assembly and the line section it sits in into the
// wires the loading calculation works on. Only span carrying facilities make
// it through: a transformer is a vertical load, not a wind load on a span, and
// a riser does not leave the pole at all.
func BuildWires(a *plant.Assembly, line *plant.Line, assign Assignment, d nesc.District) ([]Wire, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: no assembly", ErrBadLoad)
	}
	if line == nil {
		return nil, fmt.Errorf("%w: no line section", ErrBadLoad)
	}
	spans := line.SpansAt(a.Pole.ID)
	if len(spans) == 0 {
		return nil, fmt.Errorf("%w: %s is not on line %s", ErrBadLoad, a.Pole.ID, line.ID)
	}
	adjacent := make([]int, 0, len(spans))
	for _, s := range spans {
		adjacent = append(adjacent, s.Length.Feet())
	}
	windSpan := WindSpan(adjacent...)
	ruling := line.RulingSpan()
	longest := 0.0
	for _, s := range spans {
		if f := s.Length.Float(); f > longest {
			longest = f
		}
	}

	var out []Wire
	for _, att := range a.Attachments {
		if !att.Kind.IsSpanCarrying() {
			continue
		}
		code, ok := assign[att.ID]
		if !ok {
			return nil, fmt.Errorf("%w: no conductor assigned to %s", ErrBadLoad, att.ID)
		}
		c, err := sagten.Lookup(code)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", att.ID, err)
		}
		design, err := sagten.DesignCondition(c, d)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", att.ID, err)
		}
		out = append(out, Wire{
			AttachmentID:   att.ID,
			Owner:          att.Owner,
			Conductor:      c,
			WindSpanFeet:   windSpan,
			SpanFeet:       longest,
			RulingSpanFeet: ruling,
			Height:         att.Height,
			Tension:        design.Tension,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %s carries no span facilities", ErrBadLoad, a.Pole.ID)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Height > out[j].Height })
	return out, nil
}

// AnalyseAssembly runs the groundline check straight off the plant records.
func AnalyseAssembly(a *plant.Assembly, line *plant.Line, assign Assignment, d nesc.District, g nesc.Grade) (*Analysis, error) {
	wires, err := BuildWires(a, line, assign, d)
	if err != nil {
		return nil, err
	}
	return Analyse(a.Pole, wires, d, g)
}

// WithAttachment returns the wire list with one more facility on it, which is
// how the before and after of an application is worked out. The new wire takes
// the same wind span as the rest of the pole.
func WithAttachment(wires []Wire, att plant.Attachment, code string, d nesc.District) ([]Wire, error) {
	if !att.Kind.IsSpanCarrying() {
		return nil, fmt.Errorf("%w: %s does not run into the span", ErrBadLoad, att.ID)
	}
	if len(wires) == 0 {
		return nil, fmt.Errorf("%w: no existing wires to take a wind span from", ErrBadLoad)
	}
	c, err := sagten.Lookup(code)
	if err != nil {
		return nil, err
	}
	design, err := sagten.DesignCondition(c, d)
	if err != nil {
		return nil, err
	}
	out := make([]Wire, len(wires), len(wires)+1)
	copy(out, wires)
	out = append(out, Wire{
		AttachmentID:   att.ID,
		Owner:          att.Owner,
		Conductor:      c,
		WindSpanFeet:   wires[0].WindSpanFeet,
		SpanFeet:       wires[0].SpanFeet,
		RulingSpanFeet: wires[0].RulingSpanFeet,
		Height:         att.Height,
		Tension:        design.Tension,
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Height > out[j].Height })
	return out, nil
}

// OwnersOf returns the distinct owners across a wire list, sorted.
func OwnersOf(wires []Wire) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range wires {
		if w.Owner == "" || seen[w.Owner] {
			continue
		}
		seen[w.Owner] = true
		out = append(out, w.Owner)
	}
	sort.Strings(out)
	return out
}
