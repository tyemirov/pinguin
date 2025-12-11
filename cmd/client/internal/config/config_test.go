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
				authTokenKey:     "token",
				tenantIDKey:      "tenant",
				serverAddressKey: "",
			},
			wantErr: "missing gRPC server address",
		},
		{
			name: "missing token",
			values: map[string]interface{}{
				serverAddressKey: "localhost:5050",
				tenantIDKey:      "tenant",
				authTokenKey:     "",
			},
			wantErr: "missing gRPC auth token",
		},
		{
			name: "invalid connection timeout",
			values: map[string]interface{}{
				serverAddressKey:     "localhost:5050",
				authTokenKey:         "token",
				tenantIDKey:          "tenant",
				connectionTimeoutKey: 0,
			},
			wantErr: "invalid connection timeout",
		},
		{
			name: "invalid operation timeout",
			values: map[string]interface{}{
				serverAddressKey:    "localhost:5050",
				authTokenKey:        "token",
				tenantIDKey:         "tenant",
				operationTimeoutKey: 0,
			},
			wantErr: "invalid operation timeout",
		},
		{
			name: "missing tenant id",
			values: map[string]interface{}{
				serverAddressKey: "localhost:5050",
				authTokenKey:     "token",
				tenantIDKey:      "",
			},
			wantErr: "missing tenant id",
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
