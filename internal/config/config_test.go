package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/internal/tenant"
	"gopkg.in/yaml.v3"
)

func ptrBool(value bool) *bool {
	return &value
}

func TestLoadConfigFromYAMLWithEnvExpansion(t *testing.T) {
	t.Helper()

	configPath := writeConfigFile(t, `
server:
  databasePath: ${DATABASE_PATH}
  grpcAuthToken: ${GRPC_AUTH_TOKEN}
  logLevel: INFO
  maxRetries: 5
  retryIntervalSec: 4
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 3
  operationTimeoutSec: 7
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
    cookieName: custom_session
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    enabled: true
    domains: [one.test]
    emailProfile:
      host: smtp.one.test
      port: 587
      username: ${SMTP_USERNAME}
      password: ${SMTP_PASSWORD}
      fromAddress: noreply@one.test
    smsProfile:
      accountSid: ${TWILIO_ACCOUNT_SID}
      authToken: ${TWILIO_AUTH_TOKEN}
      fromNumber: ${TWILIO_FROM_NUMBER}
web:
  enabled: true
  listenAddr: :8080
  allowedOrigins:
    - https://app.local
    - https://alt.local
smtpSubmission:
  enabled: true
  hostname: smtp.one.test
  listenAddr: :587
  maxMessageBytes: 1048576
  maxRecipients: 25
  allowInsecureAuth: true
  relay:
    host: ${SMTP_SUBMISSION_RELAY_HOST}
    port: ${SMTP_SUBMISSION_RELAY_PORT}
    username: ${SMTP_SUBMISSION_RELAY_USERNAME}
    password: ${SMTP_SUBMISSION_RELAY_PASSWORD}
`)

	t.Setenv("DATABASE_PATH", "test.db")
	t.Setenv("GRPC_AUTH_TOKEN", "unit-token")
	t.Setenv("MASTER_ENCRYPTION_KEY", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Setenv("TAUTH_SIGNING_KEY", "signing-key")
	t.Setenv("SMTP_USERNAME", "apikey")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("TWILIO_ACCOUNT_SID", "sid")
	t.Setenv("TWILIO_AUTH_TOKEN", "auth")
	t.Setenv("TWILIO_FROM_NUMBER", "+10000000000")
	t.Setenv("SMTP_SUBMISSION_RELAY_HOST", "relay.one.test")
	t.Setenv("SMTP_SUBMISSION_RELAY_PORT", "2525")
	t.Setenv("SMTP_SUBMISSION_RELAY_USERNAME", "relay-user")
	t.Setenv("SMTP_SUBMISSION_RELAY_PASSWORD", "relay-secret")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	expected := Config{
		DatabasePath:        "test.db",
		GRPCAuthToken:       "unit-token",
		LogLevel:            "INFO",
		MaxRetries:          5,
		RetryIntervalSec:    4,
		MasterEncryptionKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TenantBootstrap: tenant.BootstrapConfig{
			Tenants: []tenant.BootstrapTenant{
				{
					ID:           "tenant-one",
					DisplayName:  "One Corp",
					SupportEmail: "support@one.test",
					Enabled:      ptrBool(true),
					Domains:      []string{"one.test"},
					EmailProfile: tenant.BootstrapEmailProfile{
						Host:        "smtp.one.test",
						Port:        587,
						Username:    "apikey",
						Password:    "secret",
						FromAddress: "noreply@one.test",
					},
					SMSProfile: &tenant.BootstrapSMSProfile{
						AccountSID: "sid",
						AuthToken:  "auth",
						FromNumber: "+10000000000",
					},
				},
			},
		},
		WebInterfaceEnabled: true,
		HTTPListenAddr:      ":8080",
		HTTPAllowedOrigins:  []string{"https://app.local", "https://alt.local"},
		SMTPSubmission: SMTPSubmissionConfig{
			Enabled:           true,
			Hostname:          "smtp.one.test",
			ListenAddr:        ":587",
			DeliveryMode:      "upstream",
			MaxMessageBytes:   1048576,
			MaxRecipients:     25,
			AllowInsecureAuth: true,
			Relay: SMTPSubmissionRelayConfig{
				Host:     "relay.one.test",
				Port:     2525,
				Username: "relay-user",
				Password: "relay-secret",
			},
		},
		TAuthSigningKey:      "signing-key",
		TAuthCookieName:      "custom_session",
		ConnectionTimeoutSec: 3,
		OperationTimeoutSec:  7,
	}

	if !reflect.DeepEqual(cfg, expected) {
		t.Fatalf("unexpected config:\n got: %#v\nwant: %#v", cfg, expected)
	}
	if cfg.TwilioConfigured() {
		t.Fatalf("expected global Twilio credentials to remain empty when provided via tenants")
	}
}

func TestLoadConfigAppliesDefaultsAndRespectsWebEnabledFalse(t *testing.T) {
	t.Helper()
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: DEBUG
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    enabled: true
    domains: [one.test]
    emailProfile:
      host: smtp.one.test
      port: 587
      username: smtp-user
      password: smtp-pass
      fromAddress: noreply@one.test
web:
  enabled: false
  listenAddr: :0
`)
	t.Setenv("MASTER_ENCRYPTION_KEY", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	t.Setenv("TAUTH_SIGNING_KEY", "signing-key")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.WebInterfaceEnabled {
		t.Fatalf("expected web interface to be disabled")
	}
	if cfg.TAuthCookieName != "" || cfg.HTTPAllowedOrigins != nil {
		t.Fatalf("expected web fields to be cleared when disabled")
	}
	if cfg.ConnectionTimeoutSec != 5 || cfg.OperationTimeoutSec != 10 {
		t.Fatalf("expected timeout values to be set from config")
	}
}

func TestLoadConfigDefaultsCookieAndSupportsTenantConfigPath(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: signing-key
tenants:
  configPath: tenants.yml
web:
  enabled: true
  listenAddr: :0
`)
	t.Setenv("MASTER_ENCRYPTION_KEY", "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.TenantConfigPath != "tenants.yml" {
		t.Fatalf("unexpected tenant config path %q", cfg.TenantConfigPath)
	}
	if cfg.TAuthCookieName != "app_session" {
		t.Fatalf("expected default cookie, got %q", cfg.TAuthCookieName)
	}
}

