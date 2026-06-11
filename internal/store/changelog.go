package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
)

// Every record in this system is something an attacher may argue about, and
// the argument is always the same shape: who changed this, when, and to what.
// The change log answers it. It is append only and it is written by the caller
// rather than by a trigger, because a trigger cannot know who was at the
// keyboard.

// Subject is what kind of record a change was against.
type Subject string

// The subjects the log records.
const (
	SubjectPole        Subject = "pole"
	SubjectAttachment  Subject = "attachment"
	SubjectSpan        Subject = "span"
	SubjectAnchor      Subject = "anchor"
	SubjectSurvey      Subject = "survey"
	SubjectApplication Subject = "application"
	SubjectBilling     Subject = "billing"
)

var subjectNames = map[Subject]string{
	SubjectPole: "pole", SubjectAttachment: "attachment", SubjectSpan: "span",
	SubjectAnchor: "anchor", SubjectSurvey: "survey",
	SubjectApplication: "application", SubjectBilling: "billing",
}

// KnownSubject reports whether the subject is one the log records.
func KnownSubject(s Subject) bool {
	_, ok := subjectNames[s]
	return ok
}

// Subjects returns every subject, sorted.
func Subjects() []Subject {
	out := make([]Subject, 0, len(subjectNames))
	for s := range subjectNames {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Action is what happened to a record.
type Action string

// The actions.
const (
	ActionCreated  Action = "created"
	ActionUpdated  Action = "updated"
	ActionRemoved  Action = "removed"
	ActionImported Action = "imported"
	ActionBilled   Action = "billed"
)

// KnownAction reports whether the action is one the log records.
func KnownAction(a Action) bool {
	switch a {
	case ActionCreated, ActionUpdated, ActionRemoved, ActionImported, ActionBilled:
		return true
	}
	return false
}

// Change is one entry.
type Change struct {
	Seq     int64
	At      calendar.Date
	Subject Subject
	ID      string
	Action  Action
	Actor   string
	Detail  string
}

// Validate checks an entry before it is written.
func (c Change) Validate() error {
	switch {
	case c.At.IsZero():
		return fmt.Errorf("%w: a change with no date", ErrMigration)
	case !KnownSubject(c.Subject):
		return fmt.Errorf("%w: a change against a %q", ErrMigration, c.Subject)
	case strings.TrimSpace(c.ID) == "":
		return fmt.Errorf("%w: a change against no record", ErrMigration)
	case !KnownAction(c.Action):
		return fmt.Errorf("%w: %s was %q", ErrMigration, c.ID, c.Action)
	case strings.TrimSpace(c.Actor) == "":
		return fmt.Errorf("%w: %s changed by nobody", ErrMigration, c.ID)
	}
	return nil
}

// String prints the entry the way a history reads.
func (c Change) String() string {
	line := fmt.Sprintf("%s %s %s %s by %s", c.At, c.Subject, c.ID, c.Action, c.Actor)
	if c.Detail != "" {
		line += ": " + c.Detail
	}
	return line
}

// ChangeLog reads and appends the change log.
type ChangeLog struct {
	db *sql.DB
}

// NewChangeLog builds a change log over a database.
func NewChangeLog(db *sql.DB) *ChangeLog { return &ChangeLog{db: db} }

// Append records one change and returns the sequence number it was given.
func (l *ChangeLog) Append(ctx context.Context, c Change) (int64, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	result, err := l.db.ExecContext(ctx, `
INSERT INTO change_log (at, subject, subject_id, action, actor, detail)
VALUES (?, ?, ?, ?, ?, ?)`,
		c.At.String(), string(c.Subject), c.ID, string(c.Action), c.Actor, c.Detail)
	if err != nil {
		return 0, fmt.Errorf("store: recording a change against %s: %w", c.ID, err)
	}
	seq, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("store: recording a change against %s: %w", c.ID, err)
	}
	return seq, nil
}

// AppendAll records a batch in one transaction, so an import that fails
// halfway does not leave a history claiming it succeeded.
func (l *ChangeLog) AppendAll(ctx context.Context, changes []Change) error {
	for _, c := range changes {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: recording changes: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, c := range changes {
		_, err := tx.ExecContext(ctx, `
INSERT INTO change_log (at, subject, subject_id, action, actor, detail)
VALUES (?, ?, ?, ?, ?, ?)`,
			c.At.String(), string(c.Subject), c.ID, string(c.Action), c.Actor, c.Detail)
		if err != nil {
			return fmt.Errorf("store: recording a change against %s: %w", c.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: recording changes: %w", err)
	}
	return nil
}

// For returns the history of one record, oldest first.
func (l *ChangeLog) For(ctx context.Context, subject Subject, id string) ([]Change, error) {
	rows, err := l.db.QueryContext(ctx, changeColumns+`
 WHERE subject = ? AND subject_id = ? ORDER BY seq`, string(subject), id)
	if err != nil {
		return nil, fmt.Errorf("store: reading the history of %s: %w", id, err)
	}
	return collectChanges(rows)
}

// Since returns everything recorded on or after a day, oldest first.
func (l *ChangeLog) Since(ctx context.Context, from calendar.Date) ([]Change, error) {
	rows, err := l.db.QueryContext(ctx, changeColumns+` WHERE at >= ? ORDER BY seq`, from.String())
	if err != nil {
		return nil, fmt.Errorf("store: reading the change log: %w", err)
	}
	return collectChanges(rows)
}

// ByActor returns everything one person did, oldest first.
func (l *ChangeLog) ByActor(ctx context.Context, actor string) ([]Change, error) {
	rows, err := l.db.QueryContext(ctx, changeColumns+` WHERE actor = ? ORDER BY seq`, actor)
	if err != nil {
		return nil, fmt.Errorf("store: reading what %s did: %w", actor, err)
	}
	return collectChanges(rows)
}

// Count returns how many entries the log holds.
func (l *ChangeLog) Count(ctx context.Context) (int, error) {
	var count int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM change_log`).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: counting the change log: %w", err)
	}
	return count, nil
}

const changeColumns = `
SELECT seq, at, subject, subject_id, action, actor, detail
FROM change_log`

func collectChanges(rows *sql.Rows) ([]Change, error) {
	defer func() { _ = rows.Close() }()
	var out []Change
	for rows.Next() {
		var (
			c       Change
			at      string
			subject string
			action  string
		)
		if err := rows.Scan(&c.Seq, &at, &subject, &c.ID, &action, &c.Actor, &c.Detail); err != nil {
			return nil, fmt.Errorf("store: reading a change: %w", err)
		}
		c.Subject = Subject(subject)
		c.Action = Action(action)
		if at != "" {
			parsed, err := calendar.ParseDate(at)
			if err != nil {
				return nil, fmt.Errorf("store: a change dated %q: %w", at, err)
			}
			c.At = parsed
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading changes: %w", err)
	}
	return out, nil
}
