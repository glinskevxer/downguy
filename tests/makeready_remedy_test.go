package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

func raiseItem() makeready.Item {
	return makeready.Item{
		ID: "MR-1", Action: makeready.ActionRaise, Pole: "NW-441829",
		Subject: "A3", Owner: "RIDGEPORT CABLEVISION",
		FromHeight: unit.Tenths(200), ToHeight: unit.Tenths(212),
		Because: []string{"NESC-235C1|NW-441829||A3|A2"},
	}
}

func TestActionStagesOrderTheWork(t *testing.T) {
	ordered := makeready.Actions()
	if len(ordered) != 8 {
		t.Fatalf("actions = %v", ordered)
	}
	if ordered[0] != makeready.ActionRemove {
		t.Errorf("removals come first, got %s", ordered[0])
	}
	if ordered[1] != makeready.ActionReplacePole {
		t.Errorf("the pole comes before anything on it, got %s", ordered[1])
	}
	if ordered[len(ordered)-1] != makeready.ActionResag {
		t.Errorf("resagging comes last, got %s", ordered[len(ordered)-1])
	}
	prev := -1
	for _, a := range ordered {
		s, ok := makeready.Stage(a)
		if !ok {
			t.Fatalf("%s has no stage", a)
		}
		if s < prev {
			t.Fatalf("actions are not in stage order: %v", ordered)
		}
		prev = s
	}
	if _, ok := makeready.Stage(makeready.Action("hope")); ok {
		t.Error("an unknown action has no stage")
	}
	if !makeready.KnownAction(makeready.ActionResag) || makeready.KnownAction(makeready.Action("hope")) {
		t.Error("known action check")
	}
}

func TestTransferComesAfterThePoleIsSet(t *testing.T) {
	pole, _ := makeready.Stage(makeready.ActionReplacePole)
	transfer, _ := makeready.Stage(makeready.ActionTransfer)
	guy, _ := makeready.Stage(makeready.ActionAddGuy)
	if !(pole < guy && guy < transfer) {
		t.Errorf("set the pole, guy it, then transfer onto it: %d %d %d", pole, guy, transfer)
	}
}

func TestParseAction(t *testing.T) {
	for _, in := range []string{"replace_pole", "replace pole", "REPLACE-POLE", " Replace Pole "} {
		got, err := makeready.ParseAction(in)
		if err != nil || got != makeready.ActionReplacePole {
			t.Errorf("ParseAction(%q) = %q %v", in, got, err)
		}
	}
	if _, err := makeready.ParseAction("pray"); err == nil {
		t.Error("praying is not an action")
	}
}

func TestPoleWorkSplit(t *testing.T) {
	for _, a := range []makeready.Action{makeready.ActionReplacePole, makeready.ActionAddGuy} {
		if !a.PoleWork() {
			t.Errorf("%s is pole work", a)
		}
	}
	for _, a := range []makeready.Action{makeready.ActionRaise, makeready.ActionLower, makeready.ActionTransfer, makeready.ActionRemove, makeready.ActionResag, makeready.ActionRelocateEquipment} {
		if a.PoleWork() {
			t.Errorf("%s is not pole work", a)
		}
	}
}

func TestItemValidateAccepts(t *testing.T) {
	if err := raiseItem().Validate(); err != nil {
		t.Fatalf("a good item failed: %v", err)
	}
	lower := raiseItem()
	lower.Action = makeready.ActionLower
	lower.ToHeight = unit.Tenths(190)
	if err := lower.Validate(); err != nil {
		t.Fatalf("a good lowering failed: %v", err)
	}
	replace := makeready.Item{
		ID: "MR-2", Action: makeready.ActionReplacePole, Pole: "NW-441829",
		Because: []string{"NESC-250B|NW-441829|||"}, Detail: "45/2 southern pine",
	}
	if err := replace.Validate(); err != nil {
		t.Fatalf("a pole replacement needs no subject: %v", err)
	}
}

func TestItemValidateRejects(t *testing.T) {
	cases := map[string]func(*makeready.Item){
		"no id":      func(i *makeready.Item) { i.ID = " " },
		"bad action": func(i *makeready.Item) { i.Action = makeready.Action("pray") },
		"no pole":    func(i *makeready.Item) { i.Pole = "" },
		"no reason":  func(i *makeready.Item) { i.Because = nil },
		"no subject": func(i *makeready.Item) { i.Subject = "" },
		"raise down": func(i *makeready.Item) { i.ToHeight = unit.Tenths(180) },
		"raise flat": func(i *makeready.Item) { i.ToHeight = i.FromHeight },
	}
	for name, mutate := range cases {
		item := raiseItem()
		mutate(&item)
		if err := item.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
	up := raiseItem()
	up.Action = makeready.ActionLower
	if err := up.Validate(); err == nil {
		t.Error("a lowering that goes up should be rejected")
	}
}

func TestItemMove(t *testing.T) {
	if got := raiseItem().Move().Tenths(); got != 12 {
		t.Errorf("move = %d", got)
	}
	lower := raiseItem()
	lower.Action = makeready.ActionLower
	lower.ToHeight = unit.Tenths(188)
	if got := lower.Move().Tenths(); got != 12 {
		t.Errorf("a lowering moves just as far, got %d", got)
	}
	still := makeready.Item{}
	if got := still.Move().Tenths(); got != 0 {
		t.Errorf("nothing moved, got %d", got)
	}
}

func TestItemString(t *testing.T) {
	want := "MR-1 raise A3 on NW-441829 from 20.0 ft to 21.2 ft"
	if got := raiseItem().String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
	replace := makeready.Item{
		ID: "MR-2", Action: makeready.ActionReplacePole, Pole: "NW-441829",
		Detail: "45/2 southern pine", Because: []string{"x"},
	}
	if got := replace.String(); got != "MR-2 replace NW-441829: 45/2 southern pine" {
		t.Errorf("String = %q", got)
	}
	remove := raiseItem()
	remove.Action = makeready.ActionRemove
	if got := remove.String(); got != "MR-1 remove A3 on NW-441829" {
		t.Errorf("String = %q", got)
	}
}
