package tenant

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BootstrapConfig defines the YAML layout for tenant provisioning.
type BootstrapConfig struct {
	Tenants []BootstrapTenant `json:"tenants" yaml:"tenants"`
}

func (cfg *BootstrapConfig) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*cfg = BootstrapConfig{}
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("tenant bootstrap: config must be a mapping")
	}
	if unsupportedKey := firstUnsupportedBootstrapYAMLMappingKey(value, "tenants"); unsupportedKey != "" {
		return fmt.Errorf("tenant bootstrap: %s is not supported", unsupportedKey)
	}
	type rawBootstrapConfig BootstrapConfig
	var decoded rawBootstrapConfig
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*cfg = BootstrapConfig(decoded)
	return nil
}

// BootstrapTenant declares per-tenant metadata.
type BootstrapTenant struct {
	ID           string                `json:"id" yaml:"id"`
	DisplayName  string                `json:"displayName" yaml:"displayName"`
	SupportEmail string                `json:"supportEmail" yaml:"supportEmail"`
	Enabled      *bool                 `json:"enabled" yaml:"enabled"`
	Status       string                `json:"status,omitempty" yaml:"status,omitempty"`
	Domains      []string              `json:"domains" yaml:"domains"`
	Admins       []string              `json:"admins" yaml:"admins"`
	EmailProfile BootstrapEmailProfile `json:"emailProfile" yaml:"emailProfile"`
	SMSProfile   *BootstrapSMSProfile  `json:"smsProfile" yaml:"smsProfile"`
}

func (spec *BootstrapTenant) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*spec = BootstrapTenant{}
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("tenant bootstrap: tenants[] must be a mapping")
	}
	if yamlMappingHasKey(value, "status") {
		return fmt.Errorf("tenant bootstrap: tenants[].status is no longer supported; use tenants[].enabled (true|false)")
	}
	if unsupportedKey := firstUnsupportedBootstrapYAMLMappingKey(value, "id", "displayName", "supportEmail", "enabled", "domains", "admins", "emailProfile", "smsProfile"); unsupportedKey != "" {
		return fmt.Errorf("tenant bootstrap: tenants[].%s is not supported", unsupportedKey)
	}
	type rawBootstrapTenant BootstrapTenant
	var decoded rawBootstrapTenant
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*spec = BootstrapTenant(decoded)
	return nil
}

// BootstrapEmailProfile defines SMTP credentials.
type BootstrapEmailProfile struct {
	Host        string `json:"host" yaml:"host"`
	Port        int    `json:"port" yaml:"port"`
	Username    string `json:"username" yaml:"username"`
	Password    string `json:"password" yaml:"password"`
	FromAddress string `json:"fromAddress" yaml:"fromAddress"`
}

func (profile *BootstrapEmailProfile) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*profile = BootstrapEmailProfile{}
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("tenant bootstrap: tenants[].emailProfile must be a mapping")
	}
	if unsupportedKey := firstUnsupportedBootstrapYAMLMappingKey(value, "host", "port", "username", "password", "fromAddress"); unsupportedKey != "" {
		return fmt.Errorf("tenant bootstrap: tenants[].emailProfile.%s is not supported", unsupportedKey)
	}
	type rawBootstrapEmailProfile BootstrapEmailProfile
	var decoded rawBootstrapEmailProfile
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*profile = BootstrapEmailProfile(decoded)
	return nil
}

// BootstrapSMSProfile defines Twilio credentials.
type BootstrapSMSProfile struct {
	AccountSID string `json:"accountSid" yaml:"accountSid"`
	AuthToken  string `json:"authToken" yaml:"authToken"`
	FromNumber string `json:"fromNumber" yaml:"fromNumber"`
}

func (profile *BootstrapSMSProfile) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*profile = BootstrapSMSProfile{}
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("tenant bootstrap: tenants[].smsProfile must be a mapping")
	}
	if unsupportedKey := firstUnsupportedBootstrapYAMLMappingKey(value, "accountSid", "authToken", "fromNumber"); unsupportedKey != "" {
		return fmt.Errorf("tenant bootstrap: tenants[].smsProfile.%s is not supported", unsupportedKey)
	}
	type rawBootstrapSMSProfile BootstrapSMSProfile
	var decoded rawBootstrapSMSProfile
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*profile = BootstrapSMSProfile(decoded)
	return nil
}

