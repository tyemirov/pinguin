package tests

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const deploymentManifestPath = ".mprlab/deploy/resources.yml"

var expectedGatewayWrapper = strings.Join([]string{
	".PHONY: release publish deploy",
	"",
	"release publish deploy:",
	"\t@application_root=\"$$(git rev-parse --show-toplevel)\"; \\",
	"\tgateway_root=\"$$(dirname \"$${application_root}\")/mprlab-gateway\"; \\",
	"\tif [ ! -d \"$${gateway_root}\" ]; then \\",
	"\t\tprintf \"required sibling gateway is missing: %s; clone mprlab-gateway at exactly %s\\n\" \\",
	"\t\t\t\"$${gateway_root}\" \"$${gateway_root}\" >&2; \\",
	"\t\texit 2; \\",
	"\tfi; \\",
	"\t$(MAKE) --no-print-directory -C \"$${gateway_root}\" \"app-$@\" \\",
	"\t\tMPRLAB_APP_ROOT=\"$${application_root}\"",
}, "\n")

type deploymentDocument struct {
	Resources struct {
		SchemaVersion int              `yaml:"schema_version"`
		Owner         string           `yaml:"owner"`
		Items         []map[string]any `yaml:"resources"`
	} `yaml:"mprlab_resources"`
}

func TestRepositoryOwnsCompleteSchemaV3Deployment(t *testing.T) {
	manifestData := readRepoFile(t, filepath.FromSlash(deploymentManifestPath))
	var document deploymentDocument
	if unmarshalErr := yaml.Unmarshal(manifestData, &document); unmarshalErr != nil {
		t.Fatalf("decode deployment manifest: %v", unmarshalErr)
	}
	if document.Resources.SchemaVersion != 3 || document.Resources.Owner != "pinguin" {
		t.Fatalf("unexpected manifest identity: schema=%d owner=%q", document.Resources.SchemaVersion, document.Resources.Owner)
	}

	identities := make([]string, 0, len(document.Resources.Items))
	resources := make(map[string]map[string]any, len(document.Resources.Items))
	for _, resource := range document.Resources.Items {
		identity := deploymentString(t, resource, "kind") + "/" + deploymentString(t, resource, "id")
		identities = append(identities, identity)
		resources[identity] = resource
	}
	slices.Sort(identities)
	expectedIdentities := []string{
		"caddy_listener/public-smtp-forwarding",
		"caddy_listener/public-smtp-submission",
		"caddy_route/public-api",
		"compose_project/runtime",
		"github_pages/website",
		"github_release_binary/server",
		"health_check/public-health",
		"private_values/private",
		"runtime_capability/grpc",
		"runtime_capability/http",
		"runtime_capability/smtp-forwarding",
		"runtime_capability/smtp-submission",
		"tauth_tenant/authentication",
	}
	slices.Sort(expectedIdentities)
	if !slices.Equal(identities, expectedIdentities) {
		t.Fatalf("unexpected deployment resource graph: %#v", identities)
	}

	privateBindings := deploymentMap(t, resources["private_values/private"], "bindings")
	requiredEnvironment := parseTemplateEnvKeys(string(readRepoFile(t, "configs", "config.production.yml")))
	expectedPrivateKeys := append(slices.Clone(requiredEnvironment), "TAUTH_TENANT_GOOGLE_WEB_CLIENT_ID_PINGUIN")
	slices.Sort(expectedPrivateKeys)
	actualPrivateKeys := make([]string, 0, len(privateBindings))
	for _, privateKey := range privateBindings {
		actualPrivateKeys = append(actualPrivateKeys, privateKey.(string))
	}
	slices.Sort(actualPrivateKeys)
	if !slices.Equal(actualPrivateKeys, expectedPrivateKeys) {
		t.Fatalf("private bindings do not exactly cover production inputs: %#v", actualPrivateKeys)
	}

	project := resources["compose_project/runtime"]
	if _, exists := project["placement"]; exists {
		t.Fatal("schema-v3 placement must be declared by each service")
	}
	services := deploymentList(t, project, "services")
	if len(services) != 1 {
		t.Fatalf("runtime must contain one service: %#v", services)
	}
	service := services[0].(map[string]any)
	placement := deploymentMap(t, service, "placement")
	if deploymentString(t, placement, "group") != "gateway" || deploymentString(t, placement, "cardinality") != "one" {
		t.Fatalf("unexpected runtime placement: %#v", placement)
	}
	if _, exists := service["environment_files"]; exists {
		t.Fatal("runtime must use typed environment projections, not environment files")
	}
	environment := deploymentMap(t, service, "environment")
	actualEnvironment := make([]string, 0, len(environment))
	for environmentKey, referenceValue := range environment {
		actualEnvironment = append(actualEnvironment, environmentKey)
		reference := referenceValue.(map[string]any)
		if deploymentString(t, reference, "resource") != "private" {
			t.Fatalf("environment %s does not reference the private resource", environmentKey)
		}
		output := deploymentString(t, reference, "output")
		if privateBindings[output] != environmentKey {
			t.Fatalf("environment %s does not match private output %s", environmentKey, output)
		}
	}
	slices.Sort(actualEnvironment)
	slices.Sort(requiredEnvironment)
	if !slices.Equal(actualEnvironment, requiredEnvironment) {
		t.Fatalf("service environment does not exactly cover production config: %#v", actualEnvironment)
	}

	servicePorts := deploymentList(t, service, "ports")
	actualContainerPorts := make([]int, 0, len(servicePorts))
	for _, servicePort := range servicePorts {
		actualContainerPorts = append(
			actualContainerPorts,
			deploymentInt(t, servicePort.(map[string]any), "container_port"),
		)
	}
	slices.Sort(actualContainerPorts)
	expectedContainerPorts := []int{25, 587, 8080, 50051}
	slices.Sort(expectedContainerPorts)
	if !slices.Equal(actualContainerPorts, expectedContainerPorts) {
		t.Fatalf("unexpected production container ports: %#v", actualContainerPorts)
	}

	privateSMTPSubmission := deploymentMap(
		t,
		resources["runtime_capability/smtp-submission"],
		"endpoint",
	)
	if deploymentInt(t, privateSMTPSubmission, "port") != 587 {
		t.Fatalf("private SMTP submission must use Pinguin listener 587: %#v", privateSMTPSubmission)
	}
	publicSMTPSubmission := resources["caddy_listener/public-smtp-submission"]
	if deploymentInt(t, publicSMTPSubmission, "port") != 465 {
		t.Fatalf("public SMTPS submission must remain on port 465: %#v", publicSMTPSubmission)
	}
}

