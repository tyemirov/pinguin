package smtpsubmission

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"log/slog"
)

func TestSMTPSubmissionStartTLSPlainAuthRelaysRawMessage(t *testing.T) {
	fixture := newSMTPServerFixture(t, false, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "EHLO gmail.example")
	client.expectCode(t, "250")
	client.send(t, "STARTTLS")
	client.expectCode(t, "220")
	client.startTLS(t)
	client.send(t, "EHLO gmail.example")
	client.expectCode(t, "250")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "235")
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "354")
	client.sendData(t, "From: Alice <alice@example.com>\r\nTo: recipient@example.net\r\nSubject: Test\r\n\r\nHello")
	client.expectCode(t, "250")

	if len(fixture.relay.messages) != 1 {
		t.Fatalf("expected 1 relayed message, got %d", len(fixture.relay.messages))
	}
	relayed := fixture.relay.messages[0]
	if relayed.From.String() != "alice@example.com" {
		t.Fatalf("unexpected relay sender %s", relayed.From.String())
	}
	if len(relayed.Recipients) != 1 || relayed.Recipients[0].String() != "recipient@example.net" {
		t.Fatalf("unexpected relay recipients %#v", relayed.Recipients)
	}
	if !strings.Contains(string(relayed.Data), "Subject: Test") {
		t.Fatalf("raw message was not preserved: %q", string(relayed.Data))
	}
}

func TestSMTPSubmissionLoginAuthRelaysMessage(t *testing.T) {
	fixture := newSMTPServerFixture(t, false, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "EHLO gmail.example")
	client.expectCode(t, "250")
	client.send(t, "STARTTLS")
	client.expectCode(t, "220")
	client.startTLS(t)
	client.send(t, "AUTH LOGIN")
	client.expectCode(t, "334")
	client.send(t, base64.StdEncoding.EncodeToString([]byte("smtp-user")))
	client.expectCode(t, "334")
	client.send(t, base64.StdEncoding.EncodeToString([]byte("smtp-pass")))
	client.expectCode(t, "235")
}

func TestSMTPSubmissionPlainAuthChallengeRelaysMessage(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN")
	client.expectCode(t, "334")
	client.send(t, plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "235")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "503")
}

func TestSMTPSubmissionCommandOrderingAndSessionCommands(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "EHLO gmail.example")
	client.expectCode(t, "250")
	client.send(t, "NOOP")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "503")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "503")
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "530")
	client.send(t, "AUTH CRAM-MD5")
	client.expectCode(t, "504")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "235")
	client.send(t, "MAIL FROM:alice@example.com extra")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:recipient@example.net extra")
	client.expectCode(t, "250")
	client.send(t, "RSET")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "503")
	client.send(t, "VRFY alice")
	client.expectCode(t, "502")
	client.send(t, "QUIT")
	client.expectCode(t, "221")
}

func TestSMTPSubmissionRejectsMalformedAuthAndPaths(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN not-base64")
	client.expectCode(t, "535")
	client.send(t, "AUTH PLAIN "+base64.StdEncoding.EncodeToString([]byte("missing-separators")))
	client.expectCode(t, "535")
	client.send(t, "AUTH LOGIN not-base64")
	client.expectCode(t, "535")
	client.send(t, "AUTH LOGIN "+base64.StdEncoding.EncodeToString([]byte("smtp-user")))
	client.expectCode(t, "334")
	client.send(t, "not-base64")
	client.expectCode(t, "535")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "235")
	client.send(t, "MAIL BODY:<alice@example.com>")
	client.expectCode(t, "501")
	client.send(t, "MAIL FROM:<alice@example.com")
	client.expectCode(t, "501")
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT BODY:<recipient@example.net>")
	client.expectCode(t, "501")
}

func TestSMTPSubmissionRejectsAuthBeforeTLS(t *testing.T) {
	fixture := newSMTPServerFixture(t, false, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "EHLO gmail.example")
	client.expectCode(t, "250")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "530")
}

func TestSMTPSubmissionStartTLSUnavailableAndAlreadyActive(t *testing.T) {
	address, addressErr := smtpidentity.NewAddress("alice@example.com")
	if addressErr != nil {
		t.Fatalf("identity address: %v", addressErr)
	}
	server, serverErr := NewServer(Config{
		Hostname:          "smtp.test",
		MaxMessageBytes:   1024 * 1024,
		MaxRecipients:     10,
		AllowInsecureAuth: true,
		Authenticator: &staticAuthenticator{
			username: "smtp-user",
			password: "smtp-pass",
			identity: smtpidentity.AuthenticatedIdentity{
				ID:           "identity-1",
				EmailAddress: address,
				Username:     "smtp-user",
			},
		},
		Relay:  &recordingRelay{},
		Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
	})
	if serverErr != nil {
		t.Fatalf("new server: %v", serverErr)
	}
	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatalf("listen: %v", listenErr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		listener.Close()
	})
	go func() {
		_ = server.Serve(ctx, listener, false)
	}()

	client := newSMTPTestClient(mustDial(t, listener.Addr().String()))
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "STARTTLS")
	client.expectCode(t, "454")

	secureFixture := newSMTPServerFixture(t, false, nil)
	secureClient := secureFixture.dial(t)
	defer secureClient.close()
	secureClient.expectCode(t, "220")
	secureClient.send(t, "STARTTLS")
	secureClient.expectCode(t, "220")
	secureClient.startTLS(t)
	secureClient.send(t, "STARTTLS")
	secureClient.expectCode(t, "503")
}

