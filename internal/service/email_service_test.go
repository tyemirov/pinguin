package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/smtp"
	"testing"
	"time"

	"log/slog"

	"github.com/tyemirov/pinguin/internal/model"
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
}

func (stub *stubWriteCloser) Close() error { return nil }

type stubSMTPClient struct {
	authCalled bool
	mailAddr   string
	rcptAddr   string
	payload    *stubWriteCloser
}

func (client *stubSMTPClient) Auth(smtp.Auth) error {
	client.authCalled = true
	return nil
}

func (client *stubSMTPClient) Mail(addr string) error {
	client.mailAddr = addr
	return nil
}

func (client *stubSMTPClient) Rcpt(addr string) error {
	client.rcptAddr = addr
	return nil
}

func (client *stubSMTPClient) Data() (io.WriteCloser, error) {
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
