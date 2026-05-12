package tests

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeDocument struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Profiles []string          `yaml:"profiles"`
	Build    *composeBuildSpec `yaml:"build"`
	Image    string            `yaml:"image"`
	Ports    []string          `yaml:"ports"`
	Command  []string          `yaml:"command"`
	EnvFile  []string          `yaml:"env_file"`
}

type composeBuildSpec struct {
	Context string `yaml:"context"`
}

func TestComposeProfilesProvideLocalAndImageVariants(t *testing.T) {
	t.Helper()

	documentData := readRepoFile(t, "docker-compose.yaml")

	var document composeDocument
	if unmarshalErr := yaml.Unmarshal(documentData, &document); unmarshalErr != nil {
		t.Fatalf("failed to parse docker-compose.yaml: %v", unmarshalErr)
	}

	localService, localExists := document.Services["pinguin-dev"]
	if !localExists {
		t.Fatalf("compose file missing pinguin-dev service")
	}

	assertProfileContains(t, localService.Profiles, "dev", "pinguin-dev")
	if localService.Build == nil || localService.Build.Context == "" {
		t.Fatalf("pinguin-dev should define a build context for local development")
	}
	if localService.Image != "" {
		t.Fatalf("pinguin-dev should not specify an image because it builds locally")
	}

	imageService, imageExists := document.Services["pinguin"]
	if !imageExists {
		t.Fatalf("compose file missing pinguin service for docker profile")
	}

	assertProfileContains(t, imageService.Profiles, "docker", "pinguin")
	if imageService.Image == "" || !strings.HasPrefix(imageService.Image, "ghcr.io/") {
		t.Fatalf("pinguin docker profile should pull image from ghcr.io, got %q", imageService.Image)
	}
	if imageService.Build != nil {
		t.Fatalf("pinguin docker profile should not include build configuration")
	}
}

func TestComposeDevPortsUseLocalhost8080AsBrowserOrigin(t *testing.T) {
	t.Helper()

	documentData := readRepoFile(t, "docker-compose.yaml")

	var document composeDocument
	if unmarshalErr := yaml.Unmarshal(documentData, &document); unmarshalErr != nil {
		t.Fatalf("failed to parse docker-compose.yaml: %v", unmarshalErr)
	}

	ghttp := requireComposeService(t, document, "ghttp")
	assertStringContains(t, ghttp.Ports, "8080:8080", "ghttp ports")
	assertStringContains(t, ghttp.Command, "8080", "ghttp command")

	pinguin := requireComposeService(t, document, "pinguin-dev")
	assertStringContains(t, pinguin.Ports, "8081:8081", "pinguin-dev ports")
	assertStringContains(t, pinguin.Ports, "1587:587", "pinguin-dev ports")
	assertStringContains(t, pinguin.Ports, "8025:25", "pinguin-dev ports")
	assertStringContains(t, pinguin.Ports, "8465:465", "pinguin-dev ports")
	assertStringDoesNotContain(t, pinguin.Ports, "1465:465", "pinguin-dev ports")
	assertStringContains(t, pinguin.EnvFile, "./configs/.env.pinguin", "pinguin-dev env files")

	tauth := requireComposeService(t, document, "tauth")
	assertStringContains(t, tauth.Ports, "8082:8082", "tauth ports")
	assertStringContains(t, tauth.EnvFile, "./configs/.env.tauth", "tauth env files")
}

func requireComposeService(t *testing.T, document composeDocument, serviceName string) composeService {
	t.Helper()

	service, exists := document.Services[serviceName]
	if !exists {
		t.Fatalf("compose file missing %s service", serviceName)
	}
	return service
}

func assertProfileContains(t *testing.T, profiles []string, expectedProfile string, serviceName string) {
	t.Helper()

	for _, profile := range profiles {
		if profile == expectedProfile {
			return
		}
	}

	t.Fatalf("%s service is missing %q profile tag", serviceName, expectedProfile)
}

func assertStringContains(t *testing.T, values []string, expected string, label string) {
	t.Helper()

	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("%s missing %q in %v", label, expected, values)
}

func assertStringDoesNotContain(t *testing.T, values []string, unexpected string, label string) {
	t.Helper()

	for _, value := range values {
		if value == unexpected {
			t.Fatalf("%s should not contain %q in %v", label, unexpected, values)
		}
	}
}
