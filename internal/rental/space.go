// Package rental works out what an attacher pays every year for the space it
// occupies on a pole.
//
// The rate is not a number somebody picks. It is a formula: the attacher's
// share of the pole times the annual cost of owning a pole. Both halves are
// contested, and the formula exists so that the argument is about the inputs
// rather than about the answer.
package rental

import (
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadSpace is returned when a pole cannot be divided into usable and
// unusable space.
var ErrBadSpace = errors.New("rental: cannot divide the pole")

// The standard space figures, in tenths of a foot. They are presumptions
// rather than measurements: the formula uses them for every pole regardless of
// what any one pole actually looks like, which is what makes a rate schedule
// possible at all.
const (
	// StandardPoleHeight is the presumptive pole, 37.5 feet.
	StandardPoleHeight = unit.Height(375)
	// StandardBurial is the presumptive setting depth, 6 feet.
	StandardBurial = unit.Height(60)
	// StandardGroundClearance is the presumptive clearance kept under the
	// lowest communication attachment, 18 feet.
	StandardGroundClearance = unit.Height(180)
	// StandardSafetyZone is the presumptive separation between supply and
	// communication space, 40 inches.
	StandardSafetyZone = unit.Height(33)
	// StandardOccupied is the presumptive space one attachment occupies, one
	// foot.
	StandardOccupied = unit.Height(10)
)

// Space is how a pole divides for rate making.
type Space struct {
	Height     unit.Height
	Burial     unit.Height
	Clearance  unit.Height
	SafetyZone unit.Height
	AboveGrade unit.Height
	Unusable   unit.Height
	Usable     unit.Height
}

// StandardSpace returns the presumptive division the rate schedule is built
// on.
func StandardSpace() Space {
	s, err := NewSpace(StandardPoleHeight, StandardBurial, StandardGroundClearance, StandardSafetyZone)
	if err != nil {
		panic(err)
	}
	return s
}

// NewSpace divides a pole. Unusable space is everything nobody can attach in:
// the buried part, the ground clearance under the lowest attachment, and the
// safety zone between supply and communication. Usable space is what is left.
func NewSpace(height, burial, clearance, safetyZone unit.Height) (Space, error) {
	switch {
	case height <= 0:
		return Space{}, fmt.Errorf("%w: a pole %s tall", ErrBadSpace, height)
	case burial <= 0:
		return Space{}, fmt.Errorf("%w: buried %s", ErrBadSpace, burial)
	case clearance < 0 || safetyZone < 0:
		return Space{}, fmt.Errorf("%w: negative clearance or safety zone", ErrBadSpace)
	case burial >= height:
		return Space{}, fmt.Errorf("%w: %s of a %s pole is in the ground", ErrBadSpace, burial, height)
	}
	s := Space{
		Height: height, Burial: burial, Clearance: clearance, SafetyZone: safetyZone,
	}
	s.AboveGrade = height.Sub(burial)
	s.Unusable = burial.Add(clearance).Add(safetyZone)
	s.Usable = height.Sub(s.Unusable)
	if s.Usable <= 0 {
		return Space{}, fmt.Errorf("%w: %s of a %s pole leaves no usable space", ErrBadSpace, s.Unusable, height)
	}
	return s, nil
}

// UsableShare returns the fraction of usable space one attachment occupies, in
// basis points.
func (s Space) UsableShare(occupied unit.Height) (unit.BasisPoints, error) {
	if occupied <= 0 {
		return 0, fmt.Errorf("%w: an attachment occupying %s", ErrBadSpace, occupied)
	}
	if occupied > s.Usable {
		return 0, fmt.Errorf("%w: %s occupied of %s usable", ErrBadSpace, occupied, s.Usable)
	}
	return ratio(occupied, s.Usable), nil
}

// UnusableShare returns the fraction of the whole pole that is unusable, in
// basis points. It is the number the two rate formulas disagree about how to
// treat.
func (s Space) UnusableShare() unit.BasisPoints { return ratio(s.Unusable, s.Height) }

// SpaceFactorCable returns the share the cable formula charges for: the space
// occupied over the usable space, with the unusable space ignored.
func (s Space) SpaceFactorCable(occupied unit.Height) (unit.BasisPoints, error) {
	return s.UsableShare(occupied)
}

// SpaceFactorTelecom returns the share the telecom formula charges for: the
// occupied space plus a share of the unusable space divided between the
// attaching entities, all over the height of the pole.
func (s Space) SpaceFactorTelecom(occupied unit.Height, attachers int) (unit.BasisPoints, error) {
	if attachers <= 0 {
		return 0, fmt.Errorf("%w: %d attaching entities", ErrBadSpace, attachers)
	}
	if occupied <= 0 || occupied > s.Usable {
		return 0, fmt.Errorf("%w: %s occupied of %s usable", ErrBadSpace, occupied, s.Usable)
	}
	// (occupied + (2/3) * unusable / attachers) / height. Both sides are
	// multiplied through by three times the attacher count first, because
	// dividing the unusable space on its own truncates twice and lands a tenth
	// of a percent off the filed rate.
	scale := int64(3 * attachers)
	numerator := int64(occupied)*scale + int64(s.Unusable)*2
	denominator := int64(s.Height) * scale
	return unit.BasisPoints(numerator * 10000 / denominator), nil
}

func ratio(part, whole unit.Height) unit.BasisPoints {
	if whole <= 0 {
		return 0
	}
	return unit.BasisPoints(int64(part) * 10000 / int64(whole))
}

// String prints the division the way a rate filing sets it out.
func (s Space) String() string {
	return fmt.Sprintf("%s pole: %s usable, %s unusable", s.Height, s.Usable, s.Unusable)
}
