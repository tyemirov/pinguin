package tests

import (
	"testing"

	"gopkg.in/yaml.v3"
)

const currentTAuthSessionPath = "/auth/session"

type mprUIConfigDocument struct {
	Environments []mprUIEnvironment `yaml:"environments"`
}

type mprUIEnvironment struct {
	Description      string               `yaml:"description"`
	Auth             map[string]any       `yaml:"auth"`
	AdditionalFields map[string]yaml.Node `yaml:",inline"`
}

func hasRetiredAuthButton(environment mprUIEnvironment) bool {
	_, authButtonExists := environment.AdditionalFields["authButton"]
	return authButtonExists
}

func TestMPRUIConfigUsesCurrentSessionContract(t *testing.T) {
	documentData := readRepoFile(t, "web", "config-ui.yaml")

	var document mprUIConfigDocument
	if unmarshalErr := yaml.Unmarshal(documentData, &document); unmarshalErr != nil {
		t.Fatalf("failed to parse web/config-ui.yaml: %v", unmarshalErr)
	}
	if len(document.Environments) == 0 {
		t.Fatal("web/config-ui.yaml must declare at least one environment")
	}

	for environmentIndex, environment := range document.Environments {
		environmentLabel := environment.Description
		if environmentLabel == "" {
			environmentLabel = "environment"
		}
		if hasRetiredAuthButton(environment) {
			t.Fatalf("%s at index %d declares retired authButton config", environmentLabel, environmentIndex)
		}
		sessionPath, sessionPathExists := environment.Auth["sessionPath"].(string)
		if !sessionPathExists || sessionPath != currentTAuthSessionPath {
			t.Fatalf(
				"%s at index %d must declare auth.sessionPath %q, got %#v",
				environmentLabel,
				environmentIndex,
				currentTAuthSessionPath,
				environment.Auth["sessionPath"],
			)
		}
	}
}

func TestMPRUIEnvironmentDetectsNullAuthButton(t *testing.T) {
	var environment mprUIEnvironment
	if unmarshalErr := yaml.Unmarshal([]byte("authButton: null\n"), &environment); unmarshalErr != nil {
		t.Fatalf("failed to parse null authButton fixture: %v", unmarshalErr)
	}
	if !hasRetiredAuthButton(environment) {
		t.Fatal("retired authButton key must be detected even when its value is null")
	}
}
