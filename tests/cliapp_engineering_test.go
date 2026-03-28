package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/report"
)

// Every command that engineers something takes the same two overrides now, so
// they are checked once, against all of them, rather than once per command.
var engineeringCommands = []string{"load", "clear", "makeready", "sag"}

func TestEveryEngineeringCommandTakesBothOverrides(t *testing.T) {
	t.Parallel()
	for _, name := range engineeringCommands {
		c, ok := cliapp.Lookup(name)
		if !ok {
			t.Errorf("%s is not registered", name)
			continue
		}
		for _, want := range []string{"-district", "-grade"} {
			if !strings.Contains(c.Usage, want) {
				t.Errorf("%s does not document %s:\n%s", name, want, c.Usage)
			}
		}
	}
}

func TestEveryEngineeringCommandRefusesAnUnknownDistrict(t *testing.T) {
	t.Parallel()
	path := surveyedDB(t)
	for _, name := range engineeringCommands {
		args := []string{name, "-db", path, "-district", "tropical"}
		switch name {
		case "sag":
			args = append(args, "2 ACSR SPARROW")
		default:
			args = append(args, "NW-441829")
		}
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%s gave code %d (%q)", name, got.Code, got.Stderr)
		}
	}
}

func TestEveryEngineeringCommandRefusesAnUnknownGrade(t *testing.T) {
	t.Parallel()
	path := surveyedDB(t)
	for _, name := range engineeringCommands {
		args := []string{name, "-db", path, "-grade", "A"}
		switch name {
		case "sag":
			args = append(args, "2 ACSR SPARROW")
		default:
			args = append(args, "NW-441829")
		}
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%s gave code %d (%q)", name, got.Code, got.Stderr)
		}
	}
}

func TestTheConfigurationSuppliesTheDistrictWhenNoFlagDoes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := dir + "/downguy.yaml"
	if err := writeFile(cfg, "district: light\ngrade: B\n"); err != nil {
		t.Fatal(err)
	}
	path := loadedDB(t)
	got := cli("load", "-db", path, "-config", cfg, "-json",
		"-default-wire", "2 ACSR SPARROW", "NW-441829")
	if got.Code != cliapp.CodeOK && got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back report.AnalysisJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.District != "light" || back.Grade != "B" {
		t.Errorf("the file should supply both, got %+v", back)
	}
}

func TestTheFlagBeatsTheConfiguration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := dir + "/downguy.yaml"
	if err := writeFile(cfg, "district: light\ngrade: B\n"); err != nil {
		t.Fatal(err)
	}
	path := loadedDB(t)
	got := cli("load", "-db", path, "-config", cfg, "-json",
		"-district", "heavy", "-grade", "C",
		"-default-wire", "2 ACSR SPARROW", "NW-441829")
	if got.Code != cliapp.CodeOK && got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back report.AnalysisJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if back.District != "heavy" || back.Grade != "C" {
		t.Errorf("the flags should win, got %+v", back)
	}
}
