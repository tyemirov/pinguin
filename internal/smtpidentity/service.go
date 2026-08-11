package smtpidentity

import (
	"context"
	"strings"
)

// PublicSettings describes Gmail-facing SMTP settings.
type PublicSettings struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	SecurityMode string `json:"security_mode"`
}

// Credentials contains the SMTP settings and current identity password.
type Credentials struct {
	Identity     PublicIdentity `json:"identity"`
	SMTPSettings PublicSettings `json:"smtp_settings"`
	Username     string         `json:"username"`
	Password     string         `json:"password"`
}

// Service exposes SMTP identity workflows.
type Service struct {
	repository *Repository
	settings   PublicSettings
	resolver   DNSResolver
}

// NewService constructs an identity service.
func NewService(repository *Repository, settings PublicSettings) *Service {
	return NewServiceWithDNSResolver(repository, settings, netDNSResolver{})
}

// NewServiceWithDNSResolver constructs an identity service with an explicit DNS resolver.
func NewServiceWithDNSResolver(repository *Repository, settings PublicSettings, resolver DNSResolver) *Service {
	return &Service{
		repository: repository,
		settings:   settings,
		resolver:   resolver,
	}
}

// ListForScope returns active identities owned by one tenant.
func (service *Service) ListForScope(ctx context.Context, scope AccessScope) ([]PublicIdentity, error) {
	return service.repository.ListForScope(ctx, scope)
}

// CredentialsForScope returns current SMTP settings for one tenant identity.
func (service *Service) CredentialsForScope(ctx context.Context, scope AccessScope, identityID string) (Credentials, error) {
	identity, password, err := service.repository.CredentialsForScope(ctx, scope, strings.TrimSpace(identityID))
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// CreateForScope provisions a new exact sender identity for one tenant.
func (service *Service) CreateForScope(ctx context.Context, scope AccessScope, address Address, forwardTo []Address) (Credentials, error) {
	identity, password, err := service.repository.CreateForScope(ctx, scope, address, forwardTo)
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// UpdateForwardingForScope replaces inbound forwarding recipients for one tenant identity.
func (service *Service) UpdateForwardingForScope(ctx context.Context, scope AccessScope, identityID string, forwardTo []Address) (PublicIdentity, error) {
	return service.repository.UpdateForwardingForScope(ctx, scope, strings.TrimSpace(identityID), forwardTo)
}

// RotateForScope replaces credentials for one tenant identity.
func (service *Service) RotateForScope(ctx context.Context, scope AccessScope, identityID string) (Credentials, error) {
	identity, password, err := service.repository.RotateForScope(ctx, scope, strings.TrimSpace(identityID))
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// DeleteForScope disables one tenant identity.
func (service *Service) DeleteForScope(ctx context.Context, scope AccessScope, identityID string) error {
	return service.repository.DeleteForScope(ctx, scope, strings.TrimSpace(identityID))
}

// ListSenderDomains returns sender-domain DNS setup records visible to an authenticated owner scope.
func (service *Service) ListSenderDomains(ctx context.Context, scope AccessScope) ([]PublicSenderDomain, error) {
	domains, err := service.repository.ListSenderDomainsForScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	result := make([]PublicSenderDomain, 0, len(domains))
	for _, domain := range domains {
		result = append(result, publicSenderDomain(service.settings, domain, nil))
	}
	return result, nil
}

// CreateSenderDomain starts DNS verification for one authenticated owner sender domain.
func (service *Service) CreateSenderDomain(ctx context.Context, scope AccessScope, domain string) (PublicSenderDomain, error) {
	record, err := service.repository.CreateSenderDomainForScope(ctx, scope, domain)
	if err != nil {
		return PublicSenderDomain{}, err
	}
	return publicSenderDomain(service.settings, record, nil), nil
}

// CheckSenderDomainDNS refreshes DNS verification state for one sender domain.
func (service *Service) CheckSenderDomainDNS(ctx context.Context, scope AccessScope, domainID uint) (PublicSenderDomain, error) {
	record, err := service.repository.RequireSenderDomainForScope(ctx, scope, domainID)
	if err != nil {
		return PublicSenderDomain{}, err
	}
	checks := service.checkSenderDomainDNS(ctx, record)
	nextStatus := SenderDomainStatusPending
	if allDNSChecksPassed(checks) {
		nextStatus = SenderDomainStatusVerified
	}
	updated, updateErr := service.repository.UpdateSenderDomainStatusForScope(ctx, scope, domainID, nextStatus, service.repository.clockFunc())
	if updateErr != nil {
		return PublicSenderDomain{}, updateErr
	}
	return publicSenderDomain(service.settings, updated, checks), nil
}

func (service *Service) credentials(identity PublicIdentity, password string) Credentials {
	return Credentials{
		Identity:     identity,
		SMTPSettings: service.settings,
		Username:     identity.Username,
		Password:     password,
	}
}
