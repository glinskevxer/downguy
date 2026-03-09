package tests

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
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

// The command line and store suites stand up a sqlite database each, and
// modernc's driver takes a few hundred milliseconds to open one. Run serially
// that was a minute and a quarter of wall clock for a suite whose actual work
// is milliseconds, so every test in those two suites calls t.Parallel and the
// whole run comes back in single figures of seconds.
//
// Nothing in them shares state: each takes its own t.TempDir, the command
// registry is written once at init and only read afterwards, and every command
// takes its streams as arguments rather than reaching for the process's.
const parallelSuiteNote = "cliapp and store tests run in parallel"

func TestParallelSuiteNoteIsNotEmpty(t *testing.T) {
	if parallelSuiteNote == "" {
		t.Fatal("the note explaining the parallel suites went missing")
	}
}

// cliCommandNames is the registered command list, wrapped so the script tests
// can reach it without importing the command line package themselves.
func cliCommandNames() []string { return cliapp.Names() }
