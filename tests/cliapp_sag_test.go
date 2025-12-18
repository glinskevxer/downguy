package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func TestSagPrintsARun(t *testing.T) {
	got := cli("sag", "2 ACSR SPARROW")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"2 ACSR SPARROW", "heavy district", "bare at 60 F", "deepest sag"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestSagTakesASpanAndARulingSpan(t *testing.T) {
	short := cli("sag", "-span", "150", "2 ACSR SPARROW")
	long := cli("sag", "-span", "320", "2 ACSR SPARROW")
	if short.Code != cliapp.CodeOK || long.Code != cliapp.CodeOK {
		t.Fatalf("short %+v long %+v", short, long)
	}
	if !strings.Contains(long.Stdout, "over 320 ft") {
		t.Errorf("stdout = %q", long.Stdout)
	}
	if !strings.Contains(long.Stdout, "ruling span 320 ft") {
		t.Errorf("the ruling span should default to the span, got %q", long.Stdout)
	}
	separate := cli("sag", "-span", "320", "-ruling-span", "240", "2 ACSR SPARROW")
	if !strings.Contains(separate.Stdout, "ruling span 240 ft") {
		t.Errorf("stdout = %q", separate.Stdout)
	}
}

func TestSagOverTensionReportsFindings(t *testing.T) {
	got := cli("sag", "-district", "light", "2 ACSR SPARROW")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("a line designed for the light district runs over under ice: %+v", got)
	}
	if !strings.Contains(got.Stdout, "over the working limit") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestSagAsJSON(t *testing.T) {
	got := cli("sag", "-json", "-span", "225", "4/0 ACSR PENGUIN")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back cliapp.SagTableJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Conductor != "4/0 ACSR PENGUIN" || back.District != "heavy" {
		t.Errorf("table = %+v", back)
	}
	if back.Span != 225 || back.RulingSpan != 225 {
		t.Errorf("table = %+v", back)
	}
	if len(back.Rows) != 9 {
		t.Errorf("rows = %d", len(back.Rows))
	}
	iced := 0
	for _, r := range back.Rows {
		if r.Sag <= 0 || r.Tension <= 0 {
			t.Errorf("row = %+v", r)
		}
		if r.Iced {
			iced++
		}
	}
	if iced != 2 {
		t.Errorf("the heavy and medium districts carry ice, got %d iced rows", iced)
	}
}

func TestSagJSONFlagsOverTension(t *testing.T) {
	got := cli("sag", "-json", "-district", "light", "2 ACSR SPARROW")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	var back cliapp.SagTableJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	flagged := 0
	for _, r := range back.Rows {
		if r.OverTension {
			flagged++
		}
	}
	if flagged == 0 {
		t.Errorf("nothing flagged: %+v", back.Rows)
	}
}

func TestSagRejectsBadUse(t *testing.T) {
	cases := [][]string{
		{"sag"},
		{"sag", "a", "b"},
		{"sag", "baling wire"},
		{"sag", "-district", "tropical", "2 ACSR SPARROW"},
		{"sag", "-span", "0", "2 ACSR SPARROW"},
		{"sag", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d", args, got.Code)
		}
	}
}

func TestConductorsLists(t *testing.T) {
	got := cli("conductors")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"2 ACSR SPARROW", "3/8 EHS MESSENGER", "communication", "supply"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestConductorsAsJSON(t *testing.T) {
	got := cli("conductors", "-json")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	var back []struct {
		Code     string  `json:"code"`
		Kind     string  `json:"kind"`
		Diameter float64 `json:"diameter_in"`
	}
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) < 8 {
		t.Errorf("conductors = %d", len(back))
	}
	for _, c := range back {
		if c.Code == "" || c.Diameter <= 0 {
			t.Errorf("conductor = %+v", c)
		}
	}
}

func TestConductorsRejectsBadUse(t *testing.T) {
	if got := cli("conductors", "extra"); got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if got := cli("conductors", "-nonsense"); got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
}
