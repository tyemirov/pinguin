package tenant

import (
	"time"
)

// TenantStatus captures allowed status values for tenants.
type TenantStatus string

const (
	// TenantStatusActive indicates the tenant can authenticate and enqueue notifications.
	TenantStatusActive TenantStatus = "active"
	// TenantStatusSuspended blocks access but keeps data for future reactivation.
	TenantStatusSuspended TenantStatus = "suspended"
)

// ViewScope determines whether the UI can access all tenants or only the active tenant.
type ViewScope string

const (
	// ViewScopeGlobal allows viewing notifications across all tenants.
	ViewScopeGlobal ViewScope = "global"
	// ViewScopeTenant restricts views to the resolved tenant only.
	ViewScopeTenant ViewScope = "tenant"
)

// Tenant represents a logical customer served by the deployment.
type Tenant struct {
	ID           string `gorm:"primaryKey"`
	DisplayName  string
	SupportEmail string
	Status       TenantStatus `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TenantDomain links hostnames to a tenant for HTTP routing.
type TenantDomain struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	Host      string `gorm:"uniqueIndex"`
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TenantMember declares per-tenant admin membership and roles.
type TenantMember struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  string `gorm:"index"`
	Email     string `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TenantIdentity stores per-tenant view scope metadata used by the UI.
type TenantIdentity struct {
	TenantID  string `gorm:"primaryKey"`
	ViewScope ViewScope
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EmailProfile describes SMTP delivery credentials for a tenant.
type EmailProfile struct {
	ID             string `gorm:"primaryKey"`
	TenantID       string `gorm:"index"`
	Host           string
	Port           int
	UsernameCipher []byte
	PasswordCipher []byte
	FromAddress    string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SMSProfile stores Twilio credentials per tenant.
type SMSProfile struct {
	ID               string `gorm:"primaryKey"`
	TenantID         string `gorm:"index"`
	AccountSIDCipher []byte
	AuthTokenCipher  []byte
	FromNumber       string
	IsDefault        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
