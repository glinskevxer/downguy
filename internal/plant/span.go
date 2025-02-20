package plant

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadSpan covers every rejected span record.
var ErrBadSpan = errors.New("plant: bad span record")

// Cover is what the ground under a span is used for. It is the single input
// that decides the required clearance over the ground, so it is a field on the
// span rather than something inferred later.
type Cover string

// The ground covers the clearance table distinguishes.
const (
	CoverRoad        Cover = "road"
	CoverDriveway    Cover = "driveway"
	CoverAlley       Cover = "alley"
	CoverParking     Cover = "parking"
	CoverPedestrian  Cover = "pedestrian"
	CoverField       Cover = "field"
	CoverCultivated  Cover = "cultivated"
	CoverWaterNoSail Cover = "water_no_sail"
	CoverRailroad    Cover = "railroad"
	CoverBuilding    Cover = "building"
)

var coverNames = map[Cover]string{
	CoverRoad:        "public street or road",
	CoverDriveway:    "residential driveway",
	CoverAlley:       "alley",
	CoverParking:     "parking area subject to truck traffic",
	CoverPedestrian:  "walkway restricted to pedestrians",
	CoverField:       "open field not subject to truck traffic",
	CoverCultivated:  "cultivated or grazing land",
	CoverWaterNoSail: "water not suitable for sailboating",
	CoverRailroad:    "railroad track",
	CoverBuilding:    "over a building or structure",
}

// KnownCover reports whether the cover is one the clearance table lists.
func KnownCover(c Cover) bool {
	_, ok := coverNames[c]
	return ok
}

// Name returns the long form the clearance table uses.
func (c Cover) Name() string {
	if n, ok := coverNames[c]; ok {
		return n
	}
	return "unlisted ground cover " + string(c)
}

// ParseCover reads a cover written with a space, a dash or an underscore.
func ParseCover(text string) (Cover, error) {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.ReplaceAll(t, " ", "_")
	t = strings.ReplaceAll(t, "-", "_")
	if KnownCover(Cover(t)) {
		return Cover(t), nil
	}
	return "", fmt.Errorf("%w: unknown ground cover %q", ErrBadSpan, text)
}

// TruckTraffic reports whether the cover is one where a truck can get under
// the wire. It splits the clearance table in half.
func (c Cover) TruckTraffic() bool {
	switch c {
	case CoverRoad, CoverDriveway, CoverAlley, CoverParking, CoverRailroad:
		return true
	}
	return false
}

// SpanRecord is the wire run between two poles. Ground elevation is carried as
// the drop from the near pole to the far pole, because a span crossing a ditch
// hangs lower over the ditch than the two pole heights alone would say.
type SpanRecord struct {
	ID       string
	FromPole PoleID
	ToPole   PoleID
	Length   unit.Span
	Cover    Cover
	Drop     unit.Height
	Dead     bool
}

// Validate checks a span record.
func (s SpanRecord) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("%w: no id", ErrBadSpan)
	}
	if _, err := ParsePoleID(string(s.FromPole)); err != nil {
		return fmt.Errorf("%w: %s starts at %q", ErrBadSpan, s.ID, s.FromPole)
	}
	if _, err := ParsePoleID(string(s.ToPole)); err != nil {
		return fmt.Errorf("%w: %s ends at %q", ErrBadSpan, s.ID, s.ToPole)
	}
	if s.FromPole == s.ToPole {
		return fmt.Errorf("%w: %s runs from %s back to itself", ErrBadSpan, s.ID, s.FromPole)
	}
	if !s.Length.Valid() {
		return fmt.Errorf("%w: %s is %s", ErrBadSpan, s.ID, s.Length)
	}
	if !KnownCover(s.Cover) {
		return fmt.Errorf("%w: %s crosses %q", ErrBadSpan, s.ID, s.Cover)
	}
	if s.Drop.Abs() > unit.Feet(s.Length.Feet()) {
		return fmt.Errorf("%w: %s drops %s over %s", ErrBadSpan, s.ID, s.Drop, s.Length)
	}
	return nil
}

// Touches reports whether the span lands on the pole at either end.
func (s SpanRecord) Touches(id PoleID) bool { return s.FromPole == id || s.ToPole == id }

// Other returns the pole at the far end from the one given, and reports
// whether the span touches it at all.
func (s SpanRecord) Other(id PoleID) (PoleID, bool) {
	switch id {
	case s.FromPole:
		return s.ToPole, true
	case s.ToPole:
		return s.FromPole, true
	}
	return "", false
}

// Level reports whether the two ends are within a foot of each other, which is
// when the simple level span sag formula applies without correction.
func (s SpanRecord) Level() bool { return s.Drop.Abs() <= unit.Feet(1) }

// String prints the span the way a line sheet names it.
func (s SpanRecord) String() string {
	return fmt.Sprintf("%s %s to %s, %s over %s", s.ID, s.FromPole, s.ToPole, s.Length, s.Cover)
}
