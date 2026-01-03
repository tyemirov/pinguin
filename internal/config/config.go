package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/tyemirov/pinguin/internal/tenant"
	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "configs/config.yml"

type Config struct {
	DatabasePath     string
	GRPCAuthToken    string
	LogLevel         string
	MaxRetries       int
	RetryIntervalSec int

	MasterEncryptionKey string
	TenantConfigPath    string
	TenantBootstrap     tenant.BootstrapConfig

	WebInterfaceEnabled bool
	HTTPListenAddr      string
	HTTPAllowedOrigins  []string

	TAuthSigningKey     string
	TAuthCookieName     string
	TAuthBaseURL        string
	TAuthTenantID       string
	TAuthGoogleClientID string
	TAuthAllowedUsers   []string

	SMTPUsername string
	SMTPPassword string
	SMTPHost     string
	SMTPPort     int
	FromEmail    string

	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string

	// Simplified timeout settings (in seconds)
	ConnectionTimeoutSec int
	OperationTimeoutSec  int
}

type fileConfig struct {
	Server  serverSection `yaml:"server"`
	Web     webSection    `yaml:"web"`
	Tenants tenantConfig  `yaml:"tenants"`
}

type serverSection struct {
	DatabasePath        string       `yaml:"databasePath"`
	GRPCAuthToken       string       `yaml:"grpcAuthToken"`
	LogLevel            string       `yaml:"logLevel"`
	MaxRetries          int          `yaml:"maxRetries"`
	RetryIntervalSec    int          `yaml:"retryIntervalSec"`
	MasterEncryptionKey string       `yaml:"masterEncryptionKey"`
	ConnectionTimeout   int          `yaml:"connectionTimeoutSec"`
	OperationTimeout    int          `yaml:"operationTimeoutSec"`
	TAuth               tauthSection `yaml:"tauth"`
}

type webSection struct {
	Enabled        *bool    `yaml:"enabled"`
	ListenAddr     string   `yaml:"listenAddr"`
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

type tauthSection struct {
	SigningKey     string     `yaml:"signingKey"`
	CookieName     string     `yaml:"cookieName"`
	GoogleClientID string     `yaml:"googleClientId"`
	TAuthBaseURL   string     `yaml:"tauthBaseUrl"`
	TAuthTenantID  string     `yaml:"tauthTenantId"`
	AllowedUsers   stringList `yaml:"allowedUsers"`
}

type stringList []string

func (values *stringList) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*values = nil
		return nil
	}
	switch value.Kind {
	case yaml.SequenceNode:
		var decoded []string
		for _, entry := range value.Content {
			if entry == nil {
				continue
			}
			if entry.Kind != yaml.ScalarNode {
				return fmt.Errorf("configuration: list entries must be strings")
			}
			decoded = append(decoded, strings.TrimSpace(entry.Value))
		}
		*values = decoded
		return nil
	case yaml.ScalarNode:
		*values = stringList{strings.TrimSpace(value.Value)}
		return nil
	default:
		return fmt.Errorf("configuration: list must be a sequence or string")
	}
}

type tenantConfig struct {
	ConfigPath string
	Tenants    []tenant.BootstrapTenant
}

func (cfg *tenantConfig) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*cfg = tenantConfig{}
		return nil
	}

	switch value.Kind {
	case yaml.SequenceNode:
		var tenants []tenant.BootstrapTenant
		if err := value.Decode(&tenants); err != nil {
			return fmt.Errorf("configuration: parse tenants: %w", err)
		}
		cfg.ConfigPath = ""
		cfg.Tenants = tenants
		return nil
	case yaml.MappingNode:
		var decoded struct {
			ConfigPath string                   `yaml:"configPath"`
			Tenants    []tenant.BootstrapTenant `yaml:"tenants"`
			Items      []tenant.BootstrapTenant `yaml:"items"`
		}
		if err := value.Decode(&decoded); err != nil {
			return fmt.Errorf("configuration: parse tenants: %w", err)
		}
		cfg.ConfigPath = strings.TrimSpace(decoded.ConfigPath)
		if len(decoded.Items) > 0 {
			cfg.Tenants = decoded.Items
			return nil
		}
		cfg.Tenants = decoded.Tenants
		return nil
	default:
		return fmt.Errorf("configuration: tenants must be a list")
	}
}

