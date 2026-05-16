// Package doctor provides validation utilities for Pinguin configurations across projects.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	runtimeconfig "github.com/tyemirov/pinguin/internal/config"
	"gopkg.in/yaml.v3"
)

const reportSchemaVersion = "pinguin.doctor.v1"

var errDoctor = errors.New("doctor.invalid")

// DiagnosticResult represents the outcome of validating a single configuration.
type DiagnosticResult struct {
	ConfigPath string   `json:"config_path"`
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	TenantIDs  []string `json:"tenant_ids,omitempty"`
}

// Report represents the complete doctor report for all validated configurations.
type Report struct {
	SchemaVersion   string             `json:"schema_version"`
	Timestamp       string             `json:"timestamp"`
	Service         serviceInfo        `json:"service"`
	Summary         reportSummary      `json:"summary"`
	Diagnostics     []DiagnosticResult `json:"diagnostics"`
	CrossValidation crossValidation    `json:"cross_validation,omitempty"`
}

type serviceInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type reportSummary struct {
	TotalConfigs   int `json:"total_configs"`
	ValidConfigs   int `json:"valid_configs"`
	InvalidConfigs int `json:"invalid_configs"`
	TotalErrors    int `json:"total_errors"`
	TotalWarnings  int `json:"total_warnings"`
}

