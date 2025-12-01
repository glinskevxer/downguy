// Command downguy runs joint use pole applications from survey to settlement.
package main

import (
	"os"

	"github.com/glinskevxer/downguy/internal/cliapp"
)

func main() {
	os.Exit(cliapp.RunWith(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.LookupEnv))
}