func firstUnsupportedBootstrapYAMLMappingKey(value *yaml.Node, allowedKeys ...string) string {
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

func yamlMappingHasKey(value *yaml.Node, key string) bool {
	if value == nil || value.Kind != yaml.MappingNode {
		return false
	}
	for contentIndex := 0; contentIndex+1 < len(value.Content); contentIndex += 2 {
		if strings.TrimSpace(value.Content[contentIndex].Value) == key {
			return true
		}
	}
	return false
}

// BootstrapFromFile loads tenants from a YAML file and upserts them.
func BootstrapFromFile(ctx context.Context, db *gorm.DB, keeper *SecretKeeper, path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("tenant bootstrap: read file: %w", err)
	}
	var cfg BootstrapConfig
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return fmt.Errorf("tenant bootstrap: parse yaml: %w", err)
	}
	return Bootstrap(ctx, db, keeper, cfg)
}

// Bootstrap loads tenants from an in-memory config and upserts them.
func Bootstrap(ctx context.Context, db *gorm.DB, keeper *SecretKeeper, cfg BootstrapConfig) error {
	if len(cfg.Tenants) == 0 {
		return fmt.Errorf("tenant bootstrap: no tenants configured")
	}
	tenantSpecs := prepareBootstrapTenants(cfg.Tenants)
	enabledCount := 0
	for _, tenantSpec := range tenantSpecs {
		if tenantSpec.Enabled != nil && !*tenantSpec.Enabled {
			continue
		}
		enabledCount++
	}
	if enabledCount == 0 {
		return fmt.Errorf("tenant bootstrap: no enabled tenants configured")
	}
	if err := validateBootstrapDomains(tenantSpecs); err != nil {
		return err
	}
	configuredTenantIDs := bootstrapTenantIDs(tenantSpecs)
	transactionErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := resetTenantDomains(tx); err != nil {
			return err
		}
		if err := resetTenantAdmins(tx); err != nil {
			return err
		}
		if err := resetTenantEmailProfiles(tx); err != nil {
			return err
		}
		if err := resetTenantSMSProfiles(tx); err != nil {
			return err
		}
		if err := removeStaleTenants(tx, configuredTenantIDs); err != nil {
			return err
		}
		for _, tenantSpec := range tenantSpecs {
			if err := upsertTenant(ctx, tx, keeper, tenantSpec); err != nil {
				return err
			}
		}
		return nil
	})
	if transactionErr != nil {
		return transactionErr
	}
	invalidateRegisteredRepositories()
	return nil
}

