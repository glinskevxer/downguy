package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Attachments reads and writes what is on the poles.
type Attachments struct {
	db *sql.DB
}

// NewAttachments builds an attachment repository.
func NewAttachments(db *sql.DB) *Attachments { return &Attachments{db: db} }

// Put files an attachment, replacing any earlier record of the same id.
func (a *Attachments) Put(ctx context.Context, att plant.Attachment) error {
	if err := att.Validate(); err != nil {
		return err
	}
	_, err := a.db.ExecContext(ctx, `
INSERT INTO attachments (id, pole_id, owner, kind, height, authorized, in_force_from, in_force_to, voltage, face, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    pole_id = excluded.pole_id,
    owner = excluded.owner,
    kind = excluded.kind,
    height = excluded.height,
    authorized = excluded.authorized,
    in_force_from = excluded.in_force_from,
    in_force_to = excluded.in_force_to,
    voltage = excluded.voltage,
    face = excluded.face,
    note = excluded.note`,
		att.ID, string(att.Pole), att.Owner, string(att.Kind), att.Height.Tenths(),
		boolToInt(att.Authorized), att.InForce.From.String(), att.InForce.To.String(),
		att.Voltage, string(att.Face), att.Note)
	if err != nil {
		return fmt.Errorf("store: filing attachment %s: %w", att.ID, err)
	}
	return nil
}

// PutAll files a batch of attachments in one transaction, so a partly written
// pole is never left behind.
func (a *Attachments) PutAll(ctx context.Context, atts []plant.Attachment) error {
	for _, att := range atts {
		if err := att.Validate(); err != nil {
			return err
		}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: filing attachments: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, att := range atts {
		_, err := tx.ExecContext(ctx, `
INSERT INTO attachments (id, pole_id, owner, kind, height, authorized, in_force_from, in_force_to, voltage, face, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    pole_id = excluded.pole_id, owner = excluded.owner, kind = excluded.kind,
    height = excluded.height, authorized = excluded.authorized,
    in_force_from = excluded.in_force_from, in_force_to = excluded.in_force_to,
    voltage = excluded.voltage, face = excluded.face, note = excluded.note`,
			att.ID, string(att.Pole), att.Owner, string(att.Kind), att.Height.Tenths(),
			boolToInt(att.Authorized), att.InForce.From.String(), att.InForce.To.String(),
			att.Voltage, string(att.Face), att.Note)
		if err != nil {
			return fmt.Errorf("store: filing attachment %s: %w", att.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: filing attachments: %w", err)
	}
	return nil
}

// Get returns one attachment by id.
func (a *Attachments) Get(ctx context.Context, id string) (plant.Attachment, error) {
	row := a.db.QueryRowContext(ctx, attachmentColumns+` WHERE id = ?`, id)
	att, err := scanAttachment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return plant.Attachment{}, fmt.Errorf("%w: attachment %s", ErrNotFound, id)
	}
	if err != nil {
		return plant.Attachment{}, fmt.Errorf("store: reading attachment %s: %w", id, err)
	}
	return att, nil
}

// OnPole returns everything on a pole, from the highest down.
func (a *Attachments) OnPole(ctx context.Context, pole plant.PoleID) ([]plant.Attachment, error) {
	rows, err := a.db.QueryContext(ctx,
		attachmentColumns+` WHERE pole_id = ? ORDER BY height DESC, id`, string(pole))
	if err != nil {
		return nil, fmt.Errorf("store: listing attachments on %s: %w", pole, err)
	}
	return collectAttachments(rows)
}

// OwnedBy returns everything belonging to one attacher, by pole then height.
func (a *Attachments) OwnedBy(ctx context.Context, owner string) ([]plant.Attachment, error) {
	rows, err := a.db.QueryContext(ctx,
		attachmentColumns+` WHERE owner = ? ORDER BY pole_id, height DESC, id`, owner)
	if err != nil {
		return nil, fmt.Errorf("store: listing attachments owned by %s: %w", owner, err)
	}
	return collectAttachments(rows)
}

// Unauthorized returns every attachment nobody has permission for.
func (a *Attachments) Unauthorized(ctx context.Context) ([]plant.Attachment, error) {
	rows, err := a.db.QueryContext(ctx,
		attachmentColumns+` WHERE authorized = 0 ORDER BY pole_id, id`)
	if err != nil {
		return nil, fmt.Errorf("store: listing unauthorized attachments: %w", err)
	}
	return collectAttachments(rows)
}

// Delete removes an attachment and reports whether there was one.
func (a *Attachments) Delete(ctx context.Context, id string) (bool, error) {
	result, err := a.db.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: removing attachment %s: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: removing attachment %s: %w", id, err)
	}
	return affected > 0, nil
}

// CountOnPole returns how many facilities are on a pole.
func (a *Attachments) CountOnPole(ctx context.Context, pole plant.PoleID) (int, error) {
	var count int
	err := a.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attachments WHERE pole_id = ?`, string(pole)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: counting attachments on %s: %w", pole, err)
	}
	return count, nil
}

const attachmentColumns = `
SELECT id, pole_id, owner, kind, height, authorized, in_force_from, in_force_to, voltage, face, note
FROM attachments`

func scanAttachment(s scanner) (plant.Attachment, error) {
	var (
		att        plant.Attachment
		pole       string
		kind       string
		height     int
		authorized int
		from       string
		to         string
		face       string
	)
	err := s.Scan(&att.ID, &pole, &att.Owner, &kind, &height, &authorized,
		&from, &to, &att.Voltage, &face, &att.Note)
	if err != nil {
		return plant.Attachment{}, err
	}
	att.Pole = plant.PoleID(pole)
	att.Kind = plant.Kind(kind)
	att.Height = unit.Tenths(height)
	att.Authorized = authorized != 0
	att.Face = plant.Face(face)
	if from != "" {
		parsed, err := calendar.ParseDate(from)
		if err != nil {
			return plant.Attachment{}, fmt.Errorf("store: attachment %s in force from %q: %w", att.ID, from, err)
		}
		att.InForce.From = parsed
	}
	if to != "" {
		parsed, err := calendar.ParseDate(to)
		if err != nil {
			return plant.Attachment{}, fmt.Errorf("store: attachment %s in force to %q: %w", att.ID, to, err)
		}
		att.InForce.To = parsed
	}
	return att, nil
}

func collectAttachments(rows *sql.Rows) ([]plant.Attachment, error) {
	defer func() { _ = rows.Close() }()
	var out []plant.Attachment
	for rows.Next() {
		att, err := scanAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("store: reading an attachment: %w", err)
		}
		out = append(out, att)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading attachments: %w", err)
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
