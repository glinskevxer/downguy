package plant

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadAnchor covers every rejected guy or anchor record.
var ErrBadAnchor = errors.New("plant: bad anchor record")

// guyStrength is the rated breaking strength in pounds of the strand sizes
// this utility stocks.
var guyStrength = map[string]int{
	"3/16 EHS": 3990,
	"1/4 EHS":  6650,
	"5/16 EHS": 11200,
	"3/8 EHS":  15400,
	"7/16 EHS": 20800,
	"1/2 EHS":  26900,
	"1/4 CS":   4250,
	"3/8 CS":   10800,
}

// GuyStrength returns the rated breaking strength of a strand size and reports
// whether the size is stocked.
func GuyStrength(size string) (int, bool) {
	v, ok := guyStrength[strings.ToUpper(strings.TrimSpace(size))]
	return v, ok
}

// GuySizes returns the stocked strand sizes.
func GuySizes() []string {
	out := make([]string, 0, len(guyStrength))
	for k := range guyStrength {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Guy is one strand from the pole down to an anchor.
type Guy struct {
	ID     string
	Height unit.Height
	Size   string
}

// Strength returns the rated breaking strength of the strand.
func (g Guy) Strength() (int, bool) { return GuyStrength(g.Size) }

// Anchor is a rod in the ground with the guys that land on it. The lead is the
// horizontal distance from the pole to the anchor and the bearing is the
// compass direction from the pole, in whole degrees.
type Anchor struct {
	ID      string
	Pole    PoleID
	Lead    int
	Bearing int
	Guys    []Guy
}

// Validate checks an anchor and the guys on it.
func (a Anchor) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Errorf("%w: no id", ErrBadAnchor)
	}
	if _, err := ParsePoleID(string(a.Pole)); err != nil {
		return fmt.Errorf("%w: %s names pole %q", ErrBadAnchor, a.ID, a.Pole)
	}
	if a.Lead < 3 || a.Lead > 60 {
		return fmt.Errorf("%w: %s has a %d foot lead", ErrBadAnchor, a.ID, a.Lead)
	}
	if a.Bearing < 0 || a.Bearing > 359 {
		return fmt.Errorf("%w: %s bears %d degrees", ErrBadAnchor, a.ID, a.Bearing)
	}
	if len(a.Guys) == 0 {
		return fmt.Errorf("%w: %s carries no guys", ErrBadAnchor, a.ID)
	}
	seen := map[string]bool{}
	for _, g := range a.Guys {
		if strings.TrimSpace(g.ID) == "" {
			return fmt.Errorf("%w: %s has a guy with no id", ErrBadAnchor, a.ID)
		}
		if seen[g.ID] {
			return fmt.Errorf("%w: guy %s appears twice on %s", ErrBadAnchor, g.ID, a.ID)
		}
		seen[g.ID] = true
		if g.Height <= 0 {
			return fmt.Errorf("%w: guy %s lands at %s", ErrBadAnchor, g.ID, g.Height)
		}
		if _, ok := g.Strength(); !ok {
			return fmt.Errorf("%w: guy %s is %q, which is not stocked", ErrBadAnchor, g.ID, g.Size)
		}
	}
	return nil
}

// LeadAngle returns the angle in radians between a guy and the pole, measured
// from the pole. A short lead on a tall attachment gives a shallow angle and a
// guy that pulls mostly downward instead of sideways.
func (a Anchor) LeadAngle(g Guy) float64 {
	return math.Atan2(float64(a.Lead), g.Height.Float())
}

// HoldingCapacity returns the horizontal pull the anchor can resist, in
// pounds, as the sum over its guys of the strand strength times the sine of
// the lead angle. It is the number the loading package subtracts from an
// unbalanced pull.
func (a Anchor) HoldingCapacity() (float64, error) {
	if err := a.Validate(); err != nil {
		return 0, err
	}
	total := 0.0
	for _, g := range a.Guys {
		strength, _ := g.Strength()
		total += float64(strength) * math.Sin(a.LeadAngle(g))
	}
	return total, nil
}

// LeadRatio returns the lead divided by the attachment height of the highest
// guy. Standards call for at least one to one, and anything under half is
// treated as ineffective.
func (a Anchor) LeadRatio() float64 {
	highest := unit.Height(0)
	for _, g := range a.Guys {
		if g.Height > highest {
			highest = g.Height
		}
	}
	if highest <= 0 {
		return 0
	}
	return float64(a.Lead) / highest.Float()
}
