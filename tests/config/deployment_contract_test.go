package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const deploymentManifestPath = ".mprlab/deploy/resources.yml"

func TestRepositoryOwnsCompleteVersionlessDeployment(t *testing.T) {
	manifestData := readRepoFile(t, filepath.FromSlash(deploymentManifestPath))
	manifest := currentManifestContract(t, manifestData)
	requireCurrentContractYAML(t, "owner", manifest["owner"], `pingwin`)
	requireCurrentContractYAML(t, "release", manifest["release"], `scheme: semver`)
	lifecycle := currentContractMap(t, manifest["lifecycle"])
	requireCurrentContractYAML(t, "lifecycle.defaults", lifecycle["defaults"], `timeouts: {startup: 30s, completion: 30m, readiness: 30s, request: 15s, shutdown: 5s}`)
	requireCurrentContractYAML(t, "lifecycle.operations", lifecycle["operations"], `
- {name: app_lifecycle.repository_read, polling: {initial: 1s, maximum: 5s, increment: 0s, multiplier: 2, attempt_limit: 360}}
- {name: app-release, polling: {initial: 1s, maximum: 5s, increment: 0s, multiplier: 2, attempt_limit: 360}}
- {name: app-publish, polling: {initial: 1s, maximum: 5s, increment: 0s, multiplier: 2, attempt_limit: 360}}
- {name: app-deploy, polling: {initial: 1s, maximum: 5s, increment: 0s, multiplier: 2, attempt_limit: 360}}
`)
	if strings.Contains(string(manifestData), "visibility:") {
		t.Fatal("deployment retains retired package visibility")
	}
	environment := currentContractMap(t, manifest["environment"])
	requireCurrentContractYAML(t, "environment.source", environment["source"], `.mprlab/deploy/.env`)
	declarations := currentContractIndex(t, environment["private_inputs"], "id")
	requiredEnvironment := parseTemplateEnvKeys(string(readRepoFile(t, "configs", "config.production.yml")))
	expectedPrivateKeys := append(slices.Clone(requiredEnvironment), "TAUTH_TENANT_GOOGLE_WEB_CLIENT_ID_PINGWIN")
	inputNames := append(slices.Clone(expectedPrivateKeys), "DEPLOY_SUDO_PASSWORD_TUTOSH", "MPRLAB_TAUTH_MANAGEMENT_URL", "MPRLAB_TAUTH_PROVISIONING_CREDENTIALS")
	slices.Sort(inputNames)
	actualInputNames := make([]string, 0, len(declarations))
	for _, declaration := range declarations {
		actualInputNames = append(actualInputNames, deploymentString(t, declaration, "variable"))
	}
	slices.Sort(actualInputNames)
	if !slices.Equal(actualInputNames, inputNames) {
		t.Fatalf("private inputs do not exactly cover config and host consumers: %#v", actualInputNames)
	}
	targets := currentContractMap(t, manifest["targets"])
	requireCurrentContractYAML(t, "targets.access", targets["access"], `hosts: [{host: tutosh, elevation_password: {input: deploy-sudo-password-tutosh}}]`)
	planFixture := string(readRepoFile(t, "tests", "config", "installed-gateway-plan.sh"))
	fixtureNames := make([]string, 0, len(inputNames))
	for _, match := range regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)=`).FindAllStringSubmatch(planFixture, -1) {
		fixtureNames = append(fixtureNames, match[1])
	}
	slices.Sort(fixtureNames)
	if !slices.Equal(fixtureNames, inputNames) {
		t.Fatalf("installed plan fixture lacks declared synthetic inputs: %#v", fixtureNames)
	}
	artifacts := currentContractMap(t, manifest["artifacts"])
	for catalog, identity := range map[string]string{"binaries": "server", "container_images": "pingwin-image", "web_bundles": "website"} {
		entries := currentContractIndex(t, artifacts[catalog], "id")
		if len(entries) != 1 || entries[identity] == nil {
			t.Fatalf("unexpected %s artifacts: %#v", catalog, entries)
		}
	}
	publications := currentContractMap(t, manifest["publications"])
	for provider, catalog := range map[string]string{"ghcr": "images", "github_pages": "sites", "github_releases": "binaries"} {
		if len(currentContractIndex(t, currentContractMap(t, publications[provider])[catalog], "id")) != 1 {
			t.Fatalf("unexpected %s publications", provider)
		}
	}
	projects := currentContractIndex(t, currentContractMap(t, targets["docker_compose"])["projects"], "id")
	if len(projects) != 1 {
		t.Fatalf("unexpected Compose projects: %#v", projects)
	}
	project := projects["runtime"]
	requireCurrentContractYAML(t, "runtime.volumes", project["volumes"], `[{id: data, name: mprlab-nginx-gateway_pinguin-data, retention: retain}]`)
	requireCurrentContractYAML(t, "runtime.retired_services", project["retired_services"], `[{project: mprlab-nginx-gateway, service: pinguin-api}]`)
	if _, exists := project["placement"]; exists {
		t.Fatal("placement must be declared by each service")
	}
	services := currentContractIndex(t, project["services"], "id")
	if len(services) != 1 || services["pingwin-api"] == nil {
		t.Fatalf("unexpected services: %#v", services)
	}
	service := services["pingwin-api"]
	requireCurrentContractYAML(t, "pingwin-api.placement", service["placement"], `group: gateway
