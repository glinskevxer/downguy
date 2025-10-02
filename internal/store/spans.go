package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Spans reads and writes the wire runs between poles.
type Spans struct {
	db *sql.DB
}

// NewSpans builds a span repository.
func NewSpans(db *sql.DB) *Spans { return &Spans{db: db} }

// Put files a span, replacing any earlier record of the same id.
func (s *Spans) Put(ctx context.Context, span plant.SpanRecord) error {
	if err := span.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO spans (id, from_pole, to_pole, length_ft, cover, drop_tenths, dead_end)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    from_pole = excluded.from_pole,
    to_pole = excluded.to_pole,
    length_ft = excluded.length_ft,
    cover = excluded.cover,
    drop_tenths = excluded.drop_tenths,
    dead_end = excluded.dead_end`,
		span.ID, string(span.FromPole), string(span.ToPole), span.Length.Feet(),
		string(span.Cover), span.Drop.Tenths(), boolToInt(span.Dead))
	if err != nil {
		return fmt.Errorf("store: filing span %s: %w", span.ID, err)
	}
	return nil
}

// Get returns one span by id.
func (s *Spans) Get(ctx context.Context, id string) (plant.SpanRecord, error) {
	row := s.db.QueryRowContext(ctx, spanColumns+` WHERE id = ?`, id)
	span, err := scanSpan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return plant.SpanRecord{}, fmt.Errorf("%w: span %s", ErrNotFound, id)
	}
	if err != nil {
		return plant.SpanRecord{}, fmt.Errorf("store: reading span %s: %w", id, err)
	}
	return span, nil
}

// AtPole returns every span landing on a pole, either end, in id order.
func (s *Spans) AtPole(ctx context.Context, pole plant.PoleID) ([]plant.SpanRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		spanColumns+` WHERE from_pole = ? OR to_pole = ? ORDER BY id`, string(pole), string(pole))
	if err != nil {
		return nil, fmt.Errorf("store: listing spans at %s: %w", pole, err)
	}
	return collectSpans(rows)
}

// Between returns the span joining two poles, whichever way round they are
// given.
func (s *Spans) Between(ctx context.Context, a, b plant.PoleID) (plant.SpanRecord, error) {
	row := s.db.QueryRowContext(ctx, spanColumns+`
 WHERE (from_pole = ? AND to_pole = ?) OR (from_pole = ? AND to_pole = ?)
 ORDER BY id LIMIT 1`, string(a), string(b), string(b), string(a))
	span, err := scanSpan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return plant.SpanRecord{}, fmt.Errorf("%w: no span between %s and %s", ErrNotFound, a, b)
	}
	if err != nil {
		return plant.SpanRecord{}, fmt.Errorf("store: reading the span between %s and %s: %w", a, b, err)
	}
	return span, nil
}

// OverCover returns every span crossing one kind of ground, in id order. It is
// the query the annual road crossing sweep runs.
func (s *Spans) OverCover(ctx context.Context, cover plant.Cover) ([]plant.SpanRecord, error) {
	rows, err := s.db.QueryContext(ctx, spanColumns+` WHERE cover = ? ORDER BY id`, string(cover))
	if err != nil {
		return nil, fmt.Errorf("store: listing spans over %s: %w", cover, err)
	}
	return collectSpans(rows)
}

// LongerThan returns every span over a length, longest first.
func (s *Spans) LongerThan(ctx context.Context, feet int) ([]plant.SpanRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		spanColumns+` WHERE length_ft > ? ORDER BY length_ft DESC, id`, feet)
	if err != nil {
		return nil, fmt.Errorf("store: listing spans over %d feet: %w", feet, err)
	}
	return collectSpans(rows)
}

// Delete removes a span and reports whether there was one.
func (s *Spans) Delete(ctx context.Context, id string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM spans WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: removing span %s: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: removing span %s: %w", id, err)
	}
	return affected > 0, nil
}

// Count returns how many spans are on file.
func (s *Spans) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM spans`).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: counting spans: %w", err)
	}
	return count, nil
}

const spanColumns = `
SELECT id, from_pole, to_pole, length_ft, cover, drop_tenths, dead_end
FROM spans`

func scanSpan(sc scanner) (plant.SpanRecord, error) {
	var (
		span   plant.SpanRecord
		from   string
		to     string
		length int
		cover  string
		drop   int
		dead   int
	)
	if err := sc.Scan(&span.ID, &from, &to, &length, &cover, &drop, &dead); err != nil {
		return plant.SpanRecord{}, err
	}
	span.FromPole = plant.PoleID(from)
	span.ToPole = plant.PoleID(to)
	span.Length = unit.SpanFeet(length)
	span.Cover = plant.Cover(cover)
	span.Drop = unit.Tenths(drop)
	span.Dead = dead != 0
	return span, nil
}

func collectSpans(rows *sql.Rows) ([]plant.SpanRecord, error) {
	defer func() { _ = rows.Close() }()
	var out []plant.SpanRecord
	for rows.Next() {
		span, err := scanSpan(rows)
		if err != nil {
			return nil, fmt.Errorf("store: reading a span: %w", err)
		}
		out = append(out, span)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading spans: %w", err)
	}
	return out, nil
}
