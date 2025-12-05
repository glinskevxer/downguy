package tests

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/plant"
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

// plantID turns a literal tag into a PoleID for a fixture, failing loudly on a
// typo rather than quietly building a record nothing can find.
func plantID(text string) plant.PoleID {
	id, err := plant.ParsePoleID(text)
	if err != nil {
		panic(err)
	}
	return id
}

// atoiPart pulls one dash separated field out of an ISO date for a fixture.
func atoiPart(iso string, field int) int {
	parts := strings.Split(iso, "-")
	if field >= len(parts) {
		panic("no field " + strconv.Itoa(field) + " in " + iso)
	}
	n, err := strconv.Atoi(parts[field])
	if err != nil {
		panic(err)
	}
	return n
}

// writeFile drops a fixture file into a test's own directory.
func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o600)
}