func TestLoadConfigAllowsDirectSMTPSubmissionWithoutUpstreamRelay(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  configPath: tenants.yml
web:
  enabled: false
smtpSubmission:
  enabled: true
  hostname: pinguin-api.mprlab.com
  listenAddr: :587
  publicPort: 465
  publicSecurityMode: ssl
  deliveryMode: direct
  maxMessageBytes: 26214400
  maxRecipients: 100
  allowInsecureAuth: true
`)
	t.Setenv("MASTER_ENCRYPTION_KEY", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.SMTPSubmission.DeliveryMode != "direct" {
		t.Fatalf("expected direct delivery mode, got %q", cfg.SMTPSubmission.DeliveryMode)
	}
	if cfg.SMTPSubmission.PublicPort != 465 || cfg.SMTPSubmission.PublicSecurityMode != "ssl" {
		t.Fatalf("unexpected public SMTP settings %+v", cfg.SMTPSubmission)
	}
	if cfg.SMTPSubmission.Relay.Host != "" || cfg.SMTPSubmission.Relay.Port != 0 {
		t.Fatalf("expected direct mode not to require upstream relay, got %+v", cfg.SMTPSubmission.Relay)
	}
}

func TestLoadConfigSupportsSMTPForwarding(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  configPath: tenants.yml
web:
  enabled: false
smtpForwarding:
  enabled: true
  hostname: mx.pinguin.mprlab.com
  listenAddr: :25
  maxMessageBytes: 26214400
  maxRecipients: 25
  relay:
    host: relay.example.com
    port: 587
    username: relay-user
    password: relay-pass
`)
	t.Setenv("MASTER_ENCRYPTION_KEY", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	expected := SMTPForwardingConfig{
		Enabled:         true,
		Hostname:        "mx.pinguin.mprlab.com",
		ListenAddr:      ":25",
		MaxMessageBytes: 26214400,
		MaxRecipients:   25,
		Relay: SMTPForwardingRelayConfig{
			Host:     "relay.example.com",
			Port:     587,
			Username: "relay-user",
			Password: "relay-pass",
		},
	}
	if !reflect.DeepEqual(cfg.SMTPForwarding, expected) {
		t.Fatalf("unexpected SMTP forwarding config:\n got: %#v\nwant: %#v", cfg.SMTPForwarding, expected)
	}
}

