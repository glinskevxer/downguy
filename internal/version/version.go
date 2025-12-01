// Package version carries what build this is.
//
// The values are set by the linker at build time. They default to something
// truthful rather than to "unknown", so a binary built without the flags still
// says something a support call can use.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Release is the tagged version, set with a linker flag.
var Release = "dev"

// Commit is the source revision, set with a linker flag.
var Commit = ""

// BuiltAt is when the binary was built, set with a linker flag.
var BuiltAt = ""

// Name is what the program calls itself.
const Name = "downguy"

// Info is everything worth knowing about a build.
type Info struct {
	Name      string
	Release   string
	Commit    string
	BuiltAt   string
	GoVersion string
	Platform  string
	Dirty     bool
}

// Current returns the build information, filling in from the module build
// stamps whatever the linker flags did not set.
func Current() Info {
	info := Info{
		Name: Name, Release: Release, Commit: Commit, BuiltAt: BuiltAt,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.BuiltAt == "" {
				info.BuiltAt = setting.Value
			}
		case "vcs.modified":
			info.Dirty = setting.Value == "true"
		}
	}
	return info
}

// ShortCommit returns the first seven characters of the revision, which is
// what anybody actually quotes.
func (i Info) ShortCommit() string {
	if len(i.Commit) <= 7 {
		return i.Commit
	}
	return i.Commit[:7]
}

// String prints the one line a version flag produces.
func (i Info) String() string {
	parts := []string{i.Name, i.Release}
	if commit := i.ShortCommit(); commit != "" {
		if i.Dirty {
			commit += "+dirty"
		}
		parts = append(parts, commit)
	}
	if i.BuiltAt != "" {
		parts = append(parts, "built "+i.BuiltAt)
	}
	parts = append(parts, i.GoVersion, i.Platform)
	return strings.Join(parts, " ")
}

// Long prints the several lines a version subcommand produces.
func (i Info) Long() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", i.Name, i.Release)
	if i.Commit != "" {
		fmt.Fprintf(&b, "commit    %s\n", i.Commit)
	}
	if i.BuiltAt != "" {
		fmt.Fprintf(&b, "built     %s\n", i.BuiltAt)
	}
	fmt.Fprintf(&b, "go        %s\n", i.GoVersion)
	fmt.Fprintf(&b, "platform  %s\n", i.Platform)
	if i.Dirty {
		fmt.Fprintf(&b, "tree      modified at build time\n")
	}
	return b.String()
}
