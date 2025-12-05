package cliapp

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"

	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/store"
)

// Flags wraps a flag set with the options nearly every command takes, so that
// "-db" and "-config" mean the same thing everywhere and are documented once.
type Flags struct {
	set        *flag.FlagSet
	configPath string
	database   string
	jsonOut    bool
}

// NewFlags starts a flag set for a command, with its errors going to the
// command's own stderr rather than to the process's.
func NewFlags(name string, out io.Writer) *Flags {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(out)
	f := &Flags{set: set}
	set.StringVar(&f.configPath, "config", "", "path to a configuration file")
	set.StringVar(&f.database, "db", "", "path to the database, overriding the configuration")
	return f
}

// Set returns the underlying flag set so a command can add its own options.
func (f *Flags) Set() *flag.FlagSet { return f.set }

// WithJSON adds the option that switches a command from a printed report to
// machine readable output.
func (f *Flags) WithJSON() *Flags {
	f.set.BoolVar(&f.jsonOut, "json", false, "write json instead of a printed report")
	return f
}

// JSON reports whether json output was asked for.
func (f *Flags) JSON() bool { return f.jsonOut }

// Parse reads the arguments.
func (f *Flags) Parse(args []string) error { return f.set.Parse(args) }

// Rest returns the arguments left after the options.
func (f *Flags) Rest() []string { return f.set.Args() }

// Config loads the configuration, applies the environment, and then applies
// the command line on top. The order matters and it is the usual one: a flag
// beats the environment, and the environment beats the file.
func (f *Flags) Config(env *Env) (config.Config, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return config.Config{}, err
	}
	cfg, err = cfg.FromEnv(env.Getenv)
	if err != nil {
		return config.Config{}, err
	}
	if f.database != "" {
		cfg.Database = f.database
		if err := cfg.Validate(); err != nil {
			return config.Config{}, err
		}
	}
	return cfg, nil
}

// Open loads the configuration and opens the database it names.
func (f *Flags) Open(ctx context.Context, env *Env) (*sql.DB, config.Config, error) {
	cfg, err := f.Config(env)
	if err != nil {
		return nil, config.Config{}, err
	}
	db, err := store.Open(ctx, cfg.Database)
	if err != nil {
		return nil, config.Config{}, err
	}
	return db, cfg, nil
}

// Usage builds the usage text for a command from its own description and the
// options it registered, so that help and the flag set cannot drift apart.
func (f *Flags) Usage(name, line, description string) string {
	var b writerString
	fmt.Fprintf(&b, "usage: downguy %s\n\n%s\n\noptions:\n", line, description)
	f.set.SetOutput(&b)
	f.set.PrintDefaults()
	return b.String()
}

// writerString collects written bytes into a string.
type writerString struct{ parts []byte }

func (w *writerString) Write(p []byte) (int, error) {
	w.parts = append(w.parts, p...)
	return len(p), nil
}

func (w *writerString) String() string { return string(w.parts) }
