package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrNotFound is returned when a record is not in the database. It is a
// separate error from a query failure so a caller can tell "there is no such
// pole" from "the database is down", which are very different things to do
// next.
var ErrNotFound = errors.New("store: not found")

// ErrConflict is returned when a write would break a uniqueness rule.
var ErrConflict = errors.New("store: conflicts with a record already there")

// Poles reads and writes the pole inventory.
type Poles struct {
	db *sql.DB
}

// NewPoles builds a pole repository.
func NewPoles(db *sql.DB) *Poles { return &Poles{db: db} }

// Put files a pole, replacing any earlier record of the same tag. The record
// is validated first, so nothing invalid can reach the table even through a
// caller that forgot to check.
func (p *Poles) Put(ctx context.Context, pole plant.Pole) error {
	if err := pole.Validate(); err != nil {
		return err
	}
	_, err := p.db.ExecContext(ctx, `
INSERT INTO poles (id, district, length_ft, class, species, set_depth, installed_in, owner, condition, latitude, longitude)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    district = excluded.district,
    length_ft = excluded.length_ft,
    class = excluded.class,
    species = excluded.species,
    set_depth = excluded.set_depth,
    installed_in = excluded.installed_in,
    owner = excluded.owner,
    condition = excluded.condition,
    latitude = excluded.latitude,
    longitude = excluded.longitude`,
		string(pole.ID), pole.ID.District(), pole.Length, string(pole.Class), string(pole.Species),
		pole.SetDepth.Tenths(), pole.InstalledIn, pole.Owner, string(pole.Condition),
		pole.Latitude, pole.Longitude)
	if err != nil {
		return fmt.Errorf("store: filing pole %s: %w", pole.ID, err)
	}
	return nil
}

// Get returns one pole by tag.
func (p *Poles) Get(ctx context.Context, id plant.PoleID) (plant.Pole, error) {
	row := p.db.QueryRowContext(ctx, poleColumns+` WHERE id = ?`, string(id))
	pole, err := scanPole(row)
	if errors.Is(err, sql.ErrNoRows) {
		return plant.Pole{}, fmt.Errorf("%w: pole %s", ErrNotFound, id)
	}
	if err != nil {
		return plant.Pole{}, fmt.Errorf("store: reading pole %s: %w", id, err)
	}
	return pole, nil
}

// Exists reports whether a pole is on file.
func (p *Poles) Exists(ctx context.Context, id plant.PoleID) (bool, error) {
	var count int
	err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM poles WHERE id = ?`, string(id)).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("store: looking for pole %s: %w", id, err)
	}
	return count > 0, nil
}

// Delete removes a pole and reports whether there was one to remove.
func (p *Poles) Delete(ctx context.Context, id plant.PoleID) (bool, error) {
	result, err := p.db.ExecContext(ctx, `DELETE FROM poles WHERE id = ?`, string(id))
	if err != nil {
		return false, fmt.Errorf("store: removing pole %s: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: removing pole %s: %w", id, err)
	}
	return affected > 0, nil
}

// InDistrict returns every pole in a district, in tag order.
func (p *Poles) InDistrict(ctx context.Context, district string) ([]plant.Pole, error) {
	rows, err := p.db.QueryContext(ctx, poleColumns+` WHERE district = ? ORDER BY id`, district)
	if err != nil {
		return nil, fmt.Errorf("store: listing district %s: %w", district, err)
	}
	return collectPoles(rows)
}

// OwnedBy returns every pole belonging to one utility, in tag order.
func (p *Poles) OwnedBy(ctx context.Context, owner string) ([]plant.Pole, error) {
	rows, err := p.db.QueryContext(ctx, poleColumns+` WHERE owner = ? ORDER BY id`, owner)
	if err != nil {
		return nil, fmt.Errorf("store: listing poles owned by %s: %w", owner, err)
	}
	return collectPoles(rows)
}

// All returns every pole, in tag order.
func (p *Poles) All(ctx context.Context) ([]plant.Pole, error) {
	rows, err := p.db.QueryContext(ctx, poleColumns+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: listing poles: %w", err)
	}
	return collectPoles(rows)
}

// Count returns how many poles are on file.
func (p *Poles) Count(ctx context.Context) (int, error) {
	var count int
	if err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM poles`).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: counting poles: %w", err)
	}
	return count, nil
}

const poleColumns = `
SELECT id, length_ft, class, species, set_depth, installed_in, owner, condition, latitude, longitude
FROM poles`

type scanner interface {
	Scan(dest ...any) error
}

func scanPole(s scanner) (plant.Pole, error) {
	var (
		pole     plant.Pole
		id       string
		class    string
		species  string
		setDepth int
		grade    string
	)
	err := s.Scan(&id, &pole.Length, &class, &species, &setDepth,
		&pole.InstalledIn, &pole.Owner, &grade, &pole.Latitude, &pole.Longitude)
	if err != nil {
		return plant.Pole{}, err
	}
	pole.ID = plant.PoleID(id)
	pole.Class = plant.Class(class)
	pole.Species = plant.Species(species)
	pole.SetDepth = unit.Tenths(setDepth)
	pole.Condition = plant.Condition(grade)
	return pole, nil
}

func collectPoles(rows *sql.Rows) ([]plant.Pole, error) {
	defer func() { _ = rows.Close() }()
	var out []plant.Pole
	for rows.Next() {
		pole, err := scanPole(rows)
		if err != nil {
			return nil, fmt.Errorf("store: reading a pole: %w", err)
		}
		out = append(out, pole)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading poles: %w", err)
	}
	return out, nil
}
