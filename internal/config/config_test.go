package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfigFromYAMLWithEnvExpansion(t *testing.T) {
	t.Helper()

	configPath := writeConfigFile(t, `
server:
  databasePath: ${DATABASE_PATH}
  logLevel: INFO
  maxRetries: 5
  retryIntervalSec: 4
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 3
  operationTimeoutSec: 7
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
    cookieName: custom_session
web:
  enabled: true
  listenAddr: :8080
  allowedOrigins:
    - https://app.local
    - https://alt.local
  trustedProxies:
    - 198.51.100.10
    - "  2001:db8::/32  "
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
		LogLevel:            "INFO",
		MaxRetries:          5,
		RetryIntervalSec:    4,
		MasterEncryptionKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		WebInterfaceEnabled: true,
		HTTPListenAddr:      ":8080",
		HTTPAllowedOrigins:  []string{"https://app.local", "https://alt.local"},
		HTTPTrustedProxies:  []string{"198.51.100.10", "2001:db8::/32"},
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
}

func TestLoadConfigAppliesDefaultsAndRespectsWebEnabledFalse(t *testing.T) {
	t.Helper()
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  logLevel: DEBUG
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
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
	if cfg.TAuthCookieName != "" || cfg.HTTPAllowedOrigins != nil || cfg.HTTPTrustedProxies != nil {
		t.Fatalf("expected web fields to be cleared when disabled")
	}
	if cfg.ConnectionTimeoutSec != 5 || cfg.OperationTimeoutSec != 10 {
		t.Fatalf("expected timeout values to be set from config")
	}
}

func TestLoadConfigDefaultsCookie(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: signing-key
web:
  enabled: true
  listenAddr: :0
`)
	t.Setenv("MASTER_ENCRYPTION_KEY", "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.TAuthCookieName != "app_session" {
		t.Fatalf("expected default cookie, got %q", cfg.TAuthCookieName)
	}
}

func TestLoadConfigAllowsDirectSMTPSubmissionWithoutUpstreamRelay(t *testing.T) {
	configPath := writeConfigFile(t, `
server:
  databasePath: app.db
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
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
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
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
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: signing-key
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
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
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
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
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

func TestValidateConfigRejectsInvalidSMTPForwarding(t *testing.T) {
	cfg := Config{
		DatabasePath:         "app.db",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     30,
		MasterEncryptionKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  10,
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
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     30,
		MasterEncryptionKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  10,
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

func TestLoadConfigRejectsMissingRequiredField(t *testing.T) {
	t.Helper()
	configPath := writeConfigFile(t, `
server:
  databasePath: ""
  logLevel: INFO
  maxRetries: 1
  retryIntervalSec: 10
  masterEncryptionKey: key
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
  tauth:
    signingKey: signing-key
web:
  enabled: false
`)

	_, err := loadConfigFromPath(configPath)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "server.databasePath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigAggregatesMissingFields(t *testing.T) {
	err := validateConfig(Config{
		WebInterfaceEnabled: true,
		SMTPSubmission: SMTPSubmissionConfig{
			Enabled: true,
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	for _, expected := range []string{
		"server.databasePath",
		"web.listenAddr",
		"smtpSubmission.hostname",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain %s, got %v", expected, err)
		}
	}
}

func TestValidateConfigRejectsInvalidSMTPSubmissionModeAndPublicSettings(t *testing.T) {
	cfg := Config{
		DatabasePath:         "app.db",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     30,
		MasterEncryptionKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  10,
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
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
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
  logLevel: INFO
  maxRetries: 3
  retryIntervalSec: 30
  masterEncryptionKey: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  connectionTimeoutSec: 5
  operationTimeoutSec: 10
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
	if cfg.DatabasePath != "app.db" {
		t.Fatalf("unexpected default config %+v", cfg)
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
}

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