// LoadConfig reads the YAML config file (with environment expansion) into Config.
func LoadConfig(disableWebInterface bool) (Config, error) {
	configPath := strings.TrimSpace(os.Getenv("PINGUIN_CONFIG_PATH"))
	if configPath == "" {
		configPath = defaultConfigPath
	}

	rawContents, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("configuration: read %s: %w", configPath, err)
	}
	expanded := os.ExpandEnv(string(rawContents))

	var fileCfg fileConfig
	if err := yaml.Unmarshal([]byte(expanded), &fileCfg); err != nil {
		return Config{}, fmt.Errorf("configuration: parse yaml: %w", err)
	}

	webEnabled := true
	if fileCfg.Web.Enabled != nil {
		webEnabled = *fileCfg.Web.Enabled
	}
	if disableWebInterface || parseDisabledEnv("DISABLE_WEB_INTERFACE") {
		webEnabled = false
	}

	configuration := Config{
		DatabasePath:         strings.TrimSpace(fileCfg.Server.DatabasePath),
		GRPCAuthToken:        strings.TrimSpace(fileCfg.Server.GRPCAuthToken),
		LogLevel:             strings.TrimSpace(fileCfg.Server.LogLevel),
		MaxRetries:           fileCfg.Server.MaxRetries,
		RetryIntervalSec:     fileCfg.Server.RetryIntervalSec,
		MasterEncryptionKey:  strings.TrimSpace(fileCfg.Server.MasterEncryptionKey),
		TenantConfigPath:     strings.TrimSpace(fileCfg.Tenants.ConfigPath),
		WebInterfaceEnabled:  webEnabled,
		HTTPListenAddr:       strings.TrimSpace(fileCfg.Web.ListenAddr),
		HTTPAllowedOrigins:   normalizeStrings(fileCfg.Web.AllowedOrigins),
		TAuthSigningKey:      strings.TrimSpace(fileCfg.Server.TAuth.SigningKey),
		TAuthCookieName:      strings.TrimSpace(fileCfg.Server.TAuth.CookieName),
		TAuthBaseURL:         strings.TrimSpace(fileCfg.Server.TAuth.TAuthBaseURL),
		TAuthTenantID:        strings.TrimSpace(fileCfg.Server.TAuth.TAuthTenantID),
		TAuthGoogleClientID:  strings.TrimSpace(fileCfg.Server.TAuth.GoogleClientID),
		TAuthAllowedUsers:    normalizeEmails(fileCfg.Server.TAuth.AllowedUsers),
		ConnectionTimeoutSec: fileCfg.Server.ConnectionTimeout,
		OperationTimeoutSec:  fileCfg.Server.OperationTimeout,
		TenantBootstrap: tenant.BootstrapConfig{
			Tenants: fileCfg.Tenants.Tenants,
		},
	}

	if configuration.WebInterfaceEnabled {
		if configuration.TAuthCookieName == "" {
			configuration.TAuthCookieName = "app_session"
		}
	} else {
		configuration.HTTPAllowedOrigins = nil
		configuration.TAuthSigningKey = ""
		configuration.TAuthCookieName = ""
		configuration.TAuthBaseURL = ""
		configuration.TAuthTenantID = ""
		configuration.TAuthGoogleClientID = ""
		configuration.TAuthAllowedUsers = nil
	}

	if err := validateConfig(configuration); err != nil {
		return Config{}, err
	}

	return configuration, nil
}