type crossValidation struct {
	Performed bool     `json:"performed"`
	Errors    []string `json:"errors,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Options configures the doctor behavior.
type Options struct {
	ConfigPaths          []string
	ValidateCrossConfigs bool
	ExpandEnv            bool
}

// pinguinConfig mirrors the Pinguin configuration file structure for validation.
type pinguinConfig struct {
	Server         pinguinServer         `yaml:"server"`
	Web            pinguinWeb            `yaml:"web"`
	SMTPSubmission pinguinSMTPSubmission `yaml:"smtpSubmission"`
	SMTPForwarding pinguinSMTPForwarding `yaml:"smtpForwarding"`
	Tenants        pinguinYAMLNode       `yaml:"tenants"`
}

type pinguinServer struct {
	DatabasePath        string       `yaml:"databasePath"`
	GRPCAuthToken       string       `yaml:"grpcAuthToken"`
	LogLevel            string       `yaml:"logLevel"`
	MaxRetries          int          `yaml:"maxRetries"`
	RetryIntervalSec    int          `yaml:"retryIntervalSec"`
	MasterEncryptionKey string       `yaml:"masterEncryptionKey"`
	ConnectionTimeout   int          `yaml:"connectionTimeoutSec"`
	OperationTimeout    int          `yaml:"operationTimeoutSec"`
	TAuth               pinguinTAuth `yaml:"tauth"`
}

type pinguinWeb struct {
	Enabled        *bool    `yaml:"enabled"`
	ListenAddr     string   `yaml:"listenAddr"`
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

type pinguinTAuth struct {
	SigningKey string `yaml:"signingKey"`
	CookieName string `yaml:"cookieName"`
}

type pinguinSMTPSubmission struct {
	Enabled            bool             `yaml:"enabled"`
	Hostname           string           `yaml:"hostname"`
	ListenAddr         string           `yaml:"listenAddr"`
	TLSListenAddr      string           `yaml:"tlsListenAddr"`
	TLSCertPath        string           `yaml:"tlsCertPath"`
	TLSKeyPath         string           `yaml:"tlsKeyPath"`
	PublicPort         int              `yaml:"publicPort"`
	PublicSecurityMode string           `yaml:"publicSecurityMode"`
	DeliveryMode       string           `yaml:"deliveryMode"`
	MaxMessageBytes    int64            `yaml:"maxMessageBytes"`
	MaxRecipients      int              `yaml:"maxRecipients"`
	AllowInsecureAuth  bool             `yaml:"allowInsecureAuth"`
	Relay              pinguinSMTPRelay `yaml:"relay"`
}

type pinguinSMTPRelay struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type pinguinSMTPForwarding struct {
	Enabled         bool             `yaml:"enabled"`
	Hostname        string           `yaml:"hostname"`
	ListenAddr      string           `yaml:"listenAddr"`
	MaxMessageBytes int64            `yaml:"maxMessageBytes"`
	MaxRecipients   int              `yaml:"maxRecipients"`
	Relay           pinguinSMTPRelay `yaml:"relay"`
}

type pinguinTenant struct {
	ID          string   `yaml:"id"`
	DisplayName string   `yaml:"displayName"`
	Domains     []string `yaml:"domains"`
	Admins      []string `yaml:"admins"`
}

type pinguinYAMLNode struct {
	ConfigPath string          `yaml:"configPath"`
	Tenants    []pinguinTenant `yaml:"tenants"`
	Raw        *yaml.Node      `yaml:"-"`
}

func (node *pinguinYAMLNode) UnmarshalYAML(value *yaml.Node) error {
	node.Raw = value
	switch value.Kind {
	case yaml.SequenceNode:
		var tenants []pinguinTenant
		if decodeErr := value.Decode(&tenants); decodeErr != nil {
			return fmt.Errorf("configuration: parse tenants: %w", decodeErr)
		}
		node.ConfigPath = ""
		node.Tenants = tenants
		return nil
	case yaml.MappingNode:
		if unknownKey := firstUnsupportedTenantMappingKey(value, "configPath", "tenants"); unknownKey != "" {
			return fmt.Errorf("configuration: tenants.%s is not supported", unknownKey)
		}
		type decoded struct {
			ConfigPath string          `yaml:"configPath"`
			Tenants    []pinguinTenant `yaml:"tenants"`
		}
		var decodedConfig decoded
		if decodeErr := value.Decode(&decodedConfig); decodeErr != nil {
			return fmt.Errorf("configuration: parse tenants: %w", decodeErr)
		}
		node.ConfigPath = strings.TrimSpace(decodedConfig.ConfigPath)
		node.Tenants = decodedConfig.Tenants
		return nil
	default:
		return fmt.Errorf("configuration: tenants must be a list")
	}
}

func firstUnsupportedTenantMappingKey(value *yaml.Node, allowedKeys ...string) string {
	allowed := make(map[string]struct{}, len(allowedKeys))
	for _, allowedKey := range allowedKeys {
		allowed[allowedKey] = struct{}{}
	}
	for contentIndex := 0; contentIndex+1 < len(value.Content); contentIndex += 2 {
		key := strings.TrimSpace(value.Content[contentIndex].Value)
		if _, ok := allowed[key]; !ok {
			return key
		}
	}
	return ""
}

func (node *pinguinYAMLNode) AllTenants() []pinguinTenant {
	return node.Tenants
}

// Run executes the doctor validation for the specified configurations.
func Run(_ context.Context, options Options) (*Report, error) {
	if len(options.ConfigPaths) == 0 {
		return nil, fmt.Errorf("%w: no config paths provided", errDoctor)
	}

	report := &Report{
		SchemaVersion: reportSchemaVersion,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Service: serviceInfo{
			Name:    "pinguin",
			Version: "doctor-v1",
		},
		Diagnostics: make([]DiagnosticResult, 0, len(options.ConfigPaths)),
	}

	allConfigsByPath := make(map[string]*pinguinConfig)

	for _, configPath := range options.ConfigPaths {
		diagnostic, config := validateConfig(configPath, options.ExpandEnv)
		report.Diagnostics = append(report.Diagnostics, diagnostic)
		if diagnostic.Valid && config != nil {
			allConfigsByPath[configPath] = config
		}
	}

	report.Summary = buildSummary(report.Diagnostics)

	if options.ValidateCrossConfigs && len(allConfigsByPath) > 1 {
		report.CrossValidation = validateCrossConfigs(allConfigsByPath)
	}

	return report, nil
}

func validateConfig(configPath string, expandEnv bool) (DiagnosticResult, *pinguinConfig) {
	result := DiagnosticResult{
		ConfigPath: configPath,
		Valid:      true,
	}

	rawContents, readErr := os.ReadFile(configPath)
	if readErr != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("read_config: %v", readErr))
		return result, nil
	}

	contents := string(rawContents)
	if expandEnv {
		expandedContents, expandErr := runtimeconfig.ExpandConfigEnvironment(contents)
		if expandErr != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expand_env: %v", expandErr))
			return result, nil
		}
		contents = expandedContents
	}

	var config pinguinConfig
	decoder := yaml.NewDecoder(strings.NewReader(contents))
	decoder.KnownFields(true)
	if decodeErr := decoder.Decode(&config); decodeErr != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("parse_yaml: %v", decodeErr))
		return result, nil
	}

	webEnabled := true
	if config.Web.Enabled != nil {
		webEnabled = *config.Web.Enabled
	}
	validateServerConfig(config.Server, webEnabled, &result)

	if webEnabled {
		validateWebConfig(config.Web, &result)
	}
	validateSMTPSubmissionConfig(config.SMTPSubmission, &result)
	validateSMTPForwardingConfig(config.SMTPForwarding, &result)

	for _, tenant := range config.Tenants.AllTenants() {
		tenantID := strings.TrimSpace(tenant.ID)
		if tenantID != "" {
			result.TenantIDs = append(result.TenantIDs, tenantID)
		}
		validateTenantConfig(tenant, webEnabled, &result)
	}

	sort.Strings(result.Errors)
	sort.Strings(result.Warnings)
	sort.Strings(result.TenantIDs)

	return result, &config
}

func validateServerConfig(server pinguinServer, webEnabled bool, result *DiagnosticResult) {
	if strings.TrimSpace(server.DatabasePath) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "server.databasePath is required")
	}
	if strings.TrimSpace(server.GRPCAuthToken) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "server.grpcAuthToken is required")
	}
	if strings.TrimSpace(server.LogLevel) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "server.logLevel is required")
	}
	if server.MaxRetries <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "server.maxRetries must be positive")
	}
	if server.RetryIntervalSec <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "server.retryIntervalSec must be positive")
	}
	if strings.TrimSpace(server.MasterEncryptionKey) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "server.masterEncryptionKey is required")
	} else if len(server.MasterEncryptionKey) < 32 {
		result.Warnings = append(result.Warnings, "server.masterEncryptionKey should be at least 32 characters")
	}
	if server.ConnectionTimeout <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "server.connectionTimeoutSec must be positive")
	}
	if server.OperationTimeout <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "server.operationTimeoutSec must be positive")
	}
	if webEnabled {
		validateServerTAuthConfig(server.TAuth, result)
	}
}

func validateServerTAuthConfig(tauth pinguinTAuth, result *DiagnosticResult) {
	if strings.TrimSpace(tauth.SigningKey) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "server.tauth.signingKey is required when web is enabled")
	}
}

func validateWebConfig(web pinguinWeb, result *DiagnosticResult) {
	if strings.TrimSpace(web.ListenAddr) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "web.listenAddr is required when web is enabled")
	}
}

func validateSMTPSubmissionConfig(submission pinguinSMTPSubmission, result *DiagnosticResult) {
	if !submission.Enabled {
		return
	}
	if strings.TrimSpace(submission.Hostname) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpSubmission.hostname is required when SMTP submission is enabled")
	}
	if strings.TrimSpace(submission.ListenAddr) == "" && strings.TrimSpace(submission.TLSListenAddr) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpSubmission.listenAddr or smtpSubmission.tlsListenAddr is required when SMTP submission is enabled")
	}
	if submission.MaxMessageBytes <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpSubmission.maxMessageBytes must be positive")
	}
	if submission.MaxRecipients <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpSubmission.maxRecipients must be positive")
	}
	deliveryMode := normalizeSMTPDeliveryMode(submission.DeliveryMode)
	switch deliveryMode {
	case "upstream":
		if strings.TrimSpace(submission.Relay.Host) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "smtpSubmission.relay.host is required when SMTP submission is enabled")
		}
		if submission.Relay.Port <= 0 {
			result.Valid = false
			result.Errors = append(result.Errors, "smtpSubmission.relay.port must be positive")
		}
		if strings.TrimSpace(submission.Relay.Username) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "smtpSubmission.relay.username is required when SMTP submission is enabled")
		}
		if strings.TrimSpace(submission.Relay.Password) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "smtpSubmission.relay.password is required when SMTP submission is enabled")
		}
	case "direct":
	default:
		result.Valid = false
		result.Errors = append(result.Errors, "smtpSubmission.deliveryMode must be upstream or direct")
	}
	if submission.PublicPort < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpSubmission.publicPort must be positive")
	}
	if strings.TrimSpace(submission.PublicSecurityMode) != "" {
		securityMode := strings.ToLower(strings.TrimSpace(submission.PublicSecurityMode))
		if securityMode != "starttls" && securityMode != "ssl" {
			result.Valid = false
			result.Errors = append(result.Errors, "smtpSubmission.publicSecurityMode must be starttls or ssl")
		}
	}
	if !submission.AllowInsecureAuth {
		if strings.TrimSpace(submission.TLSCertPath) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "smtpSubmission.tlsCertPath is required when SMTP submission is enabled")
		}
		if strings.TrimSpace(submission.TLSKeyPath) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "smtpSubmission.tlsKeyPath is required when SMTP submission is enabled")
		}
	}
}

func validateSMTPForwardingConfig(forwarding pinguinSMTPForwarding, result *DiagnosticResult) {
	if !forwarding.Enabled {
		return
	}
	if strings.TrimSpace(forwarding.Hostname) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.hostname is required when SMTP forwarding is enabled")
	}
	if strings.TrimSpace(forwarding.ListenAddr) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.listenAddr is required when SMTP forwarding is enabled")
	}
	if forwarding.MaxMessageBytes <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.maxMessageBytes must be positive")
	}
	if forwarding.MaxRecipients <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.maxRecipients must be positive")
	}
	if strings.TrimSpace(forwarding.Relay.Host) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.relay.host is required when SMTP forwarding is enabled")
	}
	if forwarding.Relay.Port <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.relay.port must be positive")
	}
	if strings.TrimSpace(forwarding.Relay.Username) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.relay.username is required when SMTP forwarding is enabled")
	}
	if strings.TrimSpace(forwarding.Relay.Password) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "smtpForwarding.relay.password is required when SMTP forwarding is enabled")
	}
}

func normalizeSMTPDeliveryMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "upstream"
	}
	return normalized
}

func validateTenantConfig(tenant pinguinTenant, webEnabled bool, result *DiagnosticResult) {
	tenantID := strings.TrimSpace(tenant.ID)
	tenantLabel := tenantID
	if tenantLabel == "" {
		tenantLabel = "(unknown)"
	}

	if strings.TrimSpace(tenant.DisplayName) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("tenant[%s]: displayName is required", tenantLabel))
	}

	validDomains := 0
	for _, domain := range tenant.Domains {
		if strings.TrimSpace(domain) != "" {
			validDomains++
		}
	}
	if validDomains == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("tenant[%s]: at least one domain is required", tenantLabel))
	}

	if webEnabled {
		validAdmins := 0
		for _, admin := range tenant.Admins {
			if strings.TrimSpace(admin) != "" {
				validAdmins++
			}
		}
		if validAdmins == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("tenant[%s]: at least one admin is required when web is enabled", tenantLabel))
		}
	}
}

func validateCrossConfigs(configsByPath map[string]*pinguinConfig) crossValidation {
	validation := crossValidation{
		Performed: true,
	}

	type tenantLocation struct {
		ConfigPath string
		TenantID   string
	}

	domainsByTenant := make(map[string]tenantLocation)

	for configPath, config := range configsByPath {
		for _, tenant := range config.Tenants.AllTenants() {
			tenantID := strings.TrimSpace(tenant.ID)
			location := tenantLocation{
				ConfigPath: configPath,
				TenantID:   tenantID,
			}

			for _, domain := range tenant.Domains {
				normalizedDomain := strings.ToLower(strings.TrimSpace(domain))
				if normalizedDomain == "" {
					continue
				}
				if existing, exists := domainsByTenant[normalizedDomain]; exists {
					if existing.ConfigPath != configPath || existing.TenantID != tenantID {
						validation.Errors = append(validation.Errors,
							fmt.Sprintf("domain %q claimed by tenant[%s] in %s conflicts with tenant[%s] in %s",
								domain, tenantID, configPath, existing.TenantID, existing.ConfigPath))
					}
				} else {
					domainsByTenant[normalizedDomain] = location
				}
			}
		}
	}

	sort.Strings(validation.Errors)
	sort.Strings(validation.Warnings)

	return validation
}

func buildSummary(diagnostics []DiagnosticResult) reportSummary {
	summary := reportSummary{
		TotalConfigs: len(diagnostics),
	}
	for _, diag := range diagnostics {
		if diag.Valid {
			summary.ValidConfigs++
		} else {
			summary.InvalidConfigs++
		}
		summary.TotalErrors += len(diag.Errors)
		summary.TotalWarnings += len(diag.Warnings)
	}
	return summary
}

// FormatReport formats the report as indented JSON.
func FormatReport(report *Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// FormatSummary formats a human-readable summary of the report.
func FormatSummary(report *Report) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Pinguin Doctor Report (%s)\n", report.Timestamp))
	builder.WriteString(strings.Repeat("=", 60))
	builder.WriteString("\n\n")

	builder.WriteString(fmt.Sprintf("Summary: %d/%d configs valid",
		report.Summary.ValidConfigs, report.Summary.TotalConfigs))
	if report.Summary.TotalErrors > 0 {
		builder.WriteString(fmt.Sprintf(", %d errors", report.Summary.TotalErrors))
	}
	if report.Summary.TotalWarnings > 0 {
		builder.WriteString(fmt.Sprintf(", %d warnings", report.Summary.TotalWarnings))
	}
	builder.WriteString("\n\n")

	for _, diag := range report.Diagnostics {
		status := "✓ VALID"
		if !diag.Valid {
			status = "✗ INVALID"
		}
		builder.WriteString(fmt.Sprintf("%s: %s\n", diag.ConfigPath, status))
		if len(diag.TenantIDs) > 0 {
			builder.WriteString(fmt.Sprintf("  Tenants: %s\n", strings.Join(diag.TenantIDs, ", ")))
		}
		for _, err := range diag.Errors {
			builder.WriteString(fmt.Sprintf("  ERROR: %s\n", err))
		}
		for _, warn := range diag.Warnings {
			builder.WriteString(fmt.Sprintf("  WARN: %s\n", warn))
		}
		builder.WriteString("\n")
	}

	if report.CrossValidation.Performed {
		builder.WriteString("Cross-Config Validation:\n")
		if len(report.CrossValidation.Errors) == 0 && len(report.CrossValidation.Warnings) == 0 {
			builder.WriteString("  No cross-config issues detected\n")
		}
		for _, err := range report.CrossValidation.Errors {
			builder.WriteString(fmt.Sprintf("  ERROR: %s\n", err))
		}
		for _, warn := range report.CrossValidation.Warnings {
			builder.WriteString(fmt.Sprintf("  WARN: %s\n", warn))
		}
	}

	return builder.String()
}
