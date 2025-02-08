package tests

import (
	"errors"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

func TestKindClassification(t *testing.T) {
	supply := []plant.Kind{plant.KindPrimary, plant.KindSecondary, plant.KindNeutral, plant.KindStreetlight, plant.KindTransformer, plant.KindServiceDrop}
	for _, k := range supply {
		if !k.IsSupply() || k.IsCommunication() {
			t.Errorf("%s should be supply", k)
		}
	}
	comm := []plant.Kind{plant.KindCommCable, plant.KindFiber, plant.KindCommDrop, plant.KindRiser, plant.KindPowerSupply, plant.KindEquipmentBox, plant.KindStandoffBracket}
	for _, k := range comm {
		if !k.IsCommunication() || k.IsSupply() {
			t.Errorf("%s should be communication", k)
		}
	}
	if plant.Kind("nonsense").IsSupply() || plant.Kind("nonsense").IsCommunication() {
		t.Error("an unknown kind is neither")
	}
}

func TestKindSpanCarrying(t *testing.T) {
	carrying := []plant.Kind{plant.KindPrimary, plant.KindNeutral, plant.KindCommCable, plant.KindFiber, plant.KindCommDrop, plant.KindServiceDrop, plant.KindSecondary}
	for _, k := range carrying {
		if !k.IsSpanCarrying() {
			t.Errorf("%s runs into the span", k)
		}
	}
	terminating := []plant.Kind{plant.KindTransformer, plant.KindRiser, plant.KindEquipmentBox, plant.KindPowerSupply, plant.KindStreetlight, plant.KindStandoffBracket}
	for _, k := range terminating {
		if k.IsSpanCarrying() {
			t.Errorf("%s terminates on the pole", k)
		}
	}
}

func TestParseKind(t *testing.T) {
	for _, in := range []string{"comm_cable", "comm cable", "COMM-CABLE", " Comm Cable "} {
		got, err := plant.ParseKind(in)
		if err != nil || got != plant.KindCommCable {
			t.Errorf("ParseKind(%q) = %q %v", in, got, err)
		}
	}
	if _, err := plant.ParseKind("clothesline"); !errors.Is(err, plant.ErrBadAttachment) {
		t.Errorf("unknown kind error = %v", err)
	}
	if plant.KindFiber.String() != "fiber" {
		t.Errorf("String = %q", plant.KindFiber.String())
	}
}

func sampleAttachment() plant.Attachment {
	return plant.Attachment{
		ID:         "ATT-0001",
		Pole:       "NW-441829",
		Owner:      "RIDGEPORT CABLEVISION",
		Kind:       plant.KindCommCable,
		Height:     unit.Tenths(200),
		Authorized: true,
		InForce:    calendar.Range{From: calendar.MustNew(2019, 4, 1)},
	}
}

func TestAttachmentValidateAccepts(t *testing.T) {
	if err := sampleAttachment().Validate(); err != nil {
		t.Fatalf("a good attachment failed: %v", err)
	}
	primary := sampleAttachment()
	primary.Kind = plant.KindPrimary
	primary.Voltage = 7620
	primary.Height = unit.Tenths(340)
	if err := primary.Validate(); err != nil {
		t.Fatalf("a good primary failed: %v", err)
	}
}

func TestAttachmentValidateRejects(t *testing.T) {
	cases := map[string]func(*plant.Attachment){
		"no id":          func(a *plant.Attachment) { a.ID = "  " },
		"bad pole":       func(a *plant.Attachment) { a.Pole = "nope" },
		"no owner":       func(a *plant.Attachment) { a.Owner = "" },
		"bad kind":       func(a *plant.Attachment) { a.Kind = plant.Kind("clothesline") },
		"no height":      func(a *plant.Attachment) { a.Height = 0 },
		"energised comm": func(a *plant.Attachment) { a.Voltage = 120 },
		"backwards range": func(a *plant.Attachment) {
			a.InForce = calendar.Range{From: calendar.MustNew(2020, 1, 1), To: calendar.MustNew(2019, 1, 1)}
		},
		"unenergised prim": func(a *plant.Attachment) { a.Kind = plant.KindPrimary; a.Voltage = 240 },
	}
	for name, mutate := range cases {
		a := sampleAttachment()
		mutate(&a)
		if err := a.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestAttachmentActiveOn(t *testing.T) {
	a := sampleAttachment()
	if !a.ActiveOn(calendar.MustNew(2025, 1, 1)) {
		t.Error("an open ended attachment is still there")
	}
	if a.ActiveOn(calendar.MustNew(2018, 1, 1)) {
		t.Error("it was not there before it went up")
	}
	a.InForce.To = calendar.MustNew(2022, 12, 31)
	if a.ActiveOn(calendar.MustNew(2025, 1, 1)) {
		t.Error("a removed attachment is not there")
	}
}

func TestAttachmentString(t *testing.T) {
	a := sampleAttachment()
	want := "ATT-0001 RIDGEPORT CABLEVISION comm_cable at 20.0 ft (authorized)"
	if got := a.String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	a.Authorized = false
	if got := a.String(); got[len(got)-14:] != "unauthorized)" && got == want {
		t.Errorf("unauthorized String = %q", got)
	}
}
