package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

func packetItem(id, pole string, action makeready.Action, owner string) makeready.Item {
	item := makeready.Item{
		ID: id, Action: action, Pole: pole, Owner: owner, Because: []string{"x"},
	}
	switch action {
	case makeready.ActionRaise:
		item.Subject = "A3"
		item.FromHeight = unit.Tenths(200)
		item.ToHeight = unit.Tenths(212)
	case makeready.ActionLower:
		item.Subject = "A3"
		item.FromHeight = unit.Tenths(200)
		item.ToHeight = unit.Tenths(190)
	case makeready.ActionReplacePole, makeready.ActionAddGuy:
		item.Detail = "structure"
	default:
		item.Subject = "A3"
	}
	return item
}

func TestDisciplineForEveryAction(t *testing.T) {
	t.Parallel()
	line := []makeready.Action{makeready.ActionReplacePole, makeready.ActionAddGuy}
	for _, a := range line {
		if got := makeready.DisciplineFor(a); got != makeready.DisciplineLine {
			t.Errorf("%s goes to the %s crew", a, got)
		}
	}
	for _, a := range makeready.Actions() {
		got := makeready.DisciplineFor(a)
		if !makeready.KnownDiscipline(got) {
			t.Errorf("%s goes to %q", a, got)
		}
		if got == makeready.DisciplineUnassigned {
			t.Errorf("%s has no crew", a)
		}
	}
	if got := makeready.DisciplineFor(makeready.Action("pray")); got != makeready.DisciplineUnassigned {
		t.Errorf("an unknown action = %q", got)
	}
	if makeready.KnownDiscipline(makeready.Discipline("arborist")) {
		t.Error("that is not a crew here")
	}
}

func TestPacketsGroupByPoleThenCrew(t *testing.T) {
	t.Parallel()
	plan := planWith(
		packetItem("MR-0001", "NW-441830", makeready.ActionRaise, "RIDGEPORT CABLEVISION"),
		packetItem("MR-0002", "NW-441829", makeready.ActionRaise, "RIDGEPORT CABLEVISION"),
		packetItem("MR-0003", "NW-441829", makeready.ActionAddGuy, "SUMMIT VALLEY ELECTRIC"),
		packetItem("MR-0004", "NW-441829", makeready.ActionLower, "NORTHVALE TELEPHONE"),
	)
	got := makeready.Packets(plan)
	if len(got) != 3 {
		t.Fatalf("packets = %v", got)
	}
	if got[0].Pole != "NW-441829" || got[0].Discipline != makeready.DisciplineLine {
		t.Errorf("the line crew goes first on a shared pole, got %+v", got[0])
	}
	if got[1].Pole != "NW-441829" || got[1].Discipline != makeready.DisciplineComm {
		t.Errorf("packet = %+v", got[1])
	}
	if got[1].Size() != 2 {
		t.Errorf("the communication crew has two items on that pole, got %d", got[1].Size())
	}
	if got[2].Pole != "NW-441830" {
		t.Errorf("packet = %+v", got[2])
	}
	if len(got[1].Owners) != 2 || got[1].Owners[0] != "NORTHVALE TELEPHONE" {
		t.Errorf("owners = %v", got[1].Owners)
	}
	if got[0].String() == "" {
		t.Error("String should say something")
	}
}

func TestPacketsMarkTheOutageAndWhatWaitsOnIt(t *testing.T) {
	t.Parallel()
	plan := planWith(
		packetItem("MR-0001", "NW-441829", makeready.ActionReplacePole, "SUMMIT VALLEY ELECTRIC"),
		packetItem("MR-0002", "NW-441829", makeready.ActionTransfer, "RIDGEPORT CABLEVISION"),
		packetItem("MR-0003", "NW-441830", makeready.ActionRaise, "RIDGEPORT CABLEVISION"),
	)
	got := makeready.Packets(plan)
	byPole := map[string][]makeready.Packet{}
	for _, p := range got {
		byPole[p.Pole] = append(byPole[p.Pole], p)
	}
	line := byPole["NW-441829"][0]
	comm := byPole["NW-441829"][1]
	if !line.NeedsOutage {
		t.Error("setting a pole needs an outage")
	}
	if comm.NeedsOutage {
		t.Error("the transfer itself does not need one")
	}
	if !comm.NeedsPole {
		t.Error("but it waits on the pole being set")
	}
	if byPole["NW-441830"][0].NeedsPole {
		t.Error("nothing structural is happening on the other pole")
	}
	poles := makeready.PolesNeedingAnOutage(plan)
	if len(poles) != 1 || poles[0] != "NW-441829" {
		t.Errorf("outages = %v", poles)
	}
}

func TestPacketItemsStayInStageOrder(t *testing.T) {
	t.Parallel()
	plan := planWith(
		packetItem("MR-0009", "NW-441829", makeready.ActionRaise, "A"),
		packetItem("MR-0001", "NW-441829", makeready.ActionRemove, "A"),
	)
	got := makeready.PacketsFor(plan, makeready.DisciplineComm)
	if len(got) != 1 {
		t.Fatalf("packets = %v", got)
	}
	if got[0].Items[0].Action != makeready.ActionRemove {
		t.Errorf("removals come first, got %v", got[0].Items)
	}
}

func TestPacketsForOneCrew(t *testing.T) {
	t.Parallel()
	plan := planWith(
		packetItem("MR-0001", "NW-441829", makeready.ActionAddGuy, "SUMMIT VALLEY ELECTRIC"),
		packetItem("MR-0002", "NW-441829", makeready.ActionRaise, "RIDGEPORT CABLEVISION"),
	)
	if got := makeready.PacketsFor(plan, makeready.DisciplineLine); len(got) != 1 {
		t.Errorf("line packets = %v", got)
	}
	if got := makeready.PacketsFor(plan, makeready.DisciplineComm); len(got) != 1 {
		t.Errorf("communication packets = %v", got)
	}
	if got := makeready.PacketsFor(plan, makeready.DisciplineUnassigned); len(got) != 0 {
		t.Errorf("nothing is unassigned, got %v", got)
	}
}

func TestPacketsOfNothing(t *testing.T) {
	t.Parallel()
	if got := makeready.Packets(nil); got != nil {
		t.Errorf("no plan, no packets, got %v", got)
	}
	if got := makeready.Packets(planWith()); len(got) != 0 {
		t.Errorf("an empty plan makes no packets, got %v", got)
	}
	if got := makeready.PolesNeedingAnOutage(nil); got != nil {
		t.Errorf("outages = %v", got)
	}
}
