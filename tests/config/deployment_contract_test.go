package tests

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
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
		Owner   string `yaml:"owner"`
		Release struct {
			Scheme string `yaml:"scheme"`
		} `yaml:"release"`
		Items map[string]map[string]any `yaml:"resources"`
	} `yaml:"mprlab_resources"`
}

func TestRepositoryOwnsCompleteVersionlessDeployment(t *testing.T) {
	manifestData := readRepoFile(t, filepath.FromSlash(deploymentManifestPath))
	var manifestRoot map[string]map[string]any
	if unmarshalErr := yaml.Unmarshal(manifestData, &manifestRoot); unmarshalErr != nil {
		t.Fatalf("decode deployment manifest root: %v", unmarshalErr)
	}
	resourceKeys := make([]string, 0, len(manifestRoot["mprlab_resources"]))
	for resourceKey := range manifestRoot["mprlab_resources"] {
		resourceKeys = append(resourceKeys, resourceKey)
	}
	slices.Sort(resourceKeys)
	if !slices.Equal(resourceKeys, []string{"ci", "defaults", "operations", "owner", "release", "resources"}) {
		t.Fatalf("deployment manifest root is not the exact versionless contract: %#v", resourceKeys)
	}
	manifest := manifestRoot["mprlab_resources"]
	expectedDefaults := map[string]any{"timeouts": map[string]any{
		"startup": "30s", "completion": "30m", "readiness": "30s", "request": "15s", "shutdown": "5s",
	}}
	if !reflect.DeepEqual(manifest["defaults"], expectedDefaults) {
		t.Fatalf("unexpected lifecycle timeout policy: %#v", manifest["defaults"])
	}
	expectedPolling := map[string]any{
		"initial": "1s", "maximum": "5s", "increment": "0s", "multiplier": 2, "attempt_limit": 360,
	}
	expectedOperations := map[string]any{}
	for _, operation := range []string{"app_lifecycle.repository_read", "app-release", "app-publish", "app-deploy"} {
		expectedOperations[operation] = map[string]any{"polling": expectedPolling}
	}
	if !reflect.DeepEqual(manifest["operations"], expectedOperations) {
		t.Fatalf("unexpected lifecycle observation policy: %#v", manifest["operations"])
	}
	var document deploymentDocument
	if unmarshalErr := yaml.Unmarshal(manifestData, &document); unmarshalErr != nil {
		t.Fatalf("decode deployment manifest: %v", unmarshalErr)
	}
	if document.Resources.Owner != "pinguin" || document.Resources.Release.Scheme != "semver" {
		t.Fatalf(
			"unexpected manifest identity: owner=%q scheme=%q",
			document.Resources.Owner,
			document.Resources.Release.Scheme,
		)
	}
	if strings.Contains(string(manifestData), "visibility:") {
		t.Fatal("deployment manifest retains retired package visibility")
	}

	identities := make([]string, 0, len(document.Resources.Items))
	resources := make(map[string]map[string]any, len(document.Resources.Items))
	for resourceID, resource := range document.Resources.Items {
		identity := deploymentString(t, resource, "kind") + "/" + resourceID
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
		"health_check/website-health",
		"private_values/private",
		"runtime_capability/grpc",
		"runtime_capability/grpc-private-lan",
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
		t.Fatal("placement must be declared by each service")
	}
	services := deploymentMap(t, project, "services")
	if len(services) != 1 {
		t.Fatalf("runtime must contain one service: %#v", services)
	}
	service := services["pinguin-api"].(map[string]any)
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

	servicePorts := deploymentMap(t, service, "ports")
	actualContainerPorts := make([]int, 0, len(servicePorts))
	for portKey := range servicePorts {
		containerPort, parseErr := strconv.Atoi(portKey)
		if parseErr != nil {
			t.Fatalf("invalid container port key %q: %v", portKey, parseErr)
		}
		actualContainerPorts = append(actualContainerPorts, containerPort)
	}
	slices.Sort(actualContainerPorts)
	expectedContainerPorts := []int{25, 587, 8080, 50051}
	slices.Sort(expectedContainerPorts)
	if !slices.Equal(actualContainerPorts, expectedContainerPorts) {
		t.Fatalf("unexpected production container ports: %#v", actualContainerPorts)
	}

	grpcPublication := deploymentMap(t, servicePorts, "50051")
	if deploymentInt(t, grpcPublication, "host_port") != 50052 || deploymentString(t, grpcPublication, "scope") != "private_lan" {
		t.Fatalf("unexpected private LAN gRPC publication: %#v", grpcPublication)
	}
	grpcLAN := resources["runtime_capability/grpc-private-lan"]
	if deploymentString(t, grpcLAN, "name") != "pinguin.grpc.private-lan" || deploymentInt(t, grpcLAN, "version") != 1 || deploymentString(t, grpcLAN, "project") != "runtime" || deploymentString(t, grpcLAN, "service") != "pinguin-api" {
		t.Fatalf("unexpected private LAN gRPC capability: %#v", grpcLAN)
	}
	grpcEndpoint := deploymentMap(t, grpcLAN, "endpoint")
	if len(grpcEndpoint) != 3 || deploymentString(t, grpcEndpoint, "scope") != "private_lan" || deploymentString(t, grpcEndpoint, "scheme") != "grpc" || deploymentInt(t, grpcEndpoint, "port") != 50052 {
		t.Fatalf("unexpected private LAN gRPC endpoint: %#v", grpcEndpoint)
	}
	grpcHealth := deploymentMap(t, grpcLAN, "health")
	if len(grpcHealth) != 1 || deploymentString(t, grpcHealth, "protocol") != "tcp" {
		t.Fatalf("unexpected private LAN gRPC health: %#v", grpcHealth)
	}
	originalGRPC := resources["runtime_capability/grpc"]
	expectedOriginalEndpoint := map[string]any{"scope": "same_host", "scheme": "grpc", "alias": "pinguin-grpc", "port": 50051}
	if deploymentString(t, originalGRPC, "name") != "pinguin.grpc" || !reflect.DeepEqual(originalGRPC["endpoint"], expectedOriginalEndpoint) {
		t.Fatalf("unexpected original gRPC capability: %#v", originalGRPC)
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
		".mprlab/release.yml",
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
		"go build -o /workspace/bin/pinguin-convert-managed-tenants ./cmd/convert-managed-tenants",
		"COPY --from=builder /workspace/bin/pinguin /usr/local/bin/pinguin",
		"COPY --from=builder /workspace/bin/pinguin-doctor /usr/local/bin/pinguin-doctor",
		"COPY --from=builder /workspace/bin/pinguin-convert-managed-tenants /usr/local/bin/pinguin-convert-managed-tenants",
		`CMD ["/usr/local/bin/pinguin"]`,
	} {
		if !strings.Contains(dockerfile, requiredSnippet) {
			t.Fatalf("Dockerfile missing production contract snippet %q", requiredSnippet)
		}
	}
	for _, reservedPath := range []string{"web/.nojekyll", "web/CNAME", "web/.mprlab-release.json"} {
		if _, statErr := os.Stat(repoPath(filepath.FromSlash(reservedPath))); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Pages source contains reserved metadata path %s", reservedPath)
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
