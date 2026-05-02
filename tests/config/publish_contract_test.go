package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubActionsWorkflowsAreDisabled(t *testing.T) {
	t.Helper()

	workflowFiles, globErr := filepath.Glob(repoPath(".github", "workflows", "*.yml"))
	if globErr != nil {
		t.Fatalf("glob workflow files: %v", globErr)
	}
	if len(workflowFiles) != 0 {
		t.Fatalf("expected no GitHub Actions workflow files, got %v", workflowFiles)
	}

	workflowFiles, globErr = filepath.Glob(repoPath(".github", "workflows", "*.yaml"))
	if globErr != nil {
		t.Fatalf("glob workflow files: %v", globErr)
	}
	if len(workflowFiles) != 0 {
		t.Fatalf("expected no GitHub Actions workflow files, got %v", workflowFiles)
	}
}

func TestMakePublishTargetsDockerAndLegacyPages(t *testing.T) {
	t.Helper()

	makefile := string(readRepoFile(t, "Makefile"))
	requiredSnippets := []string{
		"DOCKER_PLATFORMS ?= linux/amd64",
		"PUBLISH_PLATFORMS ?= $(DOCKER_PLATFORMS)",
		"PAGES_PUBLISH_SOURCE_BRANCH ?= master",
		"PAGES_PUBLISH_BRANCH ?= gh-pages",
		"./scripts/publish.sh $(PUBLISH_ARGS)",
		"./scripts/build_pages_artifact.sh",
		"./scripts/publish_pages_branch.sh",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(makefile, requiredSnippet) {
			t.Fatalf("Makefile missing publish contract snippet %q", requiredSnippet)
		}
	}
}

func TestPublishScriptBuildsDockerAndLegacyPages(t *testing.T) {
	t.Helper()

	publishScript := string(readRepoFile(t, "scripts", "publish.sh"))
	requiredSnippets := []string{
		"timeout -k 350s -s SIGKILL 350s make ci",
		"\"build_type\":\"legacy\"",
		"\"source\":{\"branch\":\"${PAGES_BRANCH}\",\"path\":\"/\"}",
		"--platform \"${PLATFORMS}\"",
		"--tag \"${IMAGE}:${TAG}\"",
		"--push",
		"./scripts/publish_pages_branch.sh",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(publishScript, requiredSnippet) {
			t.Fatalf("publish script missing contract snippet %q", requiredSnippet)
		}
	}
}

func TestPagesSourceIncludesNoJekyllAndCNAME(t *testing.T) {
	t.Helper()

	requiredPaths := []string{
		repoPath("web", ".nojekyll"),
		repoPath("web", "CNAME"),
		repoPath("web", "index.html"),
		repoPath("web", "dashboard.html"),
	}
	for _, requiredPath := range requiredPaths {
		if _, statErr := os.Stat(requiredPath); statErr != nil {
			t.Fatalf("required Pages source file missing: %s: %v", requiredPath, statErr)
		}
	}
}
