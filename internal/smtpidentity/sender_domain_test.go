package smtpidentity

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestNormalizeSenderDomains(t *testing.T) {
	domains, err := NormalizeSenderDomains([]string{" Example.COM ", "", "Other.example"})
	if err != nil {
		t.Fatalf("normalize domains: %v", err)
	}
	if strings.Join(domains, ",") != "example.com,other.example" {
		t.Fatalf("unexpected domains %v", domains)
	}
}

func TestNormalizeSenderDomainsRejectsInvalidInput(t *testing.T) {
	testCases := []struct {
		name    string
		domains []string
		wantErr string
	}{
		{name: "empty", domains: []string{" ", ""}, wantErr: "no domains configured"},
		{name: "duplicate", domains: []string{"Example.com", "example.COM"}, wantErr: "duplicate domain example.com"},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NormalizeSenderDomains(testCase.domains)
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected %q error, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestReplaceSenderDomainsResetsConfiguredDomains(t *testing.T) {
	_, database := newIdentityRepository(t)
	if err := ReplaceSenderDomains(context.Background(), database, []string{"Example.com", "Second.example"}); err != nil {
		t.Fatalf("replace domains: %v", err)
	}
	if err := ReplaceSenderDomains(context.Background(), database, []string{"Final.example"}); err != nil {
		t.Fatalf("replace domains second time: %v", err)
	}

	var domains []SenderDomain
	if err := database.Order("domain").Find(&domains).Error; err != nil {
		t.Fatalf("list sender domains: %v", err)
	}
	if len(domains) != 1 || domains[0].Domain != "final.example" {
		t.Fatalf("unexpected stored domains %+v", domains)
	}
}

func TestReplaceSenderDomainsReportsStorageFailures(t *testing.T) {
	t.Run("normalization", func(t *testing.T) {
		_, database := newIdentityRepository(t)
		if err := ReplaceSenderDomains(context.Background(), database, []string{"example.com", "EXAMPLE.com"}); err == nil {
			t.Fatalf("expected normalization error")
		}
	})

	t.Run("reset", func(t *testing.T) {
		_, database := newIdentityRepository(t)
		if err := database.Callback().Delete().Before("gorm:delete").Register("pinguin:force_sender_domain_reset_error", func(tx *gorm.DB) {
			tx.AddError(errors.New("forced sender domain reset failure"))
		}); err != nil {
			t.Fatalf("register callback: %v", err)
		}
		if err := ReplaceSenderDomains(context.Background(), database, []string{"example.com"}); err == nil || !strings.Contains(err.Error(), "reset") {
			t.Fatalf("expected reset storage error, got %v", err)
		}
	})

	t.Run("create", func(t *testing.T) {
		_, database := newIdentityRepository(t)
		if err := database.Callback().Create().Before("gorm:create").Register("pinguin:force_sender_domain_create_error", func(tx *gorm.DB) {
			tx.AddError(errors.New("forced sender domain create failure"))
		}); err != nil {
			t.Fatalf("register callback: %v", err)
		}
		err := ReplaceSenderDomains(context.Background(), database, []string{"example.com"})
		if err == nil || !strings.Contains(err.Error(), "sender domain example.com") {
			t.Fatalf("expected create storage error, got %v", err)
		}
	})
}
