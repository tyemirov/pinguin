package smtpidentity

import (
	"context"
	"errors"
	"testing"
)

func TestServiceWrapsRepositoryWorkflows(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	service := NewService(repository, PublicSettings{
		Host:         "smtp.example.com",
		Port:         587,
		SecurityMode: "starttls",
	})

	created, createErr := service.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if created.Username == "" || created.Password == "" {
		t.Fatalf("expected generated credentials")
	}
	if created.SMTPSettings.Host != "smtp.example.com" {
		t.Fatalf("unexpected public settings %+v", created.SMTPSettings)
	}

	identities, listErr := service.List(context.Background())
	if listErr != nil {
		t.Fatalf("list identities: %v", listErr)
	}
	if len(identities) != 1 {
		t.Fatalf("expected one identity, got %d", len(identities))
	}
	retrieved, credentialsErr := service.Credentials(context.Background(), " "+created.Identity.ID+" ")
	if credentialsErr != nil {
		t.Fatalf("retrieve credentials: %v", credentialsErr)
	}
	if retrieved.Password != created.Password || retrieved.Username != created.Username {
		t.Fatalf("unexpected retrieved credentials: %+v", retrieved)
	}

	updated, updateErr := service.UpdateForwarding(context.Background(), " "+created.Identity.ID+" ", []Address{mustAddress(t, "maria@example.com")})
	if updateErr != nil {
		t.Fatalf("update forwarding: %v", updateErr)
	}
	if len(updated.ForwardTo) != 1 || updated.ForwardTo[0] != "maria@example.com" {
		t.Fatalf("unexpected forwarding update: %+v", updated.ForwardTo)
	}

	rotated, rotateErr := service.Rotate(context.Background(), " "+created.Identity.ID+" ")
	if rotateErr != nil {
		t.Fatalf("rotate identity: %v", rotateErr)
	}
	if rotated.Password == created.Password {
		t.Fatalf("expected rotated password to change")
	}

	if deleteErr := service.Delete(context.Background(), " "+created.Identity.ID+" "); deleteErr != nil {
		t.Fatalf("delete identity: %v", deleteErr)
	}
}

func TestServicePropagatesRepositoryErrors(t *testing.T) {
	repository, _ := newIdentityRepository(t)
	service := NewService(repository, PublicSettings{})

	if _, createErr := service.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t)); createErr == nil {
		t.Fatalf("expected create error without sender domain")
	}
	if _, rotateErr := service.Rotate(context.Background(), "missing"); rotateErr == nil {
		t.Fatalf("expected rotate error")
	}
	if _, credentialsErr := service.Credentials(context.Background(), "missing"); credentialsErr == nil {
		t.Fatalf("expected credentials error")
	}
	if _, updateErr := service.UpdateForwarding(context.Background(), "missing", defaultForwardRecipients(t)); updateErr == nil {
		t.Fatalf("expected update forwarding error")
	}
	if deleteErr := service.Delete(context.Background(), "missing"); deleteErr == nil {
		t.Fatalf("expected delete error")
	}
}