func TestSMTPSubmissionRejectsWrongPassword(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "wrong-pass"))
	client.expectCode(t, "535")
}

func TestSMTPSubmissionRejectsUnknownIdentityLikeBadPassword(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("unknown-user", "smtp-pass"))
	client.expectCode(t, "535")
}

func TestSMTPSubmissionRejectsMailFromMismatch(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.authenticatedClient(t)
	defer client.close()
	client.send(t, "MAIL FROM:<other@example.com>")
	client.expectCode(t, "553")
}

func TestSMTPSubmissionRejectsHeaderFromMismatchAfterData(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.authenticatedClient(t)
	defer client.close()
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "354")
	client.sendData(t, "From: other@example.com\r\nTo: recipient@example.net\r\nSubject: Bad\r\n\r\nHello")
	client.expectCode(t, "553")
	if len(fixture.relay.messages) != 0 {
		t.Fatalf("expected no relay on header mismatch")
	}
}

func TestSMTPSubmissionRejectsMalformedMessageFromHeader(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.authenticatedClient(t)
	defer client.close()
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "354")
	client.sendData(t, "Subject: Missing From\r\n\r\nHello")
	client.expectCode(t, "553")
}

func TestSMTPSubmissionUnescapesDotStuffedData(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	client := fixture.authenticatedClient(t)
	defer client.close()
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "354")
	client.sendData(t, "From: alice@example.com\r\n\r\n..dot-started")
	client.expectCode(t, "250")
	if len(fixture.relay.messages) != 1 {
		t.Fatalf("expected relayed message")
	}
	if !strings.Contains(string(fixture.relay.messages[0].Data), "\r\n.dot-started") {
		t.Fatalf("expected dot-stuffed line to be unescaped, got %q", string(fixture.relay.messages[0].Data))
	}
}

func TestSMTPSubmissionRejectsOversizedMessage(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	fixture.server.config.MaxMessageBytes = 16
	client := fixture.authenticatedClient(t)
	defer client.close()
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "354")
	client.sendData(t, "From: alice@example.com\r\n\r\nThis message is too long")
	client.expectCode(t, "552")
}

func TestSMTPSubmissionRejectsOversizedMessageLine(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	payloadPrefix := "From: alice@example.com\r\n\r\n"
	fixture.server.config.MaxMessageBytes = int64(len(payloadPrefix) + 8)
	client := fixture.authenticatedClient(t)
	defer client.close()
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "354")
	client.sendData(t, payloadPrefix+strings.Repeat("A", 64*1024))
	client.expectCode(t, "552")
	if len(fixture.relay.messages) != 0 {
		t.Fatalf("expected no relay for oversized line")
	}
}

func TestSMTPSubmissionRejectsTooManyRecipients(t *testing.T) {
	fixture := newSMTPServerFixture(t, true, nil)
	fixture.server.config.MaxRecipients = 1
	client := fixture.authenticatedClient(t)
	defer client.close()
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<one@example.net>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<two@example.net>")
	client.expectCode(t, "452")
}

func TestSMTPSubmissionRejectsConcurrentSessionsOverGlobalLimit(t *testing.T) {
	fixture := newSMTPServerFixtureWithConfig(t, true, nil, func(cfg *Config) {
		cfg.MaxConcurrentSessions = 1
		cfg.MaxSessionsPerRemoteHost = 10
	})
	firstClient := fixture.dial(t)
	firstClient.expectCode(t, "220")
	secondClient := fixture.dial(t)
	secondClient.expectCode(t, "421")
	secondClient.close()
	firstClient.close()
	waitForActiveSMTPSessions(t, fixture.server, 0)
	thirdClient := fixture.dial(t)
	defer thirdClient.close()
	thirdClient.expectCode(t, "220")
}

func TestSMTPSubmissionRejectsConcurrentSessionsOverRemoteHostLimit(t *testing.T) {
	fixture := newSMTPServerFixtureWithConfig(t, true, nil, func(cfg *Config) {
		cfg.MaxConcurrentSessions = 10
		cfg.MaxSessionsPerRemoteHost = 1
	})
	firstClient := fixture.dial(t)
	defer firstClient.close()
	firstClient.expectCode(t, "220")
	secondClient := fixture.dial(t)
	defer secondClient.close()
	secondClient.expectCode(t, "421")
}

func TestSMTPSubmissionClosesIdleSessions(t *testing.T) {
	fixture := newSMTPServerFixtureWithConfig(t, true, nil, func(cfg *Config) {
		cfg.CommandTimeout = 20 * time.Millisecond
	})
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	if deadlineErr := client.connection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
		t.Fatalf("set client read deadline: %v", deadlineErr)
	}
	_, readErr := client.reader.ReadString('\n')
	if readErr == nil {
		t.Fatalf("expected idle session to close")
	}
	if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
		t.Fatalf("expected server to close idle session before client deadline")
	}
}

