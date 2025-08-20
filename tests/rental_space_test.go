package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestStandardSpaceDivision(t *testing.T) {
	s := rental.StandardSpace()
	if s.Height != rental.StandardPoleHeight {
		t.Errorf("height = %s", s.Height)
	}
	// 6 buried + 18 clearance + 3.3 safety zone = 27.3 unusable
	if s.Unusable.Tenths() != 273 {
		t.Errorf("unusable = %s", s.Unusable)
	}
	if s.Usable.Tenths() != 375-273 {
		t.Errorf("usable = %s", s.Usable)
	}
	if s.AboveGrade.Tenths() != 315 {
		t.Errorf("above grade = %s", s.AboveGrade)
	}
	if s.String() == "" {
		t.Error("String should say something")
	}
}

func TestNewSpaceRejectsAnImpossiblePole(t *testing.T) {
	cases := [][4]int{
		{0, 60, 180, 33},
		{375, 0, 180, 33},
		{375, 400, 180, 33},
		{375, 60, -1, 33},
		{375, 60, 180, -1},
		{100, 60, 180, 33},
	}
	for _, c := range cases {
		_, err := rental.NewSpace(unit.Tenths(c[0]), unit.Tenths(c[1]), unit.Tenths(c[2]), unit.Tenths(c[3]))
		if err == nil {
			t.Errorf("NewSpace%v should have failed", c)
		} else if !errors.Is(err, rental.ErrBadSpace) {
			t.Errorf("NewSpace%v gave %v", c, err)
		}
	}
}

func TestUsableShare(t *testing.T) {
	s := rental.StandardSpace()
	got, err := s.UsableShare(rental.StandardOccupied)
	if err != nil {
		t.Fatal(err)
	}
	// One foot of 10.2 usable feet is 9.80 percent.
	if got != unit.BasisPoints(980) {
		t.Errorf("share = %s", got)
	}
	if _, err := s.UsableShare(0); err == nil {
		t.Error("an attachment occupying nothing should be rejected")
	}
	if _, err := s.UsableShare(unit.Feet(20)); err == nil {
		t.Error("occupying more than the usable space should be rejected")
	}
	double, err := s.UsableShare(unit.Feet(2))
	if err != nil {
		t.Fatal(err)
	}
	if double <= got {
		t.Error("two feet is a larger share than one")
	}
}

func TestUnusableShare(t *testing.T) {
	s := rental.StandardSpace()
	got := s.UnusableShare()
	if got != unit.BasisPoints(273*10000/375) {
		t.Errorf("unusable share = %s", got)
	}
	if got <= unit.Percent(50) {
		t.Error("most of a distribution pole is unusable space")
	}
}

func TestCableFormulaIgnoresUnusableSpace(t *testing.T) {
	s := rental.StandardSpace()
	cable, err := s.SpaceFactorCable(rental.StandardOccupied)
	if err != nil {
		t.Fatal(err)
	}
	usable, err := s.UsableShare(rental.StandardOccupied)
	if err != nil {
		t.Fatal(err)
	}
	if cable != usable {
		t.Errorf("the cable factor is the usable share, got %s against %s", cable, usable)
	}
}

func TestTelecomFormulaChargesAShareOfUnusableSpace(t *testing.T) {
	s := rental.StandardSpace()
	cable, err := s.SpaceFactorCable(rental.StandardOccupied)
	if err != nil {
		t.Fatal(err)
	}
	telecom, err := s.SpaceFactorTelecom(rental.StandardOccupied, 5)
	if err != nil {
		t.Fatal(err)
	}
	// (1.0 + (2/3)(27.3)/5) / 37.5 = 12.37 percent
	if telecom != unit.BasisPoints(1237) {
		t.Errorf("telecom factor = %s", telecom)
	}
	if telecom <= cable {
		t.Errorf("with five attachers the telecom formula charges more: %s against %s", telecom, cable)
	}
	crowded, err := s.SpaceFactorTelecom(rental.StandardOccupied, 10)
	if err != nil {
		t.Fatal(err)
	}
	if crowded >= telecom {
		t.Error("more attachers means a smaller share of the unusable space each")
	}
}

func TestTelecomFormulaRejectsNonsense(t *testing.T) {
	s := rental.StandardSpace()
	if _, err := s.SpaceFactorTelecom(rental.StandardOccupied, 0); err == nil {
		t.Error("a pole with no attachers has no rate")
	}
	if _, err := s.SpaceFactorTelecom(0, 5); err == nil {
		t.Error("an attachment occupying nothing should be rejected")
	}
	if _, err := s.SpaceFactorTelecom(unit.Feet(30), 5); err == nil {
		t.Error("occupying more than the usable space should be rejected")
	}
}

func TestATallerPoleGivesASmallerShare(t *testing.T) {
	short := rental.StandardSpace()
	tall, err := rental.NewSpace(unit.Feet(45), rental.StandardBurial, rental.StandardGroundClearance, rental.StandardSafetyZone)
	if err != nil {
		t.Fatal(err)
	}
	a, err := short.SpaceFactorCable(rental.StandardOccupied)
	if err != nil {
		t.Fatal(err)
	}
	b, err := tall.SpaceFactorCable(rental.StandardOccupied)
	if err != nil {
		t.Fatal(err)
	}
	if b >= a {
		t.Errorf("a foot of a taller pole is a smaller share: %s against %s", b, a)
	}
}
