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

type orderedForwardedMessage struct {
	headers []orderedMessageHeader
	body    io.Reader
}

type orderedMessageHeader struct {
	name  string
	value string
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
		forwarder.logger.Warn(
			"smtp_forwarding_rewrite_skipped",
			"route_address", route.Address().String(),
			"recipient_count", len(route.ForwardTo()),
			"error", rewriteErr,
		)
		rewrittenMessage = message.Data
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
	parsedMessage, parseErr := parseOrderedForwardedMessage(rawMessage)
	if parseErr != nil {
		return nil, fmt.Errorf("parse message headers: %w", parseErr)
	}
	return rewriteParsedForwardedMessage(route, parsedMessage)
}

func rewriteParsedForwardedMessage(route Route, parsedMessage orderedForwardedMessage) ([]byte, error) {
	body, bodyErr := io.ReadAll(parsedMessage.body)
	if bodyErr != nil {
		return nil, fmt.Errorf("read message body: %w", bodyErr)
	}

	originalFrom := firstHeaderValue(parsedMessage.headers, headerFrom)
	originalReplyTo := firstHeaderValue(parsedMessage.headers, headerReplyTo)
	replyTo := originalReplyTo
	if replyTo == "" {
		replyTo = originalFrom
	}

	var rewritten bytes.Buffer
	replacementWritten := false
	if !hasHeader(parsedMessage.headers, headerFrom) {
		writeForwardingIdentityHeaders(&rewritten, route, originalFrom, replyTo)
		replacementWritten = true
	}
	for _, messageHeader := range parsedMessage.headers {
		canonicalHeaderName := textproto.CanonicalMIMEHeaderKey(messageHeader.name)
		if canonicalHeaderName == textproto.CanonicalMIMEHeaderKey(headerFrom) && !replacementWritten {
			writeForwardingIdentityHeaders(&rewritten, route, originalFrom, replyTo)
			replacementWritten = true
			continue
		}
		if _, dropped := forwardingRewriteDroppedHeaders[canonicalHeaderName]; dropped {
			continue
		}
		writeMessageHeader(&rewritten, messageHeader.name, messageHeader.value)
	}
	rewritten.WriteString("\r\n")
	rewritten.Write(body)
	return rewritten.Bytes(), nil
}

func parseOrderedForwardedMessage(rawMessage []byte) (orderedForwardedMessage, error) {
	headerBlock, body, splitErr := splitMessageHeadersAndBody(rawMessage)
	if splitErr != nil {
		return orderedForwardedMessage{}, splitErr
	}
	headers, headerErr := parseOrderedHeaders(headerBlock)
	if headerErr != nil {
		return orderedForwardedMessage{}, headerErr
	}
	return orderedForwardedMessage{
		headers: headers,
		body:    bytes.NewReader(body),
	}, nil
}

func splitMessageHeadersAndBody(rawMessage []byte) ([]byte, []byte, error) {
	if splitIndex := bytes.Index(rawMessage, []byte("\r\n\r\n")); splitIndex >= 0 {
		return rawMessage[:splitIndex], rawMessage[splitIndex+4:], nil
	}
	if splitIndex := bytes.Index(rawMessage, []byte("\n\n")); splitIndex >= 0 {
		return rawMessage[:splitIndex], rawMessage[splitIndex+2:], nil
	}
	return nil, nil, errors.New("message header terminator is required")
}

func parseOrderedHeaders(headerBlock []byte) ([]orderedMessageHeader, error) {
	if len(headerBlock) == 0 {
		return nil, nil
	}
	lines := strings.Split(string(headerBlock), "\n")
	headers := make([]orderedMessageHeader, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSuffix(rawLine, "\r")
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if len(headers) == 0 {
				return nil, errors.New("message header continuation without header")
			}
			lastIndex := len(headers) - 1
			headers[lastIndex].value = headers[lastIndex].value + "\r\n" + line
			continue
		}
		nameEnd := strings.IndexByte(line, ':')
		if nameEnd <= 0 {
			return nil, fmt.Errorf("malformed message header line %q", line)
		}
		headerName := strings.TrimSpace(line[:nameEnd])
		if !validHeaderName(headerName) {
			return nil, fmt.Errorf("invalid message header name %q", headerName)
		}
		headers = append(headers, orderedMessageHeader{
			name:  headerName,
			value: strings.TrimSpace(line[nameEnd+1:]),
		})
	}
	return headers, nil
}

func validHeaderName(headerName string) bool {
	if headerName == "" {
		return false
	}
	for _, headerRune := range headerName {
		if headerRune <= 32 || headerRune >= 127 || headerRune == ':' {
			return false
		}
	}
	return true
}

func firstHeaderValue(headers []orderedMessageHeader, headerName string) string {
	canonicalHeaderName := textproto.CanonicalMIMEHeaderKey(headerName)
	for _, messageHeader := range headers {
		if textproto.CanonicalMIMEHeaderKey(messageHeader.name) == canonicalHeaderName {
			return sanitizedHeaderValue(messageHeader.value)
		}
	}
	return ""
}

func hasHeader(headers []orderedMessageHeader, headerName string) bool {
	canonicalHeaderName := textproto.CanonicalMIMEHeaderKey(headerName)
	for _, messageHeader := range headers {
		if textproto.CanonicalMIMEHeaderKey(messageHeader.name) == canonicalHeaderName {
			return true
		}
	}
	return false
}

func writeForwardingIdentityHeaders(buffer *bytes.Buffer, route Route, originalFrom string, replyTo string) {
	writeMessageHeader(buffer, headerFrom, forwardedFromHeader(route, originalFrom))
	if replyTo != "" {
		writeMessageHeader(buffer, headerReplyTo, replyTo)
	}
	if originalFrom != "" {
		writeMessageHeader(buffer, headerXOriginalFrom, originalFrom)
	}
}

func writeMessageHeader(buffer *bytes.Buffer, headerName string, headerValue string) {
	trimmedName := strings.TrimSpace(headerName)
	trimmedValue := sanitizedHeaderValue(headerValue)
	buffer.WriteString(trimmedName)
	buffer.WriteString(": ")
	buffer.WriteString(trimmedValue)
	buffer.WriteString("\r\n")
}

func sanitizedHeaderValue(headerValue string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(headerValue))
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
