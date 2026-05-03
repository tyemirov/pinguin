package smtpsubmission

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/textproto"
	"strconv"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/service"
)

// UpstreamRelay relays submitted messages through the configured SMTP submission provider.
type UpstreamRelay struct {
	logger *slog.Logger
	sender rawEmailSender
}

type rawEmailSender interface {
	SendRawEmail(ctx context.Context, fromAddress string, recipients []string, rawMessage []byte) error
}

// NewUpstreamRelay constructs a raw SMTP relay for the independent submission feature.
func NewUpstreamRelay(logger *slog.Logger, cfg config.Config) *UpstreamRelay {
	relayProfile := cfg.SMTPSubmission.Relay
	return &UpstreamRelay{
		logger: logger,
		sender: service.NewSMTPEmailSender(service.SMTPConfig{
			Host:     relayProfile.Host,
			Port:     strconv.Itoa(relayProfile.Port),
			Username: relayProfile.Username,
			Password: relayProfile.Password,
			Timeouts: cfg,
		}, logger),
	}
}

// Relay forwards a validated raw message through the configured upstream provider.
func (relay *UpstreamRelay) Relay(ctx context.Context, message RawMessage) error {
	sendErr := relay.sender.SendRawEmail(ctx, message.From.String(), message.RecipientStrings(), message.Data)
	if sendErr != nil {
		relay.logger.Error("smtp_submission_upstream_failed", "identity_id", message.IdentityID, "error", sendErr)
		return upstreamRelayError(sendErr)
	}
	return nil
}

func upstreamRelayError(sendErr error) error {
	if isPermanentSMTPError(sendErr) {
		return fmt.Errorf("%w: upstream smtp: %v", ErrRelayPermanent, sendErr)
	}
	return fmt.Errorf("%w: upstream smtp: %v", ErrRelayTemporary, sendErr)
}

func isPermanentSMTPError(err error) bool {
	var smtpError *textproto.Error
	if !errors.As(err, &smtpError) {
		return false
	}
	return smtpError.Code >= 500 && smtpError.Code < 600
}