cardinality: one`)
	if _, exists := service["environment_files"]; exists {
		t.Fatal("runtime must use typed environment projections")
	}
	runtimeEnvironment := currentContractIndex(t, service["environment"], "name")
	actualEnvironment := make([]string, 0, len(runtimeEnvironment))
	for name, reference := range runtimeEnvironment {
		actualEnvironment = append(actualEnvironment, name)
		input := deploymentString(t, reference, "input")
		if declarations[input] == nil || declarations[input]["variable"] != name {
			t.Fatalf("environment %s does not match its native private input %s", name, input)
		}
	}
	slices.Sort(actualEnvironment)
	slices.Sort(requiredEnvironment)
	if !slices.Equal(actualEnvironment, requiredEnvironment) {
		t.Fatalf("service environment does not exactly cover production config: %#v", actualEnvironment)
	}
	ports, ok := service["ports"].([]any)
	if !ok {
		t.Fatal("service ports require an explicit list")
	}
	actualContainerPorts := make([]int, 0, len(ports))
	for _, value := range ports {
		port := currentContractMap(t, value)
		containerPort := deploymentInt(t, port, "container_port")
		actualContainerPorts = append(actualContainerPorts, containerPort)
		if containerPort == 50051 && (deploymentInt(t, port, "host_port") != 50052 || deploymentString(t, port, "scope") != "private_lan") {
			t.Fatalf("unexpected private LAN gRPC port: %#v", port)
		}
	}
	slices.Sort(actualContainerPorts)
	if !slices.Equal(actualContainerPorts, []int{25, 587, 8080, 50051}) {
		t.Fatalf("unexpected container ports: %#v", actualContainerPorts)
	}
	endpoints := currentContractIndex(t, service["endpoints"], "id")
	if len(endpoints) != 6 {
		t.Fatalf("unexpected service endpoints: %#v", endpoints)
	}
	requireCurrentContractYAML(t, "grpc-private-lan", endpoints["grpc-private-lan"], `{id: grpc-private-lan, name: pingwin.grpc.private-lan, version: 1, endpoint: {scope: private_lan, scheme: grpc, port: 50052}, health: {protocol: tcp}}`)
	requireCurrentContractYAML(t, "grpc.endpoint", endpoints["grpc"]["endpoint"], `{scope: same_host, scheme: grpc, alias: pingwin-grpc, port: 50051}`)
	requireCurrentContractYAML(t, "grpc.name", endpoints["grpc"]["name"], `pingwin.grpc`)
	requireCurrentContractYAML(t, "smtp-submission.port", currentContractMap(t, endpoints["smtp-submission"]["endpoint"])["port"], `587`)
	caddy := currentContractMap(t, currentContractMap(t, targets["ingress"])["caddy"])
	listeners := currentContractIndex(t, caddy["listeners"], "id")
	if len(listeners) != 2 || listeners["public-smtp-forwarding"] == nil {
		t.Fatalf("unexpected public listeners: %#v", listeners)
	}
	requireCurrentContractYAML(t, "public-smtp-submission.port", listeners["public-smtp-submission"]["port"], `465`)
	routes := currentContractIndex(t, caddy["routes"], "id")
	if len(routes) != 1 || routes["public-api"] == nil {
		t.Fatalf("unexpected API routes: %#v", routes)
	}
	observations := currentContractIndex(t, currentContractMap(t, lifecycle["observations"])["http"], "id")
	if len(observations) != 2 || observations["public-health"] == nil || observations["website-health"] == nil {
		t.Fatalf("unexpected health observations: %#v", observations)
	}
	if len(currentContractIndex(t, currentContractMap(t, targets["github_pages"])["sites"], "id")) != 1 {
		t.Fatal("unexpected Pages target count")
	}
	if currentContractDeclaration(t, currentContractMap(t, targets["github_pages"])["sites"], "website")["publication"] != "website" {
		t.Fatal("website target must use its declared publication")
	}
}

func TestProductionLifecycleUsesInstalledGateway(t *testing.T) {
	root, rootErr := filepath.Abs(repoPath())
	if rootErr != nil {
		t.Fatalf("resolve application root: %v", rootErr)
	}
	command := exec.Command("bash", filepath.Join(root, "tests", "installed-gateway.sh"))
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("installed Gateway Make integration: %v\n%s", err, output)
	}
	makefile := string(readRepoFile(t, "Makefile"))
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
		"go build -o /workspace/bin/pingwin ./cmd/server",
		"go build -o /workspace/bin/pingwin-doctor ./cmd/doctor",
		"go build -o /workspace/bin/pingwin-convert-managed-tenants ./cmd/convert-managed-tenants",
		"COPY --from=builder /workspace/bin/pingwin /usr/local/bin/pingwin",
		"COPY --from=builder /workspace/bin/pingwin-doctor /usr/local/bin/pingwin-doctor",
		"COPY --from=builder /workspace/bin/pingwin-convert-managed-tenants /usr/local/bin/pingwin-convert-managed-tenants",
		`CMD ["/usr/local/bin/pingwin"]`,
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

func currentContractMap(t *testing.T, value any) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected native declaration mapping, got %#v", value)
	}
	return result
}

func currentManifestContract(t *testing.T, content []byte) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := yaml.Unmarshal(content, &envelope); err != nil {
		t.Fatalf("parse selected manifest: %v", err)
	}
	if len(envelope) != 1 {
		t.Fatalf("unexpected manifest envelope: %#v", envelope)
	}
	actual := currentContractMap(t, envelope["mprlab_resources"])
	keys := make([]string, 0, len(actual))
	for key := range actual {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	expected := []string{"artifacts", "environment", "lifecycle", "owner", "publications", "release", "targets"}
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("selected manifest root keys=%v want=%v", keys, expected)
	}
	return actual
}

func currentContractDeclaration(t *testing.T, value any, identity string) map[string]any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected explicit declaration list, got %#v", value)
	}
	var found map[string]any
	for _, item := range items {
		declaration := currentContractMap(t, item)
		if declaration["id"] == identity {
			if found != nil {
				t.Fatalf("duplicate declaration %s", identity)
			}
			found = declaration
		}
	}
	if found == nil {
		t.Fatalf("missing native declaration %s", identity)
	}
	return found
}

func currentContractIndex(t *testing.T, value any, identityKey string) map[string]map[string]any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected native declaration list, got %#v", value)
	}
	result := make(map[string]map[string]any, len(items))
	for _, item := range items {
		declaration := currentContractMap(t, item)
		identity, ok := declaration[identityKey].(string)
		if !ok || identity == "" || result[identity] != nil {
			t.Fatalf("invalid or duplicate %s in %#v", identityKey, value)
		}
		result[identity] = declaration
	}
	return result
}

func requireCurrentContractYAML(t *testing.T, path string, actual any, expectedText string) {
	t.Helper()
	var expected any
	if err := yaml.Unmarshal([]byte(expectedText), &expected); err != nil {
		t.Fatalf("parse expected %s: %v", path, err)
	}
	requireCurrentContractValue(t, path, actual, expected)
}

func requireCurrentContractValue(t *testing.T, path string, actual, expected any) {
	t.Helper()
	switch wanted := expected.(type) {
	case map[string]any:
		mapping, ok := actual.(map[string]any)
		if !ok {
			t.Fatalf("%s requires a mapping, got %#v", path, actual)
		}
		if !strings.HasSuffix(path, ".assets[]") && len(mapping) != len(wanted) {
			t.Fatalf("%s has unexpected fields: %#v", path, actual)
		}
		for key, value := range wanted {
			observed, exists := mapping[key]
			if !exists {
				t.Fatalf("%s lacks %s", path, key)
			}
			requireCurrentContractValue(t, path+"."+key, observed, value)
		}
	case []any:
		items, ok := actual.([]any)
		if !ok || len(items) != len(wanted) {
			t.Fatalf("%s list=%#v want=%#v", path, actual, wanted)
		}
		for index, value := range wanted {
			observed := items[index]
			if declaration, isMapping := value.(map[string]any); isMapping {
				identityKey := "id"
				if _, exists := declaration[identityKey]; !exists {
					identityKey = "name"
				}
				if identity, exists := declaration[identityKey]; exists {
					matches := 0
					for _, candidate := range items {
						if entry, valid := candidate.(map[string]any); valid && entry[identityKey] == identity {
							observed = candidate
							matches++
						}
					}
					if matches != 1 {
						t.Fatalf("%s requires one %s=%v, got %d", path, identityKey, identity, matches)
					}
				}
			}
			requireCurrentContractValue(t, path+"[]", observed, value)
		}
	default:
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%s=%#v want=%#v", path, actual, expected)
		}
	}
}