func TestSMTPSubmissionThrottlesRepeatedAuthFailures(t *testing.T) {
	fixture := newSMTPServerFixtureWithConfig(t, true, nil, func(cfg *Config) {
		cfg.AuthFailureLimit = 2
		cfg.AuthFailureWindow = time.Hour
	})
	currentTime := time.Date(2026, time.May, 5, 12, 0, 0, 0, time.UTC)
	fixture.server.throttle.clockFunc = func() time.Time { return currentTime }
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "wrong-pass"))
	client.expectCode(t, "535")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "wrong-pass"))
	client.expectCode(t, "535")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "454")
	currentTime = currentTime.Add(2 * time.Hour)
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "235")
}

func TestSMTPSubmissionThrottlesAcceptedMessagesPerIdentity(t *testing.T) {
	fixture := newSMTPServerFixtureWithConfig(t, true, nil, func(cfg *Config) {
		cfg.MessageLimit = 1
		cfg.MessageWindow = time.Hour
	})
	currentTime := time.Date(2026, time.May, 5, 12, 0, 0, 0, time.UTC)
	fixture.server.throttle.clockFunc = func() time.Time { return currentTime }
	client := fixture.authenticatedClient(t)
	defer client.close()
	sendAcceptedSMTPMessage(t, client, "First")
	client.expectCode(t, "250")
	sendAcceptedSMTPMessage(t, client, "Second")
	client.expectCode(t, "452")
	if len(fixture.relay.messages) != 1 {
		t.Fatalf("expected only first message to relay, got %d", len(fixture.relay.messages))
	}
	currentTime = currentTime.Add(2 * time.Hour)
	sendAcceptedSMTPMessage(t, client, "Third")
	client.expectCode(t, "250")
	if len(fixture.relay.messages) != 2 {
		t.Fatalf("expected third message after window reset, got %d", len(fixture.relay.messages))
	}
}

func TestSMTPSubmissionMapsRelayFailures(t *testing.T) {
	testCases := []struct {
		name         string
		relayError   error
		expectedCode string
	}{
		{name: "Temporary", relayError: ErrRelayTemporary, expectedCode: "451"},
		{name: "Permanent", relayError: ErrRelayPermanent, expectedCode: "554"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newSMTPServerFixture(t, true, testCase.relayError)
			client := fixture.authenticatedClient(t)
			defer client.close()
			client.send(t, "MAIL FROM:<alice@example.com>")
			client.expectCode(t, "250")
			client.send(t, "RCPT TO:<recipient@example.net>")
			client.expectCode(t, "250")
			client.send(t, "DATA")
			client.expectCode(t, "354")
			client.sendData(t, "From: alice@example.com\r\nTo: recipient@example.net\r\n\r\nHello")
			client.expectCode(t, testCase.expectedCode)
		})
	}
}

func TestSMTPSubmissionStartReturnsFatalListenerError(t *testing.T) {
	address, addressErr := smtpidentity.NewAddress("alice@example.com")
	if addressErr != nil {
		t.Fatalf("identity address: %v", addressErr)
	}
	server, serverErr := NewServer(Config{
		Hostname:          "smtp.test",
		TLSConfig:         testTLSConfig(t),
		MaxMessageBytes:   1024 * 1024,
		MaxRecipients:     10,
		AllowInsecureAuth: true,
		Authenticator: &staticAuthenticator{
			username: "smtp-user",
			password: "smtp-pass",
			identity: smtpidentity.AuthenticatedIdentity{
				ID:           "identity-1",
				EmailAddress: address,
				Username:     "smtp-user",
			},
		},
		Relay:  &recordingRelay{},
		Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
	})
	if serverErr != nil {
		t.Fatalf("new server: %v", serverErr)
	}

	fatalErr := errors.New("fatal accept")
	fatalListener := newFatalAcceptListener(fatalErr)
	blockingListener := newBlockingAcceptListener()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.serveListeners(ctx, []smtpListener{
			{listener: fatalListener},
			{listener: blockingListener, implicitTLS: true},
		})
	}()

	select {
	case serveErr := <-errChan:
		if !errors.Is(serveErr, fatalErr) {
			t.Fatalf("expected fatal listener error, got %v", serveErr)
		}
	case <-time.After(time.Second):
		cancel()
		blockingListener.Close()
		t.Fatalf("expected fatal listener error without waiting for every listener")
	}
	if !blockingListener.wasClosed() {
		t.Fatalf("expected remaining listener to be closed after fatal error")
	}
}

func TestSMTPSubmissionStartTLSListenerStopsWithContext(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	server.config.TLSListenAddr = "127.0.0.1:0"
	server.config.TLSConfig = testTLSConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("expected canceled TLS listener start to stop cleanly, got %v", err)
	}
}

func TestSMTPSubmissionServeListenersReturnsNilWhenClosed(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	first := newBlockingAcceptListener()
	second := newBlockingAcceptListener()
	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.serveListeners(ctx, []smtpListener{
			{listener: first},
			{listener: second, implicitTLS: true},
		})
	}()
	cancel()
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("expected nil serve listener result, got %v", err)
		}
	case <-time.After(time.Second):
		first.Close()
		second.Close()
		t.Fatalf("expected listeners to stop after context cancellation")
	}
}

