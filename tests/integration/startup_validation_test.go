package integrationtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerExitsWhenNoEnabledTenants(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	repositoryRoot := filepath.Dir(filepath.Dir(workingDirectory))
	temporaryBinaryDirectory := t.TempDir()
	temporaryBinaryPath := filepath.Join(temporaryBinaryDirectory, "pinguin-server")

	buildCommand := exec.Command("go", "build", "-o", temporaryBinaryPath, "./cmd/server")
	buildCommand.Dir = repositoryRoot
	if output, buildErr := buildCommand.CombinedOutput(); buildErr != nil {
		t.Fatalf("go build failed: %v\n%s", buildErr, string(output))
	}

	configPath := filepath.Join(temporaryBinaryDirectory, "config.yml")
	dbPath := filepath.Join(temporaryBinaryDirectory, "pinguin.db")
	configYAML := `
server:
  databasePath: ` + dbPath + `
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 1
  retryIntervalSec: 1
  masterEncryptionKey: 000102030405060708090a0b0c0d0e0f000102030405060708090a0b0c0d0e0f
  connectionTimeoutSec: 5
  operationTimeoutSec: 5
tenants:
  - id: tenant-disabled
    displayName: Disabled Tenant
    supportEmail: support@example.com
    enabled: false
    domains: [disabled.localhost]
    admins: [admin@example.com]
    identity:
      googleClientId: test-client
      tauthBaseUrl: https://auth.example.com
    emailProfile:
      host: smtp.example.com
      port: 587
      username: user
      password: pass
      fromAddress: noreply@example.com
`
	if writeErr := os.WriteFile(configPath, []byte(strings.TrimSpace(configYAML)+"\n"), 0o600); writeErr != nil {
		t.Fatalf("write config file: %v", writeErr)
	}

	runCommand := exec.Command(temporaryBinaryPath, "--disable-web-interface")
	runCommand.Dir = repositoryRoot
	runCommand.Env = append(os.Environ(), "PINGUIN_CONFIG_PATH="+configPath)
	output, runErr := runCommand.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected server to exit non-zero; output:\n%s", string(output))
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code; output:\n%s", string(output))
	}
	if !strings.Contains(string(output), "no enabled tenants configured") {
		t.Fatalf("expected error about enabled tenants; got:\n%s", string(output))
	}
}
