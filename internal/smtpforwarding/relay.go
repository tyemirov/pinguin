package smtpforwarding

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// RawEmailSender relays a raw RFC 5322 message through an outbound SMTP profile.
type RawEmailSender interface {
	SendRawEmail(ctx context.Context, fromAddress string, recipients []string, rawMessage []byte) error
}

// RelayForwarder forwards inbound mail through a configured outbound SMTP relay.
type RelayForwarder struct {
	sender RawEmailSender
	logger *slog.Logger
}

// NewRelayForwarder constructs a relay-backed inbound forwarder.
func NewRelayForwarder(sender RawEmailSender, logger *slog.Logger) (*RelayForwarder, error) {
	if sender == nil {
		return nil, errors.New("smtp forwarding: raw email sender is required")
	}
	if logger == nil {
		return nil, errors.New("smtp forwarding: logger is required")
	}
	return &RelayForwarder{sender: sender, logger: logger}, nil
}

// Forward sends one raw inbound message to every recipient configured on the route.
func (forwarder *RelayForwarder) Forward(ctx context.Context, route Route, message Message) error {
	forwardErr := forwarder.sender.SendRawEmail(ctx, route.Address().String(), route.ForwardRecipientStrings(), message.Data)
	if forwardErr != nil {
		forwarder.logger.Error(
			"smtp_forwarding_relay_failed",
			"route_address", route.Address().String(),
			"recipient_count", len(route.ForwardTo()),
			"error", forwardErr,
		)
		return fmt.Errorf("%w: route %s: %v", ErrForwardTemporary, route.Address().String(), forwardErr)
	}
	return nil
}
