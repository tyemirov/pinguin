package tests

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type dockerWorkflow struct {
	On   dockerWorkflowTriggers       `yaml:"on"`
	Jobs map[string]dockerWorkflowJob `yaml:"jobs"`
}

type dockerWorkflowTriggers struct {
	WorkflowRun dockerWorkflowRunTrigger `yaml:"workflow_run"`
}

type goTestsWorkflow struct {
	On goTestsWorkflowTriggers `yaml:"on"`
}

type goTestsWorkflowTriggers struct {
	Push        goTestsPathTrigger `yaml:"push"`
	PullRequest goTestsPathTrigger `yaml:"pull_request"`
}

type goTestsPathTrigger struct {
	Paths []string `yaml:"paths"`
}

type dockerWorkflowRunTrigger struct {
	Workflows []string `yaml:"workflows"`
	Types     []string `yaml:"types"`
}

type dockerWorkflowJob struct {
	If string `yaml:"if"`
}

func TestDockerBuildWorkflowDependsOnSuccessfulGoTests(t *testing.T) {
	t.Helper()

	documentData := readRepoFile(t, ".github", "workflows", "docker-build.yml")

	var workflow dockerWorkflow
	if unmarshalErr := yaml.Unmarshal(documentData, &workflow); unmarshalErr != nil {
		t.Fatalf("failed to parse docker-build workflow: %v", unmarshalErr)
	}

	trigger := workflow.On.WorkflowRun
	if len(trigger.Workflows) == 0 {
		t.Fatalf("workflow_run trigger must list Go Tests workflow")
	}

	assertContains(t, trigger.Workflows, "Go Tests", "workflow_run trigger missing Go Tests entry")
	assertContains(t, trigger.Types, "completed", "workflow_run trigger must listen for completed events")

	buildJob, jobExists := workflow.Jobs["build-and-push"]
	if !jobExists {
		t.Fatalf("build-and-push job must exist")
	}

	expectedCondition := "${{ github.event_name == 'workflow_dispatch' || (github.event.workflow_run.conclusion == 'success' && github.event.workflow_run.event == 'push') }}"
	if strings.TrimSpace(buildJob.If) != expectedCondition {
		t.Fatalf("build-and-push job must guard on successful Go Tests run")
	}
}

func TestGoTestsWorkflowCoversDockerPublishInputs(t *testing.T) {
	t.Helper()

	documentData := readRepoFile(t, ".github", "workflows", "go-tests.yml")

	var workflow goTestsWorkflow
	if unmarshalErr := yaml.Unmarshal(documentData, &workflow); unmarshalErr != nil {
		t.Fatalf("failed to parse Go Tests workflow: %v", unmarshalErr)
	}

	requiredPaths := []string{
		"Dockerfile",
		".github/workflows/docker-build.yml",
	}

	for _, requiredPath := range requiredPaths {
		assertContains(t, workflow.On.Push.Paths, requiredPath, "push trigger must include Docker publish input")
		assertContains(t, workflow.On.PullRequest.Paths, requiredPath, "pull_request trigger must include Docker publish input")
	}
}

func assertContains(t *testing.T, values []string, expectedValue string, failureMessage string) {
	t.Helper()

	for _, value := range values {
		if value == expectedValue {
			return
		}
	}

	t.Fatalf("%s (expected %q)", failureMessage, expectedValue)
}
