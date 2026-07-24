package cliapp

import (
	"errors"
	"strings"
)

// listFlag collects a repeated option into a list, in the order it was given.
// The standard flag package has no repeated string, and the alternative is a
// comma separated value that then cannot carry a name with a comma in it.
type listFlag struct {
	values []string
}

// String prints what has been collected, which is what the flag package uses
// for the default in a usage line.
func (l *listFlag) String() string { return strings.Join(l.values, ",") }

// Set records one occurrence.
func (l *listFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("an empty value")
	}
	l.values = append(l.values, trimmed)
	return nil
}
