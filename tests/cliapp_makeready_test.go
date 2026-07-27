package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/report"
)

const crowdedAttachments = `attachment_id,pole_id,owner,kind,height,authorized,voltage
A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,y,7620
A2,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,28.5,y,
A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,
A7,NW-441829,MERIDIAN FIBER,fiber,19.6,y,
`

const crowdedSurvey = `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-8001,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-8001,NW-441829,2025-05-06,CREW 41,sound,A2,28.5,laser
SV-8001,NW-441829,2025-05-06,CREW 41,sound,A3,20.0,laser
SV-8001,NW-441829,2025-05-06,CREW 41,sound,A7,19.6,laser
`

func crowdedDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	for _, step := range [][2]string{
		{poleCSV, "poles"},
		{crowdedAttachments, "attachments"},
		{crowdedSurvey, "surveys"},
	} {
		if got := cliIn(step[0], "import", "-db", path, step[1]); got.Code != cliapp.CodeOK {
			t.Fatalf("%s: %+v", step[1], got)
		}
	}
	return path
}

func TestMakereadyPlansTheWork(t *testing.T) {
	t.Parallel()
	path := crowdedDB(t)
	got := cli("makeready", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("four tenths between two attachers should need work: %+v", got)
	}
	// The applicant's fiber is the bottom facility, so there is nothing under
	// it to drop into and the cable above has to come up instead. That is the
	// answer, and it is why the allocation still lands on the applicant.
	for _, want := range []string{"APP-NW-441829", "raise", "A3", "no room under A7"} {
		if !strings.Contains(got.Stdout, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got.Stdout)
		}
	}
}

func TestMakereadyLowersWhenThereIsRoomUnderneath(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	atts := `attachment_id,pole_id,owner,kind,height,authorized,voltage
A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,y,7620
A2,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,28.5,y,
A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,
A7,NW-441829,MERIDIAN FIBER,fiber,19.6,y,
A8,NW-441829,NORTHVALE TELEPHONE,comm_cable,16.0,y,
`
	sheet := `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-8003,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-8003,NW-441829,2025-05-06,CREW 41,sound,A2,28.5,laser
SV-8003,NW-441829,2025-05-06,CREW 41,sound,A3,20.0,laser
SV-8003,NW-441829,2025-05-06,CREW 41,sound,A7,19.6,laser
SV-8003,NW-441829,2025-05-06,CREW 41,sound,A8,16.0,laser
`
	for _, step := range [][2]string{{poleCSV, "poles"}, {atts, "attachments"}, {sheet, "surveys"}} {
		if got := cliIn(step[0], "import", "-db", path, step[1]); got.Code != cliapp.CodeOK {
			t.Fatalf("%s: %+v", step[1], got)
		}
	}
	got := cli("makeready", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stdout = %q", got.Code, got.Stdout)
	}
	if !strings.Contains(got.Stdout, "lower") || !strings.Contains(got.Stdout, "A7") {
		t.Errorf("with three and a half feet under it the fiber should drop:\n%s", got.Stdout)
	}
}

