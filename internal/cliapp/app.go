// Package cliapp is the command line.
//
// Every entry point goes through RunWith, which takes its argv, its streams
// and returns an exit code rather than calling os.Exit. That is what makes the
// whole command line testable from the test package without a subprocess, and
// it is the reason nothing here reaches for os.Args or os.Stdout directly.
package cliapp

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/version"
)

// Exit codes. They are part of the documented interface: a script running this
// under a make ready pipeline branches on them.
const (
	// CodeOK means the command ran and found nothing to report.
	CodeOK = 0
	// CodeFindings means the command ran and found something: violations, make
	// ready, or an overloaded pole.
	CodeFindings = 1
	// CodeUsage means the command line itself was wrong.
	CodeUsage = 2
	// CodeInternal means something failed that the caller cannot fix.
	CodeInternal = 3
)

// Env is everything a command is allowed to touch outside itself.
type Env struct {
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) (string, bool)
}

// Command is one subcommand.
type Command struct {
	Name    string
	Summary string
	Usage   string
	Run     func(*Env) int
}

var commands = map[string]Command{}

// Register adds a subcommand. It panics on a duplicate name, because two
// commands answering to one word is a programming error that should not reach
// a build.
func Register(c Command) { mustRegister(c) }

// mustRegister is where the registration guards live. Every caller is an init
// function, so a panic here is a build that will not start rather than a
// failure at runtime, which is what a duplicate command name deserves.
func mustRegister(c Command) {
	if c.Name == "" || c.Run == nil {
		panic("cliapp: a command needs a name and something to run")
	}
	if _, seen := commands[c.Name]; seen {
		panic("cliapp: two commands named " + c.Name)
	}
	commands[c.Name] = c
}

// Lookup returns a registered command.
func Lookup(name string) (Command, bool) {
	c, ok := commands[name]
	return c, ok
}

// Names returns every registered command, sorted.
func Names() []string {
	out := make([]string, 0, len(commands))
	for name := range commands {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// RunWith dispatches one command line and returns its exit code.
func RunWith(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) (string, bool)) int {
	if getenv == nil {
		getenv = func(string) (string, bool) { return "", false }
	}
	env := &Env{Stdin: stdin, Stdout: stdout, Stderr: stderr, Getenv: getenv}

	if len(args) == 0 {
		writeUsage(stderr)
		return CodeUsage
	}
	name := args[0]
	switch name {
	case "-h", "--help", "help":
		if len(args) > 1 {
			if c, ok := Lookup(args[1]); ok {
				fmt.Fprintln(stdout, strings.TrimRight(c.Usage, "\n"))
				return CodeOK
			}
			fmt.Fprintf(stderr, "no command named %q\n", args[1])
			return CodeUsage
		}
		writeUsage(stdout)
		return CodeOK
	case "-v", "--version":
		fmt.Fprintln(stdout, version.Current())
		return CodeOK
	}

	c, ok := Lookup(name)
	if !ok {
		fmt.Fprintf(stderr, "no command named %q\n", name)
		writeUsage(stderr)
		return CodeUsage
	}
	env.Args = args[1:]
	return c.Run(env)
}

func writeUsage(w io.Writer) {
	fmt.Fprintf(w, "%s runs joint use pole applications from survey to settlement.\n\n", version.Name)
	fmt.Fprintf(w, "usage: %s <command> [options]\n\n", version.Name)
	fmt.Fprintln(w, "commands:")
	names := Names()
	width := 0
	for _, name := range names {
		if len(name) > width {
			width = len(name)
		}
	}
	for _, name := range names {
		fmt.Fprintf(w, "  %-*s  %s\n", width, name, commands[name].Summary)
	}
	fmt.Fprintf(w, "\nrun %s help <command> for the options a command takes.\n", version.Name)
}

// Fail writes a message to standard error and returns an exit code, which is
// the shape every command's error path takes.
func (e *Env) Fail(code int, format string, args ...any) int {
	fmt.Fprintf(e.Stderr, format+"\n", args...)
	return code
}

// Print writes to standard output.
func (e *Env) Print(format string, args ...any) {
	fmt.Fprintf(e.Stdout, format, args...)
}

// Println writes a line to standard output.
func (e *Env) Println(args ...any) {
	fmt.Fprintln(e.Stdout, args...)
}

func init() {
	Register(Command{
		Name:    "version",
		Summary: "print the build information",
		Usage:   "usage: downguy version\n\nPrints the release, the revision it was built from and the toolchain.",
		Run: func(env *Env) int {
			fmt.Fprint(env.Stdout, version.Current().Long())
			return CodeOK
		},
	})
}
