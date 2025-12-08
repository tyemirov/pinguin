package service

import (
	"io"
	"strconv"
	"testing"

	"log/slog"

	"github.com/tyemirov/pinguin/internal/config"
)

func TestNewNotificationServiceUsesSMTPEmailSender(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	configuration := config.Config{
		DatabasePath:         "test.db",
		GRPCAuthToken:        "token",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     2,
		SMTPUsername:         "user@example.com",
		SMTPPassword:         "smtp-secret",
		SMTPHost:             "smtp.example.com",
		SMTPPort:             587,
		FromEmail:            "no-reply@example.com",
		TwilioAccountSID:     "sid",
		TwilioAuthToken:      "auth",
		TwilioFromNumber:     "+10000000000",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  10,
	}

	serviceInstance := NewNotificationService(database, logger, configuration)
	concrete, ok := serviceInstance.(*notificationServiceImpl)
	if !ok {
		t.Fatalf("unexpected service implementation type %T", serviceInstance)
	}

	smtpSender, ok := concrete.emailSender.(*SMTPEmailSender)
	if !ok {
		t.Fatalf("expected SMTPEmailSender, got %T", concrete.emailSender)
	}

	if smtpSender.Config.Host != configuration.SMTPHost {
		t.Fatalf("unexpected SMTP host %s", smtpSender.Config.Host)
	}
	if smtpSender.Config.Port != strconv.Itoa(configuration.SMTPPort) {
		t.Fatalf("unexpected SMTP port %s", smtpSender.Config.Port)
	}
	if smtpSender.Config.Username != configuration.SMTPUsername {
		t.Fatalf("unexpected SMTP username %s", smtpSender.Config.Username)
	}
	if smtpSender.Config.Password != configuration.SMTPPassword {
		t.Fatalf("unexpected SMTP password %s", smtpSender.Config.Password)
	}
	if smtpSender.Config.FromAddress != configuration.FromEmail {
		t.Fatalf("unexpected From email %s", smtpSender.Config.FromAddress)
	}
	if smtpSender.Config.Timeouts.ConnectionTimeoutSec != configuration.ConnectionTimeoutSec {
		t.Fatalf("SMTP timeouts not applied")
	}
}
