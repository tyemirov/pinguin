package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestRunValidatesCurrentServerTAuthConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, currentServerTAuthConfigYAML)
	t.Setenv("TEST_SIGNING_KEY", "test-signing-key")

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
		ExpandEnv:   true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.ValidConfigs != 1 {
		t.Fatalf("expected current server.tauth config to be valid, got errors: %v", report.Diagnostics[0].Errors)
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

func TestRunValidatesSMTPSubmissionConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, invalidSMTPSubmissionConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.ValidConfigs != 0 {
		t.Fatalf("expected invalid SMTP submission config")
	}
	if !containsDiagnosticError(report.Diagnostics[0].Errors, "smtpSubmission.listenAddr") {
		t.Fatalf("expected SMTP submission listener error, got %v", report.Diagnostics[0].Errors)
	}
}

func TestRunAllowsDirectSMTPSubmissionConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, directSMTPSubmissionConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.ValidConfigs != 1 {
		t.Fatalf("expected direct SMTP submission config to be valid, got %+v", report.Diagnostics)
	}
}

func TestRunValidatesSMTPForwardingConfig(t *testing.T) {
	tempDir := t.TempDir()
	validConfigPath := filepath.Join(tempDir, "valid-forwarding.yml")
	writeTestConfig(t, validConfigPath, validSMTPForwardingConfigYAML)

	validReport, validErr := Run(context.Background(), Options{
		ConfigPaths: []string{validConfigPath},
	})
	if validErr != nil {
		t.Fatalf("expected no valid run error, got %v", validErr)
	}
	if validReport.Summary.ValidConfigs != 1 {
		t.Fatalf("expected valid forwarding config, got %+v", validReport.Diagnostics)
	}

	invalidConfigPath := filepath.Join(tempDir, "invalid-forwarding.yml")
	writeTestConfig(t, invalidConfigPath, invalidSMTPForwardingConfigYAML)
	invalidReport, invalidErr := Run(context.Background(), Options{
		ConfigPaths: []string{invalidConfigPath},
	})
	if invalidErr != nil {
		t.Fatalf("expected no invalid run error, got %v", invalidErr)
	}
	if invalidReport.Summary.ValidConfigs != 0 {
		t.Fatalf("expected invalid forwarding config")
	}
	for _, expected := range []string{
		"smtpForwarding.listenAddr",
		"smtpForwarding.relay.password",
	} {
		if !containsDiagnosticError(invalidReport.Diagnostics[0].Errors, expected) {
			t.Fatalf("expected %s diagnostic, got %v", expected, invalidReport.Diagnostics[0].Errors)
		}
	}

	dynamicForwardingConfigPath := filepath.Join(tempDir, "dynamic-forwarding.yml")
	writeTestConfig(t, dynamicForwardingConfigPath, dynamicForwardingConfigYAML)
	dynamicForwardingReport, dynamicForwardingErr := Run(context.Background(), Options{
		ConfigPaths: []string{dynamicForwardingConfigPath},
	})
	if dynamicForwardingErr != nil {
		t.Fatalf("expected no dynamic forwarding run error, got %v", dynamicForwardingErr)
	}
	if dynamicForwardingReport.Summary.ValidConfigs != 1 {
		t.Fatalf("expected forwarding config to be valid, got %+v", dynamicForwardingReport.Summary)
	}

	unknownSMTPSubmissionConfigPath := filepath.Join(tempDir, "unknown-smtp-submission-field.yml")
	writeTestConfig(t, unknownSMTPSubmissionConfigPath, unknownSMTPSubmissionConfigYAML)
	unknownSMTPSubmissionReport, unknownSMTPSubmissionErr := Run(context.Background(), Options{
		ConfigPaths: []string{unknownSMTPSubmissionConfigPath},
	})
	if unknownSMTPSubmissionErr != nil {
		t.Fatalf("expected no unknown smtp submission run error, got %v", unknownSMTPSubmissionErr)
	}
	if unknownSMTPSubmissionReport.Summary.ValidConfigs != 0 {
		t.Fatalf("expected unknown smtp submission config to be invalid")
	}
	if !containsDiagnosticError(unknownSMTPSubmissionReport.Diagnostics[0].Errors, "field unsupportedOption not found") {
		t.Fatalf("expected unknown smtp submission diagnostic, got %v", unknownSMTPSubmissionReport.Diagnostics[0].Errors)
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
	t.Setenv("TEST_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("TEST_LISTEN_ADDR", ":8080")
	t.Setenv("TEST_SIGNING_KEY", "test-signing-key")

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

func TestRunReportsMissingEnvWhenExpansionEnabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, `
server:
  databasePath: ${TEST_DB_PATH}
  grpcAuthToken: test-token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: test-encryption-key-at-least-32
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  configPath: tenants.yml
web:
  enabled: false
`)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
		ExpandEnv:   true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.ValidConfigs != 0 {
		t.Fatalf("expected invalid config for missing env, got %d valid configs", report.Summary.ValidConfigs)
	}
	if !containsDiagnosticError(report.Diagnostics[0].Errors, "missing environment variables: TEST_DB_PATH") {
		t.Fatalf("expected missing env diagnostic, got %v", report.Diagnostics[0].Errors)
	}
}

func TestRunRejectsMappingTenantItems(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")
	writeTestConfig(t, configPath, mappingItemsSMTPSubmissionConfigYAML)

	report, err := Run(context.Background(), Options{
		ConfigPaths: []string{configPath},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Summary.ValidConfigs != 0 {
		t.Fatalf("expected tenants.items config to be invalid")
	}
	if !containsDiagnosticError(report.Diagnostics[0].Errors, "tenants.items is not supported") {
		t.Fatalf("expected tenants.items diagnostic, got %v", report.Diagnostics[0].Errors)
	}
}

func TestRunReportsParseYAMLError(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yml")
	writeTestConfig(t, configPath, "server:\n  databasePath: [unterminated\n")

	report, err := Run(context.Background(), Options{ConfigPaths: []string{configPath}})
	if err != nil {
		t.Fatalf("expected no run error, got %v", err)
	}
	if report.Summary.InvalidConfigs != 1 || !containsDiagnosticError(report.Diagnostics[0].Errors, "parse_yaml") {
		t.Fatalf("expected parse yaml diagnostic, got %+v", report.Diagnostics[0])
	}
}

func TestFormatSummaryIncludesInvalidAndCleanCrossValidation(t *testing.T) {
	report := &Report{
		Timestamp: timeNowForDoctorTest,
		Summary: reportSummary{
			TotalConfigs:   1,
			InvalidConfigs: 1,
			TotalErrors:    1,
			TotalWarnings:  1,
		},
		Diagnostics: []DiagnosticResult{
			{
				ConfigPath: "bad.yml",
				Valid:      false,
				Errors:     []string{"missing server.databasePath"},
				Warnings:   []string{"server.masterEncryptionKey should be at least 32 characters"},
			},
		},
		CrossValidation: crossValidation{Performed: true},
	}
	summary := FormatSummary(report)
	for _, expected := range []string{"INVALID", "ERROR: missing server.databasePath", "WARN: server.masterEncryptionKey", "No cross-config issues detected"} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected summary to contain %q, got %q", expected, summary)
		}
	}
}

func TestPinguinYAMLNodeDefaultKind(t *testing.T) {
	var node pinguinYAMLNode
	if err := node.UnmarshalYAML(&yaml.Node{Kind: yaml.ScalarNode, Value: "ignored"}); err == nil || !strings.Contains(err.Error(), "tenants must be a list") {
		t.Fatalf("expected invalid tenant shape error, got %v", err)
	}
	if tenants := node.AllTenants(); tenants != nil {
		t.Fatalf("expected no tenants, got %+v", tenants)
	}
}

func TestPinguinYAMLNodeAcceptsCurrentMappingShape(t *testing.T) {
	var node pinguinYAMLNode
	if err := yaml.Unmarshal([]byte(`
configPath: " tenants.yml "
tenants:
  - id: mapped
    displayName: Mapped Tenant
    domains:
      - mapped.example.com
`), &node); err != nil {
		t.Fatalf("unmarshal current mapping shape: %v", err)
	}
	if node.ConfigPath != "tenants.yml" {
		t.Fatalf("expected trimmed config path, got %q", node.ConfigPath)
	}
	if len(node.AllTenants()) != 1 || node.AllTenants()[0].ID != "mapped" {
		t.Fatalf("expected mapped tenant, got %+v", node.AllTenants())
	}
}

func TestPinguinYAMLNodeDecodeErrors(t *testing.T) {
	sequenceNode := &yaml.Node{
		Kind: yaml.SequenceNode,
		Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "domains"},
				{Kind: yaml.MappingNode},
			}},
		},
	}
	var sequence pinguinYAMLNode
	if err := sequence.UnmarshalYAML(sequenceNode); err == nil {
		t.Fatalf("expected sequence decode error")
	}

	mappingNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "items"},
			{Kind: yaml.MappingNode},
		},
	}
	var mapping pinguinYAMLNode
	if err := mapping.UnmarshalYAML(mappingNode); err == nil {
		t.Fatalf("expected mapping decode error")
	}

	invalidCurrentMappingNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "tenants"},
			{Kind: yaml.MappingNode},
		},
	}
	var currentMapping pinguinYAMLNode
	if err := currentMapping.UnmarshalYAML(invalidCurrentMappingNode); err == nil {
		t.Fatalf("expected current mapping decode error")
	}
}

