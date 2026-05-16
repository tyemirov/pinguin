package smtpidentity

import (
	"errors"
	"strings"
	"testing"
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
