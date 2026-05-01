package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

const packetPlanJSON = `{
  "application": "APP-1042",
  "items": [
    {"id": "MR-0001", "action": "replace_pole", "pole": "NW-441829",
     "owner": "SUMMIT VALLEY ELECTRIC", "because": ["x"], "detail": "45/3 southern pine"},
    {"id": "MR-0002", "action": "transfer", "pole": "NW-441829", "subject": "A3",
     "owner": "RIDGEPORT CABLEVISION", "because": ["x"], "detail": "transfer to the new pole"},
    {"id": "MR-0003", "action": "raise", "pole": "NW-441830", "subject": "A4",
     "owner": "NORTHVALE TELEPHONE", "from_ft": "18.6", "to_ft": "19.6", "because": ["x"]}
  ]
}`

func TestPacketSplitsByPoleAndCrew(t *testing.T) {
	t.Parallel()
	got := cliIn(packetPlanJSON, "packet")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	for _, want := range []string{
		"NW-441829 line crew", "outage required", "NW-441829 communication crew",
		"waits on the structure work", "NW-441830 communication crew", "attachers:",
	} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
	line := strings.Index(got.Stdout, "NW-441829 line crew")
	comm := strings.Index(got.Stdout, "NW-441829 communication crew")
	if line < 0 || comm < 0 || line > comm {
		t.Errorf("the line crew should come first:\n%s", got.Stdout)
	}
}

func TestPacketAsJSON(t *testing.T) {
	t.Parallel()
	got := cliIn(packetPlanJSON, "packet", "-json")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back []cliapp.PacketJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if len(back) != 3 {
		t.Fatalf("packets = %v", back)
	}
	if back[0].Crew != "line" || !back[0].NeedsOutage {
		t.Errorf("packet = %+v", back[0])
	}
	if back[1].Crew != "communication" || back[1].NeedsOutage || !back[1].NeedsPole {
		t.Errorf("packet = %+v", back[1])
	}
	if back[2].Pole != "NW-441830" || back[2].NeedsPole {
		t.Errorf("packet = %+v", back[2])
	}
	if len(back[2].Items) != 1 || back[2].Items[0].From != "18.6" || back[2].Items[0].To != "19.6" {
		t.Errorf("items = %v", back[2].Items)
	}
	if len(back[0].Items) != 1 || back[0].Items[0].From != "" {
		t.Errorf("a pole replacement moves nothing, got %v", back[0].Items)
	}
}

func TestPacketForOneCrew(t *testing.T) {
	t.Parallel()
	got := cliIn(packetPlanJSON, "packet", "-json", "-crew", "line")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	var back []cliapp.PacketJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0].Crew != "line" {
		t.Errorf("packets = %v", back)
	}
}

func TestPacketListsTheOutages(t *testing.T) {
	t.Parallel()
	got := cliIn(packetPlanJSON, "packet", "-outages")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", got.Code)
	}
	if strings.TrimSpace(got.Stdout) != "NW-441829" {
		t.Errorf("stdout = %q", got.Stdout)
	}
	// A plan with nothing structural in it needs no outage at all.
	liftOnly := `{"application":"APP-1","items":[{"id":"MR-1","action":"raise",` +
		`"pole":"NW-441829","subject":"A3","from_ft":"20.0","to_ft":"20.6","because":["x"]}]}`
	quiet := cliIn(liftOnly, "packet", "-outages")
	if quiet.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", quiet.Code)
	}
	if !strings.Contains(quiet.Stdout, "no pole needs an outage") {
		t.Errorf("stdout = %q", quiet.Stdout)
	}
	asJSON := cliIn(liftOnly, "packet", "-outages", "-json")
	if strings.TrimSpace(asJSON.Stdout) != "[]" {
		t.Errorf("stdout = %q", asJSON.Stdout)
	}
}

func TestPacketOfAnEmptyPlan(t *testing.T) {
	t.Parallel()
	got := cliIn(`{"application":"APP-1","items":[]}`, "packet")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "no make ready") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestPacketRejectsBadUse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stdin string
		args  []string
	}{
		{packetPlanJSON, []string{"packet", "extra"}},
		{packetPlanJSON, []string{"packet", "-crew", "arborist"}},
		{packetPlanJSON, []string{"packet", "-crew", "unassigned"}},
		{"not json", []string{"packet"}},
		{packetPlanJSON, []string{"packet", "-nonsense"}},
	}
	for _, c := range cases {
		if got := cliIn(c.stdin, c.args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", c.args, got.Code, got.Stderr)
		}
	}
}
