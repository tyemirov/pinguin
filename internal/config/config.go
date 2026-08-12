package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "configs/config.yml"

var defaultConfigPaths = []string{
	defaultConfigPath,
	"/config/config.yml",
}

type Config struct {
	DatabasePath     string
	LogLevel         string
	MaxRetries       int
	RetryIntervalSec int

	MasterEncryptionKey string

	WebInterfaceEnabled bool
	HTTPListenAddr      string
	HTTPAllowedOrigins  []string
	HTTPTrustedProxies  []string
	SMTPSubmission      SMTPSubmissionConfig
	SMTPForwarding      SMTPForwardingConfig

	TAuthSigningKey string
	TAuthCookieName string

	// Simplified timeout settings (in seconds)
	ConnectionTimeoutSec int
	OperationTimeoutSec  int
}

// SMTPSubmissionConfig controls Gmail-facing SMTP submission listeners.
type SMTPSubmissionConfig struct {
	Enabled            bool
	Hostname           string
	ListenAddr         string
	TLSListenAddr      string
	TLSCertPath        string
	TLSKeyPath         string
	PublicPort         int
	PublicSecurityMode string
	DeliveryMode       string
	MaxMessageBytes    int64
	MaxRecipients      int
	AllowInsecureAuth  bool
	Relay              SMTPSubmissionRelayConfig
}

// SMTPSubmissionRelayConfig controls the upstream relay used by SMTP submission.
type SMTPSubmissionRelayConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

// SMTPForwardingConfig controls inbound SMTP fanout forwarding.
type SMTPForwardingConfig struct {
	Enabled         bool
	Hostname        string
	ListenAddr      string
	MaxMessageBytes int64
	MaxRecipients   int
	Relay           SMTPForwardingRelayConfig
}

// SMTPForwardingRelayConfig controls the relay used for inbound forwarded copies.
type SMTPForwardingRelayConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

type fileConfig struct {
	Server         serverSection         `yaml:"server"`
	Web            webSection            `yaml:"web"`
	SMTPSubmission smtpSubmissionSection `yaml:"smtpSubmission"`
	SMTPForwarding smtpForwardingSection `yaml:"smtpForwarding"`
}

type serverSection struct {
	DatabasePath        string       `yaml:"databasePath"`
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
	TrustedProxies []string `yaml:"trustedProxies"`
}

type tauthSection struct {
	SigningKey string `yaml:"signingKey"`
	CookieName string `yaml:"cookieName"`
}

type smtpSubmissionSection struct {
	Enabled            bool                       `yaml:"enabled"`
	Hostname           string                     `yaml:"hostname"`
	ListenAddr         string                     `yaml:"listenAddr"`
	TLSListenAddr      string                     `yaml:"tlsListenAddr"`
	TLSCertPath        string                     `yaml:"tlsCertPath"`
	TLSKeyPath         string                     `yaml:"tlsKeyPath"`
	PublicPort         int                        `yaml:"publicPort"`
	PublicSecurityMode string                     `yaml:"publicSecurityMode"`
	DeliveryMode       string                     `yaml:"deliveryMode"`
	MaxMessageBytes    int64                      `yaml:"maxMessageBytes"`
	MaxRecipients      int                        `yaml:"maxRecipients"`
	AllowInsecureAuth  bool                       `yaml:"allowInsecureAuth"`
	Relay              smtpSubmissionRelaySection `yaml:"relay"`
}

type smtpSubmissionRelaySection struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type smtpForwardingSection struct {
	Enabled         bool                       `yaml:"enabled"`
	Hostname        string                     `yaml:"hostname"`
	ListenAddr      string                     `yaml:"listenAddr"`
	MaxMessageBytes int64                      `yaml:"maxMessageBytes"`
	MaxRecipients   int                        `yaml:"maxRecipients"`
	Relay           smtpForwardingRelaySection `yaml:"relay"`
}

type smtpForwardingRelaySection struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// LoadConfig reads the YAML config file (with environment expansion) into Config.
func LoadConfig() (Config, error) {
	return loadConfigFromPath(defaultConfigFilePath())
}

func defaultConfigFilePath() string {
	for _, configPath := range defaultConfigPaths {
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}
	return defaultConfigPath
}

