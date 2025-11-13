package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/rental"
)

func TestWriterHoldsTheHeaderUntilTheFirstRow(t *testing.T) {
	var buf bytes.Buffer
	w, err := ingest.NewWriter(&buf, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if w.Started() {
		t.Error("nothing has been written yet")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("an export of nothing should be an empty file, got %q", buf.String())
	}

	buf.Reset()
	w, err = ingest.NewWriter(&buf, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write("1", "2"); err != nil {
		t.Fatal(err)
	}
	if !w.Started() {
		t.Error("the header has gone out")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "a,b\n1,2\n" {
		t.Errorf("file = %q", buf.String())
	}
}

func TestWriterRejectsBadUse(t *testing.T) {
	if _, err := ingest.NewWriter(nil, "a"); err == nil {
		t.Error("nowhere to write should be rejected")
	}
	var buf bytes.Buffer
	if _, err := ingest.NewWriter(&buf); err == nil {
		t.Error("an export with no columns should be rejected")
	}
	w, err := ingest.NewWriter(&buf, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write("only one"); err == nil {
		t.Error("a row of the wrong width should be rejected")
	}
}

func TestWriteFindings(t *testing.T) {
	var buf bytes.Buffer
	fs := clearance.Findings{
		finding("NESC-235C1", "stop", clearance.BlameApplicant, "MERIDIAN FIBER", 28, 33),
		finding("NESC-232A", "correct", clearance.BlamePreExisting, "RIDGEPORT CABLEVISION", 148, 155),
	}
	if err := ingest.WriteFindings(&buf, fs); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("file = %q", buf.String())
	}
	if !strings.HasPrefix(lines[0], "rule,severity,pole") {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.Contains(lines[1], "NESC-235C1") || !strings.Contains(lines[1], "2.8") {
		t.Errorf("row = %q", lines[1])
	}
	if !strings.Contains(lines[1], "0.5") {
		t.Errorf("the deficit should be on the row, got %q", lines[1])
	}
	var empty bytes.Buffer
	if err := ingest.WriteFindings(&empty, nil); err != nil {
		t.Fatal(err)
	}
	if empty.Len() != 0 {
		t.Errorf("no findings should write nothing, got %q", empty.String())
	}
}

func TestWriteWork(t *testing.T) {
	var buf bytes.Buffer
	plan := planWith(
		raise("MR-0001", "NW-441829", "RIDGEPORT CABLEVISION"),
		makeready.Item{ID: "MR-0002", Action: makeready.ActionAddGuy, Pole: "NW-441829",
			Owner: "SUMMIT VALLEY ELECTRIC", Because: []string{"x"}, Detail: "guy the pull"},
	)
	if err := ingest.WriteWork(&buf, plan); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if !strings.Contains(text, "MR-0001") || !strings.Contains(text, "add_guy") {
		t.Errorf("file = %q", text)
	}
	if !strings.Contains(text, "20.0") || !strings.Contains(text, "21.2") {
		t.Errorf("the heights should be on the row, got %q", text)
	}
	var empty bytes.Buffer
	if err := ingest.WriteWork(&empty, nil); err != nil {
		t.Fatal(err)
	}
	if empty.Len() != 0 {
		t.Errorf("no plan should write nothing, got %q", empty.String())
	}
}

func TestWriteBills(t *testing.T) {
	bills, err := rental.Bills(sampleInventory(t), nil, calendar.MustNew(2026, 1, 15), "RB-2025")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ingest.WriteBills(&buf, bills); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("one header and one row per attachment, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "attachers") {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.Contains(buf.String(), "184") {
		t.Errorf("the part year line should carry its day count:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "$") {
		t.Error("money should go out without a currency sign")
	}
}

func TestWriteAttachmentsRoundTrips(t *testing.T) {
	original := []plant.Attachment{
		att("A1", "SUMMIT VALLEY ELECTRIC", plant.KindPrimary, 335),
		att("A3", "RIDGEPORT CABLEVISION", plant.KindCommCable, 200),
	}
	original[1].Face = plant.FaceStreet
	original[1].Note = "on the transfer list"
	original[1].InForce.To = calendar.MustNew(2030, 12, 31)

	var buf bytes.Buffer
	if err := ingest.WriteAttachments(&buf, original); err != nil {
		t.Fatal(err)
	}
	got, err := ingest.Attachments(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Clean() || got.Count() != 2 {
		t.Fatalf("result = %+v", got)
	}
	for i, want := range original {
		back := got.Rows[i]
		if back.ID != want.ID || back.Pole != want.Pole || back.Owner != want.Owner {
			t.Errorf("row %d = %+v", i, back)
		}
		if back.Kind != want.Kind || back.Height != want.Height {
			t.Errorf("row %d = %+v", i, back)
		}
		if back.Authorized != want.Authorized || back.Face != want.Face {
			t.Errorf("row %d = %+v", i, back)
		}
		if back.Voltage != want.Voltage || back.Note != want.Note {
			t.Errorf("row %d = %+v", i, back)
		}
		if !back.InForce.From.Equal(want.InForce.From) || !back.InForce.To.Equal(want.InForce.To) {
			t.Errorf("row %d in force %s, want %s", i, back.InForce, want.InForce)
		}
	}
}

func TestNegativeValuesPrintWithTheirSign(t *testing.T) {
	var buf bytes.Buffer
	plan := planWith(makeready.Item{
		ID: "MR-0001", Action: makeready.ActionLower, Pole: "NW-441829",
		Subject: "A3", Owner: "X", Because: []string{"x"},
		FromHeight: -12, ToHeight: -34,
	})
	if err := ingest.WriteWork(&buf, plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "-1.2") || !strings.Contains(buf.String(), "-3.4") {
		t.Errorf("file = %q", buf.String())
	}
}