func TestMakereadyAttributesToTheApplicant(t *testing.T) {
	t.Parallel()
	path := crowdedDB(t)
	got := cli("makeready", "-db", path, "-json",
		"-applicant", "MERIDIAN FIBER", "-application", "APP-1042", "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var back report.PlanJSON
	if err := json.Unmarshal([]byte(got.Stdout), &back); err != nil {
		t.Fatalf("%v\n%s", err, got.Stdout)
	}
	if back.Application != "APP-1042" {
		t.Errorf("application = %q", back.Application)
	}
	if len(back.Items) == 0 {
		t.Fatalf("no work planned: %s", got.Stdout)
	}
	cited := false
	for _, item := range back.Items {
		for _, because := range item.Because {
			if strings.Contains(because, "A7") {
				cited = true
			}
		}
		if item.ID == "" || item.Action == "" || item.Pole != "NW-441829" {
			t.Errorf("item = %+v", item)
		}
	}
	if !cited {
		t.Errorf("the work should cite the finding the applicant's fiber caused, got %v", back.Items)
	}
}

func TestMakereadyOnACleanPole(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	atts := `attachment_id,pole_id,owner,kind,height,authorized,voltage
A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,y,7620
A2,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,28.5,y,
A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,20.0,y,
`
	survey := `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-8002,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-8002,NW-441829,2025-05-06,CREW 41,sound,A2,28.5,laser
SV-8002,NW-441829,2025-05-06,CREW 41,sound,A3,20.0,laser
`
	for _, step := range [][2]string{{poleCSV, "poles"}, {atts, "attachments"}, {survey, "surveys"}} {
		if got := cliIn(step[0], "import", "-db", path, step[1]); got.Code != cliapp.CodeOK {
			t.Fatalf("%s: %+v", step[1], got)
		}
	}
	got := cli("makeready", "-db", path, "NW-441829")
	if got.Code != cliapp.CodeOK {
		t.Fatalf("a clean pole needs no work: %+v", got)
	}
	if !strings.Contains(got.Stdout, "no make ready") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestMakereadyNeedsASurvey(t *testing.T) {
	t.Parallel()
	path := loadedDB(t)
	got := cli("makeready", "-db", path, "NW-441830")
	if got.Code != cliapp.CodeFindings {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "no survey") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestMakereadyRejectsBadUse(t *testing.T) {
	t.Parallel()
	path := crowdedDB(t)
	cases := [][]string{
		{"makeready", "-db", path},
		{"makeready", "-db", path, "NW-441829", "NW-441830"},
		{"makeready", "-db", path, "nope"},
		{"makeready", "-db", path, "-grade", "A", "NW-441829"},
		{"makeready", "-db", path, "-district", "tropical", "NW-441829"},
		{"makeready", "-nonsense"},
	}
	for _, args := range cases {
		if got := cli(args...); got.Code != cliapp.CodeUsage {
			t.Errorf("%v gave code %d (%q)", args, got.Code, got.Stderr)
		}
	}
	if got := cli("makeready", "-db", path, "NW-999999"); got.Code != cliapp.CodeFindings {
		t.Errorf("a missing pole gave code %d", got.Code)
	}
}

const stackedAttachments = `attachment_id,pole_id,owner,kind,height,authorized,voltage,face
A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,y,7620,
A2,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,28.5,y,,
A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,18.6,y,,
A6,NW-441829,RIDGEPORT CABLEVISION,power_supply,16.8,y,,climbing
`

const stackedSurvey = `survey_id,pole_id,taken,crew,condition,subject,height,method
SV-8100,NW-441829,2025-05-06,CREW 41,sound,A1,31.5,laser
SV-8100,NW-441829,2025-05-06,CREW 41,sound,A2,28.5,laser
SV-8100,NW-441829,2025-05-06,CREW 41,sound,A3,18.6,laser
SV-8100,NW-441829,2025-05-06,CREW 41,sound,A6,16.8,stick
`

func stackedDB(t *testing.T) string {
	t.Helper()
	path := tempDB(t)
	for _, step := range [][2]string{
		{poleCSV, "poles"},
		{stackedAttachments, "attachments"},
		{stackedSurvey, "surveys"},
	} {
		if got := cliIn(step[0], "import", "-db", path, step[1]); got.Code != cliapp.CodeOK {
			t.Fatalf("%s: %+v", step[1], got)
		}
	}
	return path
}

func TestMakereadyFoldsTheFindingsAFacilityIsAlreadyMovingFor(t *testing.T) {
	t.Parallel()
	path := stackedDB(t)
	// The power supply is bolted to the climbing face, which forces it to move
	// outright, and it is also eighteen tenths under the cable above it, which
	// on its own would be a separate lift. One facility, two findings, one job.
	folded := cli("makeready", "-db", path, "NW-441829")
	if folded.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", folded.Code, folded.Stdout, folded.Stderr)
	}
	if !strings.Contains(folded.Stdout, "covered by work already planned") {
		t.Errorf("stdout does not mention the suppression:\n%s", folded.Stdout)
	}
	if !strings.Contains(folded.Stdout, "NESC-238A") {
		t.Errorf("the folded finding should be named:\n%s", folded.Stdout)
	}

	every := cli("makeready", "-db", path, "-json", "-every-finding", "NW-441829")
	if every.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", every.Code)
	}
	var withAll report.PlanJSON
	if err := json.Unmarshal([]byte(every.Stdout), &withAll); err != nil {
		t.Fatal(err)
	}
	collapsed := cli("makeready", "-db", path, "-json", "NW-441829")
	var withFolded report.PlanJSON
	if err := json.Unmarshal([]byte(collapsed.Stdout), &withFolded); err != nil {
		t.Fatal(err)
	}
	if len(withFolded.Items) >= len(withAll.Items) {
		t.Errorf("folding should plan less work: %d against %d", len(withFolded.Items), len(withAll.Items))
	}
	if !strings.Contains(collapsed.Stderr, "covered:") {
		t.Errorf("the json run should still say what it folded, got %q", collapsed.Stderr)
	}
}

func TestMakereadySaysNothingAboutTheStructureUntilTheWiresAreNamed(t *testing.T) {
	t.Parallel()
	path := crowdedDB(t)
	quiet := cli("makeready", "-db", path, "NW-441829")
	if quiet.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d", quiet.Code)
	}
	if strings.Contains(quiet.Stdout, "carries the new attachment") || strings.Contains(quiet.Stdout, "%") {
		t.Errorf("with no conductor named there is nothing to work a load out from:\n%s", quiet.Stdout)
	}
}

