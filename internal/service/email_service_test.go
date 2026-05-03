package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
	"log/slog"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func TestSendEmailPlain(t *testing.T) {
	originalSendMail := sendMailFunc
	defer func() {
		sendMailFunc = originalSendMail
	}()

	var captured struct {
		addr string
		from string
		to   []string
		body string
	}

	sendMailFunc = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		captured.addr = addr
		captured.from = from
		captured.to = append([]string(nil), to...)
		captured.body = string(msg)
		return nil
	}

	sender := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtp.example.com",
		Port:        "587",
		Username:    "user",
		Password:    "pass",
		FromAddress: "from@example.com",
	}, newDiscardLogger())

	if err := sender.SendEmail(context.Background(), "to@example.com", "Greetings", "Hello body", nil); err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}
	if captured.addr != "smtp.example.com:587" {
		t.Fatalf("unexpected smtp address %q", captured.addr)
	}
	if captured.from != "from@example.com" {
		t.Fatalf("unexpected from %q", captured.from)
	}
	if len(captured.to) != 1 || captured.to[0] != "to@example.com" {
		t.Fatalf("unexpected recipients %#v", captured.to)
	}
	if captured.body == "" {
		t.Fatalf("expected body content to be sent")
	}
}

type stubConn struct{}

func (stubConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (stubConn) Write(b []byte) (int, error)      { return len(b), nil }
func (stubConn) Close() error                     { return nil }
func (stubConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (stubConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (stubConn) SetDeadline(time.Time) error      { return nil }
func (stubConn) SetReadDeadline(time.Time) error  { return nil }
func (stubConn) SetWriteDeadline(time.Time) error { return nil }

type stubWriteCloser struct {
	bytes.Buffer
	closeErr error
	writeErr error
}

func (stub *stubWriteCloser) Write(payload []byte) (int, error) {
	if stub.writeErr != nil {
		return 0, stub.writeErr
	}
	return stub.Buffer.Write(payload)
}

func (stub *stubWriteCloser) Close() error { return stub.closeErr }

type stubSMTPClient struct {
	authCalled bool
	mailAddr   string
	rcptAddr   string
	payload    *stubWriteCloser
	authErr    error
	mailErr    error
	rcptErr    error
	dataErr    error
}

func (client *stubSMTPClient) Auth(smtp.Auth) error {
	client.authCalled = true
	return client.authErr
}

func (client *stubSMTPClient) Mail(addr string) error {
	client.mailAddr = addr
	return client.mailErr
}

func (client *stubSMTPClient) Rcpt(addr string) error {
	client.rcptAddr = addr
	return client.rcptErr
}

func (client *stubSMTPClient) Data() (io.WriteCloser, error) {
	if client.dataErr != nil {
		return nil, client.dataErr
	}
	if client.payload != nil {
		return client.payload, nil
	}
	client.payload = &stubWriteCloser{}
	return client.payload, nil
}

func (client *stubSMTPClient) Quit() error {
	return nil
}

func TestSendEmailTLS(t *testing.T) {
	originalDial := dialTLSFunc
	originalClient := newSMTPClient
	defer func() {
		dialTLSFunc = originalDial
		newSMTPClient = originalClient
	}()

	dialTLSFunc = func(*net.Dialer, string, string, *tls.Config) (net.Conn, error) {
		return stubConn{}, nil
	}

	client := &stubSMTPClient{}
	newSMTPClient = func(net.Conn, string) (smtpClient, error) {
		return client, nil
	}

	sender := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtp.example.com",
		Port:        "465",
		Username:    "user",
		Password:    "pass",
		FromAddress: "from@example.com",
	}, newDiscardLogger())

	attachments := []model.EmailAttachment{
		{
			Filename:    "demo.txt",
			ContentType: "text/plain",
			Data:        []byte("hello"),
		},
	}

	if err := sender.SendEmail(context.Background(), "to@example.com", "Greetings", "Hello body", attachments); err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}
	if !client.authCalled {
		t.Fatalf("expected Auth to be called")
	}
	if client.mailAddr != "from@example.com" {
		t.Fatalf("unexpected MAIL address %q", client.mailAddr)
	}
	if client.rcptAddr != "to@example.com" {
		t.Fatalf("unexpected RCPT address %q", client.rcptAddr)
	}
	if client.payload == nil || client.payload.Len() == 0 {
		t.Fatalf("expected payload to be written")
	}
}