func TestProductionLifecycleDelegatesOnlyToSiblingGateway(t *testing.T) {
	makefile := string(readRepoFile(t, "Makefile"))
	if !strings.Contains(makefile, expectedGatewayWrapper) {
		t.Fatal("Makefile does not expose the exact sibling-gateway lifecycle wrapper")
	}
	for _, obsoleteTarget := range []string{"\npages-deploy:", "\npublish-release:", "\ncontainer-artifacts:", "\nrelease-artifacts:"} {
		if strings.Contains(makefile, obsoleteTarget) {
			t.Fatalf("Makefile retains obsolete production target %q", obsoleteTarget)
		}
	}

	for _, forbiddenPath := range []string{
		".mprlab/deploy/config.yml",
		".mprlab/deploy/runtime.env.example",
		"scripts/deploy.sh",
		"scripts/release.sh",
		"scripts/publish-release.sh",
		"scripts/release",
		"tests/release_pages_contract_test.py",
	} {
		_, statErr := os.Stat(repoPath(filepath.FromSlash(forbiddenPath)))
		if !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("repository retains obsolete production lifecycle path %s", forbiddenPath)
		}
	}

	deployEntries, readErr := os.ReadDir(repoPath(".mprlab", "deploy"))
	if readErr != nil {
		t.Fatalf("read deployment directory: %v", readErr)
	}
	visibleDeployFiles := make([]string, 0, len(deployEntries))
	for _, entry := range deployEntries {
		if entry.Name() != ".env" {
			visibleDeployFiles = append(visibleDeployFiles, entry.Name())
		}
	}
	if !slices.Equal(visibleDeployFiles, []string{"resources.yml"}) {
		t.Fatalf("deployment directory contains unexpected visible files: %#v", visibleDeployFiles)
	}
	if !strings.Contains(string(readRepoFile(t, ".dockerignore")), ".mprlab/deploy/.env\n") {
		t.Fatal("Docker build context does not exclude the canonical private input")
	}
}

func TestGitHubActionsWorkflowsAreDisabled(t *testing.T) {
	for _, extension := range []string{"*.yml", "*.yaml"} {
		workflowFiles, globErr := filepath.Glob(repoPath(".github", "workflows", extension))
		if globErr != nil {
			t.Fatalf("glob workflow files: %v", globErr)
		}
		if len(workflowFiles) != 0 {
			t.Fatalf("expected no GitHub Actions workflow files, got %v", workflowFiles)
		}
	}
}

func TestProductionImageAndPagesSourcesRemainComplete(t *testing.T) {
	dockerfile := string(readRepoFile(t, "Dockerfile"))
	for _, requiredSnippet := range []string{
		"go build -o /workspace/bin/pinguin ./cmd/server",
		"go build -o /workspace/bin/pinguin-doctor ./cmd/doctor",
		"COPY --from=builder /workspace/bin/pinguin /usr/local/bin/pinguin",
		"COPY --from=builder /workspace/bin/pinguin-doctor /usr/local/bin/pinguin-doctor",
		`CMD ["/usr/local/bin/pinguin"]`,
	} {
		if !strings.Contains(dockerfile, requiredSnippet) {
			t.Fatalf("Dockerfile missing production contract snippet %q", requiredSnippet)
		}
	}
	for _, requiredPath := range []string{"web/.nojekyll"} {
		if _, statErr := os.Stat(repoPath(filepath.FromSlash(requiredPath))); statErr != nil {
			t.Fatalf("Pages source is missing %s: %v", requiredPath, statErr)
		}
	}
}

func deploymentMap(t *testing.T, document map[string]any, key string) map[string]any {
	t.Helper()
	value, available := document[key].(map[string]any)
	if !available {
		t.Fatalf("deployment field %s is not a mapping: %#v", key, document[key])
	}
	return value
}

func deploymentList(t *testing.T, document map[string]any, key string) []any {
	t.Helper()
	value, available := document[key].([]any)
	if !available {
		t.Fatalf("deployment field %s is not a list: %#v", key, document[key])
	}
	return value
}

func deploymentString(t *testing.T, document map[string]any, key string) string {
	t.Helper()
	value, available := document[key].(string)
	if !available || value == "" {
		t.Fatalf("deployment field %s is not a non-empty string: %#v", key, document[key])
	}
	return value
}

func deploymentInt(t *testing.T, document map[string]any, key string) int {
	t.Helper()
	value, available := document[key].(int)
	if !available || value <= 0 {
		t.Fatalf("deployment field %s is not a positive integer: %#v", key, document[key])
	}
	return value
}
