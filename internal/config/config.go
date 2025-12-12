package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/tyemirov/pinguin/internal/tenant"
	"gopkg.in/yaml.v3"
)

const defaultHTTPStaticRoot = "/web"
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
	HTTPStaticRoot      string
	HTTPAllowedOrigins  []string

	TAuthSigningKey string
	TAuthIssuer     string
	TAuthCookieName string

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
	Tenants tenantSection `yaml:"tenants"`
}

type serverSection struct {
	DatabasePath        string `yaml:"databasePath"`
	GRPCAuthToken       string `yaml:"grpcAuthToken"`
	LogLevel            string `yaml:"logLevel"`
	MaxRetries          int    `yaml:"maxRetries"`
	RetryIntervalSec    int    `yaml:"retryIntervalSec"`
	MasterEncryptionKey string `yaml:"masterEncryptionKey"`
	ConnectionTimeout   int    `yaml:"connectionTimeoutSec"`
	OperationTimeout    int    `yaml:"operationTimeoutSec"`
}

type webSection struct {
	Enabled        *bool        `yaml:"enabled"`
	ListenAddr     string       `yaml:"listenAddr"`
	StaticRoot     string       `yaml:"staticRoot"`
	AllowedOrigins []string     `yaml:"allowedOrigins"`
	TAuth          tauthSection `yaml:"tauth"`
}

type tauthSection struct {
	SigningKey string `yaml:"signingKey"`
	Issuer     string `yaml:"issuer"`
	CookieName string `yaml:"cookieName"`
}

type tenantSection struct {
	ConfigPath string                   `yaml:"configPath"`
	Tenants    []tenant.BootstrapTenant `yaml:"tenants"`
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
		HTTPStaticRoot:       strings.TrimSpace(fileCfg.Web.StaticRoot),
		HTTPAllowedOrigins:   normalizeStrings(fileCfg.Web.AllowedOrigins),
		TAuthSigningKey:      strings.TrimSpace(fileCfg.Web.TAuth.SigningKey),
		TAuthIssuer:          strings.TrimSpace(fileCfg.Web.TAuth.Issuer),
		TAuthCookieName:      strings.TrimSpace(fileCfg.Web.TAuth.CookieName),
		ConnectionTimeoutSec: fileCfg.Server.ConnectionTimeout,
		OperationTimeoutSec:  fileCfg.Server.OperationTimeout,
		TenantBootstrap: tenant.BootstrapConfig{
			Tenants: fileCfg.Tenants.Tenants,
		},
	}

	if configuration.WebInterfaceEnabled {
		if configuration.HTTPStaticRoot == "" {
			configuration.HTTPStaticRoot = defaultHTTPStaticRoot
		}
		if configuration.TAuthCookieName == "" {
			configuration.TAuthCookieName = "app_session"
		}
	} else {
		configuration.HTTPStaticRoot = ""
		configuration.HTTPAllowedOrigins = nil
		configuration.TAuthSigningKey = ""
		configuration.TAuthIssuer = ""
		configuration.TAuthCookieName = ""
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
		requireString(cfg.TAuthSigningKey, "web.tauth.signingKey", &errors)
		requireString(cfg.TAuthIssuer, "web.tauth.issuer", &errors)
	}

	if len(cfg.TenantBootstrap.Tenants) > 0 {
		for idx, tenantSpec := range cfg.TenantBootstrap.Tenants {
			tenantPrefix := fmt.Sprintf("tenants.tenants[%d]", idx)
			requireString(strings.TrimSpace(tenantSpec.Slug), tenantPrefix+".slug", &errors)
			requireString(strings.TrimSpace(tenantSpec.DisplayName), tenantPrefix+".displayName", &errors)
			if countNonEmptyStrings(tenantSpec.Domains) == 0 {
				errors = append(errors, fmt.Sprintf("missing %s.domains", tenantPrefix))
			}
			if cfg.WebInterfaceEnabled {
				requireString(strings.TrimSpace(tenantSpec.Identity.GoogleClientID), tenantPrefix+".identity.googleClientId", &errors)
				requireString(strings.TrimSpace(tenantSpec.Identity.TAuthBaseURL), tenantPrefix+".identity.tauthBaseUrl", &errors)
				if countNonEmptyAdminEmails(tenantSpec.Admins) == 0 {
					errors = append(errors, fmt.Sprintf("missing %s.admins", tenantPrefix))
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

func countNonEmptyAdminEmails(values tenant.BootstrapAdmins) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		count++
	}
	return count
}
