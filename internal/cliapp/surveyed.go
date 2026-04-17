package cliapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
)

// loadSurveyed is the three step sequence every command that checks a pole
// runs: read the assembly, read the latest survey of it, and lay the survey
// over the record so the rules see what the crew found rather than what the
// paperwork says. Three commands each had their own copy with their own
// handling of the two not found cases, and one of the three got the staleness
// check in a slightly different place.
func loadSurveyed(ctx context.Context, db *sql.DB, id plant.PoleID) (*survey.Applied, error) {
	loader := store.NewPlant(db)
	assembly, err := loader.Assembly(ctx, id)
	if err != nil {
		return nil, err
	}
	sheet, err := store.NewSurveys(db).Latest(ctx, id)
	if err != nil {
		return nil, err
	}
	return survey.Apply(sheet, assembly)
}

// staleErr builds the message a command gives when the only survey on file is
// past its shelf life, with the way to override it.
func staleErr(id plant.PoleID, age int) error {
	return fmt.Errorf("the survey of %s is %d days old, past the %d day shelf life; pass -allow-stale to use it anyway",
		id, age, survey.StaleAfterDays)
}

// notFound reports whether an error is the store saying there is no such
// record, which every one of these commands reports as a finding rather than
// as a failure.
func notFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
