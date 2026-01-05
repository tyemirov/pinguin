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
	Server  pinguinServer   `yaml:"server"`
	Web     pinguinWeb      `yaml:"web"`
	Tenants pinguinYAMLNode `yaml:"tenants"`
}

type pinguinServer struct {
	DatabasePath        string `yaml:"databasePath"`
	GRPCAuthToken       string `yaml:"grpcAuthToken"`
	LogLevel            string `yaml:"logLevel"`
	MaxRetries          int    `yaml:"maxRetries"`
	RetryIntervalSec    int    `yaml:"retryIntervalSec"`
	MasterEncryptionKey string `yaml:"masterEncryptionKey"`
	ConnectionTimeout   int    `yaml:"connectionTimeoutSec"`
	OperationTimeout    int    `yaml:"operationTimeoutSec"`
}

type pinguinWeb struct {
	Enabled        *bool        `yaml:"enabled"`
	ListenAddr     string       `yaml:"listenAddr"`
	AllowedOrigins []string     `yaml:"allowedOrigins"`
	TAuth          pinguinTAuth `yaml:"tauth"`
}

type pinguinTAuth struct {
	SigningKey string `yaml:"signingKey"`
	Issuer     string `yaml:"issuer"`
	CookieName string `yaml:"cookieName"`
}

type pinguinTenant struct {
	ID          string          `yaml:"id"`
	DisplayName string          `yaml:"displayName"`
	Domains     []string        `yaml:"domains"`
	Identity    pinguinIdentity `yaml:"identity"`
	Admins      []string        `yaml:"admins"`
}

type pinguinIdentity struct {
	GoogleClientID string `yaml:"googleClientId"`
	TAuthBaseURL   string `yaml:"tauthBaseUrl"`
}

type pinguinYAMLNode struct {
	ConfigPath string          `yaml:"configPath"`
	Tenants    []pinguinTenant `yaml:"tenants"`
	Items      []pinguinTenant `yaml:"items"`
	Raw        *yaml.Node      `yaml:"-"`
}

func (n *pinguinYAMLNode) UnmarshalYAML(value *yaml.Node) error {
	n.Raw = value
	switch value.Kind {
	case yaml.SequenceNode:
		var tenants []pinguinTenant
		if err := value.Decode(&tenants); err != nil {
			return err
		}
		n.Tenants = tenants
		return nil
	case yaml.MappingNode:
		type decoded struct {
			ConfigPath string          `yaml:"configPath"`
			Tenants    []pinguinTenant `yaml:"tenants"`
			Items      []pinguinTenant `yaml:"items"`
		}
		var d decoded
		if err := value.Decode(&d); err != nil {
			return err
		}
		n.ConfigPath = d.ConfigPath
		n.Tenants = d.Tenants
		n.Items = d.Items
		return nil
	default:
		return nil
	}
}

func (n *pinguinYAMLNode) AllTenants() []pinguinTenant {
	if len(n.Items) > 0 {
		return n.Items
	}
	return n.Tenants
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
		contents = os.ExpandEnv(contents)
	}

	var config pinguinConfig
	if decodeErr := yaml.Unmarshal([]byte(contents), &config); decodeErr != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("parse_yaml: %v", decodeErr))
		return result, nil
	}

	validateServerConfig(config.Server, &result)

	webEnabled := true
	if config.Web.Enabled != nil {
		webEnabled = *config.Web.Enabled
	}

	if webEnabled {
		validateWebConfig(config.Web, &result)
	}

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

func validateServerConfig(server pinguinServer, result *DiagnosticResult) {
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
}

func validateWebConfig(web pinguinWeb, result *DiagnosticResult) {
	if strings.TrimSpace(web.ListenAddr) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "web.listenAddr is required when web is enabled")
	}
	if strings.TrimSpace(web.TAuth.SigningKey) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "web.tauth.signingKey is required when web is enabled")
	}
	if strings.TrimSpace(web.TAuth.Issuer) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "web.tauth.issuer is required when web is enabled")
	}
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
		if strings.TrimSpace(tenant.Identity.GoogleClientID) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("tenant[%s]: identity.googleClientId is required when web is enabled", tenantLabel))
		}
		if strings.TrimSpace(tenant.Identity.TAuthBaseURL) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("tenant[%s]: identity.tauthBaseUrl is required when web is enabled", tenantLabel))
		}
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
	googleClientIDByTenant := make(map[string][]tenantLocation)

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

			googleClientID := strings.TrimSpace(tenant.Identity.GoogleClientID)
			if googleClientID != "" {
				googleClientIDByTenant[googleClientID] = append(googleClientIDByTenant[googleClientID], location)
			}
		}
	}

	for clientID, locations := range googleClientIDByTenant {
		if len(locations) > 1 {
			configPaths := make([]string, 0, len(locations))
			for _, loc := range locations {
				configPaths = append(configPaths, fmt.Sprintf("tenant[%s] in %s", loc.TenantID, loc.ConfigPath))
			}
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("googleClientId %q shared by: %s", clientID, strings.Join(configPaths, ", ")))
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
