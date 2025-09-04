package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
)

func TestSanctionForEachWayOfFindingOne(t *testing.T) {
	noticed := calendar.MustNew(2026, 3, 1)
	got, err := rental.SanctionFor(rental.DiscoveryAudit, calendar.MustNew(2026, 5, 1), noticed)
	if err != nil || got != rental.SanctionPenalty {
		t.Errorf("audit = %q %v", got, err)
	}
	got, err = rental.SanctionFor(rental.DiscoverySelfReported, calendar.MustNew(2026, 3, 15), noticed)
	if err != nil || got != rental.SanctionNone {
		t.Errorf("inside the grace period = %q %v", got, err)
	}
	got, err = rental.SanctionFor(rental.DiscoverySelfReported, noticed.AddDays(rental.SelfReportGraceDays), noticed)
	if err != nil || got != rental.SanctionNone {
		t.Errorf("the last day of grace = %q %v", got, err)
	}
	got, err = rental.SanctionFor(rental.DiscoverySelfReported, noticed.AddDays(rental.SelfReportGraceDays+1), noticed)
	if err != nil || got != rental.SanctionBackRent {
		t.Errorf("the day after = %q %v", got, err)
	}
	got, err = rental.SanctionFor(rental.DiscoverySelfReported, calendar.MustNew(2026, 5, 1), calendar.Zero)
	if err != nil || got != rental.SanctionNone {
		t.Errorf("with no audit under way = %q %v", got, err)
	}
	if _, err := rental.SanctionFor(rental.Discovery("rumour"), noticed, noticed); err == nil {
		t.Error("a rumour is not a discovery")
	}
	if !rental.KnownDiscovery(rental.DiscoveryAudit) || rental.KnownDiscovery(rental.Discovery("rumour")) {
		t.Error("known discovery check")
	}
}

func TestAuditClaimCarriesBackRentAndPenalty(t *testing.T) {
	o := occ("B9", "NW-441830", "UNIDENTIFIED", false, "2019-04-01", "")
	got, err := rental.AssessClaim(o, rental.DiscoveryAudit, calendar.MustNew(2026, 5, 1), calendar.MustNew(2026, 3, 1), 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sanction != rental.SanctionPenalty {
		t.Errorf("sanction = %s", got.Sanction)
	}
	// The agreement reaches back five years, so 2022 to 2026, but only 2024,
	// 2025 and 2026 carry a filed rate.
	if len(got.BackYears) != 3 {
		t.Errorf("back years = %v", got.BackYears)
	}
	if got.BackRent.IsZero() || got.Penalty.IsZero() {
		t.Errorf("claim = %+v", got)
	}
	if got.Total != got.BackRent.Add(got.Penalty) {
		t.Errorf("total = %s", got.Total)
	}
	if got.Note == "" {
		t.Error("the unfiled years should be explained")
	}
	if got.String() == "" {
		t.Error("String should say something")
	}
}

func TestSelfReportedInsideGraceOwesNothingExtra(t *testing.T) {
	o := occ("B9", "NW-441830", "CASCADE BROADBAND", false, "2019-04-01", "")
	got, err := rental.AssessClaim(o, rental.DiscoverySelfReported, calendar.MustNew(2026, 3, 15), calendar.MustNew(2026, 3, 1), 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sanction != rental.SanctionNone {
		t.Errorf("sanction = %s", got.Sanction)
	}
	if !got.BackRent.IsZero() || !got.Penalty.IsZero() || !got.Total.IsZero() {
		t.Errorf("claim = %+v", got)
	}
	if got.Note == "" {
		t.Error("the grace period should be explained")
	}
}

func TestLateSelfReportOwesBackRentButNoPenalty(t *testing.T) {
	o := occ("B9", "NW-441830", "CASCADE BROADBAND", false, "2019-04-01", "")
	noticed := calendar.MustNew(2026, 3, 1)
	got, err := rental.AssessClaim(o, rental.DiscoverySelfReported, noticed.AddDays(90), noticed, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sanction != rental.SanctionBackRent {
		t.Errorf("sanction = %s", got.Sanction)
	}
	if got.BackRent.IsZero() {
		t.Error("back rent is owed")
	}
	if !got.Penalty.IsZero() {
		t.Errorf("no penalty is owed, got %s", got.Penalty)
	}
	if got.Total != got.BackRent {
		t.Errorf("total = %s", got.Total)
	}
}

func TestBackRentStartsWhenTheAttachmentWentUp(t *testing.T) {
	recent := occ("B9", "NW-441830", "CASCADE BROADBAND", false, "2025-06-01", "")
	got, err := rental.AssessClaim(recent, rental.DiscoveryAudit, calendar.MustNew(2026, 5, 1), calendar.Zero, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.BackYears) != 2 {
		t.Errorf("back years = %v, want 2025 and 2026", got.BackYears)
	}
	if got.BackYears[0] != 2025 {
		t.Errorf("first back year = %s", got.BackYears[0])
	}
}

func TestTheTelecomFormulaRaisesAClaim(t *testing.T) {
	o := occ("B9", "NW-441830", "NORTHVALE TELEPHONE", false, "2024-01-01", "")
	found := calendar.MustNew(2026, 5, 1)
	cable, err := rental.AssessClaim(o, rental.DiscoveryAudit, found, calendar.Zero, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	telecom, err := rental.AssessClaim(o, rental.DiscoveryAudit, found, calendar.Zero, 3,
		rental.Assignment{"NORTHVALE TELEPHONE": rental.FormulaTelecom})
	if err != nil {
		t.Fatal(err)
	}
	if telecom.Total <= cable.Total {
		t.Errorf("the telecom formula claims more: %s against %s", telecom.Total, cable.Total)
	}
}

func TestAssessClaimRejectsBadInput(t *testing.T) {
	o := occ("B9", "NW-441830", "X", false, "2024-01-01", "")
	if _, err := rental.AssessClaim(rental.Occupancy{}, rental.DiscoveryAudit, calendar.MustNew(2026, 5, 1), calendar.Zero, 3, nil); err == nil {
		t.Error("a claim against nothing should be rejected")
	}
	if _, err := rental.AssessClaim(o, rental.DiscoveryAudit, calendar.Zero, calendar.Zero, 3, nil); err == nil {
		t.Error("a claim with no discovery date should be rejected")
	}
	if _, err := rental.AssessClaim(o, rental.Discovery("rumour"), calendar.MustNew(2026, 5, 1), calendar.Zero, 3, nil); err == nil {
		t.Error("a rumour should be rejected")
	}
}

func TestAssessAllAcrossAnInventory(t *testing.T) {
	got, err := rental.AssessAll(sampleInventory(t), rental.DiscoveryAudit, calendar.MustNew(2026, 5, 1), calendar.Zero, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AttachmentID != "B9" {
		t.Errorf("claims = %v", got)
	}
	if rental.ClaimTotal(got) != got[0].Total {
		t.Errorf("claim total = %s", rental.ClaimTotal(got))
	}
	if !rental.ClaimTotal(nil).IsZero() {
		t.Error("no claims, no money")
	}
	if _, err := rental.AssessAll(nil, rental.DiscoveryAudit, calendar.MustNew(2026, 5, 1), calendar.Zero, nil); err == nil {
		t.Error("no inventory should be rejected")
	}
}
