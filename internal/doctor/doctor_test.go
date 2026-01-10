package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunValidatesValidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, validConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.TotalConfigs != 1 {
		t.Fatalf("expected 1 config, got %d", report.Summary.TotalConfigs)
	}
	if report.Summary.ValidConfigs != 1 {
		t.Fatalf("expected 1 valid config, got %d", report.Summary.ValidConfigs)
	}
	if report.Summary.InvalidConfigs != 0 {
		t.Fatalf("expected 0 invalid configs, got %d", report.Summary.InvalidConfigs)
	}
}

func TestRunValidatesInvalidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, invalidConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
	})
	if err != nil {
		t.Fatalf("expected no error for run, got %v", err)
	}
	if report.Summary.ValidConfigs != 0 {
		t.Fatalf("expected 0 valid configs, got %d", report.Summary.ValidConfigs)
	}
	if report.Summary.InvalidConfigs != 1 {
		t.Fatalf("expected 1 invalid config, got %d", report.Summary.InvalidConfigs)
	}
	if len(report.Diagnostics[0].Errors) == 0 {
		t.Fatalf("expected errors in diagnostic")
	}
}

func TestRunValidatesMissingFile(t *testing.T) {
	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{"/nonexistent/config.yml"},
	})
	if err != nil {
		t.Fatalf("expected no error for run, got %v", err)
	}
	if report.Summary.InvalidConfigs != 1 {
		t.Fatalf("expected 1 invalid config, got %d", report.Summary.InvalidConfigs)
	}
}

