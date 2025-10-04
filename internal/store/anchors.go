package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Anchors reads and writes anchors together with the guys on them. The guys
// are a child table, so every write is a transaction: an anchor whose guys are
// half rewritten reports a holding capacity that was never true.
type Anchors struct {
	db *sql.DB
}

// NewAnchors builds an anchor repository.
func NewAnchors(db *sql.DB) *Anchors { return &Anchors{db: db} }

// Put files an anchor and replaces its guys outright.
func (a *Anchors) Put(ctx context.Context, anchor plant.Anchor) error {
	if err := anchor.Validate(); err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: filing anchor %s: %w", anchor.ID, err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
INSERT INTO anchors (id, pole_id, lead_ft, bearing) VALUES (?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    pole_id = excluded.pole_id, lead_ft = excluded.lead_ft, bearing = excluded.bearing`,
		anchor.ID, string(anchor.Pole), anchor.Lead, anchor.Bearing)
	if err != nil {
		return fmt.Errorf("store: filing anchor %s: %w", anchor.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM guys WHERE anchor_id = ?`, anchor.ID); err != nil {
		return fmt.Errorf("store: clearing the guys on %s: %w", anchor.ID, err)
	}
	for _, g := range anchor.Guys {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO guys (id, anchor_id, height, size) VALUES (?, ?, ?, ?)`,
			g.ID, anchor.ID, g.Height.Tenths(), g.Size)
		if err != nil {
			return fmt.Errorf("store: filing guy %s on %s: %w", g.ID, anchor.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: filing anchor %s: %w", anchor.ID, err)
	}
	return nil
}

// Get returns one anchor with its guys.
func (a *Anchors) Get(ctx context.Context, id string) (plant.Anchor, error) {
	row := a.db.QueryRowContext(ctx,
		`SELECT id, pole_id, lead_ft, bearing FROM anchors WHERE id = ?`, id)
	anchor, err := scanAnchor(row)
	if errors.Is(err, sql.ErrNoRows) {
		return plant.Anchor{}, fmt.Errorf("%w: anchor %s", ErrNotFound, id)
	}
	if err != nil {
		return plant.Anchor{}, fmt.Errorf("store: reading anchor %s: %w", id, err)
	}
	guys, err := a.guysOn(ctx, id)
	if err != nil {
		return plant.Anchor{}, err
	}
	anchor.Guys = guys
	return anchor, nil
}

// AtPole returns every anchor on a pole, each with its guys, in id order.
func (a *Anchors) AtPole(ctx context.Context, pole plant.PoleID) ([]plant.Anchor, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, pole_id, lead_ft, bearing FROM anchors WHERE pole_id = ? ORDER BY id`,
		string(pole))
	if err != nil {
		return nil, fmt.Errorf("store: listing anchors at %s: %w", pole, err)
	}
	anchors, err := collectAnchors(rows)
	if err != nil {
		return nil, err
	}
	for i := range anchors {
		guys, err := a.guysOn(ctx, anchors[i].ID)
		if err != nil {
			return nil, err
		}
		anchors[i].Guys = guys
	}
	return anchors, nil
}

// Delete removes an anchor and its guys, and reports whether there was one.
func (a *Anchors) Delete(ctx context.Context, id string) (bool, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: removing anchor %s: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM guys WHERE anchor_id = ?`, id); err != nil {
		return false, fmt.Errorf("store: removing the guys on %s: %w", id, err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM anchors WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: removing anchor %s: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: removing anchor %s: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: removing anchor %s: %w", id, err)
	}
	return affected > 0, nil
}

// CountGuys returns how many guys are on file across every anchor. It is the
// check that a rewrite left nothing orphaned.
func (a *Anchors) CountGuys(ctx context.Context) (int, error) {
	var count int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM guys`).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: counting guys: %w", err)
	}
	return count, nil
}

func (a *Anchors) guysOn(ctx context.Context, anchorID string) ([]plant.Guy, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, height, size FROM guys WHERE anchor_id = ? ORDER BY height DESC, id`, anchorID)
	if err != nil {
		return nil, fmt.Errorf("store: listing the guys on %s: %w", anchorID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []plant.Guy
	for rows.Next() {
		var (
			g      plant.Guy
			height int
		)
		if err := rows.Scan(&g.ID, &height, &g.Size); err != nil {
			return nil, fmt.Errorf("store: reading a guy on %s: %w", anchorID, err)
		}
		g.Height = unit.Tenths(height)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: listing the guys on %s: %w", anchorID, err)
	}
	return out, nil
}

func scanAnchor(s scanner) (plant.Anchor, error) {
	var (
		anchor plant.Anchor
		pole   string
	)
	if err := s.Scan(&anchor.ID, &pole, &anchor.Lead, &anchor.Bearing); err != nil {
		return plant.Anchor{}, err
	}
	anchor.Pole = plant.PoleID(pole)
	return anchor, nil
}

func collectAnchors(rows *sql.Rows) ([]plant.Anchor, error) {
	defer func() { _ = rows.Close() }()
	var out []plant.Anchor
	for rows.Next() {
		anchor, err := scanAnchor(rows)
		if err != nil {
			return nil, fmt.Errorf("store: reading an anchor: %w", err)
		}
		out = append(out, anchor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading anchors: %w", err)
	}
	return out, nil
}