func TestDoctorValidationHelpersCoverErrorBranches(t *testing.T) {
	smtpResult := DiagnosticResult{Valid: true}
	validateSMTPSubmissionConfig(pinguinSMTPSubmission{Enabled: true}, &smtpResult)
	for _, expected := range []string{
		"smtpSubmission.hostname",
		"smtpSubmission.listenAddr",
		"smtpSubmission.maxMessageBytes",
		"smtpSubmission.maxRecipients",
		"smtpSubmission.relay.host",
		"smtpSubmission.relay.port",
		"smtpSubmission.relay.username",
		"smtpSubmission.relay.password",
		"smtpSubmission.tlsCertPath",
		"smtpSubmission.tlsKeyPath",
	} {
		if !containsDiagnosticError(smtpResult.Errors, expected) {
			t.Fatalf("expected SMTP validation error %q in %v", expected, smtpResult.Errors)
		}
	}
	invalidSMTPResult := DiagnosticResult{Valid: true}
	validateSMTPSubmissionConfig(pinguinSMTPSubmission{
		Enabled:            true,
		Hostname:           "smtp.example.com",
		ListenAddr:         ":587",
		DeliveryMode:       "bogus",
		PublicPort:         -1,
		PublicSecurityMode: "plaintext",
		MaxMessageBytes:    1048576,
		MaxRecipients:      25,
		AllowInsecureAuth:  true,
	}, &invalidSMTPResult)
	for _, expected := range []string{
		"smtpSubmission.deliveryMode",
		"smtpSubmission.publicPort",
		"smtpSubmission.publicSecurityMode",
	} {
		if !containsDiagnosticError(invalidSMTPResult.Errors, expected) {
			t.Fatalf("expected SMTP validation error %q in %v", expected, invalidSMTPResult.Errors)
		}
	}

	tenantResult := DiagnosticResult{Valid: true}
	validateTenantConfig(pinguinTenant{}, true, &tenantResult)
	for _, expected := range []string{
		"tenant[(unknown)]: displayName",
		"tenant[(unknown)]: at least one domain",
		"tenant[(unknown)]: at least one admin",
	} {
		if !containsDiagnosticError(tenantResult.Errors, expected) {
			t.Fatalf("expected tenant validation error %q in %v", expected, tenantResult.Errors)
		}
	}
}