func TestSMTPSubmissionConstructorAndStartValidation(t *testing.T) {
	address, addressErr := smtpidentity.NewAddress("alice@example.com")
	if addressErr != nil {
		t.Fatalf("identity address: %v", addressErr)
	}
	authenticator := &staticAuthenticator{
		username: "smtp-user",
		password: "smtp-pass",
		identity: smtpidentity.AuthenticatedIdentity{
			ID:           "identity-1",
			EmailAddress: address,
			Username:     "smtp-user",
		},
	}
	validConfig := Config{
		Hostname:          "smtp.test",
		MaxMessageBytes:   1024,
		MaxRecipients:     5,
		AllowInsecureAuth: true,
		Authenticator:     authenticator,
		Relay:             &recordingRelay{},
		Logger:            slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
	}
	testCases := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "hostname", mutate: func(cfg *Config) { cfg.Hostname = "" }},
		{name: "authenticator", mutate: func(cfg *Config) { cfg.Authenticator = nil }},
		{name: "relay", mutate: func(cfg *Config) { cfg.Relay = nil }},
		{name: "logger", mutate: func(cfg *Config) { cfg.Logger = nil }},
		{name: "tls listener config", mutate: func(cfg *Config) { cfg.TLSListenAddr = "127.0.0.1:0" }},
		{name: "starttls config", mutate: func(cfg *Config) {
			cfg.ListenAddr = "127.0.0.1:0"
			cfg.AllowInsecureAuth = false
		}},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			cfg := validConfig
			testCase.mutate(&cfg)
			if _, err := NewServer(cfg); err == nil {
				t.Fatalf("expected constructor error")
			}
		})
	}

	defaulted, defaultErr := NewServer(Config{
		Hostname:          "smtp.test",
		AllowInsecureAuth: true,
		Authenticator:     authenticator,
		Relay:             &recordingRelay{},
		Logger:            slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
	})
	if defaultErr != nil {
		t.Fatalf("defaulted server: %v", defaultErr)
	}
	if defaulted.config.MaxMessageBytes != defaultMaxMessageBytes || defaulted.config.MaxRecipients != defaultMaxRecipients {
		t.Fatalf("expected default limits, got %d/%d", defaulted.config.MaxMessageBytes, defaulted.config.MaxRecipients)
	}
	if defaulted.config.CommandTimeout != defaultCommandTimeout ||
		defaulted.config.MaxConcurrentSessions != defaultMaxConcurrentSessions ||
		defaulted.config.MaxSessionsPerRemoteHost != defaultMaxSessionsPerRemoteHost ||
		defaulted.config.AuthFailureLimit != defaultAuthFailureLimit ||
		defaulted.config.AuthFailureWindow != defaultAuthFailureWindow ||
		defaulted.config.MessageLimit != defaultMessageLimit ||
		defaulted.config.MessageWindow != defaultMessageWindow {
		t.Fatalf("expected default throttle limits, got %+v", defaulted.config)
	}
	if err := defaulted.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "no listeners configured") {
		t.Fatalf("expected no listeners error, got %v", err)
	}
	defaulted.config.ListenAddr = "bad address"
	if err := defaulted.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("expected listen error, got %v", err)
	}
	defaulted.config.ListenAddr = "127.0.0.1:0"
	defaulted.config.TLSListenAddr = "bad address"
	defaulted.config.TLSConfig = testTLSConfig(t)
	if err := defaulted.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "tls listen") {
		t.Fatalf("expected tls listen error, got %v", err)
	}
}