func TestLoadConfigDoesNotDisableWebFromEnvironment(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: signing-key
tenants:
  configPath: tenants.yml
web:
  enabled: true
  listenAddr: :0
`)
	t.Setenv("MASTER_ENCRYPTION_KEY", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	t.Setenv("DISABLE_WEB_INTERFACE", "yes")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if !cfg.WebInterfaceEnabled {
		t.Fatalf("expected web.enabled from config.yml to control the web interface")
	}
}

func TestLoadConfigRejectsMissingEnvironmentVariables(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  configPath: tenants.yml
web:
  enabled: false
smtpSubmission:
  enabled: true
  hostname: smtp.one.test
  listenAddr: :587
  maxMessageBytes: 1048576
  maxRecipients: 25
  allowInsecureAuth: true
  relay:
    host: relay.one.test
    port: 2525
    username: relay-user
    password: ${SMTP_SUBMISSION_RELAY_PASSWORD}
`)

	_, err := loadConfigFromPath(configPath)
	if err == nil {
		t.Fatalf("expected missing environment variable error")
	}
	if !strings.Contains(err.Error(), "configuration: missing environment variables: SMTP_SUBMISSION_RELAY_PASSWORD") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsUnknownSMTPSubmissionFields(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  configPath: tenants.yml
web:
  enabled: false
smtpSubmission:
  enabled: true
  hostname: smtp.one.test
  listenAddr: :587
  maxMessageBytes: 1048576
  maxRecipients: 25
  allowInsecureAuth: true
  unsupportedOption: true
  relay:
    host: relay.one.test
    port: 2525
    username: relay-user
    password: relay-secret
`)

	_, err := loadConfigFromPath(configPath)
	if err == nil || !strings.Contains(err.Error(), "field unsupportedOption not found") {
		t.Fatalf("expected unknown smtpSubmission field error, got %v", err)
	}
}

func TestLoadConfigRejectsUnsupportedTenantFields(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		tenantSnippet string
		expected      string
	}{
		{
			name: "legacy_status",
			tenantSnippet: `
    status: active`,
			expected: "tenants[].status is no longer supported",
		},
		{
			name: "legacy_identity",
			tenantSnippet: `
    identity:
      googleClientId: google-client
      tauthBaseUrl: https://tauth-api.mprlab.com`,
			expected: "tenants[].identity is not supported",
		},
		{
			name: "unknown_tenant_field",
			tenantSnippet: `
    unsupportedOption: true`,
			expected: "tenants[].unsupportedOption is not supported",
		},
		{
			name: "unknown_email_profile_field",
			tenantSnippet: `
    emailProfile:
      host: smtp.one.test
      port: 587
      username: smtp-user
      password: smtp-pass
      fromAddress: noreply@one.test
      unsupportedOption: true`,
			expected: "tenants[].emailProfile.unsupportedOption is not supported",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			configPath := writeConfigFile(t, tenantConfigWithSnippet(testCase.tenantSnippet))

			_, err := loadConfigFromPath(configPath)
			if err == nil || !strings.Contains(err.Error(), testCase.expected) {
				t.Fatalf("expected tenant schema error containing %q, got %v", testCase.expected, err)
			}
		})
	}
}

func TestValidateConfigRejectsInvalidSMTPForwarding(t *testing.T) {
	cfg := Config{
		DatabasePath:         "app.db",
		GRPCAuthToken:        "token",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     30,
		MasterEncryptionKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  10,
		TenantConfigPath:     "tenants.yml",
		WebInterfaceEnabled:  false,
		SMTPForwarding: SMTPForwardingConfig{
			Enabled:         true,
			MaxMessageBytes: -1,
			MaxRecipients:   0,
		},
	}
	err := validateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	for _, expected := range []string{
		"smtpForwarding.hostname",
		"smtpForwarding.listenAddr",
		"smtpForwarding.maxMessageBytes",
		"smtpForwarding.maxRecipients",
		"smtpForwarding.relay.host",
		"smtpForwarding.relay.port",
		"smtpForwarding.relay.username",
		"smtpForwarding.relay.password",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain %s, got %v", expected, err)
		}
	}
}

