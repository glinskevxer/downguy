package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dockerfile(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// dockerfileInstructions returns the file with its comments stripped, for the
// checks that are about what the image does rather than about what the file
// says it does.
func dockerfileInstructions(t *testing.T) string {
	t.Helper()
	var kept []string
	for _, line := range strings.Split(dockerfile(t), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func TestDockerfilePinsItsBaseImages(t *testing.T) {
	t.Parallel()
	text := dockerfile(t)
	var froms []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "FROM ") {
			froms = append(froms, trimmed)
		}
	}
	if len(froms) != 2 {
		t.Fatalf("a two stage build has two FROM lines, got %v", froms)
	}
	for _, from := range froms {
		if strings.Contains(from, ":latest") || !strings.Contains(from, ":") {
			t.Errorf("%q is not pinned", from)
		}
	}
	if !strings.Contains(froms[0], "golang:1.23.5") {
		t.Errorf("the build stage should pin the toolchain the module asks for, got %q", froms[0])
	}
	if !strings.Contains(froms[1], "distroless") {
		t.Errorf("the runtime stage should be a distroless base, got %q", froms[1])
	}
}

func TestDockerfileBuildsWithoutCgoAndStampsTheVersion(t *testing.T) {
	t.Parallel()
	text := dockerfile(t)
	if !strings.Contains(text, "CGO_ENABLED=0") {
		t.Error("the build should be cgo free, which is what lets the runtime stage be static")
	}
	for _, want := range []string{
		"internal/version.Release=",
		"internal/version.Commit=",
		"internal/version.BuiltAt=",
		"-trimpath",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the build does not pass %q", want)
		}
	}
}

func TestDockerfileRunsUnprivilegedWithAHealthcheck(t *testing.T) {
	t.Parallel()
	text := dockerfile(t)
	if !strings.Contains(text, "USER nonroot") {
		t.Error("the runtime stage should not run as root")
	}
	if !strings.Contains(text, "HEALTHCHECK") {
		t.Error("the image should carry a health check")
	}
	instructions := dockerfileInstructions(t)
	if strings.Contains(instructions, "curl") || strings.Contains(instructions, "wget") {
		t.Error("a distroless image carries no http client, so the health check cannot reach for one")
	}
	if !strings.Contains(text, "EXPOSE 8471") {
		t.Error("the image should expose the port the default configuration listens on")
	}
	if !strings.Contains(text, "ENTRYPOINT") || !strings.Contains(text, "CMD") {
		t.Error("the image should have both an entrypoint and a default command")
	}
}

func TestDockerfileDownloadsModulesBeforeCopyingSource(t *testing.T) {
	t.Parallel()
	text := dockerfile(t)
	download := strings.Index(text, "go mod download")
	copyAll := strings.Index(text, "\nCOPY . .")
	if download < 0 || copyAll < 0 {
		t.Fatalf("the build stage is missing a step:\n%s", text)
	}
	if download > copyAll {
		t.Error("the module download should come before the source copy, or every source change refetches the module cache")
	}
}

func TestDockerignoreKeepsTheContextSmall(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".dockerignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{".git", "bin", "coverage.out", "*.db"} {
		if !strings.Contains(text, want) {
			t.Errorf(".dockerignore does not exclude %q", want)
		}
	}
}

func TestTheDefaultEnvironmentMatchesWhatTheConfigReads(t *testing.T) {
	t.Parallel()
	text := dockerfile(t)
	// The image sets these, so the configuration has to be the thing that
	// reads them. A rename on either side breaks a container that still boots
	// and then quietly writes its database somewhere nobody mounted.
	for _, want := range []string{"DOWNGUY_DATABASE=", "DOWNGUY_ADDRESS="} {
		if !strings.Contains(text, want) {
			t.Errorf("the image does not set %q", want)
		}
	}
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "config", "config.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"DOWNGUY_DATABASE"`, `"DOWNGUY_ADDRESS"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the configuration does not read %s", want)
		}
	}
}
