package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// Counts is how much of each thing is on file.
type Counts struct {
	Poles        int
	Attachments  int
	Spans        int
	Anchors      int
	Guys         int
	Applications int
	Surveys      int
	Bills        int
	Changes      int
}

// Total returns everything counted.
func (c Counts) Total() int {
	return c.Poles + c.Attachments + c.Spans + c.Anchors + c.Guys +
		c.Applications + c.Surveys + c.Bills + c.Changes
}

// tallyTable pairs a table with where its count goes.
type tallyTable struct {
	name string
	into func(*Counts) *int
}

// tallyTables is the list Tally walks. Adding a table to the schema and
// forgetting it here is caught by the test that asserts the tally covers every
// table the migrations create.
var tallyTables = []tallyTable{
	{"poles", func(c *Counts) *int { return &c.Poles }},
	{"attachments", func(c *Counts) *int { return &c.Attachments }},
	{"spans", func(c *Counts) *int { return &c.Spans }},
	{"anchors", func(c *Counts) *int { return &c.Anchors }},
	{"guys", func(c *Counts) *int { return &c.Guys }},
	{"applications", func(c *Counts) *int { return &c.Applications }},
	{"pole_surveys", func(c *Counts) *int { return &c.Surveys }},
	{"rental_bills", func(c *Counts) *int { return &c.Bills }},
	{"change_log", func(c *Counts) *int { return &c.Changes }},
}

// TalliedTables returns the tables the tally counts, sorted.
func TalliedTables() []string {
	out := make([]string, 0, len(tallyTables))
	for _, t := range tallyTables {
		out = append(out, t.name)
	}
	sort.Strings(out)
	return out
}

// Tally counts every table.
func Tally(ctx context.Context, db *sql.DB) (Counts, error) {
	if db == nil {
		return Counts{}, fmt.Errorf("%w: no database", ErrMigration)
	}
	var c Counts
	for _, t := range tallyTables {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+t.name).Scan(t.into(&c)); err != nil {
			return Counts{}, fmt.Errorf("store: counting %s: %w", t.name, err)
		}
	}
	return c, nil
}

// Problem is one thing wrong with the database that the schema itself cannot
// forbid.
type Problem struct {
	Check  string
	Detail string
}

// Check runs the integrity checks. Foreign keys catch a row pointing at
// nothing; these catch the things a foreign key cannot: a survey of a pole
// nobody owns, an application covering a pole that is not there, an attachment
// above the top of its own pole.
func Check(ctx context.Context, db *sql.DB) ([]Problem, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: no database", ErrMigration)
	}
	var out []Problem

	if _, err := db.ExecContext(ctx, `PRAGMA quick_check`); err != nil {
		out = append(out, Problem{Check: "quick_check", Detail: err.Error()})
	}

	queries := []struct {
		name  string
		query string
		note  string
	}{
		{
			"orphan_application_poles",
			`SELECT ap.application_id || ' covers ' || ap.pole_id
			 FROM application_poles ap
			 LEFT JOIN poles p ON p.id = ap.pole_id
			 WHERE p.id IS NULL ORDER BY 1`,
			"an application covering a pole that is not on file",
		},
		{
			"attachment_above_pole",
			`SELECT a.id || ' on ' || a.pole_id
			 FROM attachments a JOIN poles p ON p.id = a.pole_id
			 WHERE a.height >= (p.length_ft * 10) - p.set_depth ORDER BY 1`,
			"an attachment at or above the top of its own pole",
		},
		{
			"span_to_itself",
			`SELECT id FROM spans WHERE from_pole = to_pole ORDER BY 1`,
			"a span running from a pole back to itself",
		},
		{
			"guy_without_anchor",
			`SELECT g.anchor_id || '/' || g.id
			 FROM guys g LEFT JOIN anchors a ON a.id = g.anchor_id
			 WHERE a.id IS NULL ORDER BY 1`,
			"a guy on an anchor that is not on file",
		},
		{
			"application_without_history",
			`SELECT a.id FROM applications a
			 LEFT JOIN application_events e ON e.application_id = a.id
			 WHERE e.application_id IS NULL ORDER BY 1`,
			"an application with no history at all",
		},
	}
	for _, q := range queries {
		rows, err := db.QueryContext(ctx, q.query)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", q.name, err)
		}
		found, err := collectStrings(rows)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", q.name, err)
		}
		for _, detail := range found {
			out = append(out, Problem{Check: q.name, Detail: q.note + ": " + detail})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Check != out[j].Check {
			return out[i].Check < out[j].Check
		}
		return out[i].Detail < out[j].Detail
	})
	return out, nil
}

func collectStrings(rows *sql.Rows) ([]string, error) {
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

// Compact reclaims the space a long run of deletions leaves behind and
// refreshes the query planner's statistics. Sqlite will not vacuum inside a
// transaction, so this is deliberately not wrapped in one.
func Compact(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("%w: no database", ErrMigration)
	}
	for _, statement := range []string{`VACUUM`, `ANALYZE`} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("store: %s: %w", statement, err)
		}
	}
	return nil
}

// PageCount returns how many pages the database occupies, which is what a
// compaction is measured by.
func PageCount(ctx context.Context, db *sql.DB) (int, error) {
	var pages int
	if err := db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pages); err != nil {
		return 0, fmt.Errorf("store: reading the page count: %w", err)
	}
	return pages, nil
}
