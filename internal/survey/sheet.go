package survey

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadSheet covers every rejected survey sheet.
var ErrBadSheet = errors.New("survey: bad sheet")

// PoleSheet is what one crew wrote down at one pole on one day.
type PoleSheet struct {
	ID           string
	Pole         plant.PoleID
	Taken        calendar.Date
	Crew         string
	Measurements []Measurement
	Condition    plant.Condition
	GroundLine   unit.Height
	Photos       []string
	Notes        string
}

// Validate checks a sheet and every reading on it.
func (s PoleSheet) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: no id", ErrBadSheet)
	}
	if _, err := plant.ParsePoleID(string(s.Pole)); err != nil {
		return fmt.Errorf("%w: %s names pole %q", ErrBadSheet, s.ID, s.Pole)
	}
	if s.Taken.IsZero() {
		return fmt.Errorf("%w: %s has no date", ErrBadSheet, s.ID)
	}
	if strings.TrimSpace(s.Crew) == "" {
		return fmt.Errorf("%w: %s has no crew", ErrBadSheet, s.ID)
	}
	if !plant.KnownCondition(s.Condition) {
		return fmt.Errorf("%w: %s grades the pole %q", ErrBadSheet, s.ID, s.Condition)
	}
	if len(s.Measurements) == 0 {
		return fmt.Errorf("%w: %s carries no readings", ErrBadSheet, s.ID)
	}
	seen := map[string]bool{}
	for _, m := range s.Measurements {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("%s: %w", s.ID, err)
		}
		if seen[m.Subject] {
			return fmt.Errorf("%w: %s reads %s twice", ErrBadSheet, s.ID, m.Subject)
		}
		seen[m.Subject] = true
	}
	return nil
}

// For returns the reading of one subject and reports whether the sheet carries
// it.
func (s PoleSheet) For(subject string) (Measurement, bool) {
	for _, m := range s.Measurements {
		if m.Subject == subject {
			return m, true
		}
	}
	return Measurement{}, false
}

// Subjects returns everything the sheet measured, sorted.
func (s PoleSheet) Subjects() []string {
	out := make([]string, 0, len(s.Measurements))
	for _, m := range s.Measurements {
		out = append(out, m.Subject)
	}
	sort.Strings(out)
	return out
}

// Missing returns the subjects that were asked for and are not on the sheet.
func (s PoleSheet) Missing(wanted []string) []string {
	var out []string
	for _, w := range wanted {
		if _, ok := s.For(w); !ok {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

// Unreliable returns the readings taken by a method that is not good enough to
// decide a clearance on.
func (s PoleSheet) Unreliable() []Measurement {
	var out []Measurement
	for _, m := range s.Measurements {
		if !m.Method.Reliable() {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Subject < out[j].Subject })
	return out
}

// Ordered returns the readings from the highest down, which is the order the
// clearance rules walk a pole in.
func (s PoleSheet) Ordered() []Measurement {
	out := make([]Measurement, len(s.Measurements))
	copy(out, s.Measurements)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Height != out[j].Height {
			return out[i].Height > out[j].Height
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}

// Age returns how many days old the sheet is as of a date. A survey more than
// a season old is not evidence about a pole that has had a storm across it.
func (s PoleSheet) Age(asOf calendar.Date) int {
	if s.Taken.IsZero() || asOf.IsZero() {
		return 0
	}
	return s.Taken.DaysUntil(asOf)
}

// StaleAfterDays is how long a survey stands before it has to be redone.
const StaleAfterDays = 180

// Stale reports whether the sheet is too old to engineer from.
func (s PoleSheet) Stale(asOf calendar.Date) bool { return s.Age(asOf) > StaleAfterDays }

// String prints the sheet header.
func (s PoleSheet) String() string {
	return fmt.Sprintf("%s %s on %s by %s, %d readings",
		s.ID, s.Pole, s.Taken, s.Crew, len(s.Measurements))
}
