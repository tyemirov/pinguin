package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
	"gorm.io/gorm"
	"log/slog"
)

func TestListNotificationsFiltersByStatus(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)

	now := time.Now().UTC()
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-queued",
		NotificationType: model.NotificationEmail,
		Recipient:        "queued@example.com",
		Message:          "queued",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-sent",
		NotificationType: model.NotificationEmail,
		Recipient:        "sent@example.com",
		Message:          "sent",
		Status:           model.StatusSent,
		CreatedAt:        now.Add(time.Second),
		UpdatedAt:        now.Add(time.Second),
	})
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-legacy-failed",
		NotificationType: model.NotificationEmail,
		Recipient:        "errored@example.com",
		Message:          "errored",
		Status:           model.StatusFailed,
		CreatedAt:        now.Add(2 * time.Second),
		UpdatedAt:        now.Add(2 * time.Second),
	})

	responses, err := serviceInstance.ListNotifications(
		context.Background(),
		model.NotificationListFilters{Statuses: []model.NotificationStatus{model.StatusQueued, model.StatusErrored, model.StatusErrored}},
	)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(responses))
	}
	statusSet := map[model.NotificationStatus]struct{}{}
	for _, response := range responses {
		statusSet[response.Status] = struct{}{}
	}
	if _, ok := statusSet[model.StatusQueued]; !ok {
		t.Fatalf("expected queued record in results")
	}
	if _, ok := statusSet[model.StatusErrored]; !ok {
		t.Fatalf("expected errored record in results")
	}
}

func TestRescheduleNotificationUpdatesQueuedRecord(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)

	now := time.Now().UTC()
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-reschedulable",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "queued",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	future := time.Now().UTC().Add(30 * time.Minute)
	response, err := serviceInstance.RescheduleNotification(context.Background(), "notif-reschedulable", future)
	if err != nil {
		t.Fatalf("reschedule error: %v", err)
	}
	if response.ScheduledFor == nil {
		t.Fatalf("expected scheduled time to be set")
	}
	if !response.ScheduledFor.Equal(future.UTC()) {
		t.Fatalf("scheduled time mismatch: want %s got %s", future.UTC(), response.ScheduledFor.UTC())
	}

	stored, fetchErr := model.GetNotificationByID(context.Background(), database, "notif-reschedulable")
	if fetchErr != nil {
		t.Fatalf("fetch error: %v", fetchErr)
	}
	if stored.ScheduledFor == nil || !stored.ScheduledFor.Equal(future.UTC()) {
		t.Fatalf("stored schedule mismatch")
	}
}

func TestRescheduleNotificationRejectsInvalidStates(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)

	now := time.Now().UTC()
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-sent",
		NotificationType: model.NotificationEmail,
		Recipient:        "sent@example.com",
		Message:          "sent",
		Status:           model.StatusSent,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	past := time.Now().UTC().Add(-5 * time.Minute)
	if _, err := serviceInstance.RescheduleNotification(context.Background(), "notif-sent", past); !errors.Is(err, ErrScheduleInPast) {
		t.Fatalf("expected ErrScheduleInPast, got %v", err)
	}

	future := time.Now().UTC().Add(10 * time.Minute)
	if _, err := serviceInstance.RescheduleNotification(context.Background(), "notif-sent", future); !errors.Is(err, ErrNotificationNotEditable) {
		t.Fatalf("expected ErrNotificationNotEditable, got %v", err)
	}
}

func TestCancelNotificationTransitionsStatus(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)

	now := time.Now().UTC()
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-cancel",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "queued",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	response, err := serviceInstance.CancelNotification(context.Background(), "notif-cancel")
	if err != nil {
		t.Fatalf("cancel error: %v", err)
	}
	if response.Status != model.StatusCancelled {
		t.Fatalf("expected cancelled status, got %s", response.Status)
	}
	if response.ScheduledFor != nil {
		t.Fatalf("expected scheduled time cleared on cancellation")
	}

	stored, fetchErr := model.GetNotificationByID(context.Background(), database, "notif-cancel")
	if fetchErr != nil {
		t.Fatalf("fetch error: %v", fetchErr)
	}
	if stored.Status != model.StatusCancelled {
		t.Fatalf("stored status mismatch, got %s", stored.Status)
	}
	if stored.ScheduledFor != nil {
		t.Fatalf("stored schedule should be nil for cancelled")
	}
}

func TestCancelNotificationRejectsNonQueued(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)

	now := time.Now().UTC()
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-sent",
		NotificationType: model.NotificationEmail,
		Recipient:        "sent@example.com",
		Message:          "sent",
		Status:           model.StatusSent,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	if _, err := serviceInstance.CancelNotification(context.Background(), "notif-sent"); !errors.Is(err, ErrNotificationNotEditable) {
		t.Fatalf("expected ErrNotificationNotEditable, got %v", err)
	}
}

func newNotificationServiceForDomainTests(database *gorm.DB) *notificationServiceImpl {
	return &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      &stubEmailSender{},
		smsSender:        &stubSmsSender{},
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       true,
	}
}

func insertNotificationRecord(t *testing.T, database *gorm.DB, record model.Notification) {
	t.Helper()

	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := model.CreateNotification(context.Background(), database, &record); err != nil {
		t.Fatalf("create notification error: %v", err)
	}
}