func TestSMTPSubmissionReaderAndWriterErrorPaths(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	t.Run("plain auth challenge write error", func(t *testing.T) {
		_, _, err := server.readPlainAuth(bufio.NewReader(strings.NewReader("")), bufio.NewWriterSize(failingSMTPWriter{}, 1), "")
		if err == nil {
			t.Fatalf("expected plain auth write error")
		}
	})
	t.Run("plain auth challenge read error", func(t *testing.T) {
		var output bytes.Buffer
		_, _, err := server.readPlainAuth(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(&output), "")
		if err == nil {
			t.Fatalf("expected plain auth read error")
		}
	})
	t.Run("login username prompt write error", func(t *testing.T) {
		_, _, err := server.readLoginAuth(bufio.NewReader(strings.NewReader("")), bufio.NewWriterSize(failingSMTPWriter{}, 1), "")
		if err == nil {
			t.Fatalf("expected login username prompt write error")
		}
	})
	t.Run("login username read error", func(t *testing.T) {
		var output bytes.Buffer
		_, _, err := server.readLoginAuth(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(&output), "")
		if err == nil {
			t.Fatalf("expected login username read error")
		}
	})
	t.Run("login password prompt write error", func(t *testing.T) {
		_, _, err := server.readLoginAuth(
			bufio.NewReader(strings.NewReader("")),
			bufio.NewWriterSize(failingSMTPWriter{}, 1),
			base64.StdEncoding.EncodeToString([]byte("smtp-user")),
		)
		if err == nil {
			t.Fatalf("expected login password prompt write error")
		}
	})
	t.Run("login password read error", func(t *testing.T) {
		var output bytes.Buffer
		_, _, err := server.readLoginAuth(
			bufio.NewReader(strings.NewReader("")),
			bufio.NewWriter(&output),
			base64.StdEncoding.EncodeToString([]byte("smtp-user")),
		)
		if err == nil {
			t.Fatalf("expected login password read error")
		}
	})
	t.Run("data prompt write error", func(t *testing.T) {
		from := mustSMTPSubmissionAddress(t, "alice@example.com")
		session := &sessionState{
			authenticated: &smtpidentity.AuthenticatedIdentity{ID: "identity", EmailAddress: from},
			mailFrom:      &from,
			recipients:    []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
		}
		server.handleData(context.Background(), failingSMTPConn{}, bufio.NewReader(strings.NewReader("")), bufio.NewWriterSize(failingSMTPWriter{}, 1), session)
	})
	t.Run("data read error after prompt", func(t *testing.T) {
		from := mustSMTPSubmissionAddress(t, "alice@example.com")
		session := &sessionState{
			authenticated: &smtpidentity.AuthenticatedIdentity{ID: "identity", EmailAddress: from},
			mailFrom:      &from,
			recipients:    []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
		}
		var output bytes.Buffer
		server.handleData(context.Background(), failingSMTPConn{}, bufio.NewReader(strings.NewReader("unterminated")), bufio.NewWriter(&output), session)
		if !strings.Contains(output.String(), "354") {
			t.Fatalf("expected data prompt before read error, got %q", output.String())
		}
	})
	t.Run("data read eof", func(t *testing.T) {
		if _, _, err := server.readData(bufio.NewReader(strings.NewReader("unterminated"))); !errors.Is(err, io.EOF) {
			t.Fatalf("expected EOF from unterminated data, got %v", err)
		}
	})
	t.Run("data read non eof", func(t *testing.T) {
		readErr := errors.New("read failed")
		if _, _, err := server.readData(bufio.NewReader(smtpErrorReader{err: readErr})); !errors.Is(err, readErr) {
			t.Fatalf("expected custom read error, got %v", err)
		}
	})
	t.Run("message parse read error", func(t *testing.T) {
		if _, err := parseMessageFrom([]byte("not a message")); err == nil {
			t.Fatalf("expected message parse error")
		}
	})
	t.Run("write string error", func(t *testing.T) {
		if err := writeSMTPLine(bufio.NewWriterSize(failingSMTPWriter{}, 1), strings.Repeat("x", 32)); err == nil {
			t.Fatalf("expected write string error")
		}
	})
}

func TestSMTPSubmissionConnectionHandshakeFailure(t *testing.T) {
	fixture := newSMTPServerFixture(t, false, nil)
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "STARTTLS")
	client.expectCode(t, "220")
	client.close()
	time.Sleep(25 * time.Millisecond)
}

func TestSMTPSubmissionStartTLSResponseWriteFailure(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	server.config.TLSConfig = testTLSConfig(t)
	server.handleConnection(context.Background(), &failingAfterGreetingConn{reader: strings.NewReader("STARTTLS\r\n")}, false, "remote")
}

func TestSMTPSubmissionInitialGreetingWriteFailure(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	server.handleConnection(context.Background(), failingSMTPConn{}, false, "remote")
}

func TestSMTPSubmissionDeadlineErrorPaths(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	server.handleConnection(context.Background(), &smtpDeadlineFailingConn{reader: strings.NewReader("NOOP\r\n")}, false, "remote")
	from := mustSMTPSubmissionAddress(t, "alice@example.com")
	session := &sessionState{
		authenticated: &smtpidentity.AuthenticatedIdentity{ID: "identity", EmailAddress: from},
		mailFrom:      &from,
		recipients:    []smtpidentity.Address{mustSMTPSubmissionAddress(t, "recipient@example.net")},
	}
	var output bytes.Buffer
	server.handleData(context.Background(), smtpDeadlineFailingConn{}, bufio.NewReader(strings.NewReader("From: alice@example.com\r\n\r\nHello\r\n.\r\n")), bufio.NewWriter(&output), session)
	if !strings.Contains(output.String(), "354") {
		t.Fatalf("expected data prompt before deadline error, got %q", output.String())
	}
}