func loadConfigFromPath(configPath string) (Config, error) {
	rawContents, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("configuration: read %s: %w", configPath, err)
	}
	expanded, expandErr := ExpandConfigEnvironment(string(rawContents))
	if expandErr != nil {
		return Config{}, expandErr
	}

	var fileCfg fileConfig
	decoder := yaml.NewDecoder(strings.NewReader(expanded))
	decoder.KnownFields(true)
	if err := decoder.Decode(&fileCfg); err != nil {
		return Config{}, fmt.Errorf("configuration: parse yaml: %w", err)
	}

	webEnabled := true
	if fileCfg.Web.Enabled != nil {
		webEnabled = *fileCfg.Web.Enabled
	}
	configuration := Config{
		DatabasePath:        strings.TrimSpace(fileCfg.Server.DatabasePath),
		LogLevel:            strings.TrimSpace(fileCfg.Server.LogLevel),
		MaxRetries:          fileCfg.Server.MaxRetries,
		RetryIntervalSec:    fileCfg.Server.RetryIntervalSec,
		MasterEncryptionKey: strings.TrimSpace(fileCfg.Server.MasterEncryptionKey),
		WebInterfaceEnabled: webEnabled,
		HTTPListenAddr:      strings.TrimSpace(fileCfg.Web.ListenAddr),
		HTTPAllowedOrigins:  normalizeStrings(fileCfg.Web.AllowedOrigins),
		HTTPTrustedProxies:  normalizeStrings(fileCfg.Web.TrustedProxies),
		SMTPSubmission: SMTPSubmissionConfig{
			Enabled:            fileCfg.SMTPSubmission.Enabled,
			Hostname:           strings.TrimSpace(fileCfg.SMTPSubmission.Hostname),
			ListenAddr:         strings.TrimSpace(fileCfg.SMTPSubmission.ListenAddr),
			TLSListenAddr:      strings.TrimSpace(fileCfg.SMTPSubmission.TLSListenAddr),
			TLSCertPath:        strings.TrimSpace(fileCfg.SMTPSubmission.TLSCertPath),
			TLSKeyPath:         strings.TrimSpace(fileCfg.SMTPSubmission.TLSKeyPath),
			PublicPort:         fileCfg.SMTPSubmission.PublicPort,
			PublicSecurityMode: strings.ToLower(strings.TrimSpace(fileCfg.SMTPSubmission.PublicSecurityMode)),
			DeliveryMode:       normalizeSMTPDeliveryMode(fileCfg.SMTPSubmission.DeliveryMode),
			MaxMessageBytes:    fileCfg.SMTPSubmission.MaxMessageBytes,
			MaxRecipients:      fileCfg.SMTPSubmission.MaxRecipients,
			AllowInsecureAuth:  fileCfg.SMTPSubmission.AllowInsecureAuth,
			Relay: SMTPSubmissionRelayConfig{
				Host:     strings.TrimSpace(fileCfg.SMTPSubmission.Relay.Host),
				Port:     fileCfg.SMTPSubmission.Relay.Port,
				Username: strings.TrimSpace(fileCfg.SMTPSubmission.Relay.Username),
				Password: strings.TrimSpace(fileCfg.SMTPSubmission.Relay.Password),
			},
		},
		SMTPForwarding: SMTPForwardingConfig{
			Enabled:         fileCfg.SMTPForwarding.Enabled,
			Hostname:        strings.TrimSpace(fileCfg.SMTPForwarding.Hostname),
			ListenAddr:      strings.TrimSpace(fileCfg.SMTPForwarding.ListenAddr),
			MaxMessageBytes: fileCfg.SMTPForwarding.MaxMessageBytes,
			MaxRecipients:   fileCfg.SMTPForwarding.MaxRecipients,
			Relay: SMTPForwardingRelayConfig{
				Host:     strings.TrimSpace(fileCfg.SMTPForwarding.Relay.Host),
				Port:     fileCfg.SMTPForwarding.Relay.Port,
				Username: strings.TrimSpace(fileCfg.SMTPForwarding.Relay.Username),
				Password: strings.TrimSpace(fileCfg.SMTPForwarding.Relay.Password),
			},
		},
		TAuthSigningKey:      strings.TrimSpace(fileCfg.Server.TAuth.SigningKey),
		TAuthCookieName:      strings.TrimSpace(fileCfg.Server.TAuth.CookieName),
		ConnectionTimeoutSec: fileCfg.Server.ConnectionTimeout,
		OperationTimeoutSec:  fileCfg.Server.OperationTimeout,
	}

	if configuration.WebInterfaceEnabled {
		if configuration.TAuthCookieName == "" {
			configuration.TAuthCookieName = "app_session"
		}
	} else {
		configuration.HTTPAllowedOrigins = nil
		configuration.HTTPTrustedProxies = nil
		configuration.TAuthSigningKey = ""
		configuration.TAuthCookieName = ""
	}

	if err := validateConfig(configuration); err != nil {
		return Config{}, err
	}

	return configuration, nil
}