func TestRunValidatesMultipleConfigs(t *testing.T) {
	tempDir := t.TempDir()
	config1Path := filepath.Join(tempDir, "config1.yml")
	config2Path := filepath.Join(tempDir, "config2.yml")
	writeTestConfig(t, config1Path, validConfigYAML)
	writeTestConfig(t, config2Path, validConfig2YAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths:          []string{config1Path, config2Path},
		ValidateCrossConfigs: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.TotalConfigs != 2 {
		t.Fatalf("expected 2 configs, got %d", report.Summary.TotalConfigs)
	}
	if report.Summary.ValidConfigs != 2 {
		t.Fatalf("expected 2 valid configs, got %d", report.Summary.ValidConfigs)
	}
	if !report.CrossValidation.Performed {
		t.Fatalf("expected cross validation to be performed")
	}
}

func TestRunDetectsCrossConfigDomainConflict(t *testing.T) {
	tempDir := t.TempDir()
	config1Path := filepath.Join(tempDir, "config1.yml")
	config2Path := filepath.Join(tempDir, "config2.yml")
	writeTestConfig(t, config1Path, validConfigYAML)
	writeTestConfig(t, config2Path, conflictingDomainConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths:          []string{config1Path, config2Path},
		ValidateCrossConfigs: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(report.CrossValidation.Errors) == 0 {
		t.Fatalf("expected cross-config errors for conflicting domains")
	}
}

func TestRunReturnsErrorWithNoConfigs(t *testing.T) {
	_, err := Run(context.Background(), Options{
		ConfigPaths: []string{},
	})
	if err == nil {
		t.Fatalf("expected error for no config paths")
	}
}

func TestRunExpandsEnvWhenEnabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, configWithEnvVarsYAML)

	t.Setenv("TEST_DB_PATH", "/data/pinguin.db")
	t.Setenv("TEST_GRPC_TOKEN", "test-token-123")
	t.Setenv("TEST_LOG_LEVEL", "INFO")
	t.Setenv("TEST_ENCRYPTION_KEY", "test-encryption-key-at-least-32")
	t.Setenv("TEST_TENANT_DOMAIN", "test.example.com")
	t.Setenv("TEST_GOOGLE_CLIENT_ID", "test-client.apps.googleusercontent.com")
	t.Setenv("TEST_TAUTH_URL", "https://tauth.example.com")
	t.Setenv("TEST_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("TEST_LISTEN_ADDR", ":8080")
	t.Setenv("TEST_SIGNING_KEY", "test-signing-key")
	t.Setenv("TEST_ISSUER", "test-issuer")

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
		ExpandEnv:   true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.ValidConfigs != 1 {
		t.Fatalf("expected 1 valid config, got %d; errors: %v", report.Summary.ValidConfigs, report.Diagnostics[0].Errors)
	}
}

func TestFormatReportProducesValidJSON(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, validConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	jsonOutput, formatErr := FormatReport(report)
	if formatErr != nil {
		t.Fatalf("expected no format error, got %v", formatErr)
	}
	if len(jsonOutput) == 0 {
		t.Fatalf("expected non-empty JSON output")
	}
}

func TestFormatSummaryProducesReadableOutput(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, validConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	summary := FormatSummary(report)
	if summary == "" {
		t.Fatalf("expected non-empty summary")
	}
	if !strings.Contains(summary, "Pinguin Doctor Report") {
		t.Fatalf("expected summary to contain header")
	}
	if !strings.Contains(summary, "VALID") {
		t.Fatalf("expected summary to contain VALID status")
	}
}

func writeTestConfig(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

const validConfigYAML = `
server:
  databasePath: /data/pinguin.db
  grpcAuthToken: test-token-123
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 60
  masterEncryptionKey: test-encryption-key-at-least-32-chars
  connectionTimeoutSec: 30
  operationTimeoutSec: 60

web:
  enabled: true
  listenAddr: ":8080"
  allowedOrigins:
    - http://localhost:3000
  tauth:
    signingKey: test-signing-key
    issuer: test-issuer
    cookieName: app_session

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
    identity:
      googleClientId: demo-client.apps.googleusercontent.com
      tauthBaseUrl: https://tauth.example.com
    admins:
      - admin@example.com
`

const validConfig2YAML = `
server:
  databasePath: /data/pinguin2.db
  grpcAuthToken: test-token-456
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 60
  masterEncryptionKey: test-encryption-key-at-least-32-chars
  connectionTimeoutSec: 30
  operationTimeoutSec: 60

web:
  enabled: true
  listenAddr: ":8081"
  allowedOrigins:
    - http://localhost:3001
  tauth:
    signingKey: test-signing-key-2
    issuer: test-issuer-2
    cookieName: app_session

tenants:
  - id: other
    displayName: Other Tenant
    domains:
      - other.example.com
    identity:
      googleClientId: other-client.apps.googleusercontent.com
      tauthBaseUrl: https://tauth.example.com
    admins:
      - admin@example.com
`

const conflictingDomainConfigYAML = `
server:
  databasePath: /data/pinguin2.db
  grpcAuthToken: test-token-456
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 60
  masterEncryptionKey: test-encryption-key-at-least-32-chars
  connectionTimeoutSec: 30
  operationTimeoutSec: 60

web:
  enabled: true
  listenAddr: ":8081"
  allowedOrigins:
    - http://localhost:3001
  tauth:
    signingKey: test-signing-key-2
    issuer: test-issuer-2
    cookieName: app_session

tenants:
  - id: conflicting
    displayName: Conflicting Tenant
    domains:
      - demo.example.com
    identity:
      googleClientId: other-client.apps.googleusercontent.com
      tauthBaseUrl: https://tauth.example.com
    admins:
      - admin@example.com
`

const invalidConfigYAML = `
server:
  databasePath: ""
  grpcAuthToken: ""
  logLevel: ""
  maxRetries: 0
  retryIntervalSec: 0
  masterEncryptionKey: ""
  connectionTimeoutSec: 0
  operationTimeoutSec: 0
`

const configWithEnvVarsYAML = `
server:
  databasePath: ${TEST_DB_PATH}
  grpcAuthToken: ${TEST_GRPC_TOKEN}
  logLevel: ${TEST_LOG_LEVEL}
  maxRetries: 3
  retryIntervalSec: 60
  masterEncryptionKey: ${TEST_ENCRYPTION_KEY}
  connectionTimeoutSec: 30
  operationTimeoutSec: 60

web:
  enabled: true
  listenAddr: ${TEST_LISTEN_ADDR}
  allowedOrigins:
    - http://localhost:3000
  tauth:
    signingKey: ${TEST_SIGNING_KEY}
    issuer: ${TEST_ISSUER}
    cookieName: app_session

tenants:
  - id: test
    displayName: Test Tenant
    domains:
      - ${TEST_TENANT_DOMAIN}
    identity:
      googleClientId: ${TEST_GOOGLE_CLIENT_ID}
      tauthBaseUrl: ${TEST_TAUTH_URL}
    admins:
      - ${TEST_ADMIN_EMAIL}
`
