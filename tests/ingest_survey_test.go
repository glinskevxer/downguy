package tests

import (
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
)

const surveyCSV = `survey_id,pole_id,taken,crew,condition,ground_line,subject,height,method
SV-2201,NW-441829,2025-05-06,CREW 41,sound,0,A1,33.5,laser
SV-2201,NW-441829,2025-05-06,CREW 41,sound,0,A2,28.5,laser
SV-2201,NW-441829,2025-05-06,CREW 41,sound,0,A3,20.0,stick
SV-2202,NW-441830,2025-05-07,CREW 41,serviceable,0,B1,33.0,bucket
SV-2202,NW-441830,2025-05-07,CREW 41,serviceable,0,B3,19.6,stick
`

func TestImportSurveyRows(t *testing.T) {
	got, err := ingest.SurveyRows(strings.NewReader(surveyCSV))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Clean() || got.Count() != 5 {
		t.Fatalf("result = %+v", got)
	}
	first := got.Rows[0]
	if first.SurveyID != "SV-2201" || first.Pole != "NW-441829" {
		t.Errorf("row = %+v", first)
	}
	if first.Taken.String() != "2025-05-06" || first.Crew != "CREW 41" {
		t.Errorf("row = %+v", first)
	}
	if first.Condition != plant.ConditionSound {
		t.Errorf("condition = %s", first.Condition)
	}
	if first.Reading.Subject != "A1" || first.Reading.Method != survey.MethodLaser {
		t.Errorf("reading = %+v", first.Reading)
	}
	if first.Reading.Crew != "CREW 41" {
		t.Errorf("the reading should take the sheet's crew, got %q", first.Reading.Crew)
	}
}

func TestImportSurveysAssemblesSheets(t *testing.T) {
	got, err := ingest.Surveys(strings.NewReader(surveyCSV))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Clean() {
		t.Fatalf("problems = %v", got.Problems)
	}
	if got.Count() != 2 {
		t.Fatalf("sheets = %d", got.Count())
	}
	if got.Rows[0].ID != "SV-2201" || got.Rows[1].ID != "SV-2202" {
		t.Errorf("sheets should come back in id order, got %v", got.Rows)
	}
	if len(got.Rows[0].Measurements) != 3 {
		t.Errorf("readings = %v", got.Rows[0].Measurements)
	}
	if got.Rows[1].Condition != plant.ConditionServiceabe {
		t.Errorf("condition = %s", got.Rows[1].Condition)
	}
	for _, s := range got.Rows {
		if err := s.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", s.ID, err)
		}
	}
}

func TestImportSurveysRejectsARowThatDisagrees(t *testing.T) {
	body := surveyCSV + "SV-2201,NW-441840,2025-05-06,CREW 41,sound,0,A4,18.6,stick\n"
	got, err := ingest.Surveys(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 2 {
		t.Errorf("sheets = %d", got.Count())
	}
	if len(got.Problems) != 1 || got.Problems[0].Column != "survey_id" {
		t.Fatalf("problems = %v", got.Problems)
	}
	if !strings.Contains(got.Problems[0].Error(), "already covers") {
		t.Errorf("problem = %v", got.Problems[0])
	}
	for _, s := range got.Rows {
		if s.ID == "SV-2201" && len(s.Measurements) != 3 {
			t.Errorf("the disagreeing row should not have joined the sheet, got %v", s.Measurements)
		}
	}
}

func TestImportSurveysRejectsASheetThatWillNotValidate(t *testing.T) {
	body := "survey_id,pole_id,taken,crew,subject,height,method\nSV-1,NW-441829,2025-05-06,CREW 41,A1,33.5,laser\nSV-1,NW-441829,2025-05-06,CREW 41,A1,33.6,laser\n"
	got, err := ingest.Surveys(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 0 {
		t.Errorf("a sheet reading the same subject twice should be rejected, got %v", got.Rows)
	}
	if len(got.Problems) != 1 {
		t.Errorf("problems = %v", got.Problems)
	}
}

func TestImportSurveysCollectsBadRows(t *testing.T) {
	body := surveyCSV + `,NW-441829,2025-05-06,CREW 41,sound,0,A4,18.6,stick
SV-2203,nope,2025-05-06,CREW 41,sound,0,A1,18.6,stick
SV-2204,NW-441829,not-a-date,CREW 41,sound,0,A1,18.6,stick
SV-2205,NW-441829,2025-05-06,CREW 41,sound,high,A1,18.6,stick
SV-2206,NW-441829,2025-05-06,CREW 41,sound,0,A1,tall,stick
SV-2207,NW-441829,2025-05-06,CREW 41,sound,0,A1,18.6,hunch
SV-2208,NW-441829,2025-05-06,CREW 41,sound,0,,18.6,stick
`
	got, err := ingest.Surveys(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 2 {
		t.Errorf("sheets = %d", got.Count())
	}
	if len(got.Problems) != 7 {
		t.Fatalf("problems = %v", got.Problems)
	}
	columns := map[string]bool{}
	for _, p := range got.Problems {
		columns[p.Column] = true
	}
	for _, want := range []string{"survey_id", "pole_id", "taken", "ground_line", "height", "method", "subject"} {
		if !columns[want] {
			t.Errorf("no problem reported against %s, got %v", want, got.Problems)
		}
	}
}

func TestImportSurveysNeedsItsColumns(t *testing.T) {
	if _, err := ingest.Surveys(strings.NewReader("survey_id,pole_id\nSV-1,NW-441829\n")); err == nil {
		t.Error("a file missing its columns should be rejected")
	}
	if _, err := ingest.SurveyRows(strings.NewReader("")); err == nil {
		t.Error("an empty file should be rejected")
	}
}