func TestSendRawEmailPlainReportsSendMailError(t *testing.T) {
	originalSendMail := sendMailFunc
	defer func() {
		sendMailFunc = originalSendMail
	}()
	sendMailFunc = func(string, smtp.Auth, string, []string, []byte) error {
		return errors.New("smtp unavailable")
	}

	sender := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtp.example.com",
		Port:        "587",
		Username:    "user",
		Password:    "pass",
		FromAddress: "from@example.com",
	}, newDiscardLogger())

	err := sender.SendRawEmail(context.Background(), "from@example.com", []string{"to@example.com"}, []byte("hello"))
	if err == nil || !strings.Contains(err.Error(), "smtp send failed") {
		t.Fatalf("expected wrapped send error, got %v", err)
	}
}

func TestSendRawEmailTLSErrorPaths(t *testing.T) {
	originalDial := dialTLSFunc
	originalClient := newSMTPClient
	defer func() {
		dialTLSFunc = originalDial
		newSMTPClient = originalClient
	}()

	sender := NewSMTPEmailSender(SMTPConfig{
		Host:        "smtp.example.com",
		Port:        "465",
		Username:    "user",
		Password:    "pass",
		FromAddress: "from@example.com",
	}, newDiscardLogger())

	t.Run("dial error", func(t *testing.T) {
		dialTLSFunc = func(*net.Dialer, string, string, *tls.Config) (net.Conn, error) {
			return nil, errors.New("dial failed")
		}
		err := sender.SendRawEmail(context.Background(), "from@example.com", []string{"to@example.com"}, []byte("hello"))
		if err == nil || !strings.Contains(err.Error(), "failed to dial TLS") {
			t.Fatalf("expected dial error, got %v", err)
		}
	})

	t.Run("context canceled after dial", func(t *testing.T) {
		dialTLSFunc = func(*net.Dialer, string, string, *tls.Config) (net.Conn, error) {
			return stubConn{}, nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sender.SendRawEmail(ctx, "from@example.com", []string{"to@example.com"}, []byte("hello"))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	})

	t.Run("client creation error", func(t *testing.T) {
		dialTLSFunc = func(*net.Dialer, string, string, *tls.Config) (net.Conn, error) {
			return stubConn{}, nil
		}
		newSMTPClient = func(net.Conn, string) (smtpClient, error) {
			return nil, errors.New("client failed")
		}
		err := sender.SendRawEmail(context.Background(), "from@example.com", []string{"to@example.com"}, []byte("hello"))
		if err == nil || !strings.Contains(err.Error(), "failed to create SMTP client") {
			t.Fatalf("expected client error, got %v", err)
		}
	})

	for _, testCase := range []struct {
		name     string
		client   *stubSMTPClient
		expected string
	}{
		{name: "auth error", client: &stubSMTPClient{authErr: errors.New("auth failed")}, expected: "failed to authenticate"},
		{name: "mail error", client: &stubSMTPClient{mailErr: errors.New("mail failed")}, expected: "failed to set sender"},
		{name: "recipient error", client: &stubSMTPClient{rcptErr: errors.New("rcpt failed")}, expected: "failed to set recipient"},
		{name: "data error", client: &stubSMTPClient{dataErr: errors.New("data failed")}, expected: "failed to get data writer"},
		{name: "write error", client: &stubSMTPClient{payload: &stubWriteCloser{writeErr: errors.New("write failed")}}, expected: "failed to write email message"},
		{name: "close error", client: &stubSMTPClient{payload: &stubWriteCloser{closeErr: errors.New("close failed")}}, expected: "failed to close data writer"},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			dialTLSFunc = func(*net.Dialer, string, string, *tls.Config) (net.Conn, error) {
				return stubConn{}, nil
			}
			newSMTPClient = func(net.Conn, string) (smtpClient, error) {
				return testCase.client, nil
			}
			err := sender.SendRawEmail(context.Background(), "from@example.com", []string{"to@example.com"}, []byte("hello"))
			if err == nil || !strings.Contains(err.Error(), testCase.expected) {
				t.Fatalf("expected %q error, got %v", testCase.expected, err)
			}
		})
	}
}

