// Package ingest reads the files this system is fed and writes the ones it
// hands back.
//
// Everything arrives as a csv export from somebody else's system, with the
// columns in whatever order that system felt like, and with a handful of rows
// that will not parse. An importer that stops at the first bad row is useless
// against a fifteen thousand row plant export, so the readers here collect
// every problem with its line number and carry on.
package ingest

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ErrBadFile is returned when a file cannot be read at all.
var ErrBadFile = errors.New("ingest: cannot read the file")

// RowError is one row that would not parse, with where it was. Row counts
// records rather than file lines, with the header as row one: the csv reader
// drops wholly blank lines before anything here sees them, so a file line
// number would be a number this package cannot honestly produce.
type RowError struct {
	Row    int
	Column string
	Value  string
	Err    error
}

// Error makes RowError an error.
func (r RowError) Error() string {
	if r.Column == "" {
		return fmt.Sprintf("row %d: %v", r.Row, r.Err)
	}
	return fmt.Sprintf("row %d, column %s = %q: %v", r.Row, r.Column, r.Value, r.Err)
}

// Unwrap gives errors.Is something to walk into.
func (r RowError) Unwrap() error { return r.Err }

// Result is what an import produced: the rows that worked and the ones that
// did not.
type Result[T any] struct {
	Rows     []T
	Problems []RowError
}

// Clean reports whether every row parsed.
func (r Result[T]) Clean() bool { return len(r.Problems) == 0 }

// Count returns how many rows parsed.
func (r Result[T]) Count() int { return len(r.Rows) }

// Summary writes the line an import log opens with.
func (r Result[T]) Summary() string {
	return fmt.Sprintf("%d rows read, %d rejected", len(r.Rows), len(r.Problems))
}

// Header maps column names to their position in a file, folded so that a
// system exporting "Pole ID" matches one exporting "pole_id".
type Header struct {
	index map[string]int
	names []string
}

// FoldColumn normalises a column name for matching.
func FoldColumn(name string) string {
	folded := strings.ToLower(strings.TrimSpace(name))
	folded = strings.ReplaceAll(folded, " ", "_")
	folded = strings.ReplaceAll(folded, "-", "_")
	return folded
}

// NewHeader builds a header index from the first row of a file.
func NewHeader(row []string) (*Header, error) {
	h := &Header{index: map[string]int{}}
	for i, name := range row {
		folded := FoldColumn(name)
		if folded == "" {
			continue
		}
		if _, seen := h.index[folded]; seen {
			return nil, fmt.Errorf("%w: the column %q appears twice", ErrBadFile, name)
		}
		h.index[folded] = i
		h.names = append(h.names, folded)
	}
	if len(h.index) == 0 {
		return nil, fmt.Errorf("%w: the header row is empty", ErrBadFile)
	}
	return h, nil
}

// Has reports whether a column is in the file.
func (h *Header) Has(name string) bool {
	_, ok := h.index[FoldColumn(name)]
	return ok
}

// Names returns the columns the file carries, sorted.
func (h *Header) Names() []string {
	out := make([]string, len(h.names))
	copy(out, h.names)
	sort.Strings(out)
	return out
}

// Require reports the columns that are wanted and not present, sorted.
func (h *Header) Require(names ...string) []string {
	var missing []string
	for _, name := range names {
		if !h.Has(name) {
			missing = append(missing, FoldColumn(name))
		}
	}
	sort.Strings(missing)
	return missing
}

// Field returns the value of a column in a row, or the empty string when the
// file does not carry that column or the row is short.
func (h *Header) Field(row []string, name string) string {
	i, ok := h.index[FoldColumn(name)]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// Reader walks a csv file row by row, handing each to a parser.
type Reader struct {
	header *Header
	rows   [][]string
}

// NewReader reads a whole csv file into memory and indexes its header. The
// files this system is fed are tens of thousands of rows at most, so holding
// one is cheaper than the streaming machinery it would take not to.
func NewReader(r io.Reader) (*Reader, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nothing to read", ErrBadFile)
	}
	parser := csv.NewReader(r)
	parser.FieldsPerRecord = -1
	parser.TrimLeadingSpace = true
	records, err := parser.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFile, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%w: the file is empty", ErrBadFile)
	}
	header, err := NewHeader(records[0])
	if err != nil {
		return nil, err
	}
	return &Reader{header: header, rows: records[1:]}, nil
}

// Header returns the column index.
func (r *Reader) Header() *Header { return r.header }

// Len returns how many data rows the file carries.
func (r *Reader) Len() int { return len(r.rows) }

// Each hands every row to a parser and collects what came back. A parser
// returning an error puts the row in the problem list with its row number,
// counting the header as row one.
func Each[T any](r *Reader, parse func(*Header, []string) (T, error)) Result[T] {
	var out Result[T]
	for i, row := range r.rows {
		number := i + 2
		if isBlank(row) {
			continue
		}
		value, err := parse(r.header, row)
		if err != nil {
			var rowErr RowError
			if errors.As(err, &rowErr) {
				rowErr.Row = number
				out.Problems = append(out.Problems, rowErr)
			} else {
				out.Problems = append(out.Problems, RowError{Row: number, Err: err})
			}
			continue
		}
		out.Rows = append(out.Rows, value)
	}
	return out
}

func isBlank(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// Missing builds the error for a file that does not carry the columns an
// importer needs.
func Missing(what string, columns []string) error {
	return fmt.Errorf("%w: a %s file needs the columns %s", ErrBadFile, what, strings.Join(columns, ", "))
}

// Bad builds a RowError for one column of one row.
func Bad(column, value string, err error) error {
	return RowError{Column: FoldColumn(column), Value: value, Err: err}
}
