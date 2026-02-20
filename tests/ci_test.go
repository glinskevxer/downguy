package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// workflow is only as much of the github actions schema as these checks read.
// Parsing it rather than grepping is what catches a workflow that is valid
// yaml and does not have the job somebody thought they added.
type workflow struct {
	Name        string                 `yaml:"name"`
	On          map[string]any         `yaml:"on"`
	Permissions map[string]string      `yaml:"permissions"`
	Env         map[string]string      `yaml:"env"`
	Jobs        map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	Name     string           `yaml:"name"`
	RunsOn   string           `yaml:"runs-on"`
	Strategy workflowStrategy `yaml:"strategy"`
	Steps    []workflowStep   `yaml:"steps"`
}

type workflowStrategy struct {
	FailFast bool                `yaml:"fail-fast"`
	Matrix   map[string][]string `yaml:"matrix"`
}

type workflowStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
	Run  string            `yaml:"run"`
}

func loadWorkflow(t *testing.T) workflow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var w workflow
	if err := yaml.Unmarshal(body, &w); err != nil {
		t.Fatalf("the workflow is not valid yaml: %v", err)
	}
	return w
}

func TestTheWorkflowParsesAndCoversEveryGate(t *testing.T) {
	t.Parallel()
	w := loadWorkflow(t)
	if w.Name != "ci" {
		t.Errorf("name = %q", w.Name)
	}
	if _, ok := w.On["push"]; !ok {
		t.Error("the workflow does not run on a push")
	}
	if _, ok := w.On["pull_request"]; !ok {
		t.Error("the workflow does not run on a pull request")
	}
	for _, job := range []string{"checks", "test", "cover", "ordering", "smoke", "image", "advisories"} {
		if _, ok := w.Jobs[job]; !ok {
			t.Errorf("there is no %s job", job)
		}
	}
	if len(w.Jobs) < 7 {
		t.Errorf("jobs = %d", len(w.Jobs))
	}
}

func TestEveryJobRunsOnAnAgentAndHasSteps(t *testing.T) {
	t.Parallel()
	w := loadWorkflow(t)
	for name, job := range w.Jobs {
		if job.RunsOn == "" {
			t.Errorf("%s names no runner", name)
		}
		if len(job.Steps) == 0 {
			t.Errorf("%s has no steps", name)
		}
		checkout := false
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/checkout@") {
				checkout = true
			}
			if step.Uses != "" && !strings.Contains(step.Uses, "@") {
				t.Errorf("%s uses %q, which is not pinned", name, step.Uses)
			}
		}
		if !checkout {
			t.Errorf("%s never checks the repository out", name)
		}
	}
}

func TestTheToolchainIsPinnedAndTestedOnTwoVersions(t *testing.T) {
	t.Parallel()
	w := loadWorkflow(t)
	if w.Env["GO_VERSION"] != "1.23.5" {
		t.Errorf("GO_VERSION = %q", w.Env["GO_VERSION"])
	}
	versions := w.Jobs["test"].Strategy.Matrix["go"]
	if len(versions) != 2 {
		t.Fatalf("the test job should run on two toolchains, got %v", versions)
	}
	found := false
	for _, v := range versions {
		if v == "1.23.5" {
			found = true
		}
	}
	if !found {
		t.Errorf("the pinned toolchain should be one of the matrix entries, got %v", versions)
	}
	if w.Jobs["test"].Strategy.FailFast {
		t.Error("one toolchain failing should not cancel the other")
	}
}

func TestTheWorkflowRunsTheMakefileRatherThanItsOwnCommands(t *testing.T) {
	t.Parallel()
	w := loadWorkflow(t)
	wanted := map[string]string{
		"checks":   "make fmt-check",
		"cover":    "make cover-check",
		"ordering": "make race",
		"smoke":    "make smoke",
	}
	for job, want := range wanted {
		found := false
		for _, step := range w.Jobs[job].Steps {
			if strings.Contains(step.Run, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("the %s job never runs %q", job, want)
		}
	}
}

func TestTheImageJobActuallyExercisesTheImage(t *testing.T) {
	t.Parallel()
	w := loadWorkflow(t)
	var script strings.Builder
	for _, step := range w.Jobs["image"].Steps {
		script.WriteString(step.Run)
		script.WriteString("\n")
	}
	text := script.String()
	for _, want := range []string{"docker build", "docker run", "/readyz", "/healthz", "docker rm -f"} {
		if !strings.Contains(text, want) {
			t.Errorf("the image job never does %q", want)
		}
	}
	if !strings.Contains(text, "--build-arg RELEASE=") {
		t.Error("the image job should stamp a release into the build")
	}
}

func TestPermissionsAreReadOnly(t *testing.T) {
	t.Parallel()
	w := loadWorkflow(t)
	if w.Permissions["contents"] != "read" {
		t.Errorf("contents permission = %q", w.Permissions["contents"])
	}
	for key, value := range w.Permissions {
		if value == "write" {
			t.Errorf("the workflow asks for write on %s", key)
		}
	}
}
