package tests

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestServiceEnvExamplesCoverConfigTemplates(t *testing.T) {
	t.Helper()

	requiredByExample := map[string][]string{
		"configs/.env.pinguin.example": {
			"configs/config.pinguin.yml",
		},
		"configs/.env.tauth.example": {
			"configs/config.tauth.yml",
		},
	}

	for examplePath, templatePaths := range requiredByExample {
		exampleKeys := parseEnvExampleKeys(string(readRepoFile(t, examplePath)))
		for _, templatePath := range templatePaths {
			for _, requiredKey := range parseTemplateEnvKeys(string(readRepoFile(t, templatePath))) {
				if !exampleKeys[requiredKey] {
					t.Fatalf("%s does not define %s required by %s", examplePath, requiredKey, templatePath)
				}
			}
		}
	}
}

func TestPinguinSMTPSubmissionTemplateDoesNotUseStaticSenderDomains(t *testing.T) {
	templateKeys := parseTemplateEnvKeys(string(readRepoFile(t, "configs/config.pinguin.yml")))
	exampleKeys := parseEnvExampleKeys(string(readRepoFile(t, "configs/.env.pinguin.example")))

	for _, legacyKey := range []string{
		"SMTP_SUBMISSION_SENDER_DOMAIN_1",
		"SMTP_SUBMISSION_SENDER_DOMAIN_2",
		"SMTP_SUBMISSION_SENDER_DOMAIN_3",
		"SMTP_SUBMISSION_SENDER_DOMAIN_4",
	} {
		if stringSliceContains(templateKeys, legacyKey) {
			t.Fatalf("configs/config.pinguin.yml still references %s", legacyKey)
		}
		if exampleKeys[legacyKey] {
			t.Fatalf("configs/.env.pinguin.example still defines %s", legacyKey)
		}
	}
}

func TestLocalPinguinEnvCoversConfigTemplateWhenPresent(t *testing.T) {
	envPath := "configs/.env.pinguin"
	if _, statErr := os.Stat(repoPath(envPath)); statErr != nil {
		if os.IsNotExist(statErr) {
			t.Skipf("%s is local-only and not present", envPath)
		}
		t.Fatalf("stat %s: %v", envPath, statErr)
	}

	envKeys := parseEnvExampleKeys(string(readRepoFile(t, envPath)))
	for _, requiredKey := range parseTemplateEnvKeys(string(readRepoFile(t, "configs/config.pinguin.yml"))) {
		if !envKeys[requiredKey] {
			t.Fatalf("%s does not define %s required by configs/config.pinguin.yml", envPath, requiredKey)
		}
	}
}

func parseEnvExampleKeys(contents string) map[string]bool {
	keys := make(map[string]bool)
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, found := strings.Cut(trimmed, "=")
		if found {
			keys[strings.TrimSpace(key)] = true
		}
	}
	return keys
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func parseTemplateEnvKeys(contents string) []string {
	matches := regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`).FindAllStringSubmatch(contents, -1)
	keys := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		key := match[1]
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}