func TestLoadTLSConfig(t *testing.T) {
	certPath, keyPath := writeTLSFiles(t)
	tlsConfig, err := LoadTLSConfig(certPath, keyPath)
	if err != nil {
		t.Fatalf("load tls config: %v", err)
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 || len(tlsConfig.Certificates) != 1 {
		t.Fatalf("unexpected tls config %+v", tlsConfig)
	}
	if _, err := LoadTLSConfig(filepath.Join(t.TempDir(), "missing.pem"), keyPath); err == nil {
		t.Fatalf("expected missing cert error")
	}
}

func TestSMTPParsingHelpers(t *testing.T) {
	if _, err := parseSMTPPath("BODY:<alice@example.com>", "FROM:"); err == nil {
		t.Fatalf("expected missing prefix error")
	}
	if _, err := parseSMTPPath("FROM:<alice@example.com", "FROM:"); err == nil {
		t.Fatalf("expected unterminated path error")
	}
	address, err := parseSMTPPath("FROM:alice@example.com SIZE=1", "FROM:")
	if err != nil {
		t.Fatalf("parse bare path: %v", err)
	}
	if address.String() != "alice@example.com" {
		t.Fatalf("unexpected address %s", address.String())
	}
	var buffer bytes.Buffer
	if err := writeSMTPLine(bufio.NewWriter(failingSMTPWriter{}), "250 OK"); err == nil {
		t.Fatalf("expected write error")
	}
	if err := writeSMTPLine(bufio.NewWriter(&buffer), "250 OK"); err != nil {
		t.Fatalf("write smtp line: %v", err)
	}
	if buffer.String() != "250 OK\r\n" {
		t.Fatalf("unexpected line %q", buffer.String())
	}
}

func TestSMTPThrottleHelpers(t *testing.T) {
	if remoteHost := remoteHostForConnection(splitRemoteSMTPConn{remoteAddress: "192.0.2.10:2525"}); remoteHost != "192.0.2.10" {
		t.Fatalf("unexpected split remote host %q", remoteHost)
	}
	if remoteHost := remoteHostForConnection(splitRemoteSMTPConn{remoteAddress: "opaque"}); remoteHost != "opaque" {
		t.Fatalf("unexpected opaque remote host %q", remoteHost)
	}
	if deadlineErr := setSMTPDeadline(failingSMTPConn{}, 0); deadlineErr != nil {
		t.Fatalf("expected disabled deadline to be ignored, got %v", deadlineErr)
	}
	throttle := newSMTPThrottle(Config{
		MaxConcurrentSessions:    3,
		MaxSessionsPerRemoteHost: 3,
		AuthFailureLimit:         2,
		AuthFailureWindow:        time.Minute,
		MessageLimit:             2,
		MessageWindow:            time.Minute,
	})
	releaseFirst, firstErr := throttle.acquireSession("")
	if firstErr != nil {
		t.Fatalf("acquire first session: %v", firstErr)
	}
	releaseSecond, secondErr := throttle.acquireSession("")
	if secondErr != nil {
		t.Fatalf("acquire second session: %v", secondErr)
	}
	releaseFirst()
	if throttle.activeSessionCount() != 1 {
		t.Fatalf("expected one active session after first release")
	}
	releaseSecond()
	if throttle.activeSessionCount() != 0 {
		t.Fatalf("expected no active sessions after second release")
	}
	if !throttle.allowMessage("") {
		t.Fatalf("expected empty identity message to be allowed")
	}
	if key := authThrottleKey("", ""); key != "remote:unknown" {
		t.Fatalf("unexpected empty auth key %q", key)
	}
}

func newBareSMTPSubmissionServer(t *testing.T) *Server {
	t.Helper()
	address := mustSMTPSubmissionAddress(t, "alice@example.com")
	server, serverErr := NewServer(Config{
		Hostname:          "smtp.test",
		AllowInsecureAuth: true,
		Authenticator: &staticAuthenticator{
			username: "smtp-user",
			password: "smtp-pass",
			identity: smtpidentity.AuthenticatedIdentity{
				ID:           "identity-1",
				EmailAddress: address,
				Username:     "smtp-user",
			},
		},
		Relay:  &recordingRelay{},
		Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
	})
	if serverErr != nil {
		t.Fatalf("new server: %v", serverErr)
	}
	return server
}

type smtpErrorReader struct {
	err error
}

func (reader smtpErrorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

type smtpServerFixture struct {
	server *Server
	relay  *recordingRelay
	addr   string
	cancel context.CancelFunc
}

func newSMTPServerFixture(t *testing.T, allowInsecureAuth bool, relayError error) *smtpServerFixture {
	t.Helper()
	return newSMTPServerFixtureWithConfig(t, allowInsecureAuth, relayError, nil)
}

func newSMTPServerFixtureWithConfig(t *testing.T, allowInsecureAuth bool, relayError error, mutate func(*Config)) *smtpServerFixture {
	t.Helper()
	address, addressErr := smtpidentity.NewAddress("alice@example.com")
	if addressErr != nil {
		t.Fatalf("identity address: %v", addressErr)
	}
	relay := &recordingRelay{err: relayError}
	serverConfig := Config{
		Hostname:          "smtp.test",
		TLSConfig:         testTLSConfig(t),
		MaxMessageBytes:   1024 * 1024,
		MaxRecipients:     10,
		AllowInsecureAuth: allowInsecureAuth,
		Authenticator: &staticAuthenticator{
			username: "smtp-user",
			password: "smtp-pass",
			identity: smtpidentity.AuthenticatedIdentity{
				ID:           "identity-1",
				EmailAddress: address,
				Username:     "smtp-user",
			},
		},
		Relay:  relay,
		Logger: slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
	}
	if mutate != nil {
		mutate(&serverConfig)
	}
	server, serverErr := NewServer(serverConfig)
	if serverErr != nil {
		t.Fatalf("new server: %v", serverErr)
	}
	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatalf("listen: %v", listenErr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		listener.Close()
	})
	go func() {
		_ = server.Serve(ctx, listener, false)
	}()
	return &smtpServerFixture{
		server: server,
		relay:  relay,
		addr:   listener.Addr().String(),
		cancel: cancel,
	}
}

func (fixture *smtpServerFixture) dial(t *testing.T) *smtpTestClient {
	t.Helper()
	return newSMTPTestClient(mustDial(t, fixture.addr))
}

func (fixture *smtpServerFixture) authenticatedClient(t *testing.T) *smtpTestClient {
	t.Helper()
	client := fixture.dial(t)
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "235")
	return client
}

