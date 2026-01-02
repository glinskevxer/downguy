package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/plant"
)

const poleCSV = `pole_id,length_ft,class,species,owner,condition,installed_in,latitude,longitude
NW-441829,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1998,44.0521,-123.0868
NW-441830,45,3,douglas fir,SUMMIT VALLEY ELECTRIC,serviceable,2004,44.0530,-123.0871
SE-1129043,40,4,SPF,CASCADE POWER,,,,
`

func TestImportPoles(t *testing.T) {
	got, err := ingest.Poles(strings.NewReader(poleCSV))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Clean() {
		t.Fatalf("problems = %v", got.Problems)
	}
	if got.Count() != 3 {
		t.Fatalf("poles = %d", got.Count())
	}
	first := got.Rows[0]
	if first.ID != "NW-441829" || first.Length != 40 || first.Class != plant.Class4 {
		t.Errorf("pole = %+v", first)
	}
	if first.Species != plant.SouthernPine || first.Condition != plant.ConditionSound {
		t.Errorf("pole = %+v", first)
	}
	if first.InstalledIn != 1998 || first.Latitude == 0 || first.Longitude == 0 {
		t.Errorf("pole = %+v", first)
	}
	if got.Rows[1].Species != plant.DouglasFir {
		t.Errorf("a long species name should parse, got %s", got.Rows[1].Species)
	}
	last := got.Rows[2]
	if last.Condition != plant.ConditionUnknown {
		t.Errorf("a blank condition should read as unknown, got %s", last.Condition)
	}
	if last.SetDepth != plant.StandardSetDepth(40) {
		t.Errorf("a missing set depth should take the standard, got %s", last.SetDepth)
	}
	for _, p := range got.Rows {
		if err := p.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", p.ID, err)
		}
	}
}