func (configuration Config) TwilioConfigured() bool {
	return configuration.TwilioAccountSID != "" && configuration.TwilioAuthToken != "" && configuration.TwilioFromNumber != ""
}

func parseDisabledEnv(environmentKey string) bool {
	rawValue := strings.TrimSpace(os.Getenv(environmentKey))
	if rawValue == "" {
		return false
	}
	switch strings.ToLower(rawValue) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeStrings(values []string) []string {
	var normalized []string
	for _, value := range values {
		candidate := strings.TrimSpace(value)
		if candidate == "" {
			continue
		}
		normalized = append(normalized, candidate)
	}
	return normalized
}

func normalizeEmails(values []string) []string {
	var normalized []string
	for _, value := range values {
		for _, entry := range strings.Split(value, ",") {
			candidate := strings.ToLower(strings.TrimSpace(entry))
			if candidate == "" {
				continue
			}
			normalized = append(normalized, candidate)
		}
	}
	return normalized
}

func validateConfig(cfg Config) error {
	var errors []string
	requireString(cfg.DatabasePath, "server.databasePath", &errors)
	requireString(cfg.GRPCAuthToken, "server.grpcAuthToken", &errors)
	requireString(cfg.LogLevel, "server.logLevel", &errors)
	requirePositive(cfg.MaxRetries, "server.maxRetries", &errors)
	requirePositive(cfg.RetryIntervalSec, "server.retryIntervalSec", &errors)
	requireString(cfg.MasterEncryptionKey, "server.masterEncryptionKey", &errors)
	if len(cfg.TenantBootstrap.Tenants) == 0 {
		requireString(cfg.TenantConfigPath, "tenants.configPath", &errors)
	}
	requirePositive(cfg.ConnectionTimeoutSec, "server.connectionTimeoutSec", &errors)
	requirePositive(cfg.OperationTimeoutSec, "server.operationTimeoutSec", &errors)

	if cfg.WebInterfaceEnabled {
		requireString(cfg.HTTPListenAddr, "web.listenAddr", &errors)
		requireString(cfg.TAuthSigningKey, "server.tauth.signingKey", &errors)
		requireString(cfg.TAuthBaseURL, "server.tauth.tauthBaseUrl", &errors)
		requireString(cfg.TAuthTenantID, "server.tauth.tauthTenantId", &errors)
		requireString(cfg.TAuthGoogleClientID, "server.tauth.googleClientId", &errors)
		if len(cfg.TAuthAllowedUsers) == 0 {
			errors = append(errors, "missing server.tauth.allowedUsers")
		}
	}

	if len(cfg.TenantBootstrap.Tenants) > 0 {
		for idx, tenantSpec := range cfg.TenantBootstrap.Tenants {
			tenantPrefix := fmt.Sprintf("tenants[%d]", idx)
			requireString(strings.TrimSpace(tenantSpec.DisplayName), tenantPrefix+".displayName", &errors)
			if countNonEmptyStrings(tenantSpec.Domains) == 0 {
				errors = append(errors, fmt.Sprintf("missing %s.domains", tenantPrefix))
			}
			if cfg.WebInterfaceEnabled && strings.TrimSpace(tenantSpec.Identity.ViewScope) != "" {
				if _, err := tenant.ParseViewScope(tenantSpec.Identity.ViewScope); err != nil {
					errors = append(errors, fmt.Sprintf("invalid %s.identity.viewScope", tenantPrefix))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration errors: %s", strings.Join(errors, ", "))
	}
	return nil
}

func requireString(value string, name string, errors *[]string) {
	if strings.TrimSpace(value) == "" {
		*errors = append(*errors, fmt.Sprintf("missing %s", name))
	}
}

func requirePositive(value int, name string, errors *[]string) {
	if value <= 0 {
		*errors = append(*errors, fmt.Sprintf("missing %s", name))
	}
}

func countNonEmptyStrings(values []string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		count++
	}
	return count
}
