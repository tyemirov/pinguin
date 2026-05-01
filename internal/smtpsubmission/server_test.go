package smtpsubmission

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
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

type smtpServerFixture struct {
	server *Server
	relay  *recordingRelay
	addr   string
	cancel context.CancelFunc
}

func newSMTPServerFixture(t *testing.T, allowInsecureAuth bool, relayError error) *smtpServerFixture {
	t.Helper()
	address, addressErr := smtpidentity.NewAddress("alice@example.com")
	if addressErr != nil {
		t.Fatalf("identity address: %v", addressErr)
	}
	relay := &recordingRelay{err: relayError}
	server, serverErr := NewServer(Config{
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
				TenantID:     "tenant-1",
				EmailAddress: address,
				Username:     "smtp-user",
			},
		},
		Relay:  relay,
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
	return &smtpServerFixture{
		server: server,
		relay:  relay,
		addr:   listener.Addr().String(),
		cancel: cancel,
	}
}

func (fixture *smtpServerFixture) dial(t *testing.T) *smtpTestClient {
	t.Helper()
	connection, dialErr := net.Dial("tcp", fixture.addr)
	if dialErr != nil {
		t.Fatalf("dial smtp server: %v", dialErr)
	}
	return newSMTPTestClient(connection)
}

func (fixture *smtpServerFixture) authenticatedClient(t *testing.T) *smtpTestClient {
	t.Helper()
	client := fixture.dial(t)
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN "+plainAuthPayload("smtp-user", "smtp-pass"))
	client.expectCode(t, "235")
	return client
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

func plainAuthPayload(username string, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("\x00%s\x00%s", username, password)))
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
