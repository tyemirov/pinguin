package smtpforwarding

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/mail"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/internal/smtpidentity"
)

func TestRouteSetValidationAndAccessors(testHandle *testing.T) {
	routeAddress := mustAddress(testHandle, "support@help.example.com")
	firstRecipient := mustAddress(testHandle, "alice@example.com")
	secondRecipient := mustAddress(testHandle, "maria@example.com")
	route, routeErr := NewRoute(routeAddress, []smtpidentity.Address{firstRecipient, secondRecipient})
	if routeErr != nil {
		testHandle.Fatalf("new route: %v", routeErr)
	}
	if route.Address().String() != "support@help.example.com" {
		testHandle.Fatalf("unexpected route address %s", route.Address().String())
	}
	forwardTo := route.ForwardTo()
	forwardTo[0] = secondRecipient
	if route.ForwardTo()[0].String() != "alice@example.com" {
		testHandle.Fatalf("expected ForwardTo to return a copy")
	}
	recipientStrings := route.ForwardRecipientStrings()
	if strings.Join(recipientStrings, ",") != "alice@example.com,maria@example.com" {
		testHandle.Fatalf("unexpected recipient strings %v", recipientStrings)
	}

	secondRoute, secondRouteErr := NewRoute(
		mustAddress(testHandle, "sales@help.example.com"),
		[]smtpidentity.Address{mustAddress(testHandle, "sales@example.com")},
	)
	if secondRouteErr != nil {
		testHandle.Fatalf("new second route: %v", secondRouteErr)
	}
	routeSet, routeSetErr := NewRouteSet([]Route{secondRoute, route})
	if routeSetErr != nil {
		testHandle.Fatalf("new route set: %v", routeSetErr)
	}
	if resolved, exists, resolveErr := routeSet.Resolve(context.Background(), routeAddress); resolveErr != nil || !exists || resolved.Address().String() != routeAddress.String() {
		testHandle.Fatalf("expected route to resolve")
	}
	if _, exists, resolveErr := routeSet.Resolve(context.Background(), mustAddress(testHandle, "missing@help.example.com")); resolveErr != nil || exists {
		testHandle.Fatalf("unexpected missing route")
	}
	routes := routeSet.Routes()
	if len(routes) != 2 || routes[0].Address().String() != "sales@help.example.com" || routes[1].Address().String() != "support@help.example.com" {
		testHandle.Fatalf("expected sorted routes, got %+v", routes)
	}
}

func TestReversePathValidationAndAccessors(testHandle *testing.T) {
	nullReversePath, nullReversePathErr := NewReversePath("")
	if nullReversePathErr != nil {
		testHandle.Fatalf("new null reverse path: %v", nullReversePathErr)
	}
	if !nullReversePath.IsNull() || nullReversePath.String() != "" {
		testHandle.Fatalf("expected null reverse path, got %q", nullReversePath.String())
	}

	addressReversePath, addressReversePathErr := NewReversePath("Sender@Example.NET")
	if addressReversePathErr != nil {
		testHandle.Fatalf("new address reverse path: %v", addressReversePathErr)
	}
	if addressReversePath.IsNull() || addressReversePath.String() != "sender@example.net" {
		testHandle.Fatalf("unexpected address reverse path %q", addressReversePath.String())
	}
	address, exists := addressReversePath.Address()
	if !exists || address.String() != "sender@example.net" {
		testHandle.Fatalf("expected reverse path address, got %q exists=%v", address.String(), exists)
	}
	if _, exists := nullReversePath.Address(); exists {
		testHandle.Fatalf("null reverse path should not expose an address")
	}

	if _, reversePathErr := NewReversePath("not an address"); !errors.Is(reversePathErr, smtpidentity.ErrInvalidAddress) {
		testHandle.Fatalf("expected invalid address error, got %v", reversePathErr)
	}
}

func TestRouteValidationRejectsInvalidRoutes(testHandle *testing.T) {
	validAddress := mustAddress(testHandle, "support@help.example.com")
	validRecipient := mustAddress(testHandle, "owner@example.com")
	for _, testCase := range []struct {
		name       string
		route      Route
		routeSlice []Route
	}{
		{name: "empty address", route: Route{forwardTo: []smtpidentity.Address{validRecipient}}},
		{name: "empty recipient", route: Route{address: validAddress, forwardTo: []smtpidentity.Address{{}}}},
		{name: "no recipients", route: Route{address: validAddress}},
		{
			name:  "duplicate recipient",
			route: Route{address: validAddress, forwardTo: []smtpidentity.Address{validRecipient, validRecipient}},
		},
		{name: "empty route set address", routeSlice: []Route{{}}},
		{name: "duplicate route set", routeSlice: []Route{
			{address: validAddress, forwardTo: []smtpidentity.Address{validRecipient}},
			{address: validAddress, forwardTo: []smtpidentity.Address{validRecipient}},
		}},
		{name: "empty route set", routeSlice: []Route{}},
	} {
		testCase := testCase
		testHandle.Run(testCase.name, func(t *testing.T) {
			if testCase.routeSlice != nil {
				_, routeSetErr := NewRouteSet(testCase.routeSlice)
				if !errors.Is(routeSetErr, ErrInvalidRoute) {
					t.Fatalf("expected invalid route set error, got %v", routeSetErr)
				}
				return
			}
			_, routeErr := NewRoute(testCase.route.address, testCase.route.forwardTo)
			if !errors.Is(routeErr, ErrInvalidRoute) {
				t.Fatalf("expected invalid route error, got %v", routeErr)
			}
		})
	}
}

