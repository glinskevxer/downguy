package cliapp

import (
	"fmt"

	"github.com/glinskevxer/downguy/internal/calendar"
)

// dateFlag reads a registered date flag. An unset flag reads as the zero date
// rather than as today, because a command that quietly substitutes the clock
// for a missing date produces a different answer every day it is run and
// nobody can reproduce the first one.
func dateFlag(f *Flags, name string) (calendar.Date, error) {
	value := f.Set().Lookup(name)
	if value == nil {
		return calendar.Zero, nil
	}
	text := value.Value.String()
	if text == "" {
		return calendar.Zero, nil
	}
	parsed, err := calendar.ParseDate(text)
	if err != nil {
		return calendar.Zero, fmt.Errorf("-%s: %w", name, err)
	}
	return parsed, nil
}

// requiredDateFlag reads a date flag that a command cannot run without.
func requiredDateFlag(f *Flags, name string) (calendar.Date, error) {
	parsed, err := dateFlag(f, name)
	if err != nil {
		return calendar.Zero, err
	}
	if parsed.IsZero() {
		return calendar.Zero, fmt.Errorf("-%s is required and takes a date as yyyy-mm-dd", name)
	}
	return parsed, nil
}