func upsertTenant(ctx context.Context, tx *gorm.DB, keeper *SecretKeeper, spec BootstrapTenant) error {
	if strings.TrimSpace(spec.Status) != "" {
		return fmt.Errorf("tenant bootstrap: tenants[].status is no longer supported; use tenants[].enabled (true|false)")
	}
	status := string(TenantStatusActive)
	if spec.Enabled != nil && !*spec.Enabled {
		status = string(TenantStatusSuspended)
	}
	tenantModel := Tenant{
		ID:           spec.ID,
		DisplayName:  spec.DisplayName,
		SupportEmail: spec.SupportEmail,
		Status:       TenantStatus(status),
	}
	if err := tx.WithContext(ctx).Clauses(clauseOnConflictUpdateAll()).
		Create(&tenantModel).Error; err != nil {
		return fmt.Errorf("tenant bootstrap: upsert tenant %s: %w", spec.ID, err)
	}

	normalizedDomains := normalizeDomainHosts(spec.Domains)
	for domainIndex, host := range normalizedDomains {
		domain := TenantDomain{
			TenantID:  spec.ID,
			Host:      host,
			IsDefault: domainIndex == 0,
		}
		createResult := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "host"}},
			DoNothing: true,
		}).Create(&domain)
		if createResult.Error != nil {
			return fmt.Errorf(bootstrapDomainErrorFormat, host, createResult.Error)
		}
		var existingDomain TenantDomain
		if err := tx.Where(&TenantDomain{Host: host}).Take(&existingDomain).Error; err != nil {
			return fmt.Errorf(bootstrapDomainErrorFormat, host, err)
		}
		if existingDomain.TenantID != spec.ID || existingDomain.IsDefault != domain.IsDefault {
			return fmt.Errorf("tenant bootstrap: %s: domain %s already assigned to tenant %s", bootstrapDomainConflictCode, host, existingDomain.TenantID)
		}
	}

	if err := upsertTenantAdmins(tx, spec.ID, spec.Admins); err != nil {
		return err
	}

	usernameCipher, err := keeper.Encrypt(spec.EmailProfile.Username)
	if err != nil {
		return err
	}
	passwordCipher, err := keeper.Encrypt(spec.EmailProfile.Password)
	if err != nil {
		return err
	}
	emailProfile := EmailProfile{
		ID:             uuid.NewString(),
		TenantID:       spec.ID,
		Host:           spec.EmailProfile.Host,
		Port:           spec.EmailProfile.Port,
		UsernameCipher: usernameCipher,
		PasswordCipher: passwordCipher,
		FromAddress:    spec.EmailProfile.FromAddress,
		IsDefault:      true,
	}
	if err := tx.Create(&emailProfile).Error; err != nil {
		return fmt.Errorf("tenant bootstrap: email profile: %w", err)
	}

	if spec.SMSProfile != nil {
		accountCipher, err := keeper.Encrypt(spec.SMSProfile.AccountSID)
		if err != nil {
			return err
		}
		tokenCipher, err := keeper.Encrypt(spec.SMSProfile.AuthToken)
		if err != nil {
			return err
		}
		smsProfile := SMSProfile{
			ID:               uuid.NewString(),
			TenantID:         spec.ID,
			AccountSIDCipher: accountCipher,
			AuthTokenCipher:  tokenCipher,
			FromNumber:       spec.SMSProfile.FromNumber,
			IsDefault:        true,
		}
		if err := tx.Create(&smsProfile).Error; err != nil {
			return fmt.Errorf("tenant bootstrap: sms profile: %w", err)
		}
	}

	return nil
}

const (
	bootstrapDuplicateDomainCode   = "tenant.bootstrap.domain.duplicate"
	bootstrapMissingDomainCode     = "tenant.bootstrap.domain.missing"
	bootstrapDomainResetCode       = "tenant.bootstrap.domain.reset_failed"
	bootstrapDomainConflictCode    = "tenant.bootstrap.domain.conflict"
	bootstrapAdminResetCode        = "tenant.bootstrap.admin.reset_failed"
	bootstrapAdminCreateCode       = "tenant.bootstrap.admin.create_failed"
	bootstrapEmailProfileResetCode = "tenant.bootstrap.email_profile.reset_failed"
	bootstrapSMSProfileResetCode   = "tenant.bootstrap.sms_profile.reset_failed"
	bootstrapTenantCleanupCode     = "tenant.bootstrap.tenant.cleanup_failed"
	bootstrapDomainErrorFormat     = "tenant bootstrap: domain %s: %w"
)

func upsertTenantAdmins(db *gorm.DB, tenantID string, admins []string) error {
	for _, email := range normalizeAdminEmails(admins) {
		admin := TenantAdmin{
			TenantID: tenantID,
			Email:    email,
		}
		if err := db.Create(&admin).Error; err != nil {
			return fmt.Errorf("tenant bootstrap: %s: create tenant admin: %w", bootstrapAdminCreateCode, err)
		}
	}
	return nil
}

func prepareBootstrapTenants(tenantSpecs []BootstrapTenant) []BootstrapTenant {
	preparedTenantSpecs := make([]BootstrapTenant, len(tenantSpecs))
	copy(preparedTenantSpecs, tenantSpecs)
	for tenantIndex := range preparedTenantSpecs {
		preparedTenantSpecs[tenantIndex].ID = strings.TrimSpace(preparedTenantSpecs[tenantIndex].ID)
		if preparedTenantSpecs[tenantIndex].ID == "" {
			preparedTenantSpecs[tenantIndex].ID = uuid.NewString()
		}
	}
	return preparedTenantSpecs
}

