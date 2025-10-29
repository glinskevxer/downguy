package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/rental"
)

func TestDefaultsValidate(t *testing.T) {
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the defaults do not validate: %v", err)
	}
	district, err := cfg.LoadingDistrict()
	if err != nil || district != nesc.Heavy {
		t.Errorf("district = %q %v", district, err)
	}
	grade, err := cfg.ConstructionGrade()
	if err != nil || grade != nesc.GradeC {
		t.Errorf("grade = %q %v", grade, err)
	}
	policy, err := cfg.AllocationPolicy()
	if err != nil || policy.Name != allocate.FederalPolicy().Name {
		t.Errorf("policy = %+v %v", policy, err)
	}
	formula, err := cfg.RateFormula()
	if err != nil || formula != rental.FormulaCable {
		t.Errorf("formula = %q %v", formula, err)
	}
}

func TestLoadWithNoFileGivesTheDefaults(t *testing.T) {
	got, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if got != config.Default() {
		t.Errorf("config = %+v", got)
	}
	got, err = config.Load(filepath.Join(t.TempDir(), "nothing.yaml"))
	if err != nil {
		t.Fatalf("a missing file is not an error: %v", err)
	}
	if got != config.Default() {
		t.Errorf("config = %+v", got)
	}
}

func TestLoadOverlaysAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downguy.yaml")
	body := "district: light\ngrade: B\npolicy: state commission\nattachers: 3\nhttp:\n  address: 0.0.0.0:9000\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.District != "light" || got.Grade != "B" {
		t.Errorf("config = %+v", got)
	}
	if got.Attachers != 3 || got.HTTP.Address != "0.0.0.0:9000" {
		t.Errorf("config = %+v", got)
	}
	// Anything the file did not mention keeps its default.
	if got.Database != config.Default().Database {
		t.Errorf("database = %q", got.Database)
	}
	if got.HTTP.ReadTimeout != config.Default().HTTP.ReadTimeout {
		t.Errorf("read timeout = %d", got.HTTP.ReadTimeout)
	}
	policy, err := got.AllocationPolicy()
	if err != nil || policy.PreExistingPayer != allocate.PayerShared {
		t.Errorf("policy = %+v %v", policy, err)
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(broken, []byte("district: [not a string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(broken); err == nil {
		t.Error("unparseable yaml should be rejected")
	}
	wrong := filepath.Join(dir, "wrong.yaml")
	if err := os.WriteFile(wrong, []byte("district: tropical\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(wrong); err == nil {
		t.Error("an unknown district should be rejected")
	}
}

func TestValidateCatchesEverySetting(t *testing.T) {
	cases := map[string]func(*config.Config){
		"no database":  func(c *config.Config) { c.Database = " " },
		"no utility":   func(c *config.Config) { c.Utility = "" },
		"district":     func(c *config.Config) { c.District = "tropical" },
		"grade":        func(c *config.Config) { c.Grade = "A" },
		"policy":       func(c *config.Config) { c.Policy = "municipal" },
		"formula":      func(c *config.Config) { c.Formula = "wireless" },
		"no attachers": func(c *config.Config) { c.Attachers = 0 },
		"too many":     func(c *config.Config) { c.Attachers = 40 },
		"no address":   func(c *config.Config) { c.HTTP.Address = "" },
		"no read":      func(c *config.Config) { c.HTTP.ReadTimeout = 0 },
		"no write":     func(c *config.Config) { c.HTTP.WriteTimeout = -1 },
	}
	for name, mutate := range cases {
		cfg := config.Default()
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s should have been rejected", name)
		}
	}
}

func TestFromEnvOverlays(t *testing.T) {
	env := map[string]string{
		"DOWNGUY_DATABASE":  "/var/lib/downguy/plant.db",
		"DOWNGUY_ADDRESS":   "0.0.0.0:9100",
		"DOWNGUY_DISTRICT":  "medium",
		"DOWNGUY_ATTACHERS": "7",
	}
	lookup := func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
	got, err := config.Default().FromEnv(lookup)
	if err != nil {
		t.Fatal(err)
	}
	if got.Database != "/var/lib/downguy/plant.db" || got.HTTP.Address != "0.0.0.0:9100" {
		t.Errorf("config = %+v", got)
	}
	if got.District != "medium" || got.Attachers != 7 {
		t.Errorf("config = %+v", got)
	}
	env["DOWNGUY_ATTACHERS"] = "seven"
	if _, err := config.Default().FromEnv(lookup); err == nil {
		t.Error("a non numeric attacher count should be rejected")
	}
	env["DOWNGUY_ATTACHERS"] = "0"
	if _, err := config.Default().FromEnv(lookup); err == nil {
		t.Error("a zero attacher count should be rejected")
	}
}

func TestFromEnvIgnoresBlanksAndAbsences(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "DOWNGUY_DATABASE" {
			return "   ", true
		}
		return "", false
	}
	got, err := config.Default().FromEnv(lookup)
	if err != nil {
		t.Fatal(err)
	}
	if got != config.Default() {
		t.Errorf("config = %+v", got)
	}
}

func TestMarshalRoundTrips(t *testing.T) {
	out, err := config.Default().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{"database:", "district: heavy", "attachers: 5", "address:"} {
		if !strings.Contains(text, want) {
			t.Errorf("the marshalled config does not mention %q:\n%s", want, text)
		}
	}
	path := filepath.Join(t.TempDir(), "round.yaml")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	back, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if back != config.Default() {
		t.Errorf("round trip = %+v", back)
	}
}
