package smtpforwarding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"net/textproto"
	"sort"
	"strings"
)

const (
	headerARCAuthenticationResults = "ARC-Authentication-Results"
	headerARCMessageSignature      = "ARC-Message-Signature"
	headerARCSeal                  = "ARC-Seal"
	headerAuthenticationResults    = "Authentication-Results"
	headerBcc                      = "Bcc"
	headerDKIMSignature            = "DKIM-Signature"
	headerDomainKeySignature       = "DomainKey-Signature"
	headerFrom                     = "From"
	headerReplyTo                  = "Reply-To"
	headerReturnPath               = "Return-Path"
	headerSender                   = "Sender"
	headerXOriginalFrom            = "X-Pinguin-Original-From"
)

var forwardingRewriteDroppedHeaders = map[string]struct{}{
	textproto.CanonicalMIMEHeaderKey(headerARCAuthenticationResults): {},
	textproto.CanonicalMIMEHeaderKey(headerARCMessageSignature):      {},
	textproto.CanonicalMIMEHeaderKey(headerARCSeal):                  {},
	textproto.CanonicalMIMEHeaderKey(headerAuthenticationResults):    {},
	textproto.CanonicalMIMEHeaderKey(headerBcc):                      {},
	textproto.CanonicalMIMEHeaderKey(headerDKIMSignature):            {},
	textproto.CanonicalMIMEHeaderKey(headerDomainKeySignature):       {},
	textproto.CanonicalMIMEHeaderKey(headerFrom):                     {},
	textproto.CanonicalMIMEHeaderKey(headerReplyTo):                  {},
	textproto.CanonicalMIMEHeaderKey(headerReturnPath):               {},
	textproto.CanonicalMIMEHeaderKey(headerSender):                   {},
	textproto.CanonicalMIMEHeaderKey(headerXOriginalFrom):            {},
}

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
	rewrittenMessage, rewriteErr := rewriteForwardedMessage(route, message.Data)
	if rewriteErr != nil {
		forwarder.logger.Error(
			"smtp_forwarding_rewrite_failed",
			"route_address", route.Address().String(),
			"recipient_count", len(route.ForwardTo()),
			"error", rewriteErr,
		)
		return fmt.Errorf("%w: route %s: rewrite: %v", ErrForwardTemporary, route.Address().String(), rewriteErr)
	}
	forwardErr := forwarder.sender.SendRawEmail(ctx, route.Address().String(), route.ForwardRecipientStrings(), rewrittenMessage)
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

func rewriteForwardedMessage(route Route, rawMessage []byte) ([]byte, error) {
	parsedMessage, parseErr := mail.ReadMessage(bytes.NewReader(rawMessage))
	if parseErr != nil {
		return nil, fmt.Errorf("parse message headers: %w", parseErr)
	}
	return rewriteParsedForwardedMessage(route, parsedMessage)
}

func rewriteParsedForwardedMessage(route Route, parsedMessage *mail.Message) ([]byte, error) {
	body, bodyErr := io.ReadAll(parsedMessage.Body)
	if bodyErr != nil {
		return nil, fmt.Errorf("read message body: %w", bodyErr)
	}

	originalFrom := strings.TrimSpace(parsedMessage.Header.Get(headerFrom))
	originalReplyTo := strings.TrimSpace(parsedMessage.Header.Get(headerReplyTo))
	replyTo := originalReplyTo
	if replyTo == "" {
		replyTo = originalFrom
	}

	var rewritten bytes.Buffer
	writeMessageHeader(&rewritten, headerFrom, forwardedFromHeader(route, originalFrom))
	if replyTo != "" {
		writeMessageHeader(&rewritten, headerReplyTo, replyTo)
	}
	if originalFrom != "" {
		writeMessageHeader(&rewritten, headerXOriginalFrom, originalFrom)
	}
	headerNames := sortedForwardedHeaderNames(parsedMessage.Header)
	for _, headerName := range headerNames {
		for _, headerValue := range parsedMessage.Header[headerName] {
			writeMessageHeader(&rewritten, headerName, headerValue)
		}
	}
	rewritten.WriteString("\r\n")
	rewritten.Write(body)
	return rewritten.Bytes(), nil
}

func sortedForwardedHeaderNames(headers mail.Header) []string {
	headerNames := make([]string, 0, len(headers))
	for headerName := range headers {
		canonicalHeaderName := textproto.CanonicalMIMEHeaderKey(headerName)
		if _, dropped := forwardingRewriteDroppedHeaders[canonicalHeaderName]; dropped {
			continue
		}
		headerNames = append(headerNames, canonicalHeaderName)
	}
	sort.Strings(headerNames)
	return headerNames
}

func writeMessageHeader(buffer *bytes.Buffer, headerName string, headerValue string) {
	trimmedName := strings.TrimSpace(headerName)
	trimmedValue := strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(headerValue))
	buffer.WriteString(trimmedName)
	buffer.WriteString(": ")
	buffer.WriteString(trimmedValue)
	buffer.WriteString("\r\n")
}

func forwardedFromHeader(route Route, originalFrom string) string {
	trimmedOriginalFrom := strings.TrimSpace(originalFrom)
	if trimmedOriginalFrom == "" {
		return route.Address().String()
	}
	addresses, parseErr := mail.ParseAddressList(trimmedOriginalFrom)
	if parseErr != nil || len(addresses) == 0 {
		return (&mail.Address{Name: trimmedOriginalFrom, Address: route.Address().String()}).String()
	}
	displayName := strings.TrimSpace(addresses[0].Name)
	if displayName == "" {
		displayName = addresses[0].Address
	}
	return (&mail.Address{Name: displayName, Address: route.Address().String()}).String()
}
