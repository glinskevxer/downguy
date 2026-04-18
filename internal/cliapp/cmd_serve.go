package cliapp

import (
	"context"
	"io"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/glinskevxer/downguy/internal/apiserver"
)

func init() {
	Register(Command{
		Name:    "serve",
		Summary: "run the read side http service",
		Usage:   serveUsage(),
		Run:     runServe,
	})
}

func serveUsage() string {
	f := newServeFlags(nil)
	return f.Usage("serve [options]", "serve [options]",
		"Runs the read side service. It answers questions about plant, published\n"+
			"rates and the code rules, and it has no write side at all: anything that\n"+
			"changes a record goes through the command line.\n\n"+
			"It stops cleanly on an interrupt, finishing whatever request is in flight.")
}

func newServeFlags(out io.Writer) *Flags {
	f := NewFlags("serve", out)
	f.Set().String("address", "", "the address to listen on, overriding the configuration")
	f.Set().Bool("check", false, "build the service and print its routes without listening")
	return f
}

func runServe(env *Env) int {
	f := newServeFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "serve takes no arguments")
	}
	cfg, err := f.Config(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	if address := f.Set().Lookup("address").Value.String(); address != "" {
		cfg.HTTP.Address = address
		if err := cfg.Validate(); err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	service, err := apiserver.New(db, cfg)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	service.AllRoutes()

	if flagBool(f, "check") {
		env.Print("%s would listen on %s\n", cfg.Utility, cfg.HTTP.Address)
		for _, path := range service.Routes() {
			env.Print("  %s\n", path)
		}
		return CodeOK
	}

	env.Print("listening on %s\n", cfg.HTTP.Address)
	err = service.Serve(ctx, func(server *http.Server) error { return server.ListenAndServe() })
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	env.Println("stopped")
	return CodeOK
}