func TestRelayForwarder(testHandle *testing.T) {
	route := mustRoute(testHandle)
	message := Message{Data: []byte(strings.Join([]string{
		"From: Sender <sender@example.net>",
		"To: support@help.example.com",
		"Subject: Hello",
		"DKIM-Signature: v=1; a=rsa-sha256",
		"Return-Path: <sender@example.net>",
		"",
		"Hello",
	}, "\r\n"))}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	if _, err := NewRelayForwarder(nil, logger); err == nil {
		testHandle.Fatalf("expected nil sender error")
	}
	if _, err := NewRelayForwarder(&recordingRawEmailSender{}, nil); err == nil {
		testHandle.Fatalf("expected nil logger error")
	}

	sender := &recordingRawEmailSender{}
	forwarder, forwarderErr := NewRelayForwarder(sender, logger)
	if forwarderErr != nil {
		testHandle.Fatalf("new forwarder: %v", forwarderErr)
	}
	if err := forwarder.Forward(context.Background(), route, message); err != nil {
		testHandle.Fatalf("forward: %v", err)
	}
	if sender.fromAddress != route.Address().String() {
		testHandle.Fatalf("unexpected sender %q", sender.fromAddress)
	}
	if strings.Join(sender.recipients, ",") != "owner@example.com" {
		testHandle.Fatalf("unexpected recipients %v", sender.recipients)
	}
	parsedMessage := mustMailMessage(testHandle, sender.rawMessage)
	if parsedMessage.Header.Get("From") != `"Sender" <support@help.example.com>` {
		testHandle.Fatalf("expected rewritten From with original display name, got %q", parsedMessage.Header.Get("From"))
	}
	if parsedMessage.Header.Get("Reply-To") != "Sender <sender@example.net>" {
		testHandle.Fatalf("expected original From as Reply-To, got %q", parsedMessage.Header.Get("Reply-To"))
	}
	if parsedMessage.Header.Get("X-Pinguin-Original-From") != "Sender <sender@example.net>" {
		testHandle.Fatalf("expected original From trace header, got %q", parsedMessage.Header.Get("X-Pinguin-Original-From"))
	}
	if parsedMessage.Header.Get("To") != "support@help.example.com" {
		testHandle.Fatalf("expected original To header, got %q", parsedMessage.Header.Get("To"))
	}
	if parsedMessage.Header.Get("Subject") != "Hello" {
		testHandle.Fatalf("expected original Subject header, got %q", parsedMessage.Header.Get("Subject"))
	}
	if parsedMessage.Header.Get("DKIM-Signature") != "" {
		testHandle.Fatalf("expected stale DKIM-Signature to be stripped")
	}
	if parsedMessage.Header.Get("Return-Path") != "" {
		testHandle.Fatalf("expected Return-Path header to be stripped")
	}
	parsedBody, parsedBodyErr := io.ReadAll(parsedMessage.Body)
	if parsedBodyErr != nil {
		testHandle.Fatalf("read rewritten body: %v", parsedBodyErr)
	}
	if string(parsedBody) != "Hello" {
		testHandle.Fatalf("expected original body, got %q", string(parsedBody))
	}

	sender.err = errors.New("relay down")
	if err := forwarder.Forward(context.Background(), route, message); !errors.Is(err, ErrForwardTemporary) {
		testHandle.Fatalf("expected temporary forward error, got %v", err)
	}

	sender.err = nil
	invalidMessage := Message{Data: []byte("not a header\r\n\r\nHello")}
	if err := forwarder.Forward(context.Background(), route, invalidMessage); err != nil {
		testHandle.Fatalf("forward invalid raw message: %v", err)
	}
	if string(sender.rawMessage) != string(invalidMessage.Data) {
		testHandle.Fatalf("expected unparseable message to be relayed raw")
	}
}

