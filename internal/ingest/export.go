package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/rental"
)

// Writer wraps a csv writer with the header handling every export here needs.
// The header is written once, on the first row, so an export of nothing is an
// empty file rather than a header with no rows under it. A file with a header
// and no rows reads as "there are none"; an empty file reads as "the export
// did not run", and the difference matters to whoever opens it.
type Writer struct {
	out     *csv.Writer
	columns []string
	started bool
}

// NewWriter builds an export writer over a stream.
func NewWriter(w io.Writer, columns ...string) (*Writer, error) {
	if w == nil {
		return nil, fmt.Errorf("%w: nowhere to write", ErrBadFile)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("%w: an export with no columns", ErrBadFile)
	}
	return &Writer{out: csv.NewWriter(w), columns: columns}, nil
}

// Write puts out one row, writing the header first if it has not gone yet.
func (w *Writer) Write(values ...string) error {
	if len(values) != len(w.columns) {
		return fmt.Errorf("%w: %d values for %d columns", ErrBadFile, len(values), len(w.columns))
	}
	if !w.started {
		if err := w.out.Write(w.columns); err != nil {
			return fmt.Errorf("%w: writing the header: %v", ErrBadFile, err)
		}
		w.started = true
	}
	if err := w.out.Write(values); err != nil {
		return fmt.Errorf("%w: writing a row: %v", ErrBadFile, err)
	}
	return nil
}

// Close flushes the writer and reports anything the flush turned up.
func (w *Writer) Close() error {
	w.out.Flush()
	if err := w.out.Error(); err != nil {
		return fmt.Errorf("%w: flushing: %v", ErrBadFile, err)
	}
	return nil
}

// Started reports whether anything has been written.
func (w *Writer) Started() bool { return w.started }

// FindingColumns are the columns a findings export carries.
var FindingColumns = []string{
	"rule", "severity", "pole", "span", "subject", "against",
	"owner", "blame", "measured_ft", "required_ft", "deficit_ft", "detail",
}

// WriteFindings exports a set of clearance findings.
func WriteFindings(w io.Writer, findings clearance.Findings) error {
	out, err := NewWriter(w, FindingColumns...)
	if err != nil {
		return err
	}
	for _, f := range findings {
		err := out.Write(
			f.RuleID, string(f.Severity), f.Pole, f.Span, f.Subject, f.Against,
			f.Owner, string(f.Blame),
			tenths(f.Measured.Tenths()), tenths(f.Required.Tenths()), tenths(f.Deficit().Tenths()),
			f.Detail,
		)
		if err != nil {
			return err
		}
	}
	return out.Close()
}

// WorkColumns are the columns a make ready export carries.
var WorkColumns = []string{
	"item", "action", "pole", "span", "subject", "owner",
	"from_ft", "to_ft", "detail",
}

// WriteWork exports a make ready plan.
func WriteWork(w io.Writer, plan *makeready.Plan) error {
	out, err := NewWriter(w, WorkColumns...)
	if err != nil {
		return err
	}
	if plan == nil {
		return out.Close()
	}
	for _, item := range plan.Items {
		err := out.Write(
			item.ID, string(item.Action), item.Pole, item.Span, item.Subject, item.Owner,
			tenths(item.FromHeight.Tenths()), tenths(item.ToHeight.Tenths()), item.Detail,
		)
		if err != nil {
			return err
		}
	}
	return out.Close()
}

// BillColumns are the columns a rental billing export carries.
var BillColumns = []string{
	"number", "year", "owner", "formula", "attachment", "pole",
	"attachers", "full_rate", "days", "of_days", "amount", "note",
}

// WriteBills exports a run of annual rental bills, one row per attachment.
func WriteBills(w io.Writer, bills []rental.Bill) error {
	out, err := NewWriter(w, BillColumns...)
	if err != nil {
		return err
	}
	for _, bill := range bills {
		for _, line := range bill.Lines {
			err := out.Write(
				bill.Number, bill.Year.String(), bill.Owner, string(bill.Formula),
				line.AttachmentID, string(line.Pole),
				strconv.Itoa(line.Attachers), money(line.FullRate.AsCents()),
				strconv.Itoa(line.Days), strconv.Itoa(line.OfDays),
				money(line.Amount.AsCents()), line.Note,
			)
			if err != nil {
				return err
			}
		}
	}
	return out.Close()
}

// AttachmentColumnsOut are the columns a plant export writes back.
var AttachmentColumnsOut = []string{
	"attachment_id", "pole_id", "owner", "kind", "height", "authorized",
	"in_force_from", "in_force_to", "voltage", "face", "note",
}

// WriteAttachments exports attachments in the same shape the importer reads,
// so an export can be fed straight back in. The round trip is what makes a
// correction workflow possible at all.
func WriteAttachments(w io.Writer, atts []plant.Attachment) error {
	out, err := NewWriter(w, AttachmentColumnsOut...)
	if err != nil {
		return err
	}
	for _, att := range atts {
		authorized := "n"
		if att.Authorized {
			authorized = "y"
		}
		err := out.Write(
			att.ID, string(att.Pole), att.Owner, string(att.Kind),
			tenths(att.Height.Tenths()), authorized,
			att.InForce.From.String(), att.InForce.To.String(),
			strconv.Itoa(att.Voltage), string(att.Face), att.Note,
		)
		if err != nil {
			return err
		}
	}
	return out.Close()
}

// tenths prints a stored height as decimal feet with one place, which is the
// form every one of these files uses.
func tenths(value int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d.%d", sign, value/10, value%10)
}

// money prints cents as a plain decimal with no currency sign, because a
// spreadsheet reads a dollar sign as text.
func money(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}