func sendAcceptedSMTPMessage(t *testing.T, client *smtpTestClient, subject string) {
	t.Helper()
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	client.send(t, "RCPT TO:<recipient@example.net>")
	client.expectCode(t, "250")
	client.send(t, "DATA")
	client.expectCode(t, "354")
	client.sendData(t, "From: alice@example.com\r\nTo: recipient@example.net\r\nSubject: "+subject+"\r\n\r\nHello")
}

func waitForActiveSMTPSessions(t *testing.T, server *Server, expectedCount int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if server.throttle.activeSessionCount() == expectedCount {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d active SMTP sessions, got %d", expectedCount, server.throttle.activeSessionCount())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type smtpTestClient struct {
	connection net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
}

func newSMTPTestClient(connection net.Conn) *smtpTestClient {
	return &smtpTestClient{
		connection: connection,
		reader:     bufio.NewReader(connection),
		writer:     bufio.NewWriter(connection),
	}
}

func (client *smtpTestClient) send(t *testing.T, line string) {
	t.Helper()
	if _, writeErr := client.writer.WriteString(line + "\r\n"); writeErr != nil {
		t.Fatalf("send %q: %v", line, writeErr)
	}
	if flushErr := client.writer.Flush(); flushErr != nil {
		t.Fatalf("flush %q: %v", line, flushErr)
	}
}

func (client *smtpTestClient) sendData(t *testing.T, data string) {
	t.Helper()
	if _, writeErr := client.writer.WriteString(data + "\r\n.\r\n"); writeErr != nil {
		t.Fatalf("send data: %v", writeErr)
	}
	if flushErr := client.writer.Flush(); flushErr != nil {
		t.Fatalf("flush data: %v", flushErr)
	}
}

func (client *smtpTestClient) expectCode(t *testing.T, expectedCode string) []string {
	t.Helper()
	lines := client.readResponse(t)
	if len(lines) == 0 || !strings.HasPrefix(lines[len(lines)-1], expectedCode) {
		t.Fatalf("expected SMTP code %s, got %#v", expectedCode, lines)
	}
	return lines
}

func (client *smtpTestClient) readResponse(t *testing.T) []string {
	t.Helper()
	var lines []string
	for {
		line, readErr := client.reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("read response: %v", readErr)
		}
		trimmedLine := strings.TrimRight(line, "\r\n")
		lines = append(lines, trimmedLine)
		if len(trimmedLine) < 4 || trimmedLine[3] != '-' {
			return lines
		}
	}
}

func (client *smtpTestClient) startTLS(t *testing.T) {
	t.Helper()
	tlsConnection := tls.Client(client.connection, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "smtp.test",
	})
	if handshakeErr := tlsConnection.Handshake(); handshakeErr != nil {
		t.Fatalf("tls handshake: %v", handshakeErr)
	}
	client.connection = tlsConnection
	client.reader = bufio.NewReader(tlsConnection)
	client.writer = bufio.NewWriter(tlsConnection)
}

func (client *smtpTestClient) close() {
	client.connection.Close()
}

type staticAuthenticator struct {
	username string
	password string
	identity smtpidentity.AuthenticatedIdentity
}

func (authenticator *staticAuthenticator) Authenticate(_ context.Context, username string, password string) (smtpidentity.AuthenticatedIdentity, error) {
	if username != authenticator.username || password != authenticator.password {
		return smtpidentity.AuthenticatedIdentity{}, smtpidentity.ErrAuthenticationFailed
	}
	return authenticator.identity, nil
}

type recordingRelay struct {
	messages []RawMessage
	err      error
}

func (relay *recordingRelay) Relay(_ context.Context, message RawMessage) error {
	relay.messages = append(relay.messages, message)
	if relay.err != nil {
		return relay.err
	}
	return nil
}

type ioDiscard struct{}

func (ioDiscard) Write(bytes []byte) (int, error) {
	return len(bytes), nil
}

type failingSMTPWriter struct{}