func TestValidateConfigAllowsSMTPForwardingWithoutStaticSenderDomains(t *testing.T) {
	cfg := Config{
		DatabasePath:         "app.db",
		GRPCAuthToken:        "token",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     30,
		MasterEncryptionKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  10,
		TenantConfigPath:     "tenants.yml",
		WebInterfaceEnabled:  false,
		SMTPForwarding: SMTPForwardingConfig{
			Enabled:         true,
			Hostname:        "mx.pinguin.mprlab.com",
			ListenAddr:      ":25",
			MaxMessageBytes: 26214400,
			MaxRecipients:   25,
			Relay: SMTPForwardingRelayConfig{
				Host:     "relay.example.com",
				Port:     587,
				Username: "relay-user",
				Password: "relay-pass",
			},
		},
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("expected dynamic sender domains to satisfy forwarding config, got %v", err)
	}
}

func TestLoadConfigRejectsUnreadableAndInvalidYAML(t *testing.T) {
	missingConfigPath := filepath.Join(t.TempDir(), "missing.yml")
	if _, err := loadConfigFromPath(missingConfigPath); err == nil || !strings.Contains(err.Error(), "configuration: read") {
		t.Fatalf("expected read error, got %v", err)
	}

	configPath := writeConfigFile(t, "server:\n  databasePath: [")
	if _, err := loadConfigFromPath(configPath); err == nil || !strings.Contains(err.Error(), "configuration: parse yaml") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestTenantConfigUnmarshalShapes(t *testing.T) {
	var nilConfig tenantConfig
	if err := nilConfig.UnmarshalYAML(nil); err != nil {
		t.Fatalf("nil unmarshal: %v", err)
	}

	var sequence tenantConfig
	if err := yaml.Unmarshal([]byte("- id: tenant-one\n  displayName: One\n"), &sequence); err != nil {
		t.Fatalf("unmarshal sequence: %v", err)
	}
	if len(sequence.Tenants) != 1 || sequence.ConfigPath != "" {
		t.Fatalf("unexpected sequence config %+v", sequence)
	}

	var unsupported tenantConfig
	if err := yaml.Unmarshal([]byte("items:\n  - id: tenant-two\n    displayName: Two\nconfigPath: ignored.yml\n"), &unsupported); err == nil || !strings.Contains(err.Error(), "tenants.items is not supported") {
		t.Fatalf("expected unsupported items error, got %v", err)
	}

	var invalid tenantConfig
	if err := yaml.Unmarshal([]byte("123"), &invalid); err == nil || !strings.Contains(err.Error(), "tenants must be a list") {
		t.Fatalf("expected invalid tenant shape error, got %v", err)
	}

	sequenceNode := &yaml.Node{
		Kind: yaml.SequenceNode,
		Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "domains"},
				{Kind: yaml.MappingNode},
			}},
		},
	}
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
	if err := unsupported.UnmarshalYAML(mappingNode); err == nil {
		t.Fatalf("expected mapping decode error")
	}

	invalidKnownMappingNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "tenants"},
			{Kind: yaml.MappingNode},
		},
	}
	if err := unsupported.UnmarshalYAML(invalidKnownMappingNode); err == nil {
		t.Fatalf("expected known mapping decode error")
	}
}