func TestFormatSummaryIncludesCrossValidationErrors(t *testing.T) {
	report := &Report{
		Timestamp:   timeNowForDoctorTest,
		Summary:     reportSummary{TotalConfigs: 2, ValidConfigs: 2},
		Diagnostics: []DiagnosticResult{{ConfigPath: "a.yml", Valid: true}, {ConfigPath: "b.yml", Valid: true}},
		CrossValidation: crossValidation{
			Performed: true,
			Errors:    []string{"domain conflict"},
		},
	}
	summary := FormatSummary(report)
	if !strings.Contains(summary, "ERROR: domain conflict") {
		t.Fatalf("expected cross validation error, got %q", summary)
	}
}

func TestFormatSummaryIncludesCrossValidationWarnings(t *testing.T) {
	report := &Report{
		Timestamp:   timeNowForDoctorTest,
		Summary:     reportSummary{TotalConfigs: 2, ValidConfigs: 2},
		Diagnostics: []DiagnosticResult{{ConfigPath: "a.yml", Valid: true}, {ConfigPath: "b.yml", Valid: true}},
		CrossValidation: crossValidation{
			Performed: true,
			Warnings:  []string{"configuration warning"},
		},
	}
	summary := FormatSummary(report)
	if !strings.Contains(summary, "WARN: configuration warning") {
		t.Fatalf("expected cross validation warning, got %q", summary)
	}
}

func TestValidateCrossConfigsSkipsBlankDomains(t *testing.T) {
	validation := validateCrossConfigs(map[string]*pinguinConfig{
		"a.yml": {
			Tenants: pinguinYAMLNode{Tenants: []pinguinTenant{
				{ID: "tenant-a", Domains: []string{" ", "Alpha.example"}},
			}},
		},
		"b.yml": {
			Tenants: pinguinYAMLNode{Tenants: []pinguinTenant{
				{ID: "tenant-b", Domains: []string{"Beta.example"}},
			}},
		},
	})
	if !validation.Performed {
		t.Fatalf("expected cross validation to run")
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("blank domains should not create conflicts: %v", validation.Errors)
	}
}

