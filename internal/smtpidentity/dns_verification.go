package smtpidentity

import (
	"context"
	"fmt"
	"net"
	"strings"
)

const (
	dnsRecordTypeTXT = "TXT"
)

// DNSResolver is the DNS lookup boundary used by sender-domain verification.
type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type netDNSResolver struct{}

func (resolver netDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

func senderDomainDNSRecords(settings PublicSettings, domain SenderDomain) []DNSRecord {
	return []DNSRecord{
		{
			Type:    dnsRecordTypeTXT,
			Host:    challengeHost(domain.Domain),
			Value:   challengeValue(domain.VerificationToken),
			Purpose: "Verify domain ownership",
		},
		{
			Type:    dnsRecordTypeTXT,
			Host:    domain.Domain,
			Value:   spfStarterValue(settings),
			Purpose: "Authorize Pinguin SMTP relay",
		},
		{
			Type:    dnsRecordTypeTXT,
			Host:    dmarcHost(domain.Domain),
			Value:   "v=DMARC1; p=none",
			Purpose: "Publish a DMARC policy",
		},
	}
}

func (service *Service) checkSenderDomainDNS(ctx context.Context, domain SenderDomain) []DNSCheck {
	return []DNSCheck{
		service.checkTXT(ctx, challengeHost(domain.Domain), challengeValue(domain.VerificationToken), exactTXTMatch, "ownership TXT matched", "ownership TXT is missing"),
		service.checkTXT(ctx, domain.Domain, spfMechanism(service.settings), spfTXTMatch, "SPF authorizes Pinguin relay", "SPF is missing the Pinguin relay mechanism"),
		service.checkTXT(ctx, dmarcHost(domain.Domain), "v=DMARC1", dmarcTXTMatch, "DMARC policy found", "DMARC policy is missing"),
	}
}

func (service *Service) checkTXT(ctx context.Context, host string, expected string, matcher func([]string, string) bool, successMessage string, failureMessage string) DNSCheck {
	values, lookupErr := service.resolver.LookupTXT(ctx, host)
	if lookupErr != nil {
		return DNSCheck{
			Type:     dnsRecordTypeTXT,
			Host:     host,
			Expected: expected,
			Passed:   false,
			Message:  failureMessage,
		}
	}
	if matcher(values, expected) {
		return DNSCheck{
			Type:     dnsRecordTypeTXT,
			Host:     host,
			Expected: expected,
			Passed:   true,
			Message:  successMessage,
		}
	}
	return DNSCheck{
		Type:     dnsRecordTypeTXT,
		Host:     host,
		Expected: expected,
		Passed:   false,
		Message:  failureMessage,
	}
}

func allDNSChecksPassed(checks []DNSCheck) bool {
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}

func exactTXTMatch(values []string, expected string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == expected {
			return true
		}
	}
	return false
}

func spfTXTMatch(values []string, expectedMechanism string) bool {
	for _, value := range values {
		fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
		if len(fields) == 0 || fields[0] != "v=spf1" {
			continue
		}
		for _, field := range fields[1:] {
			if strings.TrimSpace(field) == strings.ToLower(expectedMechanism) {
				return true
			}
		}
	}
	return false
}

func dmarcTXTMatch(values []string, _ string) bool {
	for _, value := range values {
		normalizedValue := strings.ToLower(strings.TrimSpace(value))
		if strings.HasPrefix(normalizedValue, "v=dmarc1") && strings.Contains(normalizedValue, "p=") {
			return true
		}
	}
	return false
}

func publicSenderDomain(settings PublicSettings, domain SenderDomain, checks []DNSCheck) PublicSenderDomain {
	return PublicSenderDomain{
		ID:            domain.ID,
		Domain:        domain.Domain,
		Status:        string(domain.Status),
		DNSRecords:    senderDomainDNSRecords(settings, domain),
		DNSChecks:     checks,
		LastCheckedAt: domain.LastCheckedAt,
		CreatedAt:     domain.CreatedAt,
		UpdatedAt:     domain.UpdatedAt,
	}
}

func challengeHost(domain string) string {
	return "_pinguin-challenge." + domain
}

func dmarcHost(domain string) string {
	return "_dmarc." + domain
}

func challengeValue(token string) string {
	return "pinguin-domain-verification=" + strings.TrimSpace(token)
}

func spfMechanism(settings PublicSettings) string {
	return "a:" + strings.TrimSpace(settings.Host)
}

func spfStarterValue(settings PublicSettings) string {
	return fmt.Sprintf("v=spf1 %s ~all", spfMechanism(settings))
}
