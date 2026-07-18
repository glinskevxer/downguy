package tests

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/report"
)

// Every step of an application has its own tests and they all pass. What this
// walks is the joins between them: the plan the makeready command writes has
// to be the plan the estimate command reads, the findings it writes have to be
// the findings the allocation keys on, and the packet command has to make
// sense of the same document. Those four shapes have broken against each other
// more than once, and no unit test on either side would have said so.

const e2ePoles = `pole_id,length_ft,class,species,owner,condition,installed_in
NW-441829,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1996
NW-441830,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1998
`

const e2eAttachments = `attachment_id,pole_id,owner,kind,height,authorized,in_force_from,voltage,face
PR29,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,y,2001-06-01,7620,
NE29,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,28.5,y,2001-06-01,,
CA29,NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,2019-04-01,,
FI29,NW-441829,MERIDIAN FIBER,fiber,19.6,y,2025-07-01,,
`

const e2eSpans = `span_id,from_pole,to_pole,length_ft,cover
SP-1,NW-441829,NW-441830,200,road
`

const e2eSurveys = `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-29,NW-441829,2025-05-06,CREW 41,sound,PR29,31.5,laser
SV-29,NW-441829,2025-05-06,CREW 41,sound,NE29,28.5,laser
SV-29,NW-441829,2025-05-06,CREW 41,sound,CA29,20.0,laser
SV-29,NW-441829,2025-05-06,CREW 41,sound,FI29,19.6,laser
`

func TestOneApplicationFromImportToInvoice(t *testing.T) {
	t.Parallel()
	path := tempDB(t)

	for _, step := range [][2]string{
		{e2ePoles, "poles"},
		{e2eAttachments, "attachments"},
		{e2eSpans, "spans"},
		{e2eSurveys, "surveys"},
	} {
		got := cliIn(step[0], "import", "-db", path,
			"-by", "j.aldana", "-on", "2025-05-06", step[1])
		if got.Code != cliapp.CodeOK {
			t.Fatalf("importing %s: %+v", step[1], got)
		}
	}

	// The pole carries what it is given.
	load := cli("load", "-db", path, "-json", "-default-wire", "2 ACSR SPARROW", "NW-441829")
	if load.Code != cliapp.CodeOK {
		t.Fatalf("load: %+v", load)
	}
	var analysis report.AnalysisJSON
	if err := json.Unmarshal([]byte(load.Stdout), &analysis); err != nil {
		t.Fatal(err)
	}
	if !analysis.Passes {
		t.Fatalf("the fixture pole should carry its load: %+v", analysis)
	}

	// The fibre is four tenths under the cable, which is the finding the whole
	// rest of the run hangs off.
	clear := cli("clear", "-db", path, "-json", "-applicant", "MERIDIAN FIBER", "NW-441829")
	if clear.Code != cliapp.CodeFindings {
		t.Fatalf("clear: %+v", clear)
	}
	var findings []report.FindingJSON
	if err := json.Unmarshal([]byte(clear.Stdout), &findings); err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("no findings to carry forward")
	}

	// The plan the makeready command writes.
	plan := cli("makeready", "-db", path, "-json",
		"-applicant", "MERIDIAN FIBER", "-application", "APP-1042", "NW-441829")
	if plan.Code != cliapp.CodeFindings {
		t.Fatalf("makeready: %+v", plan)
	}
	var planned report.PlanJSON
	if err := json.Unmarshal([]byte(plan.Stdout), &planned); err != nil {
		t.Fatal(err)
	}
	if planned.Application != "APP-1042" || len(planned.Items) == 0 {
		t.Fatalf("plan = %+v", planned)
	}

	// The estimate command reads exactly what makeready wrote.
	estimate := cliIn(plan.Stdout, "estimate", "-json", "-on", "2025-06-01", "-quote")
	if estimate.Code != cliapp.CodeOK {
		t.Fatalf("estimate: %+v", estimate)
	}
	var quoted cliapp.EstimateJSON
	if err := json.Unmarshal([]byte(estimate.Stdout), &quoted); err != nil {
		t.Fatal(err)
	}
	if quoted.Application != "APP-1042" || len(quoted.Lines) == 0 {
		t.Fatalf("estimate = %+v", quoted)
	}
	if quoted.Total == quoted.Direct {
		t.Error("a quotation should carry its adjustments")
	}

	// So does the packet command.
	packets := cliIn(plan.Stdout, "packet", "-json")
	if packets.Code != cliapp.CodeFindings {
		t.Fatalf("packet: %+v", packets)
	}
	var sheets []cliapp.PacketJSON
	if err := json.Unmarshal([]byte(packets.Stdout), &sheets); err != nil {
		t.Fatal(err)
	}
	if len(sheets) == 0 {
		t.Fatal("no crew sheets came out of the plan")
	}
	planItems := 0
	for _, s := range sheets {
		planItems += len(s.Items)
	}
	if planItems != len(planned.Items) {
		t.Errorf("the packets carry %d items and the plan has %d", planItems, len(planned.Items))
	}

	// The plan document carries the attributed findings the allocation keys
	// on. Handing it the plain clearance output instead would not work: those
	// findings are unattributed, and an unattributed finding cannot be billed.
	if len(planned.Findings) == 0 {
		t.Fatal("the plan should carry the findings it was worked out from")
	}
	for _, f := range planned.Findings {
		if f.Blame == "unattributed" {
			t.Errorf("the plan carries an unattributed finding: %+v", f)
		}
	}
	findingsFile := filepath.Join(t.TempDir(), "findings.json")
	if err := writeFile(findingsFile, plan.Stdout); err != nil {
		t.Fatal(err)
	}
	split := cliIn(plan.Stdout, "allocate", "-json", "-on", "2025-06-01",
		"-owner", "SUMMIT VALLEY ELECTRIC=OWN-1",
		"-applicant", "MERIDIAN FIBER=ATT-9",
		"-attacher", "RIDGEPORT CABLEVISION=ATT-3",
		"-findings", findingsFile)
	if split.Code != cliapp.CodeOK {
		t.Fatalf("allocate: %+v", split)
	}
	var allocated cliapp.SplitJSON
	if err := json.Unmarshal([]byte(split.Stdout), &allocated); err != nil {
		t.Fatal(err)
	}
	if len(allocated.Charges) == 0 || len(allocated.Invoices) == 0 {
		t.Fatalf("allocation = %+v", allocated)
	}
	if len(allocated.Unallocated) != 0 {
		t.Errorf("every item should have been billable, got %v", allocated.Unallocated)
	}

	// And the annual rental still runs over the same plant.
	bill := cli("bill", "-db", path, "-json", "-issued", "2026-01-15", "2025")
	if bill.Code != cliapp.CodeOK {
		t.Fatalf("bill: %+v", bill)
	}
	var bills []cliapp.BillJSON
	if err := json.Unmarshal([]byte(bill.Stdout), &bills); err != nil {
		t.Fatal(err)
	}
	if len(bills) != 2 {
		t.Fatalf("two attachers are on the pole, got %v", bills)
	}
	for _, b := range bills {
		if b.Total == "" || b.Year != 2025 {
			t.Errorf("bill = %+v", b)
		}
	}

	// The import left a trail.
	history := cli("history", "-db", path, "-json", "-since", "2020-01-01")
	if history.Code != cliapp.CodeOK {
		t.Fatalf("history: %+v", history)
	}
	var changes []cliapp.ChangeJSON
	if err := json.Unmarshal([]byte(history.Stdout), &changes); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 4 {
		t.Errorf("four imports, four entries, got %v", changes)
	}

	// And nothing about the run left the database inconsistent.
	maintain := cli("maintain", "-db", path)
	if maintain.Code != cliapp.CodeOK {
		t.Fatalf("the integrity checks found something: %+v", maintain)
	}
}

