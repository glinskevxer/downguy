package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/allocate"
)

func sampleRoster(t *testing.T) *allocate.Roster {
	t.Helper()
	r := allocate.NewRoster()
	for _, p := range []allocate.Party{
		{Name: "SUMMIT VALLEY ELECTRIC", Role: allocate.RoleOwner, Account: "OWN-1"},
		{Name: "MERIDIAN FIBER", Role: allocate.RoleApplicant, Account: "ATT-9"},
		{Name: "RIDGEPORT CABLEVISION", Role: allocate.RoleExisting, Account: "ATT-3"},
		{Name: "NORTHVALE TELEPHONE", Role: allocate.RoleExisting, Account: "ATT-4"},
	} {
		if err := r.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestRoleNamesAndParsing(t *testing.T) {
	for _, r := range []allocate.Role{allocate.RoleOwner, allocate.RoleApplicant, allocate.RoleExisting, allocate.RoleUnknown} {
		if !allocate.KnownRole(r) {
			t.Errorf("%s should be known", r)
		}
		if r.Name() == "" {
			t.Errorf("%s has no long form", r)
		}
	}
	if allocate.KnownRole(allocate.Role("landlord")) {
		t.Error("landlord is not a role here")
	}
	if allocate.Role("landlord").Name() != "unknown role landlord" {
		t.Errorf("name = %q", allocate.Role("landlord").Name())
	}
	got, err := allocate.ParseRole(" Applicant ")
	if err != nil || got != allocate.RoleApplicant {
		t.Errorf("ParseRole = %q %v", got, err)
	}
	if _, err := allocate.ParseRole("landlord"); err == nil {
		t.Error("landlord should be rejected")
	}
}

func TestPartyValidate(t *testing.T) {
	good := allocate.Party{Name: "MERIDIAN FIBER", Role: allocate.RoleApplicant, Account: "ATT-9"}
	if err := good.Validate(); err != nil {
		t.Fatalf("a good party failed: %v", err)
	}
	cases := map[string]allocate.Party{
		"no name":    {Role: allocate.RoleApplicant, Account: "ATT-9"},
		"bad role":   {Name: "X", Role: allocate.Role("landlord"), Account: "ATT-9"},
		"no account": {Name: "X", Role: allocate.RoleApplicant},
	}
	for name, p := range cases {
		if err := p.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
	unidentified := allocate.Party{Name: "UNIDENTIFIED", Role: allocate.RoleUnknown}
	if err := unidentified.Validate(); err != nil {
		t.Errorf("an unidentified party needs no account: %v", err)
	}
}

func TestRosterFoldsNames(t *testing.T) {
	r := sampleRoster(t)
	p, ok := r.Lookup("  meridian   fiber ")
	if !ok || p.Role != allocate.RoleApplicant {
		t.Errorf("lookup = %+v ok=%v", p, ok)
	}
	if _, ok := r.Lookup("nobody"); ok {
		t.Error("lookup should miss")
	}
	if got := r.RoleOf("ridgeport cablevision"); got != allocate.RoleExisting {
		t.Errorf("role = %s", got)
	}
	if got := r.RoleOf("nobody"); got != allocate.RoleUnknown {
		t.Errorf("an unknown name is unknown, got %s", got)
	}
	if allocate.Fold(" a  b ") != "A B" {
		t.Errorf("Fold = %q", allocate.Fold(" a  b "))
	}
}

func TestRosterRefusesToChangeARole(t *testing.T) {
	r := sampleRoster(t)
	err := r.Add(allocate.Party{Name: "meridian fiber", Role: allocate.RoleExisting, Account: "ATT-9"})
	if err == nil {
		t.Error("a party cannot be two things on one application")
	}
	if err := r.Add(allocate.Party{Name: "MERIDIAN FIBER", Role: allocate.RoleApplicant, Account: "ATT-99"}); err != nil {
		t.Errorf("re-filing the same role should be allowed: %v", err)
	}
	p, _ := r.Lookup("MERIDIAN FIBER")
	if p.Account != "ATT-99" {
		t.Errorf("the later record should win, got %q", p.Account)
	}
	if err := r.Add(allocate.Party{Name: "", Role: allocate.RoleExisting}); err == nil {
		t.Error("an invalid party should be rejected")
	}
}

func TestRosterAccessors(t *testing.T) {
	r := sampleRoster(t)
	if r.Len() != 4 {
		t.Errorf("len = %d", r.Len())
	}
	names := r.Names()
	if len(names) != 4 || names[0] != "MERIDIAN FIBER" {
		t.Errorf("names = %v", names)
	}
	existing := r.InRole(allocate.RoleExisting)
	if len(existing) != 2 || existing[0].Name != "NORTHVALE TELEPHONE" {
		t.Errorf("existing = %v", existing)
	}
	if got := r.InRole(allocate.RoleUnknown); len(got) != 0 {
		t.Errorf("unknown = %v", got)
	}
	applicant, ok := r.Applicant()
	if !ok || applicant.Name != "MERIDIAN FIBER" {
		t.Errorf("applicant = %+v ok=%v", applicant, ok)
	}
	owner, ok := r.Owner()
	if !ok || owner.Name != "SUMMIT VALLEY ELECTRIC" {
		t.Errorf("owner = %+v ok=%v", owner, ok)
	}
	if err := r.Validate(); err != nil {
		t.Errorf("a complete roster failed: %v", err)
	}
}

func TestRosterNeedsExactlyOneOwnerAndApplicant(t *testing.T) {
	empty := allocate.NewRoster()
	if err := empty.Validate(); err == nil {
		t.Error("an empty roster should be rejected")
	}
	if _, ok := empty.Applicant(); ok {
		t.Error("no applicant")
	}
	if _, ok := empty.Owner(); ok {
		t.Error("no owner")
	}
	two := allocate.NewRoster()
	for _, p := range []allocate.Party{
		{Name: "SUMMIT VALLEY ELECTRIC", Role: allocate.RoleOwner, Account: "OWN-1"},
		{Name: "MERIDIAN FIBER", Role: allocate.RoleApplicant, Account: "ATT-9"},
		{Name: "CASCADE BROADBAND", Role: allocate.RoleApplicant, Account: "ATT-10"},
	} {
		if err := two.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := two.Applicant(); ok {
		t.Error("two applicants is not one applicant")
	}
	if err := two.Validate(); err == nil {
		t.Error("and the roster should be rejected")
	}
	noApplicant := allocate.NewRoster()
	if err := noApplicant.Add(allocate.Party{Name: "SUMMIT VALLEY ELECTRIC", Role: allocate.RoleOwner, Account: "OWN-1"}); err != nil {
		t.Fatal(err)
	}
	if err := noApplicant.Validate(); err == nil {
		t.Error("an owner alone is not an application")
	}
}
