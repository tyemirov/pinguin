package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	serverAddressKey     = "grpc_server_addr"
	authTokenKey         = "grpc_auth_token"
	tenantIDKey          = "tenant_id"
	connectionTimeoutKey = "connection_timeout_sec"
	operationTimeoutKey  = "operation_timeout_sec"
	logLevelKey          = "log_level"
)

type Config struct {
	serverAddress     string
	authToken         string
	tenantID          string
	connectionTimeout int
	operationTimeout  int
	logLevel          string
}

func Load(provider *viper.Viper) (Config, error) {
	if provider == nil {
		return Config{}, fmt.Errorf("nil config provider")
	}

	provider.SetEnvPrefix("PINGUIN")
	provider.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	provider.AutomaticEnv()
	provider.SetDefault(serverAddressKey, "localhost:50051")
	provider.SetDefault(connectionTimeoutKey, 5)
	provider.SetDefault(operationTimeoutKey, 30)
	provider.SetDefault(logLevelKey, "INFO")

	serverAddress := strings.TrimSpace(provider.GetString(serverAddressKey))
	if serverAddress == "" {
		return Config{}, fmt.Errorf("missing gRPC server address")
	}

	authToken := strings.TrimSpace(provider.GetString(authTokenKey))
	if authToken == "" {
		return Config{}, fmt.Errorf("missing gRPC auth token")
	}

	tenantID := strings.TrimSpace(provider.GetString(tenantIDKey))
	if tenantID == "" {
		return Config{}, fmt.Errorf("missing tenant id (set PINGUIN_TENANT_ID)")
	}

	connectionTimeout := provider.GetInt(connectionTimeoutKey)
	if connectionTimeout <= 0 {
		return Config{}, fmt.Errorf("invalid connection timeout %d", connectionTimeout)
	}

	operationTimeout := provider.GetInt(operationTimeoutKey)
	if operationTimeout <= 0 {
		return Config{}, fmt.Errorf("invalid operation timeout %d", operationTimeout)
	}

	logLevel := strings.TrimSpace(provider.GetString(logLevelKey))
	if logLevel == "" {
		logLevel = "INFO"
	}

	return Config{
		serverAddress:     serverAddress,
		authToken:         authToken,
		tenantID:          tenantID,
		connectionTimeout: connectionTimeout,
		operationTimeout:  operationTimeout,
		logLevel:          strings.ToUpper(logLevel),
	}, nil
}

func (configuration Config) ServerAddress() string {
	return configuration.serverAddress
}

func (configuration Config) AuthToken() string {
	return configuration.authToken
}

func (configuration Config) ConnectionTimeoutSeconds() int {
	return configuration.connectionTimeout
}

func (configuration Config) OperationTimeoutSeconds() int {
	return configuration.operationTimeout
}

func (configuration Config) TenantID() string {
	return configuration.tenantID
}

func (configuration Config) ConnectionTimeout() time.Duration {
	return time.Duration(configuration.connectionTimeout) * time.Second
}

func (configuration Config) OperationTimeout() time.Duration {
	return time.Duration(configuration.operationTimeout) * time.Second
}

func (configuration Config) LogLevel() string {
	return configuration.logLevel
}