func TestTheExitCodesAreWhatAPipelineBranchesOn(t *testing.T) {
	t.Parallel()
	path := demoDB(t)
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"a clean read", []string{"pole", "-db", path, "NW-441829"}, cliapp.CodeOK},
		{"a missing record", []string{"pole", "-db", path, "NW-999999"}, cliapp.CodeFindings},
		{"a bad command line", []string{"pole", "-db", path}, cliapp.CodeUsage},
		{"an unknown command", []string{"frobnicate"}, cliapp.CodeUsage},
		{"a clean sweep target", []string{"maintain", "-db", path}, cliapp.CodeOK},
		{"findings", []string{"audit", "-db", path, "-found", "2026-05-01", "2025"}, cliapp.CodeFindings},
	}
	for _, c := range cases {
		if got := cli(c.args...); got.Code != c.want {
			t.Errorf("%s: %v gave %d, want %d (%q)", c.name, c.args, got.Code, c.want, got.Stderr)
		}
	}
}

func TestNothingWritesToTheWrongStream(t *testing.T) {
	t.Parallel()
	path := demoDB(t)
	// A successful run says nothing on standard error, and a refused one says
	// nothing on standard output. A pipeline reading one and logging the other
	// depends on both.
	ok := cli("pole", "-db", path, "NW-441829")
	if ok.Stderr != "" {
		t.Errorf("a clean run wrote to standard error: %q", ok.Stderr)
	}
	bad := cli("pole", "-db", path, "nope")
	if bad.Stdout != "" {
		t.Errorf("a refused run wrote to standard output: %q", bad.Stdout)
	}
	if !strings.Contains(bad.Stderr, "nope") {
		t.Errorf("the refusal should name what it refused: %q", bad.Stderr)
	}
}
