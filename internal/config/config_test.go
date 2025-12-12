package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/internal/tenant"
)

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
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    status: active
    domains: [one.test]
    admins: [admin@one.test]
    identity:
      googleClientId: google-one
      tauthBaseUrl: https://auth.one.test
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
  staticRoot: web
  allowedOrigins:
    - https://app.local
    - https://alt.local
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
    issuer: tauth
    cookieName: custom_session
`)

	t.Setenv("PINGUIN_CONFIG_PATH", configPath)
	t.Setenv("DATABASE_PATH", "test.db")
	t.Setenv("GRPC_AUTH_TOKEN", "unit-token")
	t.Setenv("MASTER_ENCRYPTION_KEY", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Setenv("TAUTH_SIGNING_KEY", "signing-key")
	t.Setenv("SMTP_USERNAME", "apikey")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("TWILIO_ACCOUNT_SID", "sid")
	t.Setenv("TWILIO_AUTH_TOKEN", "auth")
	t.Setenv("TWILIO_FROM_NUMBER", "+10000000000")

	cfg, err := LoadConfig(false)
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
					Status:       "active",
					Domains:      []string{"one.test"},
					Admins:       tenant.BootstrapAdmins{"admin@one.test"},
					Identity: tenant.BootstrapIdentity{
						GoogleClientID: "google-one",
						TAuthBaseURL:   "https://auth.one.test",
					},
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
		WebInterfaceEnabled:  true,
		HTTPListenAddr:       ":8080",
		HTTPStaticRoot:       "web",
		HTTPAllowedOrigins:   []string{"https://app.local", "https://alt.local"},
		TAuthSigningKey:      "signing-key",
		TAuthIssuer:          "tauth",
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

func TestLoadConfigAppliesDefaultsAndRespectsDisableWeb(t *testing.T) {
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
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    status: active
    domains: [one.test]
    admins: [admin@one.test]
    identity:
      googleClientId: google-one
      tauthBaseUrl: https://auth.one.test
    emailProfile:
      host: smtp.one.test
      port: 587
      username: smtp-user
      password: smtp-pass
      fromAddress: noreply@one.test
web:
  enabled: true
  listenAddr: :0
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
    issuer: tauth
`)
	t.Setenv("PINGUIN_CONFIG_PATH", configPath)
	t.Setenv("MASTER_ENCRYPTION_KEY", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	t.Setenv("TAUTH_SIGNING_KEY", "signing-key")

	cfg, err := LoadConfig(true)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.WebInterfaceEnabled {
		t.Fatalf("expected web interface to be disabled")
	}
	if cfg.HTTPStaticRoot != "" || cfg.TAuthCookieName != "" || cfg.HTTPAllowedOrigins != nil {
		t.Fatalf("expected web fields to be cleared when disabled")
	}
	if cfg.ConnectionTimeoutSec != 5 || cfg.OperationTimeoutSec != 10 {
		t.Fatalf("expected timeout values to be set from config")
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
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    status: active
    domains: [one.test]
    admins: [admin@one.test]
    identity:
      googleClientId: google-one
      tauthBaseUrl: https://auth.one.test
    emailProfile:
      host: smtp.one.test
      port: 587
      username: smtp-user
      password: smtp-pass
      fromAddress: noreply@one.test
web:
  enabled: false
`)
	t.Setenv("PINGUIN_CONFIG_PATH", configPath)

	_, err := LoadConfig(false)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "server.grpcAuthToken") {
		t.Fatalf("unexpected error: %v", err)
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
