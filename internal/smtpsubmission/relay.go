package smtpsubmission

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/tenant"
)

// UpstreamRelay relays submitted messages through tenant email credentials.
type UpstreamRelay struct {
	tenantRepository *tenant.Repository
	logger           *slog.Logger
	config           config.Config
	senderMutex      sync.RWMutex
	senders          map[string]*service.SMTPEmailSender
}

// NewUpstreamRelay constructs a tenant-aware raw SMTP relay.
func NewUpstreamRelay(tenantRepository *tenant.Repository, logger *slog.Logger, cfg config.Config) *UpstreamRelay {
	return &UpstreamRelay{
		tenantRepository: tenantRepository,
		logger:           logger,
		config:           cfg,
		senders:          make(map[string]*service.SMTPEmailSender),
	}
}

// Relay forwards a validated raw message through the tenant upstream provider.
func (relay *UpstreamRelay) Relay(ctx context.Context, message RawMessage) error {
	runtimeConfig, resolveErr := relay.tenantRepository.ResolveByID(ctx, message.TenantID)
	if resolveErr != nil {
		return fmt.Errorf("%w: resolve tenant: %v", ErrRelayTemporary, resolveErr)
	}
	sender := relay.senderForTenant(runtimeConfig)
	sendErr := sender.SendRawEmail(ctx, message.From.String(), message.RecipientStrings(), message.Data)
	if sendErr != nil {
		relay.logger.Error("smtp_submission_upstream_failed", "tenant_id", message.TenantID, "identity_id", message.IdentityID, "error", sendErr)
		return fmt.Errorf("%w: upstream smtp: %v", ErrRelayTemporary, sendErr)
	}
	return nil
}

func (relay *UpstreamRelay) senderForTenant(runtimeConfig tenant.RuntimeConfig) *service.SMTPEmailSender {
	relay.senderMutex.RLock()
	cachedSender := relay.senders[runtimeConfig.Tenant.ID]
	relay.senderMutex.RUnlock()
	if cachedSender != nil {
		return cachedSender
	}
	relay.senderMutex.Lock()
	defer relay.senderMutex.Unlock()
	if existingSender := relay.senders[runtimeConfig.Tenant.ID]; existingSender != nil {
		return existingSender
	}
	sender := service.NewSMTPEmailSender(service.SMTPConfig{
		Host:        runtimeConfig.Email.Host,
		Port:        strconv.Itoa(runtimeConfig.Email.Port),
		Username:    runtimeConfig.Email.Username,
		Password:    runtimeConfig.Email.Password,
		FromAddress: runtimeConfig.Email.FromAddress,
		Timeouts:    relay.config,
	}, relay.logger)
	relay.senders[runtimeConfig.Tenant.ID] = sender
	return sender
}