func TestDefaultSMTPDialAndClientConstructors(t *testing.T) {
	dialer := &net.Dialer{Timeout: time.Millisecond}
	if _, err := dialTLSFunc(dialer, "tcp", "127.0.0.1:1", &tls.Config{}); err == nil {
		t.Fatalf("expected default TLS dial to report connection failure")
	}
	if _, err := newSMTPClient(stubConn{}, "localhost"); err == nil {
		t.Fatalf("expected default SMTP client constructor to report greeting failure")
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	done := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		if _, err := io.WriteString(serverConn, "220 localhost ESMTP\r\n"); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	clientInstance, err := newSMTPClient(clientConn, "localhost")
	if err != nil {
		t.Fatalf("expected default SMTP client constructor success, got %v", err)
	}
	if wrapper, ok := clientInstance.(smtpClientWrapper); ok {
		_ = wrapper.client.Close()
	}
	if err := <-done; err != nil {
		t.Fatalf("smtp constructor server: %v", err)
	}
}

func TestSMTPClientWrapperUsesUnderlyingClient(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	done := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		reader := make([]byte, 1024)
		write := func(line string) error {
			_, err := io.WriteString(serverConn, line)
			return err
		}
		readLine := func() (string, error) {
			n, err := serverConn.Read(reader)
			return string(reader[:n]), err
		}
		if err := write("220 localhost ESMTP\r\n"); err != nil {
			done <- err
			return
		}
		if line, err := readLine(); err != nil || !strings.HasPrefix(line, "EHLO ") {
			done <- errors.New("expected EHLO")
			return
		}
		if err := write("250-localhost\r\n250 AUTH PLAIN\r\n"); err != nil {
			done <- err
			return
		}
		if line, err := readLine(); err != nil || !strings.HasPrefix(line, "AUTH PLAIN ") {
			done <- errors.New("expected AUTH PLAIN")
			return
		}
		for _, response := range []string{"235 ok\r\n", "250 ok\r\n", "250 ok\r\n", "354 go\r\n"} {
			if err := write(response); err != nil {
				done <- err
				return
			}
			if _, err := readLine(); err != nil {
				done <- err
				return
			}
		}
		if err := write("250 accepted\r\n"); err != nil {
			done <- err
			return
		}
		if line, err := readLine(); err != nil || !strings.HasPrefix(line, "QUIT") {
			done <- errors.New("expected QUIT")
			return
		}
		done <- write("221 bye\r\n")
	}()

	smtpClientInstance, err := smtp.NewClient(clientConn, "localhost")
	if err != nil {
		t.Fatalf("new smtp client: %v", err)
	}
	wrapper := smtpClientWrapper{client: smtpClientInstance}
	if err := wrapper.Auth(smtp.PlainAuth("", "user", "pass", "localhost")); err != nil {
		t.Fatalf("auth: %v", err)
	}
	if err := wrapper.Mail("from@example.com"); err != nil {
		t.Fatalf("mail: %v", err)
	}
	if err := wrapper.Rcpt("to@example.com"); err != nil {
		t.Fatalf("rcpt: %v", err)
	}
	writer, err := wrapper.Data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	if _, err := writer.Write([]byte("hello\r\n")); err != nil {
		t.Fatalf("write data: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close data: %v", err)
	}
	if err := wrapper.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("smtp script: %v", err)
	}
}

func TestEmailMessageHelpers(t *testing.T) {
	if encoded := encodeBase64Chunked(nil); encoded != "" {
		t.Fatalf("expected empty encoding, got %q", encoded)
	}
	if filename := sanitizeFilename(" \x00\"\\\\ "); filename != "attachment" {
		t.Fatalf("expected fallback attachment filename, got %q", filename)
	}
	if filename := sanitizeFilename("   "); filename != "attachment" {
		t.Fatalf("expected blank filename fallback, got %q", filename)
	}
	message := buildEmailMessage("from@example.com", "to@example.com", "Subject", "Body", []model.EmailAttachment{
		{Filename: " \x00report\".txt ", Data: []byte("hello")},
	})
	if !strings.Contains(message, "application/octet-stream") {
		t.Fatalf("expected default attachment content type, got %q", message)
	}
	if strings.Contains(message, "\"report\"") {
		t.Fatalf("expected sanitized filename, got %q", message)
	}
}
