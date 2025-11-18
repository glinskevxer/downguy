// Package report renders what the rest of the system worked out.
//
// Nothing here decides anything. Every number it prints was computed
// somewhere else, and the renderers are deliberately dull, because a report
// that quietly rounds differently from the engine it reports on is worse than
// no report at all.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/loading"
	"github.com/glinskevxer/downguy/internal/makeready"
)

// Width is how wide a printed report is. It is the width of the paper the
// field copies are printed on, which is what decides this rather than any
// terminal.
const Width = 96

// Rule writes a horizontal rule.
func Rule(w io.Writer) error {
	_, err := fmt.Fprintln(w, strings.Repeat("-", Width))
	return err
}

// Heading writes a section heading with a rule under it.
func Heading(w io.Writer, text string) error {
	if _, err := fmt.Fprintln(w, strings.ToUpper(text)); err != nil {
		return err
	}
	return Rule(w)
}

// Table lays out rows in aligned columns, sized to the widest cell in each.
type Table struct {
	columns []string
	rows    [][]string
	right   map[int]bool
}

// NewTable starts a table with its column headings.
func NewTable(columns ...string) *Table {
	return &Table{columns: columns, right: map[int]bool{}}
}

// RightAlign marks a column as right aligned, which is what every number wants
// and no name does.
func (t *Table) RightAlign(columns ...int) *Table {
	for _, c := range columns {
		t.right[c] = true
	}
	return t
}

// Row adds a row. A row of the wrong width is padded or trimmed rather than
// refused, because a half rendered report is more useful than an error at the
// bottom of a page.
func (t *Table) Row(values ...string) *Table {
	row := make([]string, len(t.columns))
	copy(row, values)
	t.rows = append(t.rows, row)
	return t
}

// Len returns how many rows the table holds.
func (t *Table) Len() int { return len(t.rows) }

// Render writes the table.
func (t *Table) Render(w io.Writer) error {
	widths := make([]int, len(t.columns))
	for i, c := range t.columns {
		widths[i] = len(c)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	if err := t.writeRow(w, widths, t.columns); err != nil {
		return err
	}
	separator := make([]string, len(t.columns))
	for i := range separator {
		separator[i] = strings.Repeat("-", widths[i])
	}
	if err := t.writeRow(w, widths, separator); err != nil {
		return err
	}
	for _, row := range t.rows {
		if err := t.writeRow(w, widths, row); err != nil {
			return err
		}
	}
	return nil
}

func (t *Table) writeRow(w io.Writer, widths []int, row []string) error {
	cells := make([]string, len(row))
	for i, cell := range row {
		if t.right[i] {
			cells[i] = fmt.Sprintf("%*s", widths[i], cell)
		} else {
			cells[i] = fmt.Sprintf("%-*s", widths[i], cell)
		}
	}
	_, err := fmt.Fprintln(w, strings.TrimRight(strings.Join(cells, "  "), " "))
	return err
}

// Findings renders a set of clearance findings.
func Findings(w io.Writer, findings clearance.Findings) error {
	if len(findings) == 0 {
		_, err := fmt.Fprintln(w, "no findings")
		return err
	}
	table := NewTable("rule", "where", "subject", "owner", "blame", "short by", "detail")
	table.RightAlign(5)
	for _, f := range findings {
		where := f.Pole
		if f.Span != "" {
			where = f.Span
		}
		short := ""
		if f.Deficit() > 0 {
			short = f.Deficit().String()
		}
		table.Row(f.RuleID, where, f.Subject, f.Owner, string(f.Blame), short, f.Detail)
	}
	return table.Render(w)
}

// Work renders a make ready plan in the order it happens.
func Work(w io.Writer, plan *makeready.Plan) error {
	if plan == nil || plan.Empty() {
		_, err := fmt.Fprintln(w, "no make ready")
		return err
	}
	table := NewTable("item", "action", "pole", "subject", "owner", "from", "to", "detail")
	table.RightAlign(5, 6)
	for _, item := range plan.Items {
		from, to := "", ""
		if item.FromHeight != 0 || item.ToHeight != 0 {
			from = item.FromHeight.String()
			to = item.ToHeight.String()
		}
		table.Row(item.ID, string(item.Action), item.Pole, item.Subject,
			item.Owner, from, to, item.Detail)
	}
	if err := table.Render(w); err != nil {
		return err
	}
	if len(plan.Unresolved) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\n%d findings have no remedy in this plan:\n", len(plan.Unresolved)); err != nil {
		return err
	}
	for _, f := range plan.Unresolved {
		if _, err := fmt.Fprintf(w, "  %s\n", f); err != nil {
			return err
		}
	}
	return nil
}

// Analysis renders a groundline loading check with its components.
func Analysis(w io.Writer, a *loading.Analysis) error {
	if a == nil {
		_, err := fmt.Fprintln(w, "no analysis")
		return err
	}
	if _, err := fmt.Fprintln(w, a); err != nil {
		return err
	}
	table := NewTable("source", "kind", "moment ft lb", "factored ft lb")
	table.RightAlign(2, 3)
	for _, c := range a.Components {
		table.Row(c.Source, c.Kind,
			fmt.Sprintf("%.0f", c.Moment), fmt.Sprintf("%.0f", c.Factored))
	}
	table.Row("", "total", "", fmt.Sprintf("%.0f", a.FactoredMoment))
	table.Row("", "capacity", "", fmt.Sprintf("%.0f", a.FactoredCapacty))
	return table.Render(w)
}

// OwnerTotals renders a breakdown by attacher, sorted by name.
func OwnerTotals(w io.Writer, heading string, totals map[string]string) error {
	if err := Heading(w, heading); err != nil {
		return err
	}
	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Strings(names)
	table := NewTable("party", "amount")
	table.RightAlign(1)
	for _, name := range names {
		table.Row(name, totals[name])
	}
	return table.Render(w)
}
