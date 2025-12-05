package cliapp

import (
	"context"

	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "init",
		Summary: "create or migrate the database",
		Usage:   initUsage(),
		Run:     runInit,
	})
	Register(Command{
		Name:    "config",
		Summary: "print the settings a run would use",
		Usage:   configUsage(),
		Run:     runConfig,
	})
}

func initUsage() string {
	f := NewFlags("init", nil)
	return f.Usage("init [options]", "init [options]",
		"Creates the database if it is not there and brings its schema up to date.\nRunning it against an already current database does nothing and says so.")
}

func runInit(env *Env) int {
	f := NewFlags("init", env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "init takes no arguments")
	}
	ctx := context.Background()
	cfg, err := f.Config(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	before, err := currentVersion(ctx, cfg.Database)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	after, err := store.CurrentVersion(ctx, db)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if after == before {
		env.Print("%s is already at schema version %d\n", cfg.Database, after)
		return CodeOK
	}
	env.Print("%s migrated from schema version %d to %d\n", cfg.Database, before, after)
	return CodeOK
}

// currentVersion opens a database without migrating it, so that init can say
// what it changed rather than only what it ended at.
func currentVersion(ctx context.Context, dsn string) (int, error) {
	db, err := store.OpenRaw(dsn)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()
	return store.CurrentVersion(ctx, db)
}

func configUsage() string {
	f := NewFlags("config", nil)
	return f.Usage("config [options]", "config [options]",
		"Prints the settings this run would use, after the file, the environment\nand the command line have all been applied.")
}

func runConfig(env *Env) int {
	f := NewFlags("config", env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "config takes no arguments")
	}
	cfg, err := f.Config(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	out, err := cfg.Marshal()
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	env.Print("%s", out)
	return CodeOK
}