func TestServiceChecksSenderDomainDNS(t *testing.T) {
	repository, _ := newIdentityRepository(t)
	resolver := serviceFakeDNSResolver{}
	service := NewServiceWithDNSResolver(repository, PublicSettings{
		Host:         "smtp.example.com",
		Port:         465,
		SecurityMode: "ssl",
	}, resolver)
	scope := AccessScope{OwnerEmail: "member@example.com"}

	created, createErr := service.CreateSenderDomain(context.Background(), scope, "Customer.Example")
	if createErr != nil {
		t.Fatalf("create sender domain: %v", createErr)
	}
	if created.Domain != "customer.example" || created.Status != string(SenderDomainStatusPending) || len(created.DNSRecords) != 3 {
		t.Fatalf("unexpected sender domain setup payload: %+v", created)
	}
	failed, failedErr := service.CheckSenderDomainDNS(context.Background(), scope, created.ID)
	if failedErr != nil {
		t.Fatalf("check missing DNS: %v", failedErr)
	}
	if failed.Status != string(SenderDomainStatusPending) || len(failed.DNSChecks) != 3 {
		t.Fatalf("expected pending failed DNS checks, got %+v", failed)
	}
	for _, check := range failed.DNSChecks {
		if check.Passed {
			t.Fatalf("expected missing DNS check to fail, got %+v", failed.DNSChecks)
		}
	}

	resolver.set(created.DNSRecords[0].Host, []string{created.DNSRecords[0].Value})
	resolver.set("customer.example", []string{"v=spf1 include:_spf.example.invalid a:smtp.example.com ~all"})
	resolver.set("_dmarc.customer.example", []string{"v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"})
	verified, verifyErr := service.CheckSenderDomainDNS(context.Background(), scope, created.ID)
	if verifyErr != nil {
		t.Fatalf("check verified DNS: %v", verifyErr)
	}
	if verified.Status != string(SenderDomainStatusVerified) {
		t.Fatalf("expected verified sender domain, got %+v", verified)
	}
	for _, check := range verified.DNSChecks {
		if !check.Passed {
			t.Fatalf("expected all DNS checks to pass, got %+v", verified.DNSChecks)
		}
	}
}

func TestServiceReportsMismatchedSenderDomainDNS(t *testing.T) {
	if _, lookupErr := (netDNSResolver{}).LookupTXT(cancelledContext(), "example.invalid"); lookupErr == nil {
		t.Fatalf("expected cancelled DNS lookup to fail")
	}
	repository, _ := newIdentityRepository(t)
	resolver := serviceFakeDNSResolver{}
	service := NewServiceWithDNSResolver(repository, PublicSettings{
		Host:         "smtp.example.com",
		Port:         465,
		SecurityMode: "ssl",
	}, resolver)
	scope := AccessScope{OwnerEmail: "member@example.com"}

	created, createErr := service.CreateSenderDomain(context.Background(), scope, "customer.example")
	if createErr != nil {
		t.Fatalf("create sender domain: %v", createErr)
	}
	resolver.set(created.DNSRecords[0].Host, []string{"wrong-token"})
	resolver.set("customer.example", []string{"v=spf1 include:_spf.example.invalid ~all"})
	resolver.set("_dmarc.customer.example", []string{"v=DMARC1"})
	checked, checkErr := service.CheckSenderDomainDNS(context.Background(), scope, created.ID)
	if checkErr != nil {
		t.Fatalf("check mismatched DNS: %v", checkErr)
	}
	if checked.Status != string(SenderDomainStatusPending) || len(checked.DNSChecks) != 3 {
		t.Fatalf("unexpected mismatched DNS payload: %+v", checked)
	}
	for _, check := range checked.DNSChecks {
		if check.Passed {
			t.Fatalf("expected mismatched DNS check to fail, got %+v", checked.DNSChecks)
		}
	}
	if spfTXTMatch([]string{""}, "a:smtp.example.com") {
		t.Fatalf("empty TXT value must not satisfy SPF")
	}
	if spfTXTMatch([]string{"not-spf"}, "a:smtp.example.com") {
		t.Fatalf("non-SPF TXT value must not satisfy SPF")
	}
}

