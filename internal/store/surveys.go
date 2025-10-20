package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Surveys reads and writes what the field crews wrote down.
type Surveys struct {
	db *sql.DB
}

// NewSurveys builds a survey repository.
func NewSurveys(db *sql.DB) *Surveys { return &Surveys{db: db} }

// Put files a survey sheet and its readings in one transaction.
func (s *Surveys) Put(ctx context.Context, sheet survey.PoleSheet) error {
	if err := sheet.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: filing survey %s: %w", sheet.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
INSERT INTO pole_surveys (id, pole_id, taken, crew, condition, ground_line, note)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    pole_id = excluded.pole_id, taken = excluded.taken, crew = excluded.crew,
    condition = excluded.condition, ground_line = excluded.ground_line, note = excluded.note`,
		sheet.ID, string(sheet.Pole), sheet.Taken.String(), sheet.Crew,
		string(sheet.Condition), sheet.GroundLine.Tenths(), sheet.Notes)
	if err != nil {
		return fmt.Errorf("store: filing survey %s: %w", sheet.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM survey_measurements WHERE survey_id = ?`, sheet.ID); err != nil {
		return fmt.Errorf("store: clearing the readings on %s: %w", sheet.ID, err)
	}
	for _, m := range sheet.Measurements {
		_, err := tx.ExecContext(ctx, `
INSERT INTO survey_measurements (survey_id, subject, height, method, crew, note)
VALUES (?, ?, ?, ?, ?, ?)`,
			sheet.ID, m.Subject, m.Height.Tenths(), string(m.Method), m.Crew, m.Note)
		if err != nil {
			return fmt.Errorf("store: filing reading %s on %s: %w", m.Subject, sheet.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: filing survey %s: %w", sheet.ID, err)
	}
	return nil
}

// Get returns one survey sheet with its readings.
func (s *Surveys) Get(ctx context.Context, id string) (survey.PoleSheet, error) {
	row := s.db.QueryRowContext(ctx, surveyColumns+` WHERE id = ?`, id)
	sheet, err := scanSurvey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return survey.PoleSheet{}, fmt.Errorf("%w: survey %s", ErrNotFound, id)
	}
	if err != nil {
		return survey.PoleSheet{}, fmt.Errorf("store: reading survey %s: %w", id, err)
	}
	readings, err := s.readingsOn(ctx, id)
	if err != nil {
		return survey.PoleSheet{}, err
	}
	sheet.Measurements = readings
	return sheet, nil
}

// Latest returns the most recent survey of a pole, and reports whether there
// is one. It is the query nearly every caller actually wants: an engineer
// works from the last survey, not from the first.
func (s *Surveys) Latest(ctx context.Context, pole plant.PoleID) (survey.PoleSheet, error) {
	row := s.db.QueryRowContext(ctx,
		surveyColumns+` WHERE pole_id = ? ORDER BY taken DESC, id DESC LIMIT 1`, string(pole))
	sheet, err := scanSurvey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return survey.PoleSheet{}, fmt.Errorf("%w: no survey of %s", ErrNotFound, pole)
	}
	if err != nil {
		return survey.PoleSheet{}, fmt.Errorf("store: reading the latest survey of %s: %w", pole, err)
	}
	readings, err := s.readingsOn(ctx, sheet.ID)
	if err != nil {
		return survey.PoleSheet{}, err
	}
	sheet.Measurements = readings
	return sheet, nil
}

// OnPole returns every survey of a pole, newest first, with their readings.
func (s *Surveys) OnPole(ctx context.Context, pole plant.PoleID) ([]survey.PoleSheet, error) {
	rows, err := s.db.QueryContext(ctx,
		surveyColumns+` WHERE pole_id = ? ORDER BY taken DESC, id DESC`, string(pole))
	if err != nil {
		return nil, fmt.Errorf("store: listing surveys of %s: %w", pole, err)
	}
	sheets, err := collectSurveys(rows)
	if err != nil {
		return nil, err
	}
	for i := range sheets {
		readings, err := s.readingsOn(ctx, sheets[i].ID)
		if err != nil {
			return nil, err
		}
		sheets[i].Measurements = readings
	}
	return sheets, nil
}

// StaleAsOf returns the poles whose most recent survey is too old to engineer
// from, in tag order.
func (s *Surveys) StaleAsOf(ctx context.Context, on calendar.Date) ([]plant.PoleID, error) {
	cutoff := on.AddDays(-survey.StaleAfterDays)
	rows, err := s.db.QueryContext(ctx, `
SELECT pole_id FROM pole_surveys
GROUP BY pole_id
HAVING MAX(taken) < ?
ORDER BY pole_id`, cutoff.String())
	if err != nil {
		return nil, fmt.Errorf("store: listing stale surveys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []plant.PoleID
	for rows.Next() {
		var pole string
		if err := rows.Scan(&pole); err != nil {
			return nil, fmt.Errorf("store: reading a stale pole: %w", err)
		}
		out = append(out, plant.PoleID(pole))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: listing stale surveys: %w", err)
	}
	return out, nil
}

// Count returns how many survey sheets are on file.
func (s *Surveys) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pole_surveys`).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: counting surveys: %w", err)
	}
	return count, nil
}

func (s *Surveys) readingsOn(ctx context.Context, id string) ([]survey.Measurement, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT subject, height, method, crew, note FROM survey_measurements
WHERE survey_id = ? ORDER BY height DESC, subject`, id)
	if err != nil {
		return nil, fmt.Errorf("store: reading the readings on %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	var out []survey.Measurement
	for rows.Next() {
		var (
			m      survey.Measurement
			height int
			method string
		)
		if err := rows.Scan(&m.Subject, &height, &method, &m.Crew, &m.Note); err != nil {
			return nil, fmt.Errorf("store: reading a reading on %s: %w", id, err)
		}
		m.Height = unit.Tenths(height)
		m.Method = survey.Method(method)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading the readings on %s: %w", id, err)
	}
	return out, nil
}

const surveyColumns = `
SELECT id, pole_id, taken, crew, condition, ground_line, note
FROM pole_surveys`

func scanSurvey(sc scanner) (survey.PoleSheet, error) {
	var (
		sheet  survey.PoleSheet
		pole   string
		taken  string
		grade  string
		ground int
	)
	if err := sc.Scan(&sheet.ID, &pole, &taken, &sheet.Crew, &grade, &ground, &sheet.Notes); err != nil {
		return survey.PoleSheet{}, err
	}
	sheet.Pole = plant.PoleID(pole)
	sheet.Condition = plant.Condition(grade)
	sheet.GroundLine = unit.Tenths(ground)
	if taken != "" {
		parsed, err := calendar.ParseDate(taken)
		if err != nil {
			return survey.PoleSheet{}, fmt.Errorf("store: survey %s taken %q: %w", sheet.ID, taken, err)
		}
		sheet.Taken = parsed
	}
	return sheet, nil
}

func collectSurveys(rows *sql.Rows) ([]survey.PoleSheet, error) {
	defer func() { _ = rows.Close() }()
	var out []survey.PoleSheet
	for rows.Next() {
		sheet, err := scanSurvey(rows)
		if err != nil {
			return nil, fmt.Errorf("store: reading a survey: %w", err)
		}
		out = append(out, sheet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading surveys: %w", err)
	}
	return out, nil
}
