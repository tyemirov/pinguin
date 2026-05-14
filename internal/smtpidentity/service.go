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

// List returns active identities.
func (service *Service) List(ctx context.Context) ([]PublicIdentity, error) {
	return service.repository.List(ctx)
}

// ListForScope returns active identities visible to an authenticated owner scope.
func (service *Service) ListForScope(ctx context.Context, scope AccessScope) ([]PublicIdentity, error) {
	return service.repository.ListForScope(ctx, scope)
}

// Credentials returns current SMTP settings for an existing identity.
func (service *Service) Credentials(ctx context.Context, identityID string) (Credentials, error) {
	identity, password, err := service.repository.Credentials(ctx, strings.TrimSpace(identityID))
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// CredentialsForScope returns current SMTP settings visible to an authenticated owner scope.
func (service *Service) CredentialsForScope(ctx context.Context, scope AccessScope, identityID string) (Credentials, error) {
	identity, password, err := service.repository.CredentialsForScope(ctx, scope, strings.TrimSpace(identityID))
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// Create provisions a new exact sender identity with inbound forwarding owners.
func (service *Service) Create(ctx context.Context, address Address, forwardTo []Address) (Credentials, error) {
	identity, password, err := service.repository.Create(ctx, address, forwardTo)
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// CreateForScope provisions a new exact sender identity for an authenticated owner scope.
func (service *Service) CreateForScope(ctx context.Context, scope AccessScope, address Address, forwardTo []Address) (Credentials, error) {
	identity, password, err := service.repository.CreateForScope(ctx, scope, address, forwardTo)
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// UpdateForwarding replaces inbound forwarding recipients for an existing identity.
func (service *Service) UpdateForwarding(ctx context.Context, identityID string, forwardTo []Address) (PublicIdentity, error) {
	return service.repository.UpdateForwarding(ctx, strings.TrimSpace(identityID), forwardTo)
}

// UpdateForwardingForScope replaces inbound forwarding recipients visible to an authenticated owner scope.
func (service *Service) UpdateForwardingForScope(ctx context.Context, scope AccessScope, identityID string, forwardTo []Address) (PublicIdentity, error) {
	return service.repository.UpdateForwardingForScope(ctx, scope, strings.TrimSpace(identityID), forwardTo)
}

// Rotate replaces credentials for an existing identity.
func (service *Service) Rotate(ctx context.Context, identityID string) (Credentials, error) {
	identity, password, err := service.repository.Rotate(ctx, strings.TrimSpace(identityID))
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// RotateForScope replaces credentials for an identity visible to an authenticated owner scope.
func (service *Service) RotateForScope(ctx context.Context, scope AccessScope, identityID string) (Credentials, error) {
	identity, password, err := service.repository.RotateForScope(ctx, scope, strings.TrimSpace(identityID))
	if err != nil {
		return Credentials{}, err
	}
	return service.credentials(identity, password), nil
}

// Delete disables an identity.
func (service *Service) Delete(ctx context.Context, identityID string) error {
	return service.repository.Delete(ctx, strings.TrimSpace(identityID))
}

// DeleteForScope disables an identity visible to an authenticated owner scope.
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
