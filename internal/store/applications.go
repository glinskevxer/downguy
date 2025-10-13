package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
)

// Applications reads and writes joint use applications with their pole list
// and their history.
type Applications struct {
	db *sql.DB
}

// NewApplications builds an application repository.
func NewApplications(db *sql.DB) *Applications { return &Applications{db: db} }

// Put files an application, its poles and its whole history in one
// transaction. The history is rewritten rather than appended to, because the
// caller holds the authoritative copy and a partial append would leave two
// disagreeing records of the same application.
func (a *Applications) Put(ctx context.Context, app *permit.Application) error {
	if app == nil {
		return fmt.Errorf("%w: no application to file", ErrMigration)
	}
	if app.ID == "" {
		return fmt.Errorf("%w: an application with no id", ErrMigration)
	}
	if !permit.KnownState(app.State) {
		return fmt.Errorf("%w: %s is in state %q", ErrMigration, app.ID, app.State)
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: filing application %s: %w", app.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
INSERT INTO applications (id, applicant, owner, state) VALUES (?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    applicant = excluded.applicant, owner = excluded.owner, state = excluded.state`,
		app.ID, app.Applicant, app.Owner, string(app.State))
	if err != nil {
		return fmt.Errorf("store: filing application %s: %w", app.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM application_poles WHERE application_id = ?`, app.ID); err != nil {
		return fmt.Errorf("store: clearing the poles on %s: %w", app.ID, err)
	}
	for _, p := range app.Poles {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO application_poles (application_id, pole_id) VALUES (?, ?)`, app.ID, string(p))
		if err != nil {
			return fmt.Errorf("store: filing pole %s on %s: %w", p, app.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM application_events WHERE application_id = ?`, app.ID); err != nil {
		return fmt.Errorf("store: clearing the history of %s: %w", app.ID, err)
	}
	for i, e := range app.History {
		_, err := tx.ExecContext(ctx, `
INSERT INTO application_events (application_id, seq, from_state, to_state, on_date, by_who, note)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
			app.ID, i, string(e.From), string(e.To), e.On.String(), e.By, e.Note)
		if err != nil {
			return fmt.Errorf("store: filing event %d on %s: %w", i, app.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: filing application %s: %w", app.ID, err)
	}
	return nil
}

// Get returns an application with its poles and history.
func (a *Applications) Get(ctx context.Context, id string) (*permit.Application, error) {
	var (
		app   permit.Application
		state string
	)
	err := a.db.QueryRowContext(ctx,
		`SELECT id, applicant, owner, state FROM applications WHERE id = ?`, id).
		Scan(&app.ID, &app.Applicant, &app.Owner, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: application %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("store: reading application %s: %w", id, err)
	}
	app.State = permit.State(state)

	poles, err := a.polesOn(ctx, id)
	if err != nil {
		return nil, err
	}
	app.Poles = poles

	history, err := a.historyOf(ctx, id)
	if err != nil {
		return nil, err
	}
	app.History = history
	return &app, nil
}

// InState returns every application at one state, in id order.
func (a *Applications) InState(ctx context.Context, s permit.State) ([]*permit.Application, error) {
	return a.list(ctx, `SELECT id FROM applications WHERE state = ? ORDER BY id`, string(s))
}

// ForApplicant returns every application from one attacher, in id order.
func (a *Applications) ForApplicant(ctx context.Context, applicant string) ([]*permit.Application, error) {
	return a.list(ctx, `SELECT id FROM applications WHERE applicant = ? ORDER BY id`, applicant)
}

// Live returns every application that is still being worked, in id order.
func (a *Applications) Live(ctx context.Context) ([]*permit.Application, error) {
	var live []any
	placeholders := ""
	for _, s := range permit.States() {
		if !s.Live() {
			continue
		}
		if placeholders != "" {
			placeholders += ", "
		}
		placeholders += "?"
		live = append(live, string(s))
	}
	query := `SELECT id FROM applications WHERE state IN (` + placeholders + `) ORDER BY id`
	return a.list(ctx, query, live...)
}

// TouchingPole returns every application covering a pole, in id order.
func (a *Applications) TouchingPole(ctx context.Context, pole plant.PoleID) ([]*permit.Application, error) {
	return a.list(ctx,
		`SELECT application_id FROM application_poles WHERE pole_id = ? ORDER BY application_id`,
		string(pole))
}

// Count returns how many applications are on file.
func (a *Applications) Count(ctx context.Context) (int, error) {
	var count int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM applications`).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: counting applications: %w", err)
	}
	return count, nil
}

func (a *Applications) list(ctx context.Context, query string, args ...any) ([]*permit.Application, error) {
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: listing applications: %w", err)
	}
	var ids []string
	func() {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return
			}
			ids = append(ids, id)
		}
	}()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: listing applications: %w", err)
	}
	out := make([]*permit.Application, 0, len(ids))
	for _, id := range ids {
		app, err := a.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, app)
	}
	return out, nil
}

func (a *Applications) polesOn(ctx context.Context, id string) ([]plant.PoleID, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT pole_id FROM application_poles WHERE application_id = ? ORDER BY pole_id`, id)
	if err != nil {
		return nil, fmt.Errorf("store: listing the poles on %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	var out []plant.PoleID
	for rows.Next() {
		var pole string
		if err := rows.Scan(&pole); err != nil {
			return nil, fmt.Errorf("store: reading a pole on %s: %w", id, err)
		}
		out = append(out, plant.PoleID(pole))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: listing the poles on %s: %w", id, err)
	}
	return out, nil
}

func (a *Applications) historyOf(ctx context.Context, id string) ([]permit.Event, error) {
	rows, err := a.db.QueryContext(ctx, `
SELECT from_state, to_state, on_date, by_who, note
FROM application_events WHERE application_id = ? ORDER BY seq`, id)
	if err != nil {
		return nil, fmt.Errorf("store: reading the history of %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	var out []permit.Event
	for rows.Next() {
		var (
			e    permit.Event
			from string
			to   string
			on   string
		)
		if err := rows.Scan(&from, &to, &on, &e.By, &e.Note); err != nil {
			return nil, fmt.Errorf("store: reading an event on %s: %w", id, err)
		}
		e.From = permit.State(from)
		e.To = permit.State(to)
		if on != "" {
			parsed, err := calendar.ParseDate(on)
			if err != nil {
				return nil, fmt.Errorf("store: event on %s dated %q: %w", id, on, err)
			}
			e.On = parsed
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading the history of %s: %w", id, err)
	}
	return out, nil
}
