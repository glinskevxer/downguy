package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func demoDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	got := cli("demo", "-db", path)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("demo: code %d, stderr %q", got.Code, got.Stderr)
	}
	return path
}

func TestDemoSeedsAWorkedExample(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("demo", "-db", path)
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"seeded", "4 poles", "17 attachments", "3 spans", "3 surveys", "1 applications"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestDemoRefusesADatabaseWithRecordsInIt(t *testing.T) {
	t.Parallel()
	path := demoDB(t)
	again := cli("demo", "-db", path)
	if again.Code != cliapp.CodeUsage {
		t.Fatalf("code = %d", again.Code)
	}
	if !strings.Contains(again.Stderr, "already holds 4 poles") {
		t.Errorf("stderr = %q", again.Stderr)
	}
	forced := cli("demo", "-db", path, "-force")
	if forced.Code != cliapp.CodeOK {
		t.Errorf("with -force it should seed anyway: %+v", forced)
	}
}

func TestTheSeededExampleWorksWithEveryReadCommand(t *testing.T) {
	t.Parallel()
	path := demoDB(t)

	pole := cli("pole", "-db", path, "-json", "-spans", "NW-441829")
	if pole.Code != cliapp.CodeOK {
		t.Fatalf("pole: %+v", pole)
	}
	var back cliapp.PoleJSON
	if err := json.Unmarshal([]byte(pole.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Attachments) != 4 || len(back.Spans) != 1 {
		t.Errorf("pole = %+v", back)
	}

	load := cli("load", "-db", path, "-default-wire", "2 ACSR SPARROW", "NW-441829")
	if load.Code != cliapp.CodeOK && load.Code != cliapp.CodeFindings {
		t.Fatalf("load: %+v", load)
	}
	if !strings.Contains(load.Stdout, "NW-441829") {
		t.Errorf("load = %q", load.Stdout)
	}

	sweep := cli("sweep", "-db", path)
	if sweep.Code != cliapp.CodeOK && sweep.Code != cliapp.CodeFindings {
		t.Fatalf("sweep: %+v", sweep)
	}
	if !strings.Contains(sweep.Stdout, "poles checked") {
		t.Errorf("sweep = %q", sweep.Stdout)
	}

	inspect := cli("inspect", "-db", path, "-year", "2026")
	if inspect.Code != cliapp.CodeFindings {
		t.Fatalf("inspect: %+v", inspect)
	}
	if !strings.Contains(inspect.Stdout, "NW-441831") {
		t.Errorf("the 1968 pole should be on the inspection list:\n%s", inspect.Stdout)
	}

	audit := cli("audit", "-db", path, "-found", "2026-05-01", "2025")
	if audit.Code != cliapp.CodeFindings {
		t.Fatalf("audit: %+v", audit)
	}
	if !strings.Contains(audit.Stdout, "UN31") {
		t.Errorf("the unclaimed cable should be found:\n%s", audit.Stdout)
	}

	bill := cli("bill", "-db", path, "-issued", "2026-01-15", "2025")
	if bill.Code != cliapp.CodeOK {
		t.Fatalf("bill: %+v", bill)
	}
	for _, want := range []string{"MERIDIAN FIBER", "RIDGEPORT CABLEVISION", "UNIDENTIFIED"} {
		if !strings.Contains(bill.Stdout, want) {
			t.Errorf("the bill run does not mention %q:\n%s", want, bill.Stdout)
		}
	}

	app := cli("application", "-db", path, "show", "APP-1042")
	if app.Code != cliapp.CodeOK {
		t.Fatalf("application: %+v", app)
	}
	if !strings.Contains(app.Stdout, "surveying") {
		t.Errorf("application = %q", app.Stdout)
	}

	compliance := cli("compliance", "-db", path, "-as-of", "2025-04-01", "APP-1042")
	if compliance.Code != cliapp.CodeOK && compliance.Code != cliapp.CodeFindings {
		t.Fatalf("compliance: %+v", compliance)
	}

	history := cli("history", "-db", path, "-since", "2020-01-01")
	if history.Code != cliapp.CodeOK {
		t.Fatalf("history: %+v", history)
	}
	if !strings.Contains(history.Stdout, "worked example seeded") {
		t.Errorf("history = %q", history.Stdout)
	}

	maintain := cli("maintain", "-db", path)
	if maintain.Code != cliapp.CodeOK {
		t.Fatalf("the seeded example should pass the integrity checks: %+v", maintain)
	}
}

func TestDemoRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	for _, args := range [][]string{
		{"demo", "-db", path, "extra"},
		{"demo", "-nonsense"},
	} {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d", args, got.Code)
		}
	}
}
