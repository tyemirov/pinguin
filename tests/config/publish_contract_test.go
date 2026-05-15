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
		"source \"${repo_root}/scripts/production_git_guard.sh\"",
		"verify_production_git_state \"publish\" \"${PUBLISH_BRANCH}\" \"${PUBLISH_REMOTE}\"",
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

func TestReleaseScriptUsesProductionGitGuard(t *testing.T) {
	t.Helper()

	releaseScript := string(readRepoFile(t, "scripts", "release.sh"))
	requiredSnippets := []string{
		"RELEASE_BRANCH=\"${RELEASE_BRANCH:-master}\"",
		"RELEASE_REMOTE=\"${RELEASE_REMOTE:-origin}\"",
		"source \"${repo_root}/scripts/production_git_guard.sh\"",
		"verify_production_git_state \"release\" \"${RELEASE_BRANCH}\" \"${RELEASE_REMOTE}\"",
		"release default branch must be ${RELEASE_BRANCH}",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(releaseScript, requiredSnippet) {
			t.Fatalf("release script missing production git guard snippet %q", requiredSnippet)
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

func TestDeployScriptDeploysBackendThenLegacyPages(t *testing.T) {
	t.Helper()

	deployScript := string(readRepoFile(t, "scripts", "deploy.sh"))
	requiredSnippets := []string{
		"SKIP_CI=\"false\"",
		"SKIP_BACKEND=\"false\"",
		"SKIP_PAGES=\"false\"",
		"SKIP_PAGES_VERIFY=\"false\"",
		"source \"${repo_root}/scripts/production_git_guard.sh\"",
		"verify_production_git_state \"deploy\" \"${PUBLISH_BRANCH}\" \"${PUBLISH_REMOTE}\"",
		"verify_gateway_smtp_port_contract",
		"PINGUIN_SMTP_HOST_PORT=8465",
		"PINGUIN_SMTP_FORWARDING_HOST_PORT=8025",
		"${PINGUIN_SMTP_HOST_PORT}:${PINGUIN_SMTP_PUBLIC_PORT}",
		"${PINGUIN_SMTP_FORWARDING_HOST_PORT}:${PINGUIN_SMTP_FORWARDING_PUBLIC_PORT}",
		"mprlab_verify_pinguin_smtp_port: 8465",
		"mprlab_verify_pinguin_mx_port: 8025",
		"make -C \"${GATEWAY_DIR}\" deploy TARGET=pinguin",
		"edge 25 -> tutosh:8025 and edge 465 -> tutosh:8465",
		"Verifying ${IMAGE_REPOSITORY}:latest matches ${TAG}",
		"./scripts/publish_pages_branch.sh",
		"trigger_legacy_pages_deploy",
		"gh api --method POST \"repos/${PAGES_REPOSITORY}/pages/builds\"",
		"\"build_type\":\"legacy\"",
		"\"path\":\"/\"",
		"curl --fail --silent --show-error --location --max-time 30 \"${PAGES_URL}\"",
		"pinguin-pages-build.json?source=",
		"verify_live_pages_source_commit",
		"sourceCommit",
		"expected_commit",
	}
	for _, requiredSnippet := range requiredSnippets {
		if !strings.Contains(deployScript, requiredSnippet) {
			t.Fatalf("deploy script missing contract snippet %q", requiredSnippet)
		}
	}
	if strings.Contains(deployScript, "PAGES_PUBLISH_FORCE") {
		t.Fatalf("deploy script must not force empty Pages publish commits")
	}

	readme := string(readRepoFile(t, "README.md"))
	for _, requiredSnippet := range []string{
		"Production deployment is intentionally parameterless",
		"make deploy",
		"defaults to the sibling `mprlab-gateway` checkout",
		"clean local `master` branch that exactly matches `origin/master`",
		"zero open pull requests",
		"After `make deploy`, configure the edge gateway to forward `25 -> tutosh:8025` and `465 -> tutosh:8465`",
		"only for non-production targets",
	} {
		if !strings.Contains(readme, requiredSnippet) {
			t.Fatalf("README missing plain deploy contract snippet %q", requiredSnippet)
		}
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
		repoPath("web", "dashboard.html"),
		repoPath("web", "event-log.html"),
		repoPath("web", "smtp-relay.html"),
	}
	for _, requiredPath := range requiredPaths {
		if _, statErr := os.Stat(requiredPath); statErr != nil {
			t.Fatalf("required Pages source file missing: %s: %v", requiredPath, statErr)
		}
	}

	dashboardRedirect := string(readRepoFile(t, "web", "dashboard.html"))
	for _, requiredSnippet := range []string{
		`content="0; url=/event-log.html"`,
		`window.location.hash === '#smtp-relay' ? '/smtp-relay.html' : '/event-log.html'`,
		`window.location.replace(target.toString())`,
	} {
		if !strings.Contains(dashboardRedirect, requiredSnippet) {
			t.Fatalf("dashboard compatibility redirect missing %q", requiredSnippet)
		}
	}
}
