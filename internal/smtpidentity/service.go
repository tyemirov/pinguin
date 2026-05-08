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

// OneTimeCredentials contains a created or rotated identity plus the only password display.
type OneTimeCredentials struct {
	Identity     PublicIdentity `json:"identity"`
	SMTPSettings PublicSettings `json:"smtp_settings"`
	Username     string         `json:"username"`
	Password     string         `json:"password"`
}

// Service exposes SMTP identity workflows.
type Service struct {
	repository *Repository
	settings   PublicSettings
}

// NewService constructs an identity service.
func NewService(repository *Repository, settings PublicSettings) *Service {
	return &Service{
		repository: repository,
		settings:   settings,
	}
}

// List returns active identities.
func (service *Service) List(ctx context.Context) ([]PublicIdentity, error) {
	return service.repository.List(ctx)
}

// Create provisions a new exact sender identity with inbound forwarding owners.
func (service *Service) Create(ctx context.Context, address Address, forwardTo []Address) (OneTimeCredentials, error) {
	identity, password, err := service.repository.Create(ctx, address, forwardTo)
	if err != nil {
		return OneTimeCredentials{}, err
	}
	return service.credentials(identity, password), nil
}

// UpdateForwarding replaces inbound forwarding recipients for an existing identity.
func (service *Service) UpdateForwarding(ctx context.Context, identityID string, forwardTo []Address) (PublicIdentity, error) {
	return service.repository.UpdateForwarding(ctx, strings.TrimSpace(identityID), forwardTo)
}

// Rotate replaces credentials for an existing identity.
func (service *Service) Rotate(ctx context.Context, identityID string) (OneTimeCredentials, error) {
	identity, password, err := service.repository.Rotate(ctx, strings.TrimSpace(identityID))
	if err != nil {
		return OneTimeCredentials{}, err
	}
	return service.credentials(identity, password), nil
}

// Delete disables an identity.
func (service *Service) Delete(ctx context.Context, identityID string) error {
	return service.repository.Delete(ctx, strings.TrimSpace(identityID))
}

func (service *Service) credentials(identity PublicIdentity, password string) OneTimeCredentials {
	return OneTimeCredentials{
		Identity:     identity,
		SMTPSettings: service.settings,
		Username:     identity.Username,
		Password:     password,
	}
}
