package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func chaseDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	openApplication(t, path, "APP-1042", "MERIDIAN FIBER")
	openApplication(t, path, "APP-1043", "CASCADE BROADBAND")
	walkApplication(t, path, "APP-1042", [][2]string{{"submitted", "2025-03-05"}})
	walkApplication(t, path, "APP-1043", [][2]string{{"submitted", "2025-05-05"}})
	return path
}

func TestChaseListsWorstFirst(t *testing.T) {
	t.Parallel()
	path := chaseDB(t)
	got := cli("chase", "-db", path, "-as-of", "2025-05-06")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{"APP-1042", "APP-1043", "overdue", "SUMMIT VALLEY ELECTRIC", "need chasing today"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	late := strings.Index(got.Stdout, "APP-1042")
	fresh := strings.Index(got.Stdout, "APP-1043")
	if late < 0 || fresh < 0 || late > fresh {
		t.Errorf("the overdue one comes first:\n%s", got.Stdout)
	}
}

func TestChaseNowFiltersToWhatIsUrgent(t *testing.T) {
	t.Parallel()
	path := chaseDB(t)
	got := cli("chase", "-db", path, "-json", "-as-of", "2025-05-06", "-now")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	var back []cliapp.ReminderJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) != 1 || back[0].Application != "APP-1042" {
		t.Fatalf("reminders = %v", back)
	}
	r := back[0]
	if r.Urgency != "overdue" || r.Remaining >= 0 {
		t.Errorf("reminder = %+v", r)
	}
	if r.HeldBy != "utility" || r.Cite == "" || r.Due == "" {
		t.Errorf("reminder = %+v", r)
	}
}

func TestChaseByParty(t *testing.T) {
	t.Parallel()
	path := chaseDB(t)
	got := cli("chase", "-db", path, "-json", "-as-of", "2025-05-06", "-party", "MERIDIAN FIBER")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("both clocks are the utility's, so the fiber company has nothing: %+v", got)
	}
	if strings.TrimSpace(got.Stdout) != "[]" {
		t.Errorf("stdout = %q", got.Stdout)
	}
	utility := cli("chase", "-db", path, "-json", "-as-of", "2025-05-06", "-party", "SUMMIT VALLEY ELECTRIC")
	var back []cliapp.ReminderJSON
	if err := json.Unmarshal([]byte(utility.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 2 {
		t.Errorf("reminders = %v", back)
	}
}

func TestChaseWithNothingToChase(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	got := cli("chase", "-db", path, "-as-of", "2025-05-06")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "nothing to chase") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestChaseEarlyEnoughThatNothingIsUrgent(t *testing.T) {
	t.Parallel()
	path := chaseDB(t)
	got := cli("chase", "-db", path, "-as-of", "2025-05-07")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("the March application is still overdue: %+v", got)
	}
	early := cli("chase", "-db", path, "-json", "-as-of", "2025-05-06", "-party", "NOBODY")
	if early.Code != cliapp.CodeOK {
		t.Errorf("code = %d", early.Code)
	}
}

func TestChaseRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := chaseDB(t)
	cases := [][]string{
		{"chase", "-db", path},
		{"chase", "-db", path, "-as-of", "not-a-date"},
		{"chase", "-db", path, "-as-of", "2019-05-06"},
		{"chase", "-db", path, "-as-of", "2025-05-06", "extra"},
		{"chase", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
}
