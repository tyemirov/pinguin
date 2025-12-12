package config

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestLoadSuccessful(t *testing.T) {
	t.Helper()
	v := viper.New()
	v.Set(serverAddressKey, "localhost:5050")
	v.Set(authTokenKey, "secret")
	v.Set(tenantIDKey, "tenant-cli")
	v.Set(connectionTimeoutKey, 7)
	v.Set(operationTimeoutKey, 13)
	v.Set(logLevelKey, "debug")

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ServerAddress() != "localhost:5050" {
		t.Fatalf("unexpected server address: %s", cfg.ServerAddress())
	}
	if cfg.AuthToken() != "secret" {
		t.Fatalf("unexpected auth token: %s", cfg.AuthToken())
	}
	if cfg.TenantID() != "tenant-cli" {
		t.Fatalf("unexpected tenant id: %s", cfg.TenantID())
	}
	if cfg.ConnectionTimeoutSeconds() != 7 {
		t.Fatalf("unexpected connection timeout seconds: %d", cfg.ConnectionTimeoutSeconds())
	}
	if cfg.OperationTimeoutSeconds() != 13 {
		t.Fatalf("unexpected operation timeout seconds: %d", cfg.OperationTimeoutSeconds())
	}
	if cfg.ConnectionTimeout() != 7*time.Second {
		t.Fatalf("unexpected connection timeout duration: %s", cfg.ConnectionTimeout())
	}
	if cfg.OperationTimeout() != 13*time.Second {
		t.Fatalf("unexpected operation timeout duration: %s", cfg.OperationTimeout())
	}
	if cfg.LogLevel() != "DEBUG" {
		t.Fatalf("unexpected log level: %s", cfg.LogLevel())
	}
}

func TestLoadBindsUnprefixedEnvFallbacks(t *testing.T) {
	t.Helper()
	t.Setenv("GRPC_SERVER_ADDR", "localhost:6060")
	t.Setenv("GRPC_AUTH_TOKEN", "secret")
	t.Setenv("TENANT_ID", "tenant-cli")
	t.Setenv("CONNECTION_TIMEOUT_SEC", "9")
	t.Setenv("OPERATION_TIMEOUT_SEC", "11")
	t.Setenv("LOG_LEVEL", "warn")

	v := viper.New()
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ServerAddress() != "localhost:6060" {
		t.Fatalf("unexpected server address: %s", cfg.ServerAddress())
	}
	if cfg.AuthToken() != "secret" {
		t.Fatalf("unexpected auth token: %s", cfg.AuthToken())
	}
	if cfg.TenantID() != "tenant-cli" {
		t.Fatalf("unexpected tenant id: %s", cfg.TenantID())
	}
	if cfg.ConnectionTimeoutSeconds() != 9 {
		t.Fatalf("unexpected connection timeout seconds: %d", cfg.ConnectionTimeoutSeconds())
	}
	if cfg.OperationTimeoutSeconds() != 11 {
		t.Fatalf("unexpected operation timeout seconds: %d", cfg.OperationTimeoutSeconds())
	}
	if cfg.LogLevel() != "WARN" {
		t.Fatalf("unexpected log level: %s", cfg.LogLevel())
	}
}

func TestLoadErrorConditions(t *testing.T) {
	t.Helper()
	testCases := []struct {
		name    string
		values  map[string]interface{}
		wantErr string
	}{
		{
			name: "missing server",
			values: map[string]interface{}{
				serverAddressKey: "",
			},
			wantErr: "missing gRPC server address",
		},
		{
			name: "invalid connection timeout",
			values: map[string]interface{}{
				serverAddressKey:     "localhost:5050",
				connectionTimeoutKey: 0,
			},
			wantErr: "invalid connection timeout",
		},
		{
			name: "invalid operation timeout",
			values: map[string]interface{}{
				serverAddressKey:    "localhost:5050",
				operationTimeoutKey: 0,
			},
			wantErr: "invalid operation timeout",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			v := viper.New()
			for key, value := range tc.values {
				v.Set(key, value)
			}
			_, err := Load(v)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}