func TestImportPolesCollectsBadRows(t *testing.T) {
	body := poleCSV + `nope,40,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1998,,
NW-441840,forty,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1998,,
NW-441841,40,4,balsa,SUMMIT VALLEY ELECTRIC,sound,1998,,
NW-441842,42,4,SPF,SUMMIT VALLEY ELECTRIC,sound,1998,,
NW-441843,40,4,SPF,,sound,1998,,
`
	got, err := ingest.Poles(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 3 {
		t.Errorf("good rows = %d", got.Count())
	}
	if len(got.Problems) != 5 {
		t.Fatalf("problems = %v", got.Problems)
	}
	columns := map[string]bool{}
	for _, p := range got.Problems {
		columns[p.Column] = true
		if p.Row < 2 {
			t.Errorf("problem has no row number: %+v", p)
		}
	}
	for _, want := range []string{"pole_id", "length_ft", "species"} {
		if !columns[want] {
			t.Errorf("no problem reported against %s, got %v", want, got.Problems)
		}
	}
}

func TestImportPolesNeedsItsColumns(t *testing.T) {
	_, err := ingest.Poles(strings.NewReader("pole_id,owner\nNW-441829,X\n"))
	if !errors.Is(err, ingest.ErrBadFile) {
		t.Errorf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "class") {
		t.Errorf("the error should name the missing columns: %v", err)
	}
	if _, err := ingest.Poles(strings.NewReader("")); err == nil {
		t.Error("an empty file should be rejected")
	}
}

func TestImportPolesReadsAnOptionalSetDepth(t *testing.T) {
	body := "pole_id,length_ft,class,species,owner,set_depth\nNW-441829,40,4,SPF,SUMMIT VALLEY ELECTRIC,6.5\n"
	got, err := ingest.Poles(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 1 || got.Rows[0].SetDepth.Tenths() != 65 {
		t.Errorf("set depth = %v", got.Rows)
	}
	bad := "pole_id,length_ft,class,species,owner,set_depth\nNW-441829,40,4,SPF,SUMMIT VALLEY ELECTRIC,deep\n"
	got, err = ingest.Poles(strings.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Problems) != 1 || got.Problems[0].Column != "set_depth" {
		t.Errorf("problems = %v", got.Problems)
	}
}

const attachmentCSV = `attachment_id,pole_id,owner,kind,height,authorized,in_force_from,voltage,face
A1,NW-441829,SUMMIT VALLEY ELECTRIC,primary,31.5,Y,2001-06-01,7620,
A3,NW-441829,RIDGEPORT CABLEVISION,comm cable,20.0,yes,2019-04-01,,street
A9,NW-441829,UNIDENTIFIED,comm_cable,17.4,n,,,
`

func TestImportAttachments(t *testing.T) {
	got, err := ingest.Attachments(strings.NewReader(attachmentCSV))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Clean() || got.Count() != 3 {
		t.Fatalf("result = %+v", got)
	}
	if got.Rows[0].Kind != plant.KindPrimary || got.Rows[0].Voltage != 7620 {
		t.Errorf("attachment = %+v", got.Rows[0])
	}
	if !got.Rows[0].Authorized || !got.Rows[1].Authorized {
		t.Error("Y and yes both mean authorized")
	}
	if got.Rows[2].Authorized {
		t.Error("n means not authorized")
	}
	if got.Rows[1].Face != plant.FaceStreet {
		t.Errorf("face = %q", got.Rows[1].Face)
	}
	if got.Rows[1].Height.Tenths() != 200 {
		t.Errorf("height = %s", got.Rows[1].Height)
	}
	if got.Rows[0].InForce.From.String() != "2001-06-01" {
		t.Errorf("in force from %s", got.Rows[0].InForce.From)
	}
}

func TestImportAttachmentsCollectsBadRows(t *testing.T) {
	body := attachmentCSV + `A4,nope,X,comm_cable,20.0,y,,,
A5,NW-441829,X,clothesline,20.0,y,,,
A6,NW-441829,X,comm_cable,high,y,,,
A7,NW-441829,X,comm_cable,20.0,y,not-a-date,,
A8,NW-441829,X,comm_cable,20.0,y,,volts,
A0,NW-441829,X,comm_cable,20.0,y,,,underside
`
	got, err := ingest.Attachments(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 3 {
		t.Errorf("good rows = %d", got.Count())
	}
	if len(got.Problems) != 6 {
		t.Errorf("problems = %v", got.Problems)
	}
	if _, err := ingest.Attachments(strings.NewReader("attachment_id\nA1\n")); err == nil {
		t.Error("a file missing its columns should be rejected")
	}
	if _, err := ingest.Attachments(strings.NewReader("")); err == nil {
		t.Error("an empty file should be rejected")
	}
}

const spanCSV = `span_id,from_pole,to_pole,length_ft,cover,drop,dead_end
SP-1,NW-441829,NW-441830,200,road,-0.8,false
SP-2,NW-441830,NW-441831,260,open field,,true
`

func TestImportSpans(t *testing.T) {
	got, err := ingest.Spans(strings.NewReader(spanCSV))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 1 || len(got.Problems) != 1 {
		t.Fatalf("result = %+v", got)
	}
	if got.Rows[0].Cover != plant.CoverRoad || got.Rows[0].Drop.Tenths() != -8 {
		t.Errorf("span = %+v", got.Rows[0])
	}
	if got.Rows[0].Dead {
		t.Error("false means not a dead end")
	}
	if got.Problems[0].Column != "cover" {
		t.Errorf("an unlisted cover should be the problem, got %+v", got.Problems[0])
	}
}

func TestImportSpansAcceptsATabulatedCover(t *testing.T) {
	body := "span_id,from_pole,to_pole,length_ft,cover,dead_end\nSP-2,NW-441830,NW-441831,260,field,T\n"
	got, err := ingest.Spans(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 1 {
		t.Fatalf("result = %+v", got)
	}
	if !got.Rows[0].Dead {
		t.Error("T means a dead end")
	}
	if _, err := ingest.Spans(strings.NewReader("span_id\nSP-1\n")); err == nil {
		t.Error("a file missing its columns should be rejected")
	}
	if _, err := ingest.Spans(strings.NewReader("")); err == nil {
		t.Error("an empty file should be rejected")
	}
}