func TestLoadConfigRejectsMissingRequiredField(t *testing.T) {
	t.Helper()
	configPath := writeConfigFile(t, `
server:
  databasePath: db.sqlite
  grpcAuthToken: ""
  logLevel: INFO
  maxRetries: 1
  retryIntervalSec: 10
  masterEncryptionKey: key
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: signing-key
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    enabled: true
    domains: [one.test]
    emailProfile:
      host: smtp.one.test
      port: 587
      username: smtp-user
      password: smtp-pass
      fromAddress: noreply@one.test
web:
  enabled: false
`)

	_, err := loadConfigFromPath(configPath)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "server.grpcAuthToken") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigAggregatesMissingFields(t *testing.T) {
	err := validateConfig(Config{
		WebInterfaceEnabled: true,
		SMTPSubmission: SMTPSubmissionConfig{
			Enabled: true,
		},
		TenantBootstrap: tenant.BootstrapConfig{
			Tenants: []tenant.BootstrapTenant{{ID: "tenant-one"}},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	for _, expected := range []string{
		"server.databasePath",
		"web.listenAddr",
		"smtpSubmission.hostname",
		"tenants[0].displayName",
		"tenants[0].domains",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain %s, got %v", expected, err)
		}
	}
}

func TestValidateConfigRejectsInvalidSMTPSubmissionModeAndPublicSettings(t *testing.T) {
	cfg := Config{
		DatabasePath:         "app.db",
		GRPCAuthToken:        "token",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     30,
		MasterEncryptionKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  10,
		TenantConfigPath:     "tenants.yml",
		WebInterfaceEnabled:  false,
		SMTPSubmission: SMTPSubmissionConfig{
			Enabled:            true,
			Hostname:           "smtp.example.com",
			ListenAddr:         ":587",
			DeliveryMode:       "bogus",
			PublicPort:         -1,
			PublicSecurityMode: "plaintext",
			MaxMessageBytes:    1048576,
			MaxRecipients:      25,
			AllowInsecureAuth:  true,
		},
	}
	err := validateConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	for _, expected := range []string{
		"smtpSubmission.deliveryMode",
		"smtpSubmission.publicPort",
		"smtpSubmission.publicSecurityMode",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain %s, got %v", expected, err)
		}
	}
}

func TestLoadConfigRejectsIncompleteSMTPSubmission(t *testing.T) {
	t.Helper()
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    enabled: true
    domains: [one.test]
    emailProfile:
      host: smtp.one.test
      port: 587
      username: smtp-user
      password: smtp-pass
      fromAddress: noreply@one.test
web:
  enabled: false
smtpSubmission:
  enabled: true
  hostname: smtp.one.test
`)
	t.Setenv("MASTER_ENCRYPTION_KEY", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")

	_, err := loadConfigFromPath(configPath)
	if err == nil {
		t.Fatalf("expected SMTP submission validation error")
	}
	if !strings.Contains(err.Error(), "smtpSubmission.listenAddr") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigUsesDefaultPath(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tempDir, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	configPath := filepath.Join(tempDir, defaultConfigPath)
	if err := os.WriteFile(configPath, []byte(`
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  configPath: tenants.yml
web:
  enabled: false
`), 0o600); err != nil {
		t.Fatalf("write default config: %v", err)
	}
	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWorkingDirectory); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig default path: %v", err)
	}
	if cfg.TenantConfigPath != "tenants.yml" {
		t.Fatalf("unexpected tenant config path %q", cfg.TenantConfigPath)
	}
}

func TestDefaultConfigFilePathFallsBackToLocalDefault(t *testing.T) {
	originalDefaultConfigPaths := defaultConfigPaths
	defaultConfigPaths = []string{filepath.Join(t.TempDir(), "missing.yml")}
	t.Cleanup(func() {
		defaultConfigPaths = originalDefaultConfigPaths
	})

	if got := defaultConfigFilePath(); got != defaultConfigPath {
		t.Fatalf("expected fallback path %q, got %q", defaultConfigPath, got)
	}
}

func TestStringHelpersSkipBlankValues(t *testing.T) {
	if normalized := normalizeStrings([]string{" one ", " ", "two"}); !reflect.DeepEqual(normalized, []string{"one", "two"}) {
		t.Fatalf("unexpected normalized strings %v", normalized)
	}
	if count := countNonEmptyStrings([]string{"one", " ", "two"}); count != 2 {
		t.Fatalf("unexpected non-empty count %d", count)
	}
}

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func tenantConfigWithSnippet(tenantSnippet string) string {
	return `
server:
  databasePath: app.db
  grpcAuthToken: token
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
tenants:
  - id: tenant-one
    displayName: One Corp
    domains: [one.test]` + tenantSnippet + `
web:
  enabled: false
`
}
