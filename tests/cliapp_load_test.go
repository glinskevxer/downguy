package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/report"
)

func TestLoadRunsTheGroundlineCheck(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("load", "-db", path,
		"-wire", "A1=2 ACSR SPARROW",
		"-wire", "A3=3/8 EHS MESSENGER",
		"-wire", "A9=1/4 EHS MESSENGER",
		"NW-441829")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q, stdout = %q", got.Code, got.Stderr, got.Stdout)
	}
	for _, want := range []string{"NW-441829", "pole wind", "wire wind", "capacity", "total"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestLoadWithADefaultWire(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("load", "-db", path, "-default-wire", "2 ACSR SPARROW", "NW-441829")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "ok") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestLoadAsJSON(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("load", "-db", path, "-json", "-default-wire", "2 ACSR SPARROW", "NW-441829")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back report.AnalysisJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Pole != "NW-441829" || back.District != "heavy" || back.Grade != "C" {
		t.Errorf("analysis = %+v", back)
	}
	if !back.Passes || back.Utilisation <= 0 {
		t.Errorf("analysis = %+v", back)
	}
	if len(back.Components) < 4 {
		t.Errorf("components = %v", back.Components)
	}
}

func TestLoadOverridesTheDistrictAndGrade(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	base := cli("load", "-db", path, "-json", "-default-wire", "2 ACSR SPARROW", "NW-441829")
	strict := cli("load", "-db", path, "-json", "-grade", "B", "-default-wire", "2 ACSR SPARROW", "NW-441829")
	if base.Code != cliapp.CodeOK || strict.Code != cliapp.CodeOK {
		t.Fatalf("base %+v strict %+v", base, strict)
	}
	var a, b report.AnalysisJSON
	if err := json.Unmarshal([]byte(base.Stdout), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(strict.Stdout), &b); err != nil {
		t.Fatal(err)
	}
	if b.Grade != "B" {
		t.Errorf("grade = %q", b.Grade)
	}
	if b.Utilisation <= a.Utilisation {
		t.Errorf("grade B should be worse: %d against %d", b.Utilisation, a.Utilisation)
	}
	light := cli("load", "-db", path, "-json", "-district", "light", "-default-wire", "2 ACSR SPARROW", "NW-441829")
	var c report.AnalysisJSON
	if err := json.Unmarshal([]byte(light.Stdout), &c); err != nil {
		t.Fatal(err)
	}
	if c.District != "light" {
		t.Errorf("district = %q", c.District)
	}
}

func TestLoadReportsAnOverloadedPole(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	poles := "pole_id,length_ft,class,species,owner,condition\nNW-441829,30,7,SPF,SUMMIT VALLEY ELECTRIC,restorable\nNW-441830,30,7,SPF,SUMMIT VALLEY ELECTRIC,sound\n"
	if got := cliIn(poles, "import", "-db", path, "poles"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	atts := "attachment_id,pole_id,owner,kind,height,authorized,voltage\n" +
		"A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,23.5,y,7620\n" +
		"A2,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,20.0,y,\n" +
		"A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,16.0,y,\n"
	if got := cliIn(atts, "import", "-db", path, "attachments"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	spans := "span_id,from_pole,to_pole,length_ft,cover\nSP-1,NW-441829,NW-441830,400,road\n"
	if got := cliIn(spans, "import", "-db", path, "spans"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	got := cli("load", "-db", path, "-grade", "B", "-default-wire", "336 ACSR MERLIN", "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("a class 7 pole under grade B should be over: %+v", got)
	}
	if !strings.Contains(got.Stdout, "over") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestLoadRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	cases := [][]string{
		{"load", "-db", path},
		{"load", "-db", path, "NW-441829", "NW-441830"},
		{"load", "-db", path, "nope"},
		{"load", "-db", path, "-grade", "A", "NW-441829"},
		{"load", "-db", path, "-district", "tropical", "NW-441829"},
		{"load", "-db", path, "-wire", "no-equals-sign", "NW-441829"},
		{"load", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}

func TestLoadNeedsAConductorForEveryWire(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("load", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "no conductor assigned") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestLoadOnAPoleWithNoSpans(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("load", "-db", path, "-default-wire", "2 ACSR SPARROW", "SE-1129043")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "no spans") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestLoadOnAMissingPole(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("load", "-db", path, "-default-wire", "2 ACSR SPARROW", "NW-999999")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
}
