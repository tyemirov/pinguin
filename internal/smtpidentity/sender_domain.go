package smtpidentity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SenderDomain declares a domain that may be used for SMTP submission senders.
type SenderDomain struct {
	ID        uint   `gorm:"primaryKey"`
	Domain    string `gorm:"uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ReplaceSenderDomains makes the SMTP submission sender-domain allowlist match configuration.
func ReplaceSenderDomains(ctx context.Context, db *gorm.DB, domains []string) error {
	normalizedDomains, normalizeErr := NormalizeSenderDomains(domains)
	if normalizeErr != nil {
		return normalizeErr
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SenderDomain{}).Error; err != nil {
			return fmt.Errorf("smtp identity sender domains: reset: %w", err)
		}
		for _, domain := range normalizedDomains {
			if err := tx.Create(&SenderDomain{Domain: domain}).Error; err != nil {
				return fmt.Errorf("smtp identity sender domain %s: %w", domain, err)
			}
		}
		return nil
	})
}

// NormalizeSenderDomains returns lower-cased unique sender domains.
func NormalizeSenderDomains(domains []string) ([]string, error) {
	seenDomains := make(map[string]struct{}, len(domains))
	normalizedDomains := make([]string, 0, len(domains))
	for _, domain := range domains {
		normalizedDomain := strings.ToLower(strings.TrimSpace(domain))
		if normalizedDomain == "" {
			continue
		}
		if _, exists := seenDomains[normalizedDomain]; exists {
			return nil, fmt.Errorf("smtp identity sender domains: duplicate domain %s", normalizedDomain)
		}
		seenDomains[normalizedDomain] = struct{}{}
		normalizedDomains = append(normalizedDomains, normalizedDomain)
	}
	if len(normalizedDomains) == 0 {
		return nil, fmt.Errorf("smtp identity sender domains: no domains configured")
	}
	return normalizedDomains, nil
}
