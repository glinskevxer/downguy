package cliapp

import (
	"fmt"
	"os"
)

// readFile loads a file a command was pointed at, with the path in the error.
// Every command that takes a path goes through it so that a missing file reads
// the same however it was reached.
func readFile(path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	return body, nil
}