func TestRelayForwarderPreservesOriginalReplyTo(testHandle *testing.T) {
	route := mustRoute(testHandle)
	message := Message{Data: []byte(strings.Join([]string{
		"From: Sender <sender@example.net>",
		"Reply-To: Reply Channel <reply-channel@example.net>",
		"Subject: Hello",
		"",
		"Hello",
	}, "\r\n"))}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	sender := &recordingRawEmailSender{}
	forwarder, forwarderErr := NewRelayForwarder(sender, logger)
	if forwarderErr != nil {
		testHandle.Fatalf("new forwarder: %v", forwarderErr)
	}
	if err := forwarder.Forward(context.Background(), route, message); err != nil {
		testHandle.Fatalf("forward: %v", err)
	}

	parsedMessage := mustMailMessage(testHandle, sender.rawMessage)
	if parsedMessage.Header.Get("From") != `"Sender" <support@help.example.com>` {
		testHandle.Fatalf("expected rewritten From with original display name, got %q", parsedMessage.Header.Get("From"))
	}
	if parsedMessage.Header.Get("Reply-To") != "Reply Channel <reply-channel@example.net>" {
		testHandle.Fatalf("expected original Reply-To to be preserved, got %q", parsedMessage.Header.Get("Reply-To"))
	}
	if parsedMessage.Header.Get("X-Pinguin-Original-From") != "Sender <sender@example.net>" {
		testHandle.Fatalf("expected original From trace header, got %q", parsedMessage.Header.Get("X-Pinguin-Original-From"))
	}
}

func TestRelayForwarderPreservesTraceAndResentHeaderOrder(testHandle *testing.T) {
	route := mustRoute(testHandle)
	message := Message{Data: []byte(strings.Join([]string{
		"Received: from mx-two.example by mx.pinguin.example",
		"Received: from mx-one.example by mx-two.example",
		"Resent-Date: Thu, 14 May 2026 18:00:00 +0000",
		"Resent-From: Relay <relay@example.net>",
		"From: Sender <sender@example.net>",
		"To: support@help.example.com",
		"Subject: Hello",
		"",
		"Hello",
	}, "\r\n"))}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	sender := &recordingRawEmailSender{}
	forwarder, forwarderErr := NewRelayForwarder(sender, logger)
	if forwarderErr != nil {
		testHandle.Fatalf("new forwarder: %v", forwarderErr)
	}
	if err := forwarder.Forward(context.Background(), route, message); err != nil {
		testHandle.Fatalf("forward: %v", err)
	}

	wantHeaderLines := []string{
		"Received: from mx-two.example by mx.pinguin.example",
		"Received: from mx-one.example by mx-two.example",
		"Resent-Date: Thu, 14 May 2026 18:00:00 +0000",
		"Resent-From: Relay <relay@example.net>",
		`From: "Sender" <support@help.example.com>`,
		"Reply-To: Sender <sender@example.net>",
		"X-Pinguin-Original-From: Sender <sender@example.net>",
		"To: support@help.example.com",
		"Subject: Hello",
	}
	if gotHeaderLines := rawMessageHeaderLines(sender.rawMessage); strings.Join(gotHeaderLines, "\n") != strings.Join(wantHeaderLines, "\n") {
		testHandle.Fatalf("unexpected header order:\n%s", strings.Join(gotHeaderLines, "\n"))
	}
}

func TestRewriteParsedForwardedMessageReportsBodyReadError(testHandle *testing.T) {
	route := mustRoute(testHandle)
	parsedMessage := orderedForwardedMessage{
		headers: []orderedMessageHeader{{name: headerFrom, value: "sender@example.net"}},
		body:    failingMessageBody{},
	}
	if _, rewriteErr := rewriteParsedForwardedMessage(route, parsedMessage); rewriteErr == nil || !strings.Contains(rewriteErr.Error(), "read message body") {
		testHandle.Fatalf("expected body read error, got %v", rewriteErr)
	}
}

func TestParseOrderedForwardedMessage(testHandle *testing.T) {
	for _, testCase := range []struct {
		name       string
		rawMessage string
		wantErr    string
	}{
		{name: "empty headers", rawMessage: "\r\n\r\nBody"},
		{name: "lf delimiter", rawMessage: "From: sender@example.net\n\nHello"},
		{name: "folded header", rawMessage: "Subject: Hello\n world\n\nBody"},
		{name: "missing terminator", rawMessage: "From: sender@example.net", wantErr: "message header terminator is required"},
		{name: "continuation without header", rawMessage: " folded\r\n\r\nBody", wantErr: "message header continuation without header"},
		{name: "leading whitespace before header", rawMessage: " : value\r\n\r\nBody", wantErr: "message header continuation without header"},
		{name: "invalid header name", rawMessage: "Bad Name: value\r\n\r\nBody", wantErr: "invalid message header name"},
	} {
		testCase := testCase
		testHandle.Run(testCase.name, func(t *testing.T) {
			parsedMessage, parseErr := parseOrderedForwardedMessage([]byte(testCase.rawMessage))
			if testCase.wantErr != "" {
				if parseErr == nil || !strings.Contains(parseErr.Error(), testCase.wantErr) {
					t.Fatalf("expected %q error, got %v", testCase.wantErr, parseErr)
				}
				return
			}
			if parseErr != nil {
				t.Fatalf("parse message: %v", parseErr)
			}
			if testCase.name != "empty headers" && len(parsedMessage.headers) == 0 {
				t.Fatalf("expected parsed headers")
			}
		})
	}
}

