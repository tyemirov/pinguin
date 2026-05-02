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

// Service exposes tenant-scoped SMTP identity workflows.
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

// List returns active tenant identities.
func (service *Service) List(ctx context.Context, tenantID string) ([]PublicIdentity, error) {
	return service.repository.List(ctx, tenantID)
}

// Create provisions a new exact sender identity.
func (service *Service) Create(ctx context.Context, tenantID string, address Address) (OneTimeCredentials, error) {
	identity, password, err := service.repository.Create(ctx, tenantID, address)
	if err != nil {
		return OneTimeCredentials{}, err
	}
	return service.credentials(identity, password), nil
}

// Rotate replaces credentials for an existing identity.
func (service *Service) Rotate(ctx context.Context, tenantID string, identityID string) (OneTimeCredentials, error) {
	identity, password, err := service.repository.Rotate(ctx, tenantID, strings.TrimSpace(identityID))
	if err != nil {
		return OneTimeCredentials{}, err
	}
	return service.credentials(identity, password), nil
}

// Delete disables an identity.
func (service *Service) Delete(ctx context.Context, tenantID string, identityID string) error {
	return service.repository.Delete(ctx, tenantID, strings.TrimSpace(identityID))
}

func (service *Service) credentials(identity PublicIdentity, password string) OneTimeCredentials {
	return OneTimeCredentials{
		Identity:     identity,
		SMTPSettings: service.settings,
		Username:     identity.Username,
		Password:     password,
	}
}
