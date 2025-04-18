package tests

import (
	"testing"

	"github.com/glinskevxer/downguy/internal/sagten"
)

// sagtenLookup is the catalogue lookup with the test failure attached, so that
// a fixture does not have to spell out the same three lines every time.
func sagtenLookup(t *testing.T, code string) (sagten.Conductor, error) {
	t.Helper()
	c, err := sagten.Lookup(code)
	if err != nil {
		return sagten.Conductor{}, err
	}
	return c, nil
}
