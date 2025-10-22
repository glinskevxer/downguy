package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/unit"
)

// PublishedRate is a rate as it was published, kept so that a bill queried
// three years later can be checked against the number that was actually filed
// rather than against whatever the formula produces today.
type PublishedRate struct {
	Year      calendar.RateYear
	Formula   rental.Formula
	Annual    unit.Money
	Published calendar.Date
}

// Billing reads and writes published rates and the bills raised against them.
type Billing struct {
	db *sql.DB
}

// NewBilling builds a billing repository.
func NewBilling(db *sql.DB) *Billing { return &Billing{db: db} }

// PublishRate files the rate for a year and formula. A year and formula can
// only carry one published rate, so filing a second replaces the first.
func (b *Billing) PublishRate(ctx context.Context, r PublishedRate) error {
	if !rental.KnownFormula(r.Formula) {
		return fmt.Errorf("%w: rate published under %q", ErrMigration, r.Formula)
	}
	if r.Annual <= 0 {
		return fmt.Errorf("%w: %s rate published at %s", ErrMigration, r.Year, r.Annual)
	}
	if r.Published.IsZero() {
		return fmt.Errorf("%w: %s rate has no publication date", ErrMigration, r.Year)
	}
	_, err := b.db.ExecContext(ctx, `
INSERT INTO rate_years (year, formula, annual_cents, published_on) VALUES (?, ?, ?, ?)
ON CONFLICT (year) DO UPDATE SET
    formula = excluded.formula,
    annual_cents = excluded.annual_cents,
    published_on = excluded.published_on`,
		int(r.Year), string(r.Formula), r.Annual.AsCents(), r.Published.String())
	if err != nil {
		return fmt.Errorf("store: publishing the %s rate: %w", r.Year, err)
	}
	return nil
}

// Rate returns the published rate for a year.
func (b *Billing) Rate(ctx context.Context, year calendar.RateYear) (PublishedRate, error) {
	var (
		r         PublishedRate
		y         int
		formula   string
		cents     int64
		published string
	)
	err := b.db.QueryRowContext(ctx,
		`SELECT year, formula, annual_cents, published_on FROM rate_years WHERE year = ?`,
		int(year)).Scan(&y, &formula, &cents, &published)
	if errors.Is(err, sql.ErrNoRows) {
		return PublishedRate{}, fmt.Errorf("%w: no rate published for %s", ErrNotFound, year)
	}
	if err != nil {
		return PublishedRate{}, fmt.Errorf("store: reading the %s rate: %w", year, err)
	}
	r.Year = calendar.RateYear(y)
	r.Formula = rental.Formula(formula)
	r.Annual = unit.Cents(cents)
	if published != "" {
		parsed, err := calendar.ParseDate(published)
		if err != nil {
			return PublishedRate{}, fmt.Errorf("store: the %s rate is dated %q: %w", year, published, err)
		}
		r.Published = parsed
	}
	return r, nil
}

// PublishedYears returns every year with a filed rate, oldest first.
func (b *Billing) PublishedYears(ctx context.Context) ([]calendar.RateYear, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT year FROM rate_years ORDER BY year`)
	if err != nil {
		return nil, fmt.Errorf("store: listing published rates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []calendar.RateYear
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, fmt.Errorf("store: reading a published year: %w", err)
		}
		out = append(out, calendar.RateYear(year))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: listing published rates: %w", err)
	}
	return out, nil
}

// SaveBills files a run of bills in one transaction, so a billing run that
// fails halfway leaves nothing behind for somebody to reconcile by hand.
func (b *Billing) SaveBills(ctx context.Context, bills []rental.Bill) error {
	for _, bill := range bills {
		if err := bill.Validate(); err != nil {
			return err
		}
	}
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: saving bills: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, bill := range bills {
		_, err := tx.ExecContext(ctx, `
INSERT INTO rental_bills (number, year, owner, formula, total_cents, issued, due)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (number) DO UPDATE SET
    year = excluded.year, owner = excluded.owner, formula = excluded.formula,
    total_cents = excluded.total_cents, issued = excluded.issued, due = excluded.due`,
			bill.Number, int(bill.Year), bill.Owner, string(bill.Formula),
			bill.Total.AsCents(), bill.Issued.String(), bill.Due.String())
		if err != nil {
			return fmt.Errorf("store: saving bill %s: %w", bill.Number, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: saving bills: %w", err)
	}
	return nil
}

// BillsFor returns the bills raised against one attacher, newest year first.
func (b *Billing) BillsFor(ctx context.Context, owner string) ([]rental.Bill, error) {
	rows, err := b.db.QueryContext(ctx, `
SELECT number, year, owner, formula, total_cents, issued, due
FROM rental_bills WHERE owner = ? ORDER BY year DESC, number`, owner)
	if err != nil {
		return nil, fmt.Errorf("store: listing bills for %s: %w", owner, err)
	}
	return collectBills(rows)
}

// BillsInYear returns every bill raised for a rate year, by attacher.
func (b *Billing) BillsInYear(ctx context.Context, year calendar.RateYear) ([]rental.Bill, error) {
	rows, err := b.db.QueryContext(ctx, `
SELECT number, year, owner, formula, total_cents, issued, due
FROM rental_bills WHERE year = ? ORDER BY owner, number`, int(year))
	if err != nil {
		return nil, fmt.Errorf("store: listing bills for %s: %w", year, err)
	}
	return collectBills(rows)
}

// YearTotal returns everything billed for a rate year.
func (b *Billing) YearTotal(ctx context.Context, year calendar.RateYear) (unit.Money, error) {
	var total sql.NullInt64
	err := b.db.QueryRowContext(ctx,
		`SELECT SUM(total_cents) FROM rental_bills WHERE year = ?`, int(year)).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("store: totalling %s: %w", year, err)
	}
	if !total.Valid {
		return 0, nil
	}
	return unit.Cents(total.Int64), nil
}

func collectBills(rows *sql.Rows) ([]rental.Bill, error) {
	defer func() { _ = rows.Close() }()
	var out []rental.Bill
	for rows.Next() {
		var (
			bill    rental.Bill
			year    int
			formula string
			cents   int64
			issued  string
			due     string
		)
		err := rows.Scan(&bill.Number, &year, &bill.Owner, &formula, &cents, &issued, &due)
		if err != nil {
			return nil, fmt.Errorf("store: reading a bill: %w", err)
		}
		bill.Year = calendar.RateYear(year)
		bill.Formula = rental.Formula(formula)
		bill.Total = unit.Cents(cents)
		for target, text := range map[*calendar.Date]string{&bill.Issued: issued, &bill.Due: due} {
			if text == "" {
				continue
			}
			parsed, err := calendar.ParseDate(text)
			if err != nil {
				return nil, fmt.Errorf("store: bill %s dated %q: %w", bill.Number, text, err)
			}
			*target = parsed
		}
		out = append(out, bill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading bills: %w", err)
	}
	return out, nil
}