func TestMakereadyChecksTheStructureWhenItCan(t *testing.T) {
	t.Parallel()
	path := crowdedDB(t)
	if got := cliIn("span_id,from_pole,to_pole,length_ft,cover\nSP-1,NW-441829,NW-441830,200,road\n",
		"import", "-db", path, "spans"); got.Code != cliapp.CodeOK {
		t.Fatal(got.Stderr)
	}
	got := cli("makeready", "-db", path, "-default-wire", "2 ACSR SPARROW", "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	if !strings.Contains(got.Stdout, "NW-441829") || !strings.Contains(got.Stdout, "%") {
		t.Errorf("the structural line should print:\n%s", got.Stdout)
	}
}

func TestMakereadyPlansThePoleWhenItWillNotCarryTheLoad(t *testing.T) {
	t.Parallel()
	path := tempDB(t)
	poles := "pole_id,length_ft,class,species,owner,condition\n" +
		"NW-441829,30,7,SPF,SUMMIT VALLEY ELECTRIC,restorable\n" +
		"NW-441830,30,7,SPF,SUMMIT VALLEY ELECTRIC,sound\n"
	atts := "attachment_id,pole_id,owner,kind,height,authorized,voltage\n" +
		"A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,23.5,y,7620\n" +
		"A2,NW-441829,SUMMIT VALLEY ELECTRIC,neutral,20.0,y,\n" +
		"A3,NW-441829,RIDGEPORT CABLEVISION,comm_cable,16.0,y,\n"
	sheet := "survey_id,pole_id,taken,crew,condition,subject,height,method\n" +
		"SV-1,NW-441829,2025-05-06,CREW 41,restorable,A1,23.5,laser\n" +
		"SV-1,NW-441829,2025-05-06,CREW 41,restorable,A2,20.0,laser\n" +
		"SV-1,NW-441829,2025-05-06,CREW 41,restorable,A3,16.0,laser\n"
	spans := "span_id,from_pole,to_pole,length_ft,cover\nSP-1,NW-441829,NW-441830,400,road\n"
	for _, step := range [][2]string{{poles, "poles"}, {atts, "attachments"}, {spans, "spans"}, {sheet, "surveys"}} {
		if got := cliIn(step[0], "import", "-db", path, step[1]); got.Code != cliapp.CodeOK {
			t.Fatalf("%s: %+v", step[1], got)
		}
	}
	got := cli("makeready", "-db", path, "-json", "-grade", "B",
		"-default-wire", "336 ACSR MERLIN", "NW-441829")
	if got.Code != cliapp.CodeFindings {
		t.Fatalf("code = %d, stderr = %q", got.Code, got.Stderr)
	}
	var plan report.PlanJSON
	if err := json.Unmarshal([]byte(got.Stdout), &plan); err != nil {
		t.Fatal(err)
	}
	replace := false
	for _, item := range plan.Items {
		if item.Action == "replace_pole" {
			replace = true
		}
	}
	if !replace {
		t.Errorf("a class 7 pole under grade B should have its replacement planned, got %v", plan.Items)
	}
	if plan.Items[0].Action != "replace_pole" {
		t.Errorf("the pole comes before anything on it, got %v", plan.Items)
	}
	cited := false
	for _, f := range plan.Findings {
		if f.Rule == "NESC-250B" {
			cited = true
		}
	}
	if !cited {
		t.Errorf("the structural finding should be on the document, got %v", plan.Findings)
	}
}
