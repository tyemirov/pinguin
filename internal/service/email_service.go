package service

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"
	"time"

	"log/slog"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/model"
)

type SMTPConfig struct {
	Host        string
	Port        string
	Username    string
	Password    string
	FromAddress string
	Timeouts    config.Config
}

type EmailSender interface {
	SendEmail(ctx context.Context, recipient string, subject string, message string, attachments []model.EmailAttachment) error
}

var (
	dialTLSFunc = func(dialer *net.Dialer, network string, addr string, config *tls.Config) (net.Conn, error) {
		return tls.DialWithDialer(dialer, network, addr, config)
	}
	newSMTPClient = func(conn net.Conn, host string) (smtpClient, error) {
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return nil, err
		}
		return smtpClientWrapper{client: client}, nil
	}
	sendMailFunc = smtp.SendMail
)

type smtpClient interface {
	Auth(smtp.Auth) error
	Mail(string) error
	Rcpt(string) error
	Data() (io.WriteCloser, error)
	Quit() error
}

type smtpClientWrapper struct {
	client *smtp.Client
}

func (wrapper smtpClientWrapper) Auth(auth smtp.Auth) error {
	return wrapper.client.Auth(auth)
}

func (wrapper smtpClientWrapper) Mail(address string) error {
	return wrapper.client.Mail(address)
}

func (wrapper smtpClientWrapper) Rcpt(address string) error {
	return wrapper.client.Rcpt(address)
}

func (wrapper smtpClientWrapper) Data() (io.WriteCloser, error) {
	return wrapper.client.Data()
}

func (wrapper smtpClientWrapper) Quit() error {
	return wrapper.client.Quit()
}

type SMTPEmailSender struct {
	Config SMTPConfig
	Logger *slog.Logger
}

func NewSMTPEmailSender(configuration SMTPConfig, logger *slog.Logger) *SMTPEmailSender {
	return &SMTPEmailSender{
		Config: configuration,
		Logger: logger,
	}
}

func (senderInstance *SMTPEmailSender) SendEmail(ctx context.Context, recipient string, subject string, message string, attachments []model.EmailAttachment) error {
	emailMessage := buildEmailMessage(senderInstance.Config.FromAddress, recipient, subject, message, attachments)

	if senderInstance.Config.Port == "465" {
		serverAddr := net.JoinHostPort(senderInstance.Config.Host, senderInstance.Config.Port)
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // In production, perform proper certificate validation.
			ServerName:         senderInstance.Config.Host,
		}

		dialer := &net.Dialer{
			Timeout: time.Duration(senderInstance.Config.Timeouts.ConnectionTimeoutSec) * time.Second,
		}

		tlsConnection, dialError := dialTLSFunc(dialer, "tcp", serverAddr, tlsConfig)
		if dialError != nil {
			return fmt.Errorf("failed to dial TLS: %w", dialError)
		}
		defer tlsConnection.Close()

		if ctx.Err() != nil {
			return ctx.Err()
		}

		smtpClient, clientError := newSMTPClient(tlsConnection, senderInstance.Config.Host)
		if clientError != nil {
			return fmt.Errorf("failed to create SMTP client: %w", clientError)
		}
		defer smtpClient.Quit()

		smtpAuth := smtp.PlainAuth("", senderInstance.Config.Username, senderInstance.Config.Password, senderInstance.Config.Host)
		if authError := smtpClient.Auth(smtpAuth); authError != nil {
			return fmt.Errorf("failed to authenticate: %w", authError)
		}

		if mailError := smtpClient.Mail(senderInstance.Config.FromAddress); mailError != nil {
			return fmt.Errorf("failed to set sender: %w", mailError)
		}
		if rcptError := smtpClient.Rcpt(recipient); rcptError != nil {
			return fmt.Errorf("failed to set recipient: %w", rcptError)
		}

		dataWriter, dataError := smtpClient.Data()
		if dataError != nil {
			return fmt.Errorf("failed to get data writer: %w", dataError)
		}
		_, writeError := dataWriter.Write([]byte(emailMessage))
		if writeError != nil {
			dataWriter.Close()
			return fmt.Errorf("failed to write email message: %w", writeError)
		}
		if closeDataError := dataWriter.Close(); closeDataError != nil {
			return fmt.Errorf("failed to close data writer: %w", closeDataError)
		}

		return nil
	}

	smtpAddress := net.JoinHostPort(senderInstance.Config.Host, senderInstance.Config.Port)
	smtpAuth := smtp.PlainAuth("", senderInstance.Config.Username, senderInstance.Config.Password, senderInstance.Config.Host)
	sendError := sendMailFunc(smtpAddress, smtpAuth, senderInstance.Config.FromAddress, []string{recipient}, []byte(emailMessage))
	if sendError != nil {
		return fmt.Errorf("smtp send failed: %w", sendError)
	}
	return nil
}

func buildEmailMessage(fromAddress string, toAddress string, subject string, body string, attachments []model.EmailAttachment) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("From: %s\r\n", fromAddress))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", toAddress))
	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	builder.WriteString("MIME-Version: 1.0\r\n")
	if len(attachments) == 0 {
		builder.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		builder.WriteString("\r\n")
		builder.WriteString(body)
		return builder.String()
	}

	boundary := fmt.Sprintf("PinguinBoundary-%d", time.Now().UnixNano())
	builder.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	builder.WriteString("\r\n")

	builder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	builder.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	builder.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	builder.WriteString(body)
	builder.WriteString("\r\n")

	for _, attachment := range attachments {
		builder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		contentType := attachment.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		builder.WriteString(fmt.Sprintf("Content-Type: %s\r\n", contentType))
		builder.WriteString("Content-Transfer-Encoding: base64\r\n")
		builder.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", sanitizeFilename(attachment.Filename)))
		builder.WriteString("\r\n")
		builder.WriteString(encodeBase64Chunked(attachment.Data))
		builder.WriteString("\r\n")
	}

	builder.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	return builder.String()
}

func encodeBase64Chunked(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	const lineLength = 76
	var builder strings.Builder
	for start := 0; start < len(encoded); start += lineLength {
		end := start + lineLength
		if end > len(encoded) {
			end = len(encoded)
		}
		builder.WriteString(encoded[start:end])
		builder.WriteString("\r\n")
	}
	return builder.String()
}

func sanitizeFilename(filename string) string {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return "attachment"
	}

	var builder strings.Builder
	for _, character := range trimmed {
		if character < 32 || character == 127 {
			continue
		}
		switch character {
		case '"', '\\':
			continue
		default:
			builder.WriteRune(character)
		}
	}
	sanitized := strings.TrimSpace(builder.String())
	if sanitized == "" {
		return "attachment"
	}
	return sanitized
}