func TestRewriteForwardedMessageWithoutOriginalFrom(testHandle *testing.T) {
	route := mustRoute(testHandle)
	rewrittenMessage, rewriteErr := rewriteForwardedMessage(route, []byte("Subject: Hello\r\n\r\nBody"))
	if rewriteErr != nil {
		testHandle.Fatalf("rewrite message: %v", rewriteErr)
	}
	gotHeaderLines := rawMessageHeaderLines(rewrittenMessage)
	if len(gotHeaderLines) < 2 || gotHeaderLines[0] != "From: support@help.example.com" || gotHeaderLines[1] != "Subject: Hello" {
		testHandle.Fatalf("unexpected no-from rewrite:\n%s", strings.Join(gotHeaderLines, "\n"))
	}
}

func TestValidHeaderNameRejectsEmptyValue(testHandle *testing.T) {
	if validHeaderName("") {
		testHandle.Fatalf("expected empty header name to be invalid")
	}
}

func TestForwardedFromHeaderUsesBestOriginalSenderLabel(testHandle *testing.T) {
	route := mustRoute(testHandle)
	for _, testCase := range []struct {
		name         string
		originalFrom string
		wantHeader   string
	}{
		{name: "blank", wantHeader: "support@help.example.com"},
		{name: "display name", originalFrom: "Sender <sender@example.net>", wantHeader: `"Sender" <support@help.example.com>`},
		{name: "address only", originalFrom: "sender@example.net", wantHeader: `"sender@example.net" <support@help.example.com>`},
		{name: "unparseable", originalFrom: "Sender at Example", wantHeader: `"Sender at Example" <support@help.example.com>`},
	} {
		testCase := testCase
		testHandle.Run(testCase.name, func(t *testing.T) {
			if gotHeader := forwardedFromHeader(route, testCase.originalFrom); gotHeader != testCase.wantHeader {
				t.Fatalf("expected %q, got %q", testCase.wantHeader, gotHeader)
			}
		})
	}
}

type recordingRawEmailSender struct {
	fromAddress string
	recipients  []string
	rawMessage  []byte
	err         error
}

func (sender *recordingRawEmailSender) SendRawEmail(_ context.Context, fromAddress string, recipients []string, rawMessage []byte) error {
	sender.fromAddress = fromAddress
	sender.recipients = append([]string(nil), recipients...)
	sender.rawMessage = append([]byte(nil), rawMessage...)
	return sender.err
}

func mustAddress(testHandle *testing.T, rawAddress string) smtpidentity.Address {
	testHandle.Helper()
	address, addressErr := smtpidentity.NewAddress(rawAddress)
	if addressErr != nil {
		testHandle.Fatalf("address %s: %v", rawAddress, addressErr)
	}
	return address
}

func mustReversePath(testHandle *testing.T, rawAddress string) ReversePath {
	testHandle.Helper()
	reversePath, reversePathErr := NewReversePath(rawAddress)
	if reversePathErr != nil {
		testHandle.Fatalf("reverse path %s: %v", rawAddress, reversePathErr)
	}
	return reversePath
}

func mustRoute(testHandle *testing.T) Route {
	testHandle.Helper()
	route, routeErr := NewRoute(
		mustAddress(testHandle, "support@help.example.com"),
		[]smtpidentity.Address{mustAddress(testHandle, "owner@example.com")},
	)
	if routeErr != nil {
		testHandle.Fatalf("route: %v", routeErr)
	}
	return route
}

type failingMessageBody struct{}

func (failingMessageBody) Read(_ []byte) (int, error) {
	return 0, errors.New("body read failed")
}

func mustMailMessage(testHandle *testing.T, rawMessage []byte) *mail.Message {
	testHandle.Helper()
	parsedMessage, parseErr := mail.ReadMessage(bytes.NewReader(rawMessage))
	if parseErr != nil {
		testHandle.Fatalf("parse message: %v\n%s", parseErr, string(rawMessage))
	}
	return parsedMessage
}

func rawMessageHeaderLines(rawMessage []byte) []string {
	headerBlock := strings.SplitN(string(rawMessage), "\r\n\r\n", 2)[0]
	return strings.Split(headerBlock, "\r\n")
}
