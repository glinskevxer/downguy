package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glinskevxer/downguy/internal/cliapp"
	"github.com/glinskevxer/downguy/internal/version"
)

type run struct {
	Code   int
	Stdout string
	Stderr string
}

func cli(args ...string) run {
	var out, errOut bytes.Buffer
	code := cliapp.RunWith(args, strings.NewReader(""), &out, &errOut, nil)
	return run{Code: code, Stdout: out.String(), Stderr: errOut.String()}
}

func cliIn(stdin string, args ...string) run {
	var out, errOut bytes.Buffer
	code := cliapp.RunWith(args, strings.NewReader(stdin), &out, &errOut, nil)
	return run{Code: code, Stdout: out.String(), Stderr: errOut.String()}
}

func TestVersionInfo(t *testing.T) {
	info := version.Current()
	if info.Name != "downguy" {
		t.Errorf("name = %q", info.Name)
	}
	if info.Release == "" || info.GoVersion == "" || info.Platform == "" {
		t.Errorf("info = %+v", info)
	}
	if !strings.Contains(info.String(), "downguy") {
		t.Errorf("String = %q", info.String())
	}
	long := info.Long()
	for _, want := range []string{"downguy", "go ", "platform"} {
		if !strings.Contains(long, want) {
			t.Errorf("Long does not mention %q:\n%s", want, long)
		}
	}
}

func TestShortCommit(t *testing.T) {
	cases := map[string]string{
		"":    "",
		"abc": "abc",
		"0123456789abcdef0123456789abcdef01234567": "0123456",
	}
	for in, want := range cases {
		info := version.Info{Commit: in}
		if got := info.ShortCommit(); got != want {
			t.Errorf("ShortCommit(%q) = %q, want %q", in, got, want)
		}
	}
	dirty := version.Info{Name: "downguy", Release: "1.0", Commit: "0123456789", Dirty: true,
		GoVersion: "go1.23.5", Platform: "linux/amd64", BuiltAt: "2025-11-30T00:00:00Z"}
	line := dirty.String()
	if !strings.Contains(line, "0123456+dirty") {
		t.Errorf("String = %q", line)
	}
	if !strings.Contains(line, "built 2025-11-30") {
		t.Errorf("String = %q", line)
	}
	if !strings.Contains(dirty.Long(), "modified at build time") {
		t.Errorf("Long = %q", dirty.Long())
	}
}

func TestNoArgumentsIsAUsageError(t *testing.T) {
	got := cli()
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "usage:") {
		t.Errorf("stderr = %q", got.Stderr)
	}
	if got.Stdout != "" {
		t.Errorf("nothing should go to stdout, got %q", got.Stdout)
	}
}

func TestHelp(t *testing.T) {
	for _, flag := range []string{"help", "-h", "--help"} {
		got := cli(flag)
		if got.Code != cliapp.CodeOK {
			t.Errorf("%s gave code %d", flag, got.Code)
		}
		if !strings.Contains(got.Stdout, "commands:") {
			t.Errorf("%s printed %q", flag, got.Stdout)
		}
		for _, name := range cliapp.Names() {
			if !strings.Contains(got.Stdout, name) {
				t.Errorf("%s does not list %q", flag, name)
			}
		}
	}
}

func TestHelpForOneCommand(t *testing.T) {
	got := cli("help", "version")
	if got.Code != cliapp.CodeOK {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "usage: downguy version") {
		t.Errorf("stdout = %q", got.Stdout)
	}
	missing := cli("help", "nonsense")
	if missing.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", missing.Code)
	}
	if !strings.Contains(missing.Stderr, "nonsense") {
		t.Errorf("stderr = %q", missing.Stderr)
	}
}

func TestVersionFlagAndCommand(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		got := cli(flag)
		if got.Code != cliapp.CodeOK || !strings.Contains(got.Stdout, "downguy") {
			t.Errorf("%s = %+v", flag, got)
		}
	}
	got := cli("version")
	if got.Code != cliapp.CodeOK {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stdout, "platform") {
		t.Errorf("stdout = %q", got.Stdout)
	}
}

func TestAnUnknownCommand(t *testing.T) {
	got := cli("frobnicate")
	if got.Code != cliapp.CodeUsage {
		t.Errorf("code = %d", got.Code)
	}
	if !strings.Contains(got.Stderr, "frobnicate") || !strings.Contains(got.Stderr, "usage:") {
		t.Errorf("stderr = %q", got.Stderr)
	}
}

func TestRegistryIsWellFormed(t *testing.T) {
	names := cliapp.Names()
	if len(names) == 0 {
		t.Fatal("nothing is registered")
	}
	for _, name := range names {
		c, ok := cliapp.Lookup(name)
		if !ok {
			t.Errorf("%s is listed but not registered", name)
			continue
		}
		if c.Name != name || c.Summary == "" || c.Usage == "" || c.Run == nil {
			t.Errorf("%s = %+v", name, c)
		}
		if !strings.HasPrefix(c.Usage, "usage: downguy "+name) {
			t.Errorf("%s usage does not start with its own invocation: %q", name, c.Usage)
		}
	}
	if _, ok := cliapp.Lookup("frobnicate"); ok {
		t.Error("Lookup should miss")
	}
}

func TestEnvHelpers(t *testing.T) {
	var out, errOut bytes.Buffer
	env := &cliapp.Env{Stdout: &out, Stderr: &errOut}
	env.Print("%d poles\n", 3)
	env.Println("done")
	if out.String() != "3 poles\ndone\n" {
		t.Errorf("stdout = %q", out.String())
	}
	if code := env.Fail(cliapp.CodeInternal, "broke: %v", "reason"); code != cliapp.CodeInternal {
		t.Errorf("code = %d", code)
	}
	if errOut.String() != "broke: reason\n" {
		t.Errorf("stderr = %q", errOut.String())
	}
}
