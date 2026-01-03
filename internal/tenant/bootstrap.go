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

// BootstrapTenant declares per-tenant metadata.
type BootstrapTenant struct {
	ID           string                `json:"id" yaml:"id"`
	DisplayName  string                `json:"displayName" yaml:"displayName"`
	SupportEmail string                `json:"supportEmail" yaml:"supportEmail"`
	Enabled      *bool                 `json:"enabled" yaml:"enabled"`
	Status       string                `json:"status" yaml:"status"`
	Domains      []string              `json:"domains" yaml:"domains"`
	EmailProfile BootstrapEmailProfile `json:"emailProfile" yaml:"emailProfile"`
	SMSProfile   *BootstrapSMSProfile  `json:"smsProfile" yaml:"smsProfile"`
}

// BootstrapEmailProfile defines SMTP credentials.
type BootstrapEmailProfile struct {
	Host        string `json:"host" yaml:"host"`
	Port        int    `json:"port" yaml:"port"`
	Username    string `json:"username" yaml:"username"`
	Password    string `json:"password" yaml:"password"`
	FromAddress string `json:"fromAddress" yaml:"fromAddress"`
}

// BootstrapSMSProfile defines Twilio credentials.
type BootstrapSMSProfile struct {
	AccountSID string `json:"accountSid" yaml:"accountSid"`
	AuthToken  string `json:"authToken" yaml:"authToken"`
	FromNumber string `json:"fromNumber" yaml:"fromNumber"`
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
	enabledCount := 0
	for _, tenantSpec := range cfg.Tenants {
		if tenantSpec.Enabled != nil && !*tenantSpec.Enabled {
			continue
		}
		enabledCount++
	}
	if enabledCount == 0 {
		return fmt.Errorf("tenant bootstrap: no enabled tenants configured")
	}
	if err := validateBootstrapDomains(cfg.Tenants); err != nil {
		return err
	}
	transactionErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := resetTenantDomains(tx); err != nil {
			return err
		}
		for _, tenantSpec := range cfg.Tenants {
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
	if strings.TrimSpace(spec.ID) == "" {
		spec.ID = uuid.NewString()
	}
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
	if err := tx.Where(&EmailProfile{TenantID: spec.ID}).Delete(&EmailProfile{}).Error; err != nil {
		return err
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
		if err := tx.Where(&SMSProfile{TenantID: spec.ID}).Delete(&SMSProfile{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&smsProfile).Error; err != nil {
			return fmt.Errorf("tenant bootstrap: sms profile: %w", err)
		}
	} else {
		if err := tx.Where(&SMSProfile{TenantID: spec.ID}).Delete(&SMSProfile{}).Error; err != nil {
			return err
		}
	}

	return nil
}

const (
	bootstrapDuplicateDomainCode = "tenant.bootstrap.domain.duplicate"
	bootstrapMissingDomainCode   = "tenant.bootstrap.domain.missing"
	bootstrapDomainResetCode     = "tenant.bootstrap.domain.reset_failed"
	bootstrapDomainConflictCode  = "tenant.bootstrap.domain.conflict"
	bootstrapDomainErrorFormat   = "tenant bootstrap: domain %s: %w"
)

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

func clauseOnConflictUpdateAll() clause.Expression {
	return clause.OnConflict{
		UpdateAll: true,
	}
}