func bootstrapTenantIDs(tenantSpecs []BootstrapTenant) []string {
	tenantIDs := make([]string, 0, len(tenantSpecs))
	for _, tenantSpec := range tenantSpecs {
		tenantIDs = append(tenantIDs, tenantSpec.ID)
	}
	return tenantIDs
}

func validateBootstrapDomains(tenantSpecs []BootstrapTenant) error {
	normalizedHosts := make(map[string]int, len(tenantSpecs))
	for tenantIndex, tenantSpec := range tenantSpecs {
		domainCount := 0
		for _, host := range tenantSpec.Domains {
			normalizedHost := normalizeDomainHost(host)
			if normalizedHost == "" {
				continue
			}
			domainCount++
			if existingIndex, exists := normalizedHosts[normalizedHost]; exists {
				return fmt.Errorf("tenant bootstrap: %s: duplicate domain %s between tenants[%d] and tenants[%d]", bootstrapDuplicateDomainCode, normalizedHost, existingIndex, tenantIndex)
			}
			normalizedHosts[normalizedHost] = tenantIndex
		}
		if domainCount == 0 {
			return fmt.Errorf("tenant bootstrap: %s: tenants[%d] has no domains", bootstrapMissingDomainCode, tenantIndex)
		}
	}
	return nil
}

func resetTenantAdmins(db *gorm.DB) error {
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TenantAdmin{}).Error; err != nil {
		return fmt.Errorf("tenant bootstrap: %s: reset tenant admins: %w", bootstrapAdminResetCode, err)
	}
	return nil
}

func resetTenantEmailProfiles(db *gorm.DB) error {
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&EmailProfile{}).Error; err != nil {
		return fmt.Errorf("tenant bootstrap: %s: reset email profiles: %w", bootstrapEmailProfileResetCode, err)
	}
	return nil
}

func resetTenantSMSProfiles(db *gorm.DB) error {
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SMSProfile{}).Error; err != nil {
		return fmt.Errorf("tenant bootstrap: %s: reset sms profiles: %w", bootstrapSMSProfileResetCode, err)
	}
	return nil
}

func removeStaleTenants(db *gorm.DB, configuredTenantIDs []string) error {
	if err := db.Where(tenantIDNotInClause(tenantColumnID, configuredTenantIDs)).Delete(&Tenant{}).Error; err != nil {
		return fmt.Errorf("tenant bootstrap: %s: remove stale tenants: %w", bootstrapTenantCleanupCode, err)
	}
	return nil
}

func tenantIDNotInClause(columnName string, tenantIDs []string) clause.Expression {
	return clause.Not(clause.IN{
		Column: clause.Column{Name: columnName},
		Values: tenantIDClauseValues(tenantIDs),
	})
}

func tenantIDClauseValues(tenantIDs []string) []interface{} {
	values := make([]interface{}, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		values = append(values, tenantID)
	}
	return values
}

func resetTenantDomains(db *gorm.DB) error {
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&TenantDomain{}).Error; err != nil {
		return fmt.Errorf("tenant bootstrap: %s: reset tenant domains: %w", bootstrapDomainResetCode, err)
	}
	return nil
}

func normalizeDomainHosts(domains []string) []string {
	normalizedDomains := make([]string, 0, len(domains))
	for _, host := range domains {
		normalizedHost := normalizeDomainHost(host)
		if normalizedHost == "" {
			continue
		}
		normalizedDomains = append(normalizedDomains, normalizedHost)
	}
	return normalizedDomains
}

func normalizeDomainHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

func normalizeAdminEmails(emails []string) []string {
	seenEmails := make(map[string]struct{}, len(emails))
	normalizedEmails := make([]string, 0, len(emails))
	for _, email := range emails {
		normalizedEmail := normalizeAdminEmail(email)
		if normalizedEmail == "" {
			continue
		}
		if _, exists := seenEmails[normalizedEmail]; exists {
			continue
		}
		seenEmails[normalizedEmail] = struct{}{}
		normalizedEmails = append(normalizedEmails, normalizedEmail)
	}
	return normalizedEmails
}

func normalizeAdminEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func clauseOnConflictUpdateAll() clause.Expression {
	return clause.OnConflict{
		UpdateAll: true,
	}
}
