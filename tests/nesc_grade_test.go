package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/nesc"
)

func TestFactorsForEachGrade(t *testing.T) {
	b, err := nesc.FactorsFor(nesc.GradeB)
	if err != nil {
		t.Fatal(err)
	}
	if b.TransverseWind != 2.50 || b.WoodStrength != 0.65 {
		t.Errorf("grade B factors = %+v", b)
	}
	c, err := nesc.FactorsFor(nesc.GradeC)
	if err != nil {
		t.Fatal(err)
	}
	if c.TransverseWind != 2.20 || c.WoodStrength != 0.85 {
		t.Errorf("grade C factors = %+v", c)
	}
	n, err := nesc.FactorsFor(nesc.GradeN)
	if err != nil {
		t.Fatal(err)
	}
	if n.TransverseWind != 1 || n.WoodStrength != 1 || n.Vertical != 1 {
		t.Errorf("grade N should not factor anything, got %+v", n)
	}
	if _, err := nesc.FactorsFor(nesc.Grade("Z")); !errors.Is(err, nesc.ErrUnknownGrade) {
		t.Errorf("unknown grade error = %v", err)
	}
}

func TestGradeBIsStricterThanC(t *testing.T) {
	b, _ := nesc.FactorsFor(nesc.GradeB)
	c, _ := nesc.FactorsFor(nesc.GradeC)
	if b.TransverseWind <= c.TransverseWind {
		t.Error("grade B factors the wind up harder")
	}
	if b.WoodStrength >= c.WoodStrength {
		t.Error("grade B knocks the wood capacity down harder")
	}
	if b.WoodAtReplacement >= c.WoodAtReplacement {
		t.Error("grade B holds the replacement threshold lower")
	}
}

func TestParseGrade(t *testing.T) {
	for _, in := range []string{"B", "b", " B "} {
		got, err := nesc.ParseGrade(in)
		if err != nil || got != nesc.GradeB {
			t.Errorf("ParseGrade(%q) = %q %v", in, got, err)
		}
	}
	if _, err := nesc.ParseGrade("A"); err == nil {
		t.Error("there is no grade A here")
	}
	if nesc.GradeB.String() != "B" {
		t.Errorf("String = %q", nesc.GradeB.String())
	}
}

func TestGradeOrdering(t *testing.T) {
	if !nesc.GradeB.Stricter(nesc.GradeC) || !nesc.GradeC.Stricter(nesc.GradeN) {
		t.Error("B beats C beats N")
	}
	if nesc.GradeC.Stricter(nesc.GradeB) {
		t.Error("C does not beat B")
	}
	if nesc.GradeB.Stricter(nesc.GradeB) {
		t.Error("a grade is not stricter than itself")
	}
	if nesc.Grade("Z").Stricter(nesc.GradeN) {
		t.Error("an unknown grade beats nothing")
	}
	if r, ok := nesc.GradeC.Rank(); !ok || r != 1 {
		t.Errorf("rank of C = %d ok=%v", r, ok)
	}
	if _, ok := nesc.Grade("Z").Rank(); ok {
		t.Error("an unknown grade has no rank")
	}
}

func TestGoverningGrade(t *testing.T) {
	if got := nesc.Governing(nesc.GradeC, nesc.GradeB); got != nesc.GradeB {
		t.Errorf("governing = %s", got)
	}
	if got := nesc.Governing(nesc.GradeB, nesc.GradeN); got != nesc.GradeB {
		t.Errorf("governing = %s", got)
	}
	if got := nesc.Governing(nesc.GradeC, nesc.GradeC); got != nesc.GradeC {
		t.Errorf("governing = %s", got)
	}
}

func TestRequiredGradeByCrossing(t *testing.T) {
	cases := []struct {
		rail, comm, supply bool
		grade              nesc.Grade
		rule               string
	}{
		{true, false, true, nesc.GradeB, "NESC-242C1"},
		{false, true, true, nesc.GradeB, "NESC-242C2"},
		{false, false, true, nesc.GradeC, "NESC-242D"},
		{true, false, false, nesc.GradeC, "NESC-242B"},
		{false, false, false, nesc.GradeN, "NESC-242A"},
		{false, true, false, nesc.GradeN, "NESC-242A"},
	}
	for _, c := range cases {
		g, rule := nesc.RequiredGrade(c.rail, c.comm, c.supply)
		if g != c.grade || rule != c.rule {
			t.Errorf("RequiredGrade(%v,%v,%v) = %s %s, want %s %s",
				c.rail, c.comm, c.supply, g, rule, c.grade, c.rule)
		}
	}
}
