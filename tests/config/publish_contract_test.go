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

func TestMakePublishAndDeploySplitDockerAndLegacyPages(t *testing.T) {
	t.Helper()

	makefile := string(readRepoFile(t, "Makefile"))
	requiredSnippets := []string{
		"DOCKER_PLATFORMS ?= linux/amd64,linux/arm64",
		"DOCKER_BUILD_CONTEXT ?= .",
		"PUBLISH_PLATFORMS ?= $(DOCKER_PLATFORMS)",
		"PUBLISH_BRANCH ?= master",
		"PUBLISH_REMOTE ?= origin",
		"PAGES_PUBLISH_SOURCE_BRANCH ?= master",
		"PAGES_PUBLISH_BRANCH ?= gh-pages",
		"DOCKER_BUILD_CONTEXT=\"$(DOCKER_BUILD_CONTEXT)\"",
		"./scripts/publish.sh $(PUBLISH_ARGS)",
		"./scripts/deploy.sh $(DEPLOY_ARGS)",
		"./scripts/build_pages_artifact.sh",
		"./scripts/publish_pages_branch.sh",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(makefile, requiredSnippet) {
			t.Fatalf("Makefile missing publish contract snippet %q", requiredSnippet)
		}
	}
	forbiddenSnippets := []string{
		"DOCKER_CONTEXT ?= .",
		"DOCKER_CONTEXT=\"$(DOCKER_CONTEXT)\"",
	}
	for _, forbiddenSnippet := range forbiddenSnippets {
		if strings.Contains(makefile, forbiddenSnippet) {
			t.Fatalf("Makefile exports Docker CLI context variable %q", forbiddenSnippet)
		}
	}
}

func TestPublishScriptBuildsDockerOnly(t *testing.T) {
	t.Helper()

	publishScript := string(readRepoFile(t, "scripts", "publish.sh"))
	requiredSnippets := []string{
		"DOCKER_CONTEXT_DIR=\"${DOCKER_BUILD_CONTEXT:-.}\"",
		"PLATFORMS=\"${PUBLISH_PLATFORMS:-linux/amd64,linux/arm64}\"",
		"timeout -k 350s -s SIGKILL 350s make ci",
		"git tag --points-at HEAD --list 'v*'",
		"--context|--build-context)",
		"--platform \"${PLATFORMS}\"",
		"tag_args=(--tag \"${IMAGE}:${TAG}\")",
		"--push",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(publishScript, requiredSnippet) {
			t.Fatalf("publish script missing contract snippet %q", requiredSnippet)
		}
	}
	forbiddenSnippets := []string{
		"\"build_type\":\"legacy\"",
		"./scripts/publish_pages_branch.sh",
		"Published Pages",
	}
	for _, forbiddenSnippet := range forbiddenSnippets {
		if strings.Contains(publishScript, forbiddenSnippet) {
			t.Fatalf("publish script still owns Pages contract snippet %q", forbiddenSnippet)
		}
	}
	if strings.Contains(publishScript, "DOCKER_CONTEXT:-") {
		t.Fatalf("publish script must not treat Docker CLI DOCKER_CONTEXT as a build context")
	}
}

func TestDeployScriptDeploysBackendThenLegacyPages(t *testing.T) {
	t.Helper()

	deployScript := string(readRepoFile(t, "scripts", "deploy.sh"))
	requiredSnippets := []string{
		"make -C \"${GATEWAY_DIR}\" deploy TARGET=pinguin",
		"Verifying ${IMAGE_REPOSITORY}:latest matches ${TAG}",
		"./scripts/publish_pages_branch.sh",
		"\"build_type\":\"legacy\"",
		"\"path\":\"/\"",
		"curl --fail --silent --show-error --location --max-time 30 \"${PAGES_URL}\"",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(deployScript, requiredSnippet) {
			t.Fatalf("deploy script missing contract snippet %q", requiredSnippet)
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
