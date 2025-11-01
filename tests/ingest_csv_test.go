package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/ingest"
)

const sampleCSV = `Pole ID,Owner,height,Note
NW-441829,SUMMIT VALLEY ELECTRIC,20.0,first

NW-441830,RIDGEPORT CABLEVISION,not a height,second
NW-441831,NORTHVALE TELEPHONE,18.6,third
`

func reader(t *testing.T, body string) *ingest.Reader {
	t.Helper()
	r, err := ingest.NewReader(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestFoldColumn(t *testing.T) {
	cases := map[string]string{
		"Pole ID":   "pole_id",
		"pole-id":   "pole_id",
		" POLE_ID ": "pole_id",
		"":          "",
	}
	for in, want := range cases {
		if got := ingest.FoldColumn(in); got != want {
			t.Errorf("FoldColumn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHeaderIndexing(t *testing.T) {
	r := reader(t, sampleCSV)
	h := r.Header()
	if !h.Has("pole_id") || !h.Has("Pole ID") || !h.Has("NOTE") {
		t.Error("the header should match however the column is written")
	}
	if h.Has("species") {
		t.Error("the file has no species column")
	}
	names := h.Names()
	if len(names) != 4 || names[0] != "height" {
		t.Errorf("names = %v", names)
	}
	missing := h.Require("pole_id", "species", "class")
	if len(missing) != 2 || missing[0] != "class" {
		t.Errorf("missing = %v", missing)
	}
	if got := h.Require("pole_id", "owner"); got != nil {
		t.Errorf("nothing should be missing, got %v", got)
	}
}

func TestHeaderRejectsBadHeaders(t *testing.T) {
	if _, err := ingest.NewHeader([]string{"id", "ID"}); err == nil {
		t.Error("a duplicate column should be rejected")
	}
	if _, err := ingest.NewHeader([]string{"", "  "}); err == nil {
		t.Error("an empty header should be rejected")
	}
}

func TestFieldReadsAndTrims(t *testing.T) {
	r := reader(t, sampleCSV)
	h := r.Header()
	row := []string{" NW-441829 ", "SUMMIT VALLEY ELECTRIC", "20.0"}
	if got := h.Field(row, "pole_id"); got != "NW-441829" {
		t.Errorf("field = %q", got)
	}
	if got := h.Field(row, "note"); got != "" {
		t.Errorf("a short row should read empty, got %q", got)
	}
	if got := h.Field(row, "species"); got != "" {
		t.Errorf("a column the file does not carry should read empty, got %q", got)
	}
}

func TestNewReaderRejectsBadFiles(t *testing.T) {
	if _, err := ingest.NewReader(nil); err == nil {
		t.Error("nothing to read should be rejected")
	}
	if _, err := ingest.NewReader(strings.NewReader("")); err == nil {
		t.Error("an empty file should be rejected")
	}
	if _, err := ingest.NewReader(strings.NewReader("a,\"b\nunterminated")); err == nil {
		t.Error("a broken quote should be rejected")
	}
	if _, err := ingest.NewReader(strings.NewReader(",\n")); err == nil {
		t.Error("a file with no usable header should be rejected")
	}
}

func TestEachSkipsBlanksAndCollectsProblems(t *testing.T) {
	r := reader(t, sampleCSV)
	// The csv reader drops the blank line before this package sees it.
	if r.Len() != 3 {
		t.Fatalf("rows = %d", r.Len())
	}
	got := ingest.Each(r, func(h *ingest.Header, row []string) (string, error) {
		height := h.Field(row, "height")
		if !strings.Contains(height, ".") {
			return "", ingest.Bad("height", height, errors.New("not a decimal"))
		}
		return h.Field(row, "pole_id"), nil
	})
	if got.Count() != 2 {
		t.Errorf("rows = %v", got.Rows)
	}
	if len(got.Problems) != 1 {
		t.Fatalf("problems = %v", got.Problems)
	}
	problem := got.Problems[0]
	if problem.Row != 3 {
		t.Errorf("the bad row is record 3, got %d", problem.Row)
	}
	if problem.Column != "height" || problem.Value != "not a height" {
		t.Errorf("problem = %+v", problem)
	}
	if !strings.Contains(problem.Error(), "row 3") {
		t.Errorf("Error = %q", problem.Error())
	}
	if got.Clean() {
		t.Error("a run with a rejected row is not clean")
	}
	if got.Summary() != "2 rows read, 1 rejected" {
		t.Errorf("Summary = %q", got.Summary())
	}
}

func TestEachWrapsAPlainError(t *testing.T) {
	r := reader(t, sampleCSV)
	sentinel := errors.New("no good")
	got := ingest.Each(r, func(h *ingest.Header, row []string) (string, error) {
		return "", sentinel
	})
	if len(got.Problems) != 3 {
		t.Fatalf("problems = %v", got.Problems)
	}
	if !errors.Is(got.Problems[0], sentinel) {
		t.Error("the original error should still be reachable")
	}
	if got.Problems[0].Column != "" {
		t.Errorf("a plain error names no column, got %q", got.Problems[0].Column)
	}
	if !strings.Contains(got.Problems[0].Error(), "row 2") {
		t.Errorf("Error = %q", got.Problems[0].Error())
	}
}

func TestACleanRun(t *testing.T) {
	r := reader(t, "id\nA\nB\n")
	got := ingest.Each(r, func(h *ingest.Header, row []string) (string, error) {
		return h.Field(row, "id"), nil
	})
	if !got.Clean() || got.Count() != 2 {
		t.Errorf("result = %+v", got)
	}
	if got.Summary() != "2 rows read, 0 rejected" {
		t.Errorf("Summary = %q", got.Summary())
	}
}

func TestMissingBuildsAReadableError(t *testing.T) {
	err := ingest.Missing("pole", []string{"class", "species"})
	if !errors.Is(err, ingest.ErrBadFile) {
		t.Errorf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "class, species") {
		t.Errorf("error = %v", err)
	}
}
