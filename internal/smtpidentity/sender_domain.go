package smtpidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	senderDomainVerifiedStatus = string(SenderDomainStatusVerified)
	senderDomainPendingStatus  = string(SenderDomainStatusPending)
)

var (
	// ErrInvalidSenderDomain indicates a domain cannot be used for SMTP relay ownership.
	ErrInvalidSenderDomain = errors.New("smtp_identity.sender_domain.invalid")
	// ErrSenderDomainExists indicates another owner already registered the sender domain.
	ErrSenderDomainExists = errors.New("smtp_identity.sender_domain.exists")
	// ErrSenderDomainNotFound indicates a sender domain setup record does not exist.
	ErrSenderDomainNotFound = errors.New("smtp_identity.sender_domain.not_found")
)

// SenderDomainStatus captures DNS verification state for a sender domain.
type SenderDomainStatus string

const (
	// SenderDomainStatusPending means DNS has not yet verified for SMTP relay use.
	SenderDomainStatusPending SenderDomainStatus = "pending"
	// SenderDomainStatusVerified means DNS matched the Pinguin sender-domain specification.
	SenderDomainStatusVerified SenderDomainStatus = "verified"
)

// SenderDomain declares a domain that may be used for SMTP submission senders.
type SenderDomain struct {
	ID                uint               `gorm:"primaryKey"`
	OwnerEmail        string             `gorm:"index"`
	Domain            string             `gorm:"uniqueIndex"`
	Status            SenderDomainStatus `gorm:"index"`
	VerificationToken string
	LastCheckedAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// DNSRecord describes one DNS record users must publish for a sender domain.
type DNSRecord struct {
	Type    string `json:"type"`
	Host    string `json:"host"`
	Value   string `json:"value"`
	Purpose string `json:"purpose"`
}

// DNSCheck describes the latest observed state for one DNS requirement.
type DNSCheck struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Expected string `json:"expected"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
}

// PublicSenderDomain is the DNS setup shape exposed to authenticated users.
type PublicSenderDomain struct {
	ID            uint        `json:"id"`
	Domain        string      `json:"domain"`
	Status        string      `json:"status"`
	DNSRecords    []DNSRecord `json:"dns_records"`
	DNSChecks     []DNSCheck  `json:"dns_checks,omitempty"`
	LastCheckedAt *time.Time  `json:"last_checked_at,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// CleanupLegacySenderDomains removes config-seeded sender domains that predate owner-scoped DNS verification.
func CleanupLegacySenderDomains(ctx context.Context, db *gorm.DB) error {
	configuredOwnerClause := clause.Or(
		clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: ""},
		clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: nil},
	)
	if err := db.WithContext(ctx).Where(configuredOwnerClause).Delete(&SenderDomain{}).Error; err != nil {
		return fmt.Errorf("smtp identity sender domains cleanup: %w", err)
	}
	return nil
}

// NormalizeSenderDomain validates and normalizes one DNS sender domain.
func NormalizeSenderDomain(domain string) (string, error) {
	normalizedDomain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if normalizedDomain == "" {
		return "", nil
	}
	if strings.ContainsAny(normalizedDomain, "@/\\: \t\r\n") || !strings.Contains(normalizedDomain, ".") || !validDNSDomainName(normalizedDomain) {
		return "", fmt.Errorf("%w: %s", ErrInvalidSenderDomain, strings.TrimSpace(domain))
	}
	return normalizedDomain, nil
}

func validDNSDomainName(domain string) bool {
	if len(domain) > 253 {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}