func TestServiceScopedWorkflows(t *testing.T) {
	repository, database := newIdentityRepository(t)
	service := NewService(repository, PublicSettings{
		Host:         "smtp.example.com",
		Port:         587,
		SecurityMode: "starttls",
	})
	scope := AccessScope{OwnerEmail: "member@example.com"}
	domain, domainErr := repository.CreateSenderDomainForScope(context.Background(), scope, "customer.example")
	if domainErr != nil {
		t.Fatalf("create sender domain: %v", domainErr)
	}
	if _, updateErr := repository.UpdateSenderDomainStatusForScope(context.Background(), scope, domain.ID, SenderDomainStatusVerified, repository.clockFunc()); updateErr != nil {
		t.Fatalf("verify sender domain: %v", updateErr)
	}

	domains, listDomainsErr := service.ListSenderDomains(context.Background(), scope)
	if listDomainsErr != nil {
		t.Fatalf("list sender domains: %v", listDomainsErr)
	}
	if len(domains) != 1 || domains[0].Domain != "customer.example" {
		t.Fatalf("unexpected sender domains: %+v", domains)
	}
	created, createErr := service.CreateForScope(context.Background(), scope, mustAddress(t, "alice@customer.example"), defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create scoped identity: %v", createErr)
	}
	identities, listErr := service.ListForScope(context.Background(), scope)
	if listErr != nil {
		t.Fatalf("list scoped identities: %v", listErr)
	}
	if len(identities) != 1 || identities[0].ID != created.Identity.ID {
		t.Fatalf("unexpected scoped identities: %+v", identities)
	}
	credentials, credentialsErr := service.CredentialsForScope(context.Background(), scope, created.Identity.ID)
	if credentialsErr != nil {
		t.Fatalf("scoped credentials: %v", credentialsErr)
	}
	if credentials.Password != created.Password {
		t.Fatalf("unexpected credentials: %+v", credentials)
	}
	updated, updateForwardingErr := service.UpdateForwardingForScope(context.Background(), scope, created.Identity.ID, []Address{mustAddress(t, "owner@example.com"), mustAddress(t, "maria@example.com")})
	if updateForwardingErr != nil {
		t.Fatalf("update scoped forwarding: %v", updateForwardingErr)
	}
	if len(updated.ForwardTo) != 2 {
		t.Fatalf("unexpected forwarding update: %+v", updated.ForwardTo)
	}
	rotated, rotateErr := service.RotateForScope(context.Background(), scope, created.Identity.ID)
	if rotateErr != nil {
		t.Fatalf("rotate scoped identity: %v", rotateErr)
	}
	if rotated.Password == created.Password {
		t.Fatalf("expected rotated password")
	}
	if deleteErr := service.DeleteForScope(context.Background(), scope, created.Identity.ID); deleteErr != nil {
		t.Fatalf("delete scoped identity: %v", deleteErr)
	}
	closeIdentityDatabase(t, database)
	if _, err := service.ListSenderDomains(context.Background(), scope); err == nil {
		t.Fatalf("expected sender domain list storage error")
	}
	if _, err := service.CreateSenderDomain(context.Background(), scope, "bad domain"); err == nil {
		t.Fatalf("expected sender domain create validation error")
	}
	if _, err := service.CheckSenderDomainDNS(context.Background(), scope, 404); err == nil {
		t.Fatalf("expected sender domain check lookup error")
	}
}

func TestServiceScopedWorkflowErrors(t *testing.T) {
	repository, database := newIdentityRepository(t)
	service := NewService(repository, PublicSettings{
		Host:         "smtp.example.com",
		Port:         587,
		SecurityMode: "starttls",
	})
	scope := AccessScope{OwnerEmail: "member@example.com"}

	if _, err := service.CredentialsForScope(context.Background(), scope, "missing"); err == nil {
		t.Fatalf("expected scoped credentials error")
	}
	if _, err := service.CreateForScope(context.Background(), scope, mustAddress(t, "alice@example.com"), defaultForwardRecipients(t)); err == nil {
		t.Fatalf("expected scoped create sender-domain error")
	}
	if _, err := service.RotateForScope(context.Background(), scope, "missing"); err == nil {
		t.Fatalf("expected scoped rotate error")
	}

	domain, domainErr := repository.CreateSenderDomainForScope(context.Background(), scope, "customer.example")
	if domainErr != nil {
		t.Fatalf("create sender domain: %v", domainErr)
	}
	registerSenderDomainUpdateError(t, database)
	if _, err := service.CheckSenderDomainDNS(context.Background(), scope, domain.ID); err == nil {
		t.Fatalf("expected sender domain status update error")
	}
}

type serviceFakeDNSResolver map[string][]string

func (resolver serviceFakeDNSResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	values, ok := resolver[name]
	if !ok {
		return nil, errors.New("dns record not found")
	}
	return values, nil
}

func (resolver serviceFakeDNSResolver) set(name string, values []string) {
	resolver[name] = values
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
