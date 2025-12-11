package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/internal/config"
	"gorm.io/gorm"
	"log/slog"
)

func TestNewNotificationServiceLogsWhenSmsDisabled(t *testing.T) {
	t.Helper()

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := config.Config{
		DatabasePath:         "ignored.db",
		GRPCAuthToken:        "token",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     2,
		SMTPUsername:         "user",
		SMTPPassword:         "pass",
		SMTPHost:             "smtp.test",
		SMTPPort:             587,
		FromEmail:            "noreply@test",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  7,
	}

	service := NewNotificationService(&gorm.DB{}, logger, cfg, nil)
	if service == nil {
		t.Fatalf("expected service instance")
	}

	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "SMS notifications disabled") {
		t.Fatalf("expected disabled sms log, got %q", logOutput)
	}

	impl, ok := service.(*notificationServiceImpl)
	if !ok {
		t.Fatalf("expected concrete implementation")
	}

	if impl.defaultSmsSender != nil {
		t.Fatalf("expected sms sender to be nil when disabled")
	}
}
