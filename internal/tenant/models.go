package tenant

import "time"

// Tenant is one owner-scoped notification tenant.
type Tenant struct {
	ID           string `gorm:"primaryKey"`
	OwnerUserID  string `gorm:"index;not null"`
	DisplayName  string `gorm:"not null"`
	SupportEmail string
	Version      uint64 `gorm:"not null;default:1"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// EmailProfile stores one tenant external SMTP delivery profile.
type EmailProfile struct {
	ID             string `gorm:"primaryKey"`
	TenantID       string `gorm:"uniqueIndex;not null"`
	Host           string `gorm:"not null"`
	Port           int    `gorm:"not null"`
	UsernameCipher []byte `gorm:"not null"`
	PasswordCipher []byte `gorm:"not null"`
	FromAddress    string `gorm:"not null"`
	Version        uint64 `gorm:"not null;default:1"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SMSProfile stores one optional tenant Twilio delivery profile.
type SMSProfile struct {
	ID               string `gorm:"primaryKey"`
	TenantID         string `gorm:"uniqueIndex;not null"`
	AccountSIDCipher []byte `gorm:"column:account_sid_cipher;not null"`
	AuthTokenCipher  []byte `gorm:"not null"`
	FromNumber       string `gorm:"not null"`
	Version          uint64 `gorm:"not null;default:1"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// APICredential stores the one programmatic credential for a tenant.
type APICredential struct {
	ID            string `gorm:"primaryKey"`
	TenantID      string `gorm:"uniqueIndex;not null"`
	SecretDigest  []byte `gorm:"not null"`
	DisplayPrefix string `gorm:"not null"`
	LastUsedAt    *time.Time
	Version       uint64 `gorm:"not null;default:1"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IdempotencyRecord binds one tenant create key to its first result.
type IdempotencyRecord struct {
	ID             string `gorm:"primaryKey"`
	OwnerUserID    string `gorm:"uniqueIndex:idx_tenant_idempotency_owner_operation_key;not null"`
	Operation      string `gorm:"uniqueIndex:idx_tenant_idempotency_owner_operation_key;not null"`
	RequestKey     string `gorm:"uniqueIndex:idx_tenant_idempotency_owner_operation_key;not null"`
	RequestDigest  []byte `gorm:"not null"`
	TenantID       string `gorm:"index;not null"`
	ResponseStatus int    `gorm:"not null"`
	ResponseBody   []byte `gorm:"not null"`
	CreatedAt      time.Time
}
