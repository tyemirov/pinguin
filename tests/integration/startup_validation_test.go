package integrationtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerExitsWhenNoEnabledTenants(t *testing.T) {
	temporaryBinaryDirectory := t.TempDir()
	temporaryBinaryPath := buildServerBinary(t, temporaryBinaryDirectory)

	configDirectory := filepath.Join(temporaryBinaryDirectory, "configs")
	if mkdirErr := os.Mkdir(configDirectory, 0o755); mkdirErr != nil {
		t.Fatalf("mkdir config directory: %v", mkdirErr)
	}
	configPath := filepath.Join(configDirectory, "config.yml")
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
    emailProfile:
      host: smtp.example.com
      port: 587
      username: user
      password: pass
      fromAddress: noreply@example.com
web:
  enabled: false
`
	if writeErr := os.WriteFile(configPath, []byte(strings.TrimSpace(configYAML)+"\n"), 0o600); writeErr != nil {
		t.Fatalf("write config file: %v", writeErr)
	}

	runCommand := exec.Command(temporaryBinaryPath)
	runCommand.Dir = temporaryBinaryDirectory
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

func TestServerExitsWhenConfigEnvironmentVariableIsMissing(t *testing.T) {
	temporaryBinaryDirectory := t.TempDir()
	temporaryBinaryPath := buildServerBinary(t, temporaryBinaryDirectory)

	configDirectory := filepath.Join(temporaryBinaryDirectory, "configs")
	if mkdirErr := os.Mkdir(configDirectory, 0o755); mkdirErr != nil {
		t.Fatalf("mkdir config directory: %v", mkdirErr)
	}
	configPath := filepath.Join(configDirectory, "config.yml")
	configYAML := `
server:
  databasePath: app.db
  grpcAuthToken: ${MISSING_GRPC_AUTH_TOKEN}
  logLevel: INFO
  maxRetries: 1
  retryIntervalSec: 1
  masterEncryptionKey: 000102030405060708090a0b0c0d0e0f000102030405060708090a0b0c0d0e0f
  connectionTimeoutSec: 5
  operationTimeoutSec: 5
tenants:
  configPath: tenants.yml
web:
  enabled: false
`
	if writeErr := os.WriteFile(configPath, []byte(strings.TrimSpace(configYAML)+"\n"), 0o600); writeErr != nil {
		t.Fatalf("write config file: %v", writeErr)
	}

	runCommand := exec.Command(temporaryBinaryPath)
	runCommand.Dir = temporaryBinaryDirectory
	runCommand.Env = minimalRuntimeEnv()
	output, runErr := runCommand.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected server to exit non-zero; output:\n%s", string(output))
	}
	if !strings.Contains(string(output), "configuration: missing environment variables: MISSING_GRPC_AUTH_TOKEN") {
		t.Fatalf("expected missing environment variable error; got:\n%s", string(output))
	}
}

func buildServerBinary(t *testing.T, outputDirectory string) string {
	t.Helper()

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	repositoryRoot := filepath.Dir(filepath.Dir(workingDirectory))
	temporaryBinaryPath := filepath.Join(outputDirectory, "pinguin-server")

	buildCommand := exec.Command("go", "build", "-o", temporaryBinaryPath, "./cmd/server")
	buildCommand.Dir = repositoryRoot
	if output, buildErr := buildCommand.CombinedOutput(); buildErr != nil {
		t.Fatalf("go build failed: %v\n%s", buildErr, string(output))
	}
	return temporaryBinaryPath
}

func minimalRuntimeEnv() []string {
	var environment []string
	for _, key := range []string{"HOME", "PATH", "TMPDIR"} {
		if value, found := os.LookupEnv(key); found {
			environment = append(environment, key+"="+value)
		}
	}
	return environment
}
