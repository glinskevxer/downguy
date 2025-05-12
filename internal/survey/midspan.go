package survey

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Station is a point along a span where the crew took readings, measured in
// feet from the near pole. Mid span readings are taken where the ground is
// tightest, which is not always the middle, so the station is recorded rather
// than assumed.
type Station struct {
	Feet         int
	Cover        plant.Cover
	Measurements []Measurement
	Note         string
}

// SpanSheet is what a crew wrote down across one span.
type SpanSheet struct {
	ID       string
	Span     string
	FromPole plant.PoleID
	ToPole   plant.PoleID
	Taken    calendar.Date
	Crew     string
	AirTemp  unit.Temperature
	Length   unit.Span
	Stations []Station
}

// Validate checks a span sheet and every station on it.
func (s SpanSheet) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: no id", ErrBadSheet)
	}
	if _, err := plant.ParsePoleID(string(s.FromPole)); err != nil {
		return fmt.Errorf("%w: %s starts at %q", ErrBadSheet, s.ID, s.FromPole)
	}
	if _, err := plant.ParsePoleID(string(s.ToPole)); err != nil {
		return fmt.Errorf("%w: %s ends at %q", ErrBadSheet, s.ID, s.ToPole)
	}
	if s.Taken.IsZero() {
		return fmt.Errorf("%w: %s has no date", ErrBadSheet, s.ID)
	}
	if strings.TrimSpace(s.Crew) == "" {
		return fmt.Errorf("%w: %s has no crew", ErrBadSheet, s.ID)
	}
	if !s.Length.Valid() {
		return fmt.Errorf("%w: %s measures %s", ErrBadSheet, s.ID, s.Length)
	}
	if len(s.Stations) == 0 {
		return fmt.Errorf("%w: %s has no stations", ErrBadSheet, s.ID)
	}
	seen := map[int]bool{}
	for _, st := range s.Stations {
		if st.Feet < 0 || st.Feet > s.Length.Feet() {
			return fmt.Errorf("%w: %s has a station at %d feet on a %s span", ErrBadSheet, s.ID, st.Feet, s.Length)
		}
		if seen[st.Feet] {
			return fmt.Errorf("%w: %s stations twice at %d feet", ErrBadSheet, s.ID, st.Feet)
		}
		seen[st.Feet] = true
		if !plant.KnownCover(st.Cover) {
			return fmt.Errorf("%w: %s crosses %q at %d feet", ErrBadSheet, s.ID, st.Cover, st.Feet)
		}
		if len(st.Measurements) == 0 {
			return fmt.Errorf("%w: %s has an empty station at %d feet", ErrBadSheet, s.ID, st.Feet)
		}
		for _, m := range st.Measurements {
			if err := m.Validate(); err != nil {
				return fmt.Errorf("%s at %d feet: %w", s.ID, st.Feet, err)
			}
		}
	}
	return nil
}

// Ordered returns the stations from the near pole outward.
func (s SpanSheet) Ordered() []Station {
	out := make([]Station, len(s.Stations))
	copy(out, s.Stations)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Feet < out[j].Feet })
	return out
}

// At returns the station at a distance and reports whether the sheet carries
// one.
func (s SpanSheet) At(feet int) (Station, bool) {
	for _, st := range s.Stations {
		if st.Feet == feet {
			return st, true
		}
	}
	return Station{}, false
}

// Lowest returns the lowest reading of a subject anywhere in the span, and the
// station it was taken at. It is the number a ground clearance check reads,
// because a wire that clears at the middle can still be low over a driveway
// two thirds of the way along.
func (s SpanSheet) Lowest(subject string) (Measurement, Station, bool) {
	var best Measurement
	var at Station
	found := false
	for _, st := range s.Ordered() {
		for _, m := range st.Measurements {
			if m.Subject != subject {
				continue
			}
			if !found || m.Height < best.Height {
				best, at, found = m, st, true
			}
		}
	}
	return best, at, found
}

// Covers returns every distinct ground cover the span crosses, sorted.
func (s SpanSheet) Covers() []plant.Cover {
	seen := map[plant.Cover]bool{}
	var out []plant.Cover
	for _, st := range s.Stations {
		if seen[st.Cover] {
			continue
		}
		seen[st.Cover] = true
		out = append(out, st.Cover)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Subjects returns every facility read anywhere in the span, sorted.
func (s SpanSheet) Subjects() []string {
	seen := map[string]bool{}
	var out []string
	for _, st := range s.Stations {
		for _, m := range st.Measurements {
			if seen[m.Subject] {
				continue
			}
			seen[m.Subject] = true
			out = append(out, m.Subject)
		}
	}
	sort.Strings(out)
	return out
}

// String prints the span sheet header.
func (s SpanSheet) String() string {
	return fmt.Sprintf("%s %s to %s on %s, %d stations over %s",
		s.ID, s.FromPole, s.ToPole, s.Taken, len(s.Stations), s.Length)
}
