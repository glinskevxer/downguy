package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
)

func occ(id, pole, owner string, authorized bool, from, to string) rental.Occupancy {
	o := rental.Occupancy{AttachmentID: id, Pole: plantID(pole), Owner: owner, Authorized: authorized}
	if from != "" {
		o.Active.From = calendar.MustNew(atoiPart(from, 0), atoiPart(from, 1), atoiPart(from, 2))
	}
	if to != "" {
		o.Active.To = calendar.MustNew(atoiPart(to, 0), atoiPart(to, 1), atoiPart(to, 2))
	}
	return o
}

func sampleInventory(t *testing.T) *rental.Inventory {
	t.Helper()
	inv := rental.NewInventory(calendar.RateYear(2025))
	for _, o := range []rental.Occupancy{
		occ("A3", "NW-441829", "RIDGEPORT CABLEVISION", true, "2019-04-01", ""),
		occ("A4", "NW-441829", "NORTHVALE TELEPHONE", true, "2012-08-15", ""),
		occ("A7", "NW-441829", "MERIDIAN FIBER", true, "2025-07-01", ""),
		occ("B3", "NW-441830", "RIDGEPORT CABLEVISION", true, "2019-04-01", ""),
		occ("B9", "NW-441830", "UNIDENTIFIED", false, "2019-04-01", ""),
	} {
		if err := inv.Add(o); err != nil {
			t.Fatal(err)
		}
	}
	return inv
}

func TestInventoryAddRejectsBadRecords(t *testing.T) {
	inv := rental.NewInventory(calendar.RateYear(2025))
	if err := inv.Add(rental.Occupancy{Owner: "X"}); err == nil {
		t.Error("no attachment id should be rejected")
	}
	if err := inv.Add(rental.Occupancy{AttachmentID: "A1"}); err == nil {
		t.Error("no owner should be rejected")
	}
	backwards := occ("A1", "NW-441829", "X", true, "2025-12-31", "")
	backwards.Active.To = calendar.MustNew(2025, 1, 1)
	if err := inv.Add(backwards); err == nil {
		t.Error("a backwards range should be rejected")
	}
}

func TestInventorySkipsWhatMissedTheYear(t *testing.T) {
	inv := rental.NewInventory(calendar.RateYear(2025))
	if err := inv.Add(occ("A1", "NW-441829", "X", true, "2010-01-01", "2018-12-31")); err != nil {
		t.Fatal(err)
	}
	if err := inv.Add(occ("A2", "NW-441829", "X", true, "2026-01-01", "")); err != nil {
		t.Fatal(err)
	}
	if inv.Count() != 0 {
		t.Errorf("neither was up in 2025, got %d", inv.Count())
	}
	if err := inv.Add(occ("A3", "NW-441829", "X", true, "2025-12-31", "")); err != nil {
		t.Fatal(err)
	}
	if inv.Count() != 1 {
		t.Errorf("one day of the year is enough, got %d", inv.Count())
	}
}

func TestInventoryGrouping(t *testing.T) {
	inv := sampleInventory(t)
	if inv.Count() != 5 {
		t.Fatalf("count = %d", inv.Count())
	}
	owners := inv.Owners()
	if len(owners) != 4 || owners[0] != "MERIDIAN FIBER" {
		t.Errorf("owners = %v", owners)
	}
	ridgeport := inv.ForOwner(" ridgeport cablevision ")
	if len(ridgeport) != 2 || ridgeport[0].AttachmentID != "A3" {
		t.Errorf("ridgeport = %v", ridgeport)
	}
	if inv.CountFor("RIDGEPORT CABLEVISION") != 2 {
		t.Errorf("count for = %d", inv.CountFor("RIDGEPORT CABLEVISION"))
	}
	if inv.CountFor("nobody") != 0 {
		t.Errorf("nobody has nothing")
	}
	poles := inv.Poles()
	if len(poles) != 2 || poles[0] != "NW-441829" {
		t.Errorf("poles = %v", poles)
	}
	if inv.String() == "" {
		t.Error("String should say something")
	}
}

func TestAttachersOnAPole(t *testing.T) {
	inv := sampleInventory(t)
	if got := inv.AttachersOn("NW-441829"); got != 3 {
		t.Errorf("attachers = %d, want 3", got)
	}
	if got := inv.AttachersOn("NW-441830"); got != 2 {
		t.Errorf("attachers = %d, want 2", got)
	}
	if got := inv.AttachersOn("SE-1129043"); got != 0 {
		t.Errorf("a pole with nothing on it has no attachers, got %d", got)
	}
}

func TestInventoryUnauthorized(t *testing.T) {
	inv := sampleInventory(t)
	got := inv.Unauthorized()
	if len(got) != 1 || got[0].AttachmentID != "B9" {
		t.Errorf("unauthorized = %v", got)
	}
}

func TestInventoryPartYear(t *testing.T) {
	inv := sampleInventory(t)
	got := inv.PartYear()
	if len(got) != 1 || got[0].AttachmentID != "A7" {
		t.Errorf("part year = %v", got)
	}
}

func TestInventoryFromAnAssembly(t *testing.T) {
	inv := rental.NewInventory(calendar.RateYear(2025))
	if err := inv.FromAssembly(sampleAssembly()); err != nil {
		t.Fatal(err)
	}
	if inv.Count() != 2 {
		t.Errorf("only the communication attachments are billable, got %d", inv.Count())
	}
	owners := inv.Owners()
	if len(owners) != 2 {
		t.Errorf("owners = %v", owners)
	}
	if err := inv.FromAssembly(nil); err == nil {
		t.Error("no assembly should be rejected")
	}
}
