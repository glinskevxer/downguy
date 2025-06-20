package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func sampleProposal() clearance.Proposal {
	return clearance.Proposal{
		AttachmentID: "A7", Owner: "MERIDIAN FIBER", Kind: plant.KindFiber,
		Conductor:  "144F ADSS",
		NearHeight: unit.Tenths(186), FarHeight: unit.Tenths(186),
		SpanFeet: 200, RulingSpan: 200,
		Cover: plant.CoverField, District: nesc.Heavy,
	}
}

func TestProposalValidate(t *testing.T) {
	if err := sampleProposal().Validate(); err != nil {
		t.Fatalf("a good proposal failed: %v", err)
	}
	cases := map[string]func(*clearance.Proposal){
		"no id":        func(p *clearance.Proposal) { p.AttachmentID = "" },
		"no owner":     func(p *clearance.Proposal) { p.Owner = "" },
		"bad kind":     func(p *clearance.Proposal) { p.Kind = plant.Kind("clothesline") },
		"not in span":  func(p *clearance.Proposal) { p.Kind = plant.KindRiser },
		"no near":      func(p *clearance.Proposal) { p.NearHeight = 0 },
		"no far":       func(p *clearance.Proposal) { p.FarHeight = 0 },
		"no span":      func(p *clearance.Proposal) { p.SpanFeet = 0 },
		"no ruling":    func(p *clearance.Proposal) { p.RulingSpan = 0 },
		"bad cover":    func(p *clearance.Proposal) { p.Cover = plant.Cover("swamp") },
		"bad wire":     func(p *clearance.Proposal) { p.Conductor = "baling wire" },
		"bad district": func(p *clearance.Proposal) { p.District = nesc.District("tropical") },
	}
	for name, mutate := range cases {
		p := sampleProposal()
		mutate(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestPredictSagsAndClears(t *testing.T) {
	got, err := clearance.Predict(sampleProposal())
	if err != nil {
		t.Fatal(err)
	}
	if got.SagFeet <= 0 {
		t.Errorf("sag = %v", got.SagFeet)
	}
	if got.Clearance >= sampleProposal().NearHeight {
		t.Error("the wire hangs below its attachment, so the clearance is under the attachment height")
	}
	if got.Required.Tenths() != 120 {
		t.Errorf("required over a field = %s", got.Required)
	}
	if got.RuleID != "NESC-232A" {
		t.Errorf("rule = %s", got.RuleID)
	}
	if !got.Clears {
		t.Errorf("18.6 feet over a field should clear: %+v", got)
	}
	if _, ok := got.Finding("SP-1"); ok {
		t.Error("a clearing prediction has no finding")
	}
}

func TestPredictFailsOverARoad(t *testing.T) {
	p := sampleProposal()
	p.Cover = plant.CoverRoad
	p.NearHeight = unit.Tenths(160)
	p.FarHeight = unit.Tenths(160)
	got, err := clearance.Predict(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Clears {
		t.Fatalf("sixteen feet at the pole cannot clear 15.5 over a road once it sags: %+v", got)
	}
	f, ok := got.Finding("SP-1")
	if !ok {
		t.Fatal("a failing prediction should make a finding")
	}
	if f.RuleID != "NESC-232A" || f.Blame != clearance.BlameApplicant {
		t.Errorf("finding = %+v", f)
	}
	if f.Owner != "MERIDIAN FIBER" || f.Span != "SP-1" {
		t.Errorf("finding = %+v", f)
	}
	if f.Deficit() <= 0 {
		t.Errorf("deficit = %s", f.Deficit())
	}
}

func TestPredictSagsMoreOnALongerSpan(t *testing.T) {
	short, err := clearance.Predict(sampleProposal())
	if err != nil {
		t.Fatal(err)
	}
	p := sampleProposal()
	p.SpanFeet = 320
	p.RulingSpan = 320
	long, err := clearance.Predict(p)
	if err != nil {
		t.Fatal(err)
	}
	if long.SagFeet <= short.SagFeet {
		t.Errorf("a longer span sags more: %v against %v", long.SagFeet, short.SagFeet)
	}
	if long.Clearance >= short.Clearance {
		t.Error("and clears less")
	}
}

func TestPredictLowPointOnALevelSpan(t *testing.T) {
	got, err := clearance.Predict(sampleProposal())
	if err != nil {
		t.Fatal(err)
	}
	if got.LowPointAt < 90 || got.LowPointAt > 110 {
		t.Errorf("a level span bottoms out in the middle, got %v", got.LowPointAt)
	}
}

func TestPredictRejectsWhatItCannotWorkWith(t *testing.T) {
	p := sampleProposal()
	p.Conductor = "baling wire"
	if _, err := clearance.Predict(p); err == nil {
		t.Error("an uncatalogued conductor should be rejected")
	}
	p = sampleProposal()
	p.Cover = plant.Cover("swamp")
	if _, err := clearance.Predict(p); err == nil {
		t.Error("an untabulated cover should be rejected")
	}
}

func TestLowestUsableHeight(t *testing.T) {
	clear, err := clearance.LowestUsableHeight(sampleProposal())
	if err != nil {
		t.Fatal(err)
	}
	if clear.Tenths() != 186 {
		t.Errorf("a clearing proposal keeps its height, got %s", clear)
	}
	p := sampleProposal()
	p.Cover = plant.CoverRoad
	p.NearHeight = unit.Tenths(160)
	p.FarHeight = unit.Tenths(160)
	raised, err := clearance.LowestUsableHeight(p)
	if err != nil {
		t.Fatal(err)
	}
	if raised <= p.NearHeight {
		t.Errorf("a failing proposal has to go up, got %s", raised)
	}
	// Putting it at the recommended height should then clear.
	p.NearHeight = raised
	p.FarHeight = raised
	got, err := clearance.Predict(p)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Clears {
		t.Errorf("the recommended height does not actually clear: %+v", got)
	}
	bad := sampleProposal()
	bad.Conductor = "baling wire"
	if _, err := clearance.LowestUsableHeight(bad); err == nil {
		t.Error("an uncatalogued conductor should be rejected")
	}
}
