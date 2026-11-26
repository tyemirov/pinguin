package smtpsubmission

import (
	"context"
	"errors"
	"time"

	"github.com/tyemirov/pingwin/internal/smtpidentity"
)

var (
	// ErrRelayTemporary indicates the upstream provider may accept a retry later.
	ErrRelayTemporary = errors.New("smtp_submission.relay_temporary")
	// ErrRelayPermanent indicates the upstream provider permanently rejected the message.
	ErrRelayPermanent = errors.New("smtp_submission.relay_permanent")
)

// Authenticator verifies SMTP credentials. It returns smtpidentity.ErrAuthenticationFailed
// for denied credentials and contextual errors when verification is unavailable.
type Authenticator interface {
	Authenticate(ctx context.Context, username string, password string) (smtpidentity.AuthenticatedIdentity, error)
}

// IdentityUsageRecorder persists the time of a successful SMTP authentication.
type IdentityUsageRecorder interface {
	RecordIdentityUse(ctx context.Context, identity smtpidentity.AuthenticatedIdentity, usedAt time.Time) error
}

// RawRelay forwards an accepted RFC 5322 message to an upstream provider.
type RawRelay interface {
	Relay(ctx context.Context, message RawMessage) error
}

// RawMessage carries a validated submission payload.
type RawMessage struct {
	IdentityID string
	From       smtpidentity.Address
	Recipients []smtpidentity.Address
	Data       []byte
}

// RecipientStrings returns normalized recipient addresses.
func (message RawMessage) RecipientStrings() []string {
	recipients := make([]string, 0, len(message.Recipients))
	for _, recipient := range message.Recipients {
		recipients = append(recipients, recipient.String())
	}
	return recipients
}
