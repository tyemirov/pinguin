package smtpidentity

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestNormalizeSenderDomainRejectsInvalidDNSNames(t *testing.T) {
	testCases := []string{
		"localhost",
		"bad..example",
		"-bad.example",
		"bad-.example",
		strings.Repeat("a", 64) + ".example",
		strings.Repeat("a", 250) + ".com",
		"bad_underscore.example",
	}
	for _, rawDomain := range testCases {
		rawDomain := rawDomain
		t.Run(rawDomain, func(t *testing.T) {
			if _, err := NormalizeSenderDomain(rawDomain); !errors.Is(err, ErrInvalidSenderDomain) {
				t.Fatalf("expected invalid domain for %q, got %v", rawDomain, err)
			}
		})
	}
}

func TestCleanupLegacySenderDomainsDeletesUnownedDomains(t *testing.T) {
	_, database := newIdentityRepository(t)
	if err := database.Omit("OwnerEmail").Create(&SenderDomain{Domain: "legacy.example"}).Error; err != nil {
		t.Fatalf("seed legacy sender domain: %v", err)
	}
	if err := database.Create(&SenderDomain{Domain: "operator.example", Status: SenderDomainStatusVerified}).Error; err != nil {
		t.Fatalf("seed configured sender domain: %v", err)
	}
	if err := database.Create(&SenderDomain{
		OwnerEmail: "member@example.com",
		Domain:     "customer.example",
		Status:     SenderDomainStatusVerified,
	}).Error; err != nil {
		t.Fatalf("seed user sender domain: %v", err)
	}

	if err := CleanupLegacySenderDomains(context.Background(), database); err != nil {
		t.Fatalf("cleanup legacy domains: %v", err)
	}

	var domains []SenderDomain
	if err := database.Order(clause.OrderByColumn{Column: clause.Column{Name: "domain"}}).Find(&domains).Error; err != nil {
		t.Fatalf("list sender domains: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("unexpected stored domain count %+v", domains)
	}
	if domains[0].Domain != "customer.example" || domains[0].OwnerEmail != "member@example.com" {
		t.Fatalf("expected user-owned domain to remain, got %+v", domains[0])
	}
}

func TestCleanupLegacySenderDomainsReportsStorageFailures(t *testing.T) {
	_, database := newIdentityRepository(t)
	if err := database.Callback().Delete().Before("gorm:delete").Register("pinguin:force_sender_domain_cleanup_error", func(tx *gorm.DB) {
		tx.AddError(errors.New("forced sender domain cleanup failure"))
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	if err := CleanupLegacySenderDomains(context.Background(), database); err == nil || !strings.Contains(err.Error(), "cleanup") {
		t.Fatalf("expected cleanup storage error, got %v", err)
	}
}