func (failingSMTPWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

type failingSMTPConn struct{}

func (failingSMTPConn) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (failingSMTPConn) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (failingSMTPConn) Close() error {
	return nil
}

func (failingSMTPConn) LocalAddr() net.Addr {
	return testListenerAddr("local")
}

func (failingSMTPConn) RemoteAddr() net.Addr {
	return testListenerAddr("remote")
}

func (failingSMTPConn) SetDeadline(time.Time) error {
	return nil
}

func (failingSMTPConn) SetReadDeadline(time.Time) error {
	return nil
}

func (failingSMTPConn) SetWriteDeadline(time.Time) error {
	return nil
}

type splitRemoteSMTPConn struct {
	remoteAddress string
}

func (conn splitRemoteSMTPConn) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (conn splitRemoteSMTPConn) Write(payload []byte) (int, error) {
	return len(payload), nil
}

func (conn splitRemoteSMTPConn) Close() error {
	return nil
}

func (conn splitRemoteSMTPConn) LocalAddr() net.Addr {
	return testListenerAddr("local")
}

func (conn splitRemoteSMTPConn) RemoteAddr() net.Addr {
	return testListenerAddr(conn.remoteAddress)
}

func (conn splitRemoteSMTPConn) SetDeadline(time.Time) error {
	return nil
}

func (conn splitRemoteSMTPConn) SetReadDeadline(time.Time) error {
	return nil
}

func (conn splitRemoteSMTPConn) SetWriteDeadline(time.Time) error {
	return nil
}

type smtpDeadlineFailingConn struct {
	reader *strings.Reader
}

func (conn smtpDeadlineFailingConn) Read(payload []byte) (int, error) {
	if conn.reader == nil {
		return 0, io.EOF
	}
	return conn.reader.Read(payload)
}

func (conn smtpDeadlineFailingConn) Write(payload []byte) (int, error) {
	return len(payload), nil
}

func (conn smtpDeadlineFailingConn) Close() error {
	return nil
}

func (conn smtpDeadlineFailingConn) LocalAddr() net.Addr {
	return testListenerAddr("local")
}

func (conn smtpDeadlineFailingConn) RemoteAddr() net.Addr {
	return testListenerAddr("remote")
}

func (conn smtpDeadlineFailingConn) SetDeadline(time.Time) error {
	return errors.New("set deadline failed")
}

func (conn smtpDeadlineFailingConn) SetReadDeadline(time.Time) error {
	return nil
}

func (conn smtpDeadlineFailingConn) SetWriteDeadline(time.Time) error {
	return nil
}

type failingAfterGreetingConn struct {
	reader *strings.Reader
	writes int
}

func (conn *failingAfterGreetingConn) Read(payload []byte) (int, error) {
	return conn.reader.Read(payload)
}

func (conn *failingAfterGreetingConn) Write(payload []byte) (int, error) {
	if conn.writes == 0 {
		conn.writes++
		return len(payload), nil
	}
	return 0, io.ErrClosedPipe
}

func (conn *failingAfterGreetingConn) Close() error {
	return nil
}

func (conn *failingAfterGreetingConn) LocalAddr() net.Addr {
	return testListenerAddr("local")
}

func (conn *failingAfterGreetingConn) RemoteAddr() net.Addr {
	return testListenerAddr("remote")
}

func (conn *failingAfterGreetingConn) SetDeadline(time.Time) error {
	return nil
}

func (conn *failingAfterGreetingConn) SetReadDeadline(time.Time) error {
	return nil
}

func (conn *failingAfterGreetingConn) SetWriteDeadline(time.Time) error {
	return nil
}

func plainAuthPayload(username string, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("\x00%s\x00%s", username, password)))
}

type fatalAcceptListener struct {
	acceptErr error
	closed    chan struct{}
}

func newFatalAcceptListener(acceptErr error) *fatalAcceptListener {
	return &fatalAcceptListener{acceptErr: acceptErr, closed: make(chan struct{})}
}

func (listener *fatalAcceptListener) Accept() (net.Conn, error) {
	return nil, listener.acceptErr
}

func (listener *fatalAcceptListener) Close() error {
	closeListenerSignal(listener.closed)
	return nil
}

func (listener *fatalAcceptListener) Addr() net.Addr {
	return testListenerAddr("fatal")
}

type blockingAcceptListener struct {
	closed chan struct{}
}

func newBlockingAcceptListener() *blockingAcceptListener {
	return &blockingAcceptListener{closed: make(chan struct{})}
}

func (listener *blockingAcceptListener) Accept() (net.Conn, error) {
	<-listener.closed
	return nil, net.ErrClosed
}

func (listener *blockingAcceptListener) Close() error {
	closeListenerSignal(listener.closed)
	return nil
}

func (listener *blockingAcceptListener) Addr() net.Addr {
	return testListenerAddr("blocking")
}

func (listener *blockingAcceptListener) wasClosed() bool {
	select {
	case <-listener.closed:
		return true
	default:
		return false
	}
}

type testListenerAddr string

func (addr testListenerAddr) Network() string {
	return string(addr)
}

func (addr testListenerAddr) String() string {
	return string(addr)
}

func closeListenerSignal(closed chan struct{}) {
	select {
	case <-closed:
	default:
		close(closed)
	}
}

func testTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	privateKey, keyErr := rsa.GenerateKey(rand.Reader, 2048)
	if keyErr != nil {
		t.Fatalf("generate tls key: %v", keyErr)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "smtp.test",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		DNSNames:  []string{"smtp.test"},
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}
	certificateBytes, certErr := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if certErr != nil {
		t.Fatalf("create tls cert: %v", certErr)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	certificate, loadErr := tls.X509KeyPair(certPEM, keyPEM)
	if loadErr != nil {
		t.Fatalf("load tls key pair: %v", loadErr)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}
}

func writeTLSFiles(t *testing.T) (string, string) {
	t.Helper()
	privateKey, keyErr := rsa.GenerateKey(rand.Reader, 2048)
	if keyErr != nil {
		t.Fatalf("generate tls key: %v", keyErr)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "smtp.test",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		DNSNames:  []string{"smtp.test"},
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}
	certificateBytes, certErr := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if certErr != nil {
		t.Fatalf("create tls cert: %v", certErr)
	}
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateBytes}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func mustDial(t *testing.T, address string) net.Conn {
	t.Helper()
	connection, dialErr := net.Dial("tcp", address)
	if dialErr != nil {
		t.Fatalf("dial smtp server: %v", dialErr)
	}
	return connection
}
