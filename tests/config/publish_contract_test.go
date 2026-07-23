package tests

import (
	"encoding/json"
	"os"
	"os/exec"
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

func TestMakeLifecyclePreparesPublishesAndDeploysDistinctArtifacts(t *testing.T) {
	t.Helper()

	makefile := string(readRepoFile(t, "Makefile"))
	requiredSnippets := []string{
		"DOCKER_PLATFORMS ?= linux/amd64,linux/arm64",
		"DOCKER_BUILD_CONTEXT ?= .",
		"PUBLISH_PLATFORMS ?= $(DOCKER_PLATFORMS)",
		"RELEASE_ARTIFACT_TARGETS ?= release-artifacts container-artifacts pages-artifact",
		"RELEASE_TOOL_DIR := $(abspath $(CURDIR)/scripts/release)",
		"PAGES_PUBLISH_BRANCH ?= gh-pages",
		"release: ## Prepare a local repository release without publishing or deploying",
		"prepare_container_artifact.sh",
		"prepare_pages_artifact.sh",
		"publish: publish-release",
		"publish_container_artifacts.sh",
		"pages-deploy:",
		"deploy_pages_artifact.sh",
		"./scripts/deploy.sh $(DEPLOY_ARGS)",
		"./scripts/build_pages_artifact.sh",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(makefile, requiredSnippet) {
			t.Fatalf("Makefile missing publish contract snippet %q", requiredSnippet)
		}
	}
	forbiddenSnippets := []string{
		"DOCKER_CONTEXT ?= .",
		"DOCKER_CONTEXT=\"$(DOCKER_CONTEXT)\"",
		"./scripts/publish.sh",
		"./scripts/publish_pages_branch.sh",
		"./scripts/deploy_pages.sh",
		"agentSkills/gitrelease",
	}
	for _, forbiddenSnippet := range forbiddenSnippets {
		if strings.Contains(makefile, forbiddenSnippet) {
			t.Fatalf("Makefile exports Docker CLI context variable %q", forbiddenSnippet)
		}
	}
}

func TestPublishConsumesPreparedArtifactsWithoutBuilding(t *testing.T) {
	t.Helper()

	makefile := string(readRepoFile(t, "Makefile"))
	publishReleaseScript := string(readRepoFile(t, "scripts", "publish-release.sh"))
	requiredMakeSnippets := []string{
		"publish: publish-release",
		"publish_container_artifacts.sh",
	}
	for _, requiredSnippet := range requiredMakeSnippets {
		if !strings.Contains(makefile, requiredSnippet) {
			t.Fatalf("Makefile missing prepared publish contract snippet %q", requiredSnippet)
		}
	}
	if !strings.Contains(publishReleaseScript, "publish_release.sh") {
		t.Fatalf("publish-release wrapper must invoke the repository-owned prepared release publisher")
	}
	forbiddenSnippets := []string{
		"docker build",
		"buildx build",
		"make ci",
		"pages-deploy",
		"deploy_pages_artifact.sh",
	}
	for _, forbiddenSnippet := range forbiddenSnippets {
		if strings.Contains(makefile, "publish: publish-release\n\t"+forbiddenSnippet) || strings.Contains(publishReleaseScript, forbiddenSnippet) {
			t.Fatalf("publish path must not contain %q", forbiddenSnippet)
		}
	}
}

func TestProductionGitGuardRequiresMasterRemoteAndNoOpenPRs(t *testing.T) {
	t.Helper()

	guardScript := string(readRepoFile(t, "scripts", "production_git_guard.sh"))
	requiredSnippets := []string{
		"verify_production_git_state()",
		"branch=\"${2:-master}\"",
		"branch}\" != \"master\"",
		"git status --porcelain",
		"git fetch --prune \"${remote}\" \"+refs/heads/${branch}:refs/remotes/${remote}/${branch}\"",
		"refs/remotes/${remote}/${branch}",
		"gh pr list --state open --limit 1000 --json number --jq 'length'",
		"requires zero open PRs",
		"branch=${branch} remote=${remote}/${branch} commit=${head_sha} open_prs=0",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(guardScript, requiredSnippet) {
			t.Fatalf("production git guard missing contract snippet %q", requiredSnippet)
		}
	}
}

func TestReleaseScriptUsesStrictlyLocalPreparationPipeline(t *testing.T) {
	t.Helper()

	releaseScript := string(readRepoFile(t, "scripts", "release.sh"))
	requiredSnippets := []string{
		"exec \"${repo_root}/scripts/release/prepare_release.sh\" \"$@\"",
	}
	if strings.Contains(releaseScript, "agentSkills/gitrelease") {
		t.Fatalf("release script must not load mutable sibling release tooling")
	}
	publishScript := string(readRepoFile(t, "scripts", "publish-release.sh"))
	if !strings.Contains(publishScript, "exec \"${repo_root}/scripts/release/publish_release.sh\" \"$@\"") {
		t.Fatalf("publish wrapper must invoke the repository-owned prepared release publisher")
	}
	if strings.Contains(publishScript, "agentSkills/gitrelease") {
		t.Fatalf("publish wrapper must not load mutable sibling release tooling")
	}
	for _, releaseScriptName := range []string{
		"prepare_release.sh",
		"publish_release.sh",
		"release_helper.py",
		"prepare_pages_artifact.sh",
		"deploy_pages_artifact.sh",
		"prepare_container_artifact.sh",
		"publish_container_artifacts.sh",
	} {
		if _, statErr := os.Stat(repoPath("scripts", "release", releaseScriptName)); statErr != nil {
			t.Fatalf("repository-owned release script missing: %s: %v", releaseScriptName, statErr)
		}
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(releaseScript, requiredSnippet) {
			t.Fatalf("release script missing local preparation snippet %q", requiredSnippet)
		}
	}
	for _, forbiddenSnippet := range []string{"git push", "gh release", "verify_production_git_state", "production_git_guard.sh"} {
		if strings.Contains(releaseScript, forbiddenSnippet) {
			t.Fatalf("release script must not contain remote operation %q", forbiddenSnippet)
		}
	}
}

func TestProductionDockerfilePublishesDoctorPreflightCommand(t *testing.T) {
	t.Helper()

	dockerfile := string(readRepoFile(t, "Dockerfile"))
	requiredSnippets := []string{
		"go build -o /workspace/bin/pinguin ./cmd/server",
		"go build -o /workspace/bin/pinguin-doctor ./cmd/doctor",
		"COPY --from=builder /workspace/bin/pinguin /usr/local/bin/pinguin",
		"COPY --from=builder /workspace/bin/pinguin-doctor /usr/local/bin/pinguin-doctor",
		`CMD ["/usr/local/bin/pinguin"]`,
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(dockerfile, requiredSnippet) {
			t.Fatalf("Dockerfile missing production preflight contract snippet %q", requiredSnippet)
		}
	}
	if strings.Contains(dockerfile, `ENTRYPOINT ["/usr/local/bin/pinguin"]`) {
		t.Fatalf("Dockerfile must allow compose run commands to invoke pinguin-doctor")
	}
}

func TestDeployScriptConsumesPublishedBackendAndPagesArtifacts(t *testing.T) {
	t.Helper()

	deployScript := string(readRepoFile(t, "scripts", "deploy.sh"))
	for _, requiredSnippet := range []string{
		"Deploys the published Pinguin backend through mprlab-gateway",
		"SKIP_IMAGE_VERIFY=\"false\"",
		"SKIP_BACKEND=\"false\"",
		"SKIP_PAGES=\"false\"",
		"SKIP_PAGES_VERIFY=\"false\"",
		"source \"${repo_root}/scripts/production_git_guard.sh\"",
		"verify_production_git_state \"deploy\" \"${PUBLISH_BRANCH}\" \"${PUBLISH_REMOTE}\"",
		"no v* release tag points at HEAD; pass --tag or run make publish first",
		"Verifying ${IMAGE_REPOSITORY}:latest matches ${TAG}",
		"deploy-pinguin-backend",
		"Activating the published Pages artifact for ${TAG}",
		"PAGES_VERSION=\"${TAG}\"",
		"make --no-print-directory pages-deploy",
		"edge 25 -> tutosh:8025 and edge 465 -> tutosh:8465",
	} {
		if !strings.Contains(deployScript, requiredSnippet) {
			t.Fatalf("deploy script missing published-artifact contract snippet %q", requiredSnippet)
		}
	}
	for _, forbiddenSnippet := range []string{
		"SKIP_CI",
		"make ci",
		"docker build ",
		"docker push ",
		"publish_container_artifacts",
		"./scripts/publish_pages_branch.sh",
		"./scripts/deploy_pages.sh",
		"MPRLAB_APP_MANIFEST",
		"require_gateway_contract_snippet",
		"verify_gateway_smtp_port_contract",
		"mprlab_verify_pinguin_smtp_port",
		"mprlab_verify_pinguin_mx_port",
	} {
		if strings.Contains(deployScript, forbiddenSnippet) {
			t.Fatalf("deploy script must not contain retired or non-activating contract %q", forbiddenSnippet)
		}
	}

	readme := string(readRepoFile(t, "README.md"))
	for _, requiredSnippet := range []string{
		"`make release` prepares",
		"`make publish` publishes",
		"`make deploy` activates",
		"clean local `master` branch that exactly matches `origin/master`",
		"zero open pull requests",
		"After `make deploy`, configure the edge gateway to forward `25 -> tutosh:8025` and `465 -> tutosh:8465`",
	} {
		if !strings.Contains(readme, requiredSnippet) {
			t.Fatalf("README missing lifecycle contract snippet %q", requiredSnippet)
		}
	}

	resourceManifest := string(readRepoFile(t, ".mprlab", "deploy", "resources.yml"))
	for _, requiredSnippet := range []string{
		"directory: pinguin",
		"dispatch_target: pinguin",
		"type: container_service",
		"image: ghcr.io/tyemirov/pinguin:latest",
		"type: github_pages",
		"target: pages-deploy",
		"url: https://pinguin.mprlab.com/",
		"source_marker_path: /.mprlab-release.json",
	} {
		if !strings.Contains(resourceManifest, requiredSnippet) {
			t.Fatalf(".mprlab/deploy/resources.yml missing lifecycle resource snippet %q", requiredSnippet)
		}
	}
	if strings.Contains(resourceManifest, "url_variable:") {
		t.Fatalf(".mprlab/deploy/resources.yml must not contain the retired Pages url_variable field")
	}
}

func TestDeployDelegatesGatewayValidationToBackendTarget(t *testing.T) {
	t.Helper()

	temporaryDirectory := t.TempDir()
	binaryDirectory := filepath.Join(temporaryDirectory, "bin")
	if mkdirErr := os.Mkdir(binaryDirectory, 0o755); mkdirErr != nil {
		t.Fatalf("create fake binary directory: %v", mkdirErr)
	}

	gatewayDirectory := filepath.Join(temporaryDirectory, "mprlab-gateway")
	if mkdirErr := os.Mkdir(gatewayDirectory, 0o755); mkdirErr != nil {
		t.Fatalf("create gateway directory: %v", mkdirErr)
	}

	writeTestExecutable(t, filepath.Join(binaryDirectory, "git"), `#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  "rev-parse --show-toplevel")
    printf '%s\n' "${TEST_REPO_ROOT}"
    ;;
  "branch --show-current")
    printf '%s\n' "master"
    ;;
  "status --porcelain"|"fetch --prune origin +refs/heads/master:refs/remotes/origin/master")
    ;;
  "rev-parse HEAD"|"rev-parse refs/remotes/origin/master")
    printf '%s\n' "1111111111111111111111111111111111111111"
    ;;
  *)
    printf 'unexpected git invocation: %s\n' "$*" >&2
    exit 1
    ;;
esac
`)
	writeTestExecutable(t, filepath.Join(binaryDirectory, "gh"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "0"
`)

	makeInvocationPath := filepath.Join(temporaryDirectory, "make-invocation")
	writeTestExecutable(t, filepath.Join(binaryDirectory, "make"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${TEST_MAKE_INVOCATION}"
`)

	repositoryRoot, absolutePathErr := filepath.Abs(repoPath())
	if absolutePathErr != nil {
		t.Fatalf("resolve repository root: %v", absolutePathErr)
	}
	command := exec.Command(
		"bash",
		filepath.Join(repositoryRoot, "scripts", "deploy.sh"),
		"--gateway-dir", gatewayDirectory,
		"--tag", "v0.4.22",
		"--skip-image-verify",
		"--skip-pages",
	)
	command.Dir = repositoryRoot
	command.Env = []string{
		"PATH=" + binaryDirectory + ":" + os.Getenv("PATH"),
		"TEST_MAKE_INVOCATION=" + makeInvocationPath,
		"TEST_REPO_ROOT=" + repositoryRoot,
	}
	output, runErr := command.CombinedOutput()
	if runErr != nil {
		t.Fatalf("deploy command failed before gateway target dispatch: %v\n%s", runErr, string(output))
	}

	makeInvocation, readErr := os.ReadFile(makeInvocationPath)
	if readErr != nil {
		t.Fatalf("read gateway make invocation: %v", readErr)
	}
	expectedInvocation := "-C " + gatewayDirectory + " deploy-pinguin-backend\n"
	if string(makeInvocation) != expectedInvocation {
		t.Fatalf("gateway make invocation = %q, want %q", string(makeInvocation), expectedInvocation)
	}
}

func writeTestExecutable(t *testing.T, path string, contents string) {
	t.Helper()
	if writeErr := os.WriteFile(path, []byte(contents), 0o755); writeErr != nil {
		t.Fatalf("write test executable %s: %v", path, writeErr)
	}
}

func TestBuildPagesArtifactWritesSourceCommitMarker(t *testing.T) {
	t.Helper()

	outputDirectory := t.TempDir()
	sourceCommit := strings.Repeat("a", 40)
	command := exec.Command("bash", "scripts/build_pages_artifact.sh", outputDirectory)
	command.Dir = repoPath()
	command.Env = append(os.Environ(), "PAGES_SOURCE_COMMIT="+sourceCommit)
	output, runErr := command.CombinedOutput()
	if runErr != nil {
		t.Fatalf("build pages artifact failed: %v\n%s", runErr, string(output))
	}

	markerBytes, readErr := os.ReadFile(filepath.Join(outputDirectory, "pinguin-pages-build.json"))
	if readErr != nil {
		t.Fatalf("read pages build marker: %v", readErr)
	}

	var marker struct {
		SourceCommit string `json:"sourceCommit"`
		SourceShort  string `json:"sourceShort"`
	}
	if decodeErr := json.Unmarshal(markerBytes, &marker); decodeErr != nil {
		t.Fatalf("decode pages build marker: %v", decodeErr)
	}
	if marker.SourceCommit != sourceCommit {
		t.Fatalf("sourceCommit = %q, want %q", marker.SourceCommit, sourceCommit)
	}
	if marker.SourceShort != sourceCommit[:12] {
		t.Fatalf("sourceShort = %q, want %q", marker.SourceShort, sourceCommit[:12])
	}
}

func TestPagesSourceIncludesNoJekyllAndCNAME(t *testing.T) {
	t.Helper()

	requiredPaths := []string{
		repoPath("web", ".nojekyll"),
		repoPath("web", "CNAME"),
		repoPath("web", "index.html"),
		repoPath("web", "event-log.html"),
		repoPath("web", "smtp-relay.html"),
	}
	for _, requiredPath := range requiredPaths {
		if _, statErr := os.Stat(requiredPath); statErr != nil {
			t.Fatalf("required Pages source file missing: %s: %v", requiredPath, statErr)
		}
	}

}
