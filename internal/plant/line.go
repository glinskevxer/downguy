package plant

import (
	"errors"
	"fmt"
	"math"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadLine covers a line section that does not hang together.
var ErrBadLine = errors.New("plant: bad line section")

// Line is a run of poles and the spans between them, from one dead end to the
// next. The sag of any one span is not decided by that span on its own: a
// suspension insulator lets tension equalise along the whole section, so the
// ruling span of the section is what the tension is worked out at.
type Line struct {
	ID    string
	Poles []PoleID
	Spans []SpanRecord
}

// NewLine builds a line section from an ordered list of spans and derives the
// pole order from them.
func NewLine(id string, spans []SpanRecord) (*Line, error) {
	if len(spans) == 0 {
		return nil, fmt.Errorf("%w: %s has no spans", ErrBadLine, id)
	}
	l := &Line{ID: id, Spans: make([]SpanRecord, len(spans))}
	copy(l.Spans, spans)
	l.Poles = append(l.Poles, spans[0].FromPole, spans[0].ToPole)
	for i := 1; i < len(spans); i++ {
		prev := l.Poles[len(l.Poles)-1]
		if spans[i].FromPole != prev {
			return nil, fmt.Errorf("%w: %s starts at %s but the run had reached %s",
				ErrBadLine, spans[i].ID, spans[i].FromPole, prev)
		}
		l.Poles = append(l.Poles, spans[i].ToPole)
	}
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return l, nil
}

// Validate checks every span and that no pole is used twice.
func (l *Line) Validate() error {
	if l.ID == "" {
		return fmt.Errorf("%w: line has no id", ErrBadLine)
	}
	seen := map[PoleID]bool{}
	for _, p := range l.Poles {
		if seen[p] {
			return fmt.Errorf("%w: %s passes %s twice", ErrBadLine, l.ID, p)
		}
		seen[p] = true
	}
	for _, s := range l.Spans {
		if err := s.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the number of spans in the section.
func (l *Line) Len() int { return len(l.Spans) }

// Total returns the length of the whole section.
func (l *Line) Total() unit.Span {
	var total unit.Span
	for _, s := range l.Spans {
		total += s.Length
	}
	return total
}

// Longest returns the longest span in the section.
func (l *Line) Longest() SpanRecord {
	longest := l.Spans[0]
	for _, s := range l.Spans[1:] {
		if s.Length > longest.Length {
			longest = s
		}
	}
	return longest
}

// RulingSpan returns the equivalent span the section's tension is worked out
// at: the square root of the sum of the cubes over the sum of the lengths. A
// section of one span rules at its own length, and a section of equal spans
// rules at that length too.
func (l *Line) RulingSpan() float64 {
	var cubes, lengths float64
	for _, s := range l.Spans {
		length := s.Length.Float()
		cubes += length * length * length
		lengths += length
	}
	if lengths == 0 {
		return 0
	}
	return math.Sqrt(cubes / lengths)
}

// SpanBetween returns the span joining two adjacent poles and reports whether
// the section contains it.
func (l *Line) SpanBetween(a, b PoleID) (SpanRecord, bool) {
	for _, s := range l.Spans {
		if (s.FromPole == a && s.ToPole == b) || (s.FromPole == b && s.ToPole == a) {
			return s, true
		}
	}
	return SpanRecord{}, false
}

// SpansAt returns the spans that land on a pole. An interior pole has two and
// a dead end has one.
func (l *Line) SpansAt(id PoleID) []SpanRecord {
	var out []SpanRecord
	for _, s := range l.Spans {
		if s.Touches(id) {
			out = append(out, s)
		}
	}
	return out
}

// IsDeadEnd reports whether the pole is at one end of the section.
func (l *Line) IsDeadEnd(id PoleID) bool { return len(l.SpansAt(id)) == 1 }

// UnbalancedAt returns the difference in span length either side of a pole, in
// feet. A large difference is what puts a longitudinal pull on the structure
// and is the usual reason a pole needs a guy.
func (l *Line) UnbalancedAt(id PoleID) int {
	spans := l.SpansAt(id)
	switch len(spans) {
	case 0:
		return 0
	case 1:
		return spans[0].Length.Feet()
	default:
		diff := spans[0].Length.Feet() - spans[1].Length.Feet()
		if diff < 0 {
			return -diff
		}
		return diff
	}
}