func containsDiagnosticError(errors []string, expected string) bool {
	for _, diagnosticError := range errors {
		if strings.Contains(diagnosticError, expected) {
			return true
		}
	}
	return false
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

const currentServerTAuthConfigYAML = `
server:
  databasePath: /data/pinguin.db
  grpcAuthToken: test-token-123
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 60
  masterEncryptionKey: test-encryption-key-at-least-32-chars
  connectionTimeoutSec: 30
  operationTimeoutSec: 60
  tauth:
    signingKey: ${TEST_SIGNING_KEY}
    cookieName: app_session

web:
  enabled: true
  listenAddr: ":8080"
  allowedOrigins:
    - http://localhost:3000

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
    admins:
      - admin@example.com
`

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
  tauth:
    signingKey: test-signing-key
    cookieName: app_session

web:
  enabled: true
  listenAddr: ":8080"
  allowedOrigins:
    - http://localhost:3000

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
    admins:
      - admin@example.com
`

const timeNowForDoctorTest = "2026-05-03T00:00:00Z"

const mappingItemsSMTPSubmissionConfigYAML = `
server:
  databasePath: /data/pinguin.db
  grpcAuthToken: test-token-123
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 60
  masterEncryptionKey: short-master-key-warning
  connectionTimeoutSec: 30
  operationTimeoutSec: 60

web:
  enabled: false

smtpSubmission:
  enabled: true
  hostname: smtp.example.com
  listenAddr: ":587"
  tlsCertPath: /certs/fullchain.pem
  tlsKeyPath: /certs/privkey.pem
  maxMessageBytes: 1048576
  maxRecipients: 25
  relay:
    host: smtp-relay.example.com
    port: 587
    username: relay-user
    password: relay-pass

tenants:
  items:
    - id: mapped
      displayName: Mapped Tenant
      domains:
        - mapped.example.com
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
  tauth:
    signingKey: test-signing-key-2
    cookieName: app_session

web:
  enabled: true
  listenAddr: ":8081"
  allowedOrigins:
    - http://localhost:3001

tenants:
  - id: other
    displayName: Other Tenant
    domains:
      - other.example.com
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
  tauth:
    signingKey: test-signing-key-2
    cookieName: app_session

web:
  enabled: true
  listenAddr: ":8081"
  allowedOrigins:
    - http://localhost:3001

tenants:
  - id: conflicting
    displayName: Conflicting Tenant
    domains:
      - demo.example.com
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

const invalidSMTPSubmissionConfigYAML = `
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
  enabled: false

smtpSubmission:
  enabled: true
  hostname: smtp.example.com
  maxMessageBytes: 1048576
  maxRecipients: 25
  allowInsecureAuth: true

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
`

const directSMTPSubmissionConfigYAML = `
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
  enabled: false

smtpSubmission:
  enabled: true
  hostname: pinguin-api.mprlab.com
  listenAddr: ":587"
  publicPort: 465
  publicSecurityMode: ssl
  deliveryMode: direct
  maxMessageBytes: 26214400
  maxRecipients: 100
  allowInsecureAuth: true

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
`

const validSMTPForwardingConfigYAML = `
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
  enabled: false

smtpForwarding:
  enabled: true
  hostname: mx.pinguin.mprlab.com
  listenAddr: ":25"
  maxMessageBytes: 26214400
  maxRecipients: 100
  relay:
    host: smtp-relay.example.com
    port: 587
    username: relay-user
    password: relay-pass

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
`

const unknownSMTPSubmissionConfigYAML = `
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
  enabled: false

smtpSubmission:
  enabled: true
  hostname: pinguin-api.mprlab.com
  listenAddr: ":587"
  maxMessageBytes: 26214400
  maxRecipients: 100
  allowInsecureAuth: true
  unsupportedOption: true
  relay:
    host: smtp-relay.example.com
    port: 587
    username: relay-user
    password: relay-pass

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
`

const dynamicForwardingConfigYAML = `
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
  enabled: false

smtpForwarding:
  enabled: true
  hostname: mx.pinguin.mprlab.com
  listenAddr: ":25"
  maxMessageBytes: 26214400
  maxRecipients: 100
  relay:
    host: smtp-relay.example.com
    port: 587
    username: relay-user
    password: relay-pass

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
`

const invalidSMTPForwardingConfigYAML = `
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
  enabled: false

smtpForwarding:
  enabled: true
  hostname:
  listenAddr:
  maxMessageBytes: 0
  maxRecipients: 0
  relay:
    host:
    port: 0
    username:
    password:

tenants:
  - id: demo
    displayName: Demo Tenant
    domains:
      - demo.example.com
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
  tauth:
    signingKey: ${TEST_SIGNING_KEY}
    cookieName: app_session

web:
  enabled: true
  listenAddr: ${TEST_LISTEN_ADDR}
  allowedOrigins:
    - http://localhost:3000

tenants:
  - id: test
    displayName: Test Tenant
    domains:
      - ${TEST_TENANT_DOMAIN}
    admins:
      - ${TEST_ADMIN_EMAIL}
`