// ExpandConfigEnvironment expands shell-style placeholders and rejects absent variables.
func ExpandConfigEnvironment(contents string) (string, error) {
	var missing []string
	seenMissing := make(map[string]bool)
	expanded := os.Expand(contents, func(key string) string {
		value, found := os.LookupEnv(key)
		if !found {
			if !seenMissing[key] {
				seenMissing[key] = true
				missing = append(missing, key)
			}
			return ""
		}
		return value
	})
	if len(missing) > 0 {
		sort.Strings(missing)
		return "", fmt.Errorf("configuration: missing environment variables: %s", strings.Join(missing, ", "))
	}
	return expanded, nil
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
	requireString(cfg.LogLevel, "server.logLevel", &errors)
	requirePositive(cfg.MaxRetries, "server.maxRetries", &errors)
	requirePositive(cfg.RetryIntervalSec, "server.retryIntervalSec", &errors)
	requireString(cfg.MasterEncryptionKey, "server.masterEncryptionKey", &errors)
	requirePositive(cfg.ConnectionTimeoutSec, "server.connectionTimeoutSec", &errors)
	requirePositive(cfg.OperationTimeoutSec, "server.operationTimeoutSec", &errors)

	if cfg.WebInterfaceEnabled {
		requireString(cfg.HTTPListenAddr, "web.listenAddr", &errors)
		requireString(cfg.TAuthSigningKey, "server.tauth.signingKey", &errors)
	}

	if cfg.SMTPSubmission.Enabled {
		requireString(cfg.SMTPSubmission.Hostname, "smtpSubmission.hostname", &errors)
		if strings.TrimSpace(cfg.SMTPSubmission.ListenAddr) == "" && strings.TrimSpace(cfg.SMTPSubmission.TLSListenAddr) == "" {
			errors = append(errors, "missing smtpSubmission.listenAddr or smtpSubmission.tlsListenAddr")
		}
		requirePositiveInt64(cfg.SMTPSubmission.MaxMessageBytes, "smtpSubmission.maxMessageBytes", &errors)
		requirePositive(cfg.SMTPSubmission.MaxRecipients, "smtpSubmission.maxRecipients", &errors)
		deliveryMode := normalizeSMTPDeliveryMode(cfg.SMTPSubmission.DeliveryMode)
		switch deliveryMode {
		case "upstream":
			requireString(cfg.SMTPSubmission.Relay.Host, "smtpSubmission.relay.host", &errors)
			requirePositive(cfg.SMTPSubmission.Relay.Port, "smtpSubmission.relay.port", &errors)
			requireString(cfg.SMTPSubmission.Relay.Username, "smtpSubmission.relay.username", &errors)
			requireString(cfg.SMTPSubmission.Relay.Password, "smtpSubmission.relay.password", &errors)
		case "direct":
		default:
			errors = append(errors, "smtpSubmission.deliveryMode must be upstream or direct")
		}
		if cfg.SMTPSubmission.PublicPort < 0 {
			errors = append(errors, "smtpSubmission.publicPort must be positive")
		}
		if cfg.SMTPSubmission.PublicSecurityMode != "" && cfg.SMTPSubmission.PublicSecurityMode != "starttls" && cfg.SMTPSubmission.PublicSecurityMode != "ssl" {
			errors = append(errors, "smtpSubmission.publicSecurityMode must be starttls or ssl")
		}
		if !cfg.SMTPSubmission.AllowInsecureAuth {
			requireString(cfg.SMTPSubmission.TLSCertPath, "smtpSubmission.tlsCertPath", &errors)
			requireString(cfg.SMTPSubmission.TLSKeyPath, "smtpSubmission.tlsKeyPath", &errors)
		}
	}

	if cfg.SMTPForwarding.Enabled {
		requireString(cfg.SMTPForwarding.Hostname, "smtpForwarding.hostname", &errors)
		requireString(cfg.SMTPForwarding.ListenAddr, "smtpForwarding.listenAddr", &errors)
		requirePositiveInt64(cfg.SMTPForwarding.MaxMessageBytes, "smtpForwarding.maxMessageBytes", &errors)
		requirePositive(cfg.SMTPForwarding.MaxRecipients, "smtpForwarding.maxRecipients", &errors)
		requireString(cfg.SMTPForwarding.Relay.Host, "smtpForwarding.relay.host", &errors)
		requirePositive(cfg.SMTPForwarding.Relay.Port, "smtpForwarding.relay.port", &errors)
		requireString(cfg.SMTPForwarding.Relay.Username, "smtpForwarding.relay.username", &errors)
		requireString(cfg.SMTPForwarding.Relay.Password, "smtpForwarding.relay.password", &errors)
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration errors: %s", strings.Join(errors, ", "))
	}
	return nil
}

func normalizeSMTPDeliveryMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "upstream"
	}
	return normalized
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

func requirePositiveInt64(value int64, name string, errors *[]string) {
	if value <= 0 {
		*errors = append(*errors, fmt.Sprintf("missing %s", name))
	}
}
