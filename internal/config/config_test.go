package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/internal/tenant"
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
    googleClientId: ${TAUTH_GOOGLE_CLIENT_ID}
    tauthBaseUrl: ${TAUTH_BASE_URL}
    tauthTenantId: ${TAUTH_TENANT_ID}
    allowedUsers: ${TAUTH_ALLOWED_USERS}
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    enabled: true
    domains: [one.test]
    admins: [admin@one.test]
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
`)

	t.Setenv("PINGUIN_CONFIG_PATH", configPath)
	t.Setenv("DATABASE_PATH", "test.db")
	t.Setenv("GRPC_AUTH_TOKEN", "unit-token")
	t.Setenv("MASTER_ENCRYPTION_KEY", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Setenv("TAUTH_SIGNING_KEY", "signing-key")
	t.Setenv("TAUTH_GOOGLE_CLIENT_ID", "google-one")
	t.Setenv("TAUTH_BASE_URL", "https://auth.one.test")
	t.Setenv("TAUTH_TENANT_ID", "tauth-one")
	t.Setenv("TAUTH_ALLOWED_USERS", "admin@one.test,viewer@one.test")
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
					Enabled:      ptrBool(true),
					Domains:      []string{"one.test"},
					Admins:       tenant.BootstrapAdmins{"admin@one.test"},
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
		HTTPAllowedOrigins:   []string{"https://app.local", "https://alt.local"},
		TAuthSigningKey:      "signing-key",
		TAuthCookieName:      "custom_session",
		TAuthBaseURL:         "https://auth.one.test",
		TAuthTenantID:        "tauth-one",
		TAuthGoogleClientID:  "google-one",
		TAuthAllowedUsers:    []string{"admin@one.test", "viewer@one.test"},
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
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
    googleClientId: google-one
    tauthBaseUrl: https://auth.one.test
    tauthTenantId: tauth-one
    allowedUsers: admin@one.test
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    enabled: true
    domains: [one.test]
    admins: [admin@one.test]
    emailProfile:
      host: smtp.one.test
      port: 587
      username: smtp-user
      password: smtp-pass
      fromAddress: noreply@one.test
web:
  enabled: true
  listenAddr: :0
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
	if cfg.TAuthCookieName != "" || cfg.HTTPAllowedOrigins != nil {
		t.Fatalf("expected web fields to be cleared when disabled")
	}
	if cfg.TAuthBaseURL != "" || cfg.TAuthTenantID != "" || cfg.TAuthGoogleClientID != "" {
		t.Fatalf("expected tauth fields to be cleared when disabled")
	}
	if cfg.TAuthAllowedUsers != nil {
		t.Fatalf("expected tauth allowed users to be cleared when disabled")
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
  tauth:
    signingKey: signing-key
    googleClientId: google-one
    tauthBaseUrl: https://auth.one.test
    tauthTenantId: tauth-one
    allowedUsers: admin@one.test
tenants:
  - id: tenant-one
    displayName: One Corp
    supportEmail: support@one.test
    enabled: true
    domains: [one.test]
    admins: [admin@one.test]
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
