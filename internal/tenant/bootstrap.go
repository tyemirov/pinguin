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
	Slug         string                `json:"slug" yaml:"slug"`
	DisplayName  string                `json:"displayName" yaml:"displayName"`
	SupportEmail string                `json:"supportEmail" yaml:"supportEmail"`
	Status       string                `json:"status" yaml:"status"`
	Domains      []string              `json:"domains" yaml:"domains"`
	Admins       []BootstrapMember     `json:"admins" yaml:"admins"`
	Identity     BootstrapIdentity     `json:"identity" yaml:"identity"`
	EmailProfile BootstrapEmailProfile `json:"emailProfile" yaml:"emailProfile"`
	SMSProfile   *BootstrapSMSProfile  `json:"smsProfile" yaml:"smsProfile"`
}

// BootstrapMember captures admin membership entries.
type BootstrapMember struct {
	Email string `json:"email" yaml:"email"`
	Role  string `json:"role" yaml:"role"`
}

// BootstrapIdentity holds GIS/TAuth metadata.
type BootstrapIdentity struct {
	GoogleClientID string `json:"googleClientId" yaml:"googleClientId"`
	TAuthBaseURL   string `json:"tauthBaseUrl" yaml:"tauthBaseUrl"`
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
	for _, tenantSpec := range cfg.Tenants {
		if err := upsertTenant(ctx, db, keeper, tenantSpec); err != nil {
			return err
		}
	}
	invalidateRegisteredRepositories()
	return nil
}

func upsertTenant(ctx context.Context, db *gorm.DB, keeper *SecretKeeper, spec BootstrapTenant) error {
	if strings.TrimSpace(spec.ID) == "" {
		spec.ID = uuid.NewString()
	}
	if spec.Status == "" {
		spec.Status = string(TenantStatusActive)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenantModel := Tenant{
			ID:           spec.ID,
			Slug:         spec.Slug,
			DisplayName:  spec.DisplayName,
			SupportEmail: spec.SupportEmail,
			Status:       TenantStatus(spec.Status),
		}
		if err := tx.Clauses(clauseOnConflictUpdateAll()).
			Create(&tenantModel).Error; err != nil {
			return fmt.Errorf("tenant bootstrap: upsert tenant %s: %w", spec.Slug, err)
		}

		if err := tx.Where("tenant_id = ?", spec.ID).Delete(&TenantDomain{}).Error; err != nil {
			return err
		}
		for idx, host := range spec.Domains {
			domain := TenantDomain{
				TenantID:  spec.ID,
				Host:      strings.ToLower(host),
				IsDefault: idx == 0,
			}
			if err := tx.Create(&domain).Error; err != nil {
				return fmt.Errorf("tenant bootstrap: domain %s: %w", host, err)
			}
		}

		identity := TenantIdentity{
			TenantID:       spec.ID,
			GoogleClientID: spec.Identity.GoogleClientID,
			TAuthBaseURL:   spec.Identity.TAuthBaseURL,
		}
		if err := tx.Clauses(clauseOnConflictUpdateAll()).Create(&identity).Error; err != nil {
			return fmt.Errorf("tenant bootstrap: identity: %w", err)
		}

		if err := tx.Where("tenant_id = ?", spec.ID).Delete(&TenantMember{}).Error; err != nil {
			return err
		}
		for _, admin := range spec.Admins {
			member := TenantMember{
				TenantID: spec.ID,
				Email:    strings.ToLower(strings.TrimSpace(admin.Email)),
				Role:     strings.TrimSpace(admin.Role),
			}
			if member.Email == "" {
				continue
			}
			if member.Role == "" {
				member.Role = "admin"
			}
			if err := tx.Create(&member).Error; err != nil {
				return fmt.Errorf("tenant bootstrap: member %s: %w", member.Email, err)
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
		if err := tx.Where("tenant_id = ?", spec.ID).Delete(&EmailProfile{}).Error; err != nil {
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
			if err := tx.Where("tenant_id = ?", spec.ID).Delete(&SMSProfile{}).Error; err != nil {
				return err
			}
			if err := tx.Create(&smsProfile).Error; err != nil {
				return fmt.Errorf("tenant bootstrap: sms profile: %w", err)
			}
		} else {
			if err := tx.Where("tenant_id = ?", spec.ID).Delete(&SMSProfile{}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func clauseOnConflictUpdateAll() clause.Expression {
	return clause.OnConflict{
		UpdateAll: true,
	}
}
