package integrationtest

import (
	"context"
	"io"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"log/slog"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestScheduledEmailDispatchesAfterWorkerRuns(t *testing.T) {
	t.Helper()

	database := openIntegrationDatabase(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	emailSender := newRecordingEmailSender()

	cfg := config.Config{
		MaxRetries:           3,
		RetryIntervalSec:     1,
		SMTPHost:             "example.com",
		SMTPPort:             587,
		SMTPUsername:         "user",
		SMTPPassword:         "pass",
		FromEmail:            "noreply@example.com",
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  5,
	}

	notificationService := service.NewNotificationServiceWithSenders(database, logger, cfg, emailSender, nil)
	scheduledFor := time.Now().UTC().Add(2 * time.Second)

	response, err := notificationService.SendNotification(context.Background(), model.NotificationRequest{
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Subject:          "Welcome",
		Message:          "Hello from Pinguin",
		ScheduledFor:     &scheduledFor,
	})
	if err != nil {
		t.Fatalf("send notification error: %v", err)
	}
	if response.Status != model.StatusQueued {
		t.Fatalf("expected queued status, got %s", response.Status)
	}
	if emailSender.CallCount() != 0 {
		t.Fatalf("expected scheduled send to skip immediate dispatch")
	}

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	go notificationService.StartRetryWorker(workerCtx)

	const waitTimeout = 8 * time.Second
	sentAt, ok := emailSender.WaitForSend(waitTimeout)
	if !ok {
		t.Fatalf("timed out waiting for scheduled email send after %s", waitTimeout)
	}
	if sentAt.Before(scheduledFor) {
		t.Fatalf("expected dispatch at or after scheduled time; scheduled=%v dispatch=%v", scheduledFor, sentAt)
	}

	finalResponse := waitForNotificationStatus(t, notificationService, response.NotificationID, model.StatusSent, 5*time.Second)
	if finalResponse.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", finalResponse.RetryCount)
	}
}

func openIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "integration.db")
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite open error: %v", err)
	}
	if migrateErr := database.AutoMigrate(&model.Notification{}, &model.NotificationAttachment{}); migrateErr != nil {
		t.Fatalf("migration error: %v", migrateErr)
	}
	return database
}

type recordingEmailSender struct {
	callCount atomic.Int32
	delivered chan time.Time
}

func newRecordingEmailSender() *recordingEmailSender {
	return &recordingEmailSender{
		delivered: make(chan time.Time, 1),
	}
}

func (sender *recordingEmailSender) SendEmail(_ context.Context, _ string, _ string, _ string, _ []model.EmailAttachment) error {
	sender.callCount.Add(1)
	select {
	case sender.delivered <- time.Now().UTC():
	default:
	}
	return nil
}

func (sender *recordingEmailSender) CallCount() int {
	return int(sender.callCount.Load())
}

func (sender *recordingEmailSender) WaitForSend(timeout time.Duration) (time.Time, bool) {
	select {
	case timestamp := <-sender.delivered:
		return timestamp, true
	case <-time.After(timeout):
		return time.Time{}, false
	}
}

func waitForNotificationStatus(t *testing.T, notificationService service.NotificationService, notificationID string, expectedStatus model.NotificationStatus, timeout time.Duration) model.NotificationResponse {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	var lastResponse model.NotificationResponse
	for {
		response, err := notificationService.GetNotificationStatus(context.Background(), notificationID)
		if err != nil {
			t.Fatalf("status retrieval error: %v", err)
		}
		lastResponse = response
		if response.Status == expectedStatus {
			return response
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected status %s within %s; last observed status %s", expectedStatus, timeout, lastResponse.Status)
		}
		<-ticker.C
	}
}
