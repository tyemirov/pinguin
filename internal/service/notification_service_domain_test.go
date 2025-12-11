package service

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/tenant"
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
		tenantContext(),
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
	response, err := serviceInstance.RescheduleNotification(tenantContext(), "notif-reschedulable", future)
	if err != nil {
		t.Fatalf("reschedule error: %v", err)
	}
	if response.ScheduledFor == nil {
		t.Fatalf("expected scheduled time to be set")
	}
	if !response.ScheduledFor.Equal(future.UTC()) {
		t.Fatalf("scheduled time mismatch: want %s got %s", future.UTC(), response.ScheduledFor.UTC())
	}

	stored, fetchErr := model.GetNotificationByID(tenantContext(), database, testTenantID, "notif-reschedulable")
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
	if _, err := serviceInstance.RescheduleNotification(tenantContext(), "notif-sent", past); !errors.Is(err, ErrScheduleInPast) {
		t.Fatalf("expected ErrScheduleInPast, got %v", err)
	}

	future := time.Now().UTC().Add(10 * time.Minute)
	if _, err := serviceInstance.RescheduleNotification(tenantContext(), "notif-sent", future); !errors.Is(err, ErrNotificationNotEditable) {
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

	response, err := serviceInstance.CancelNotification(tenantContext(), "notif-cancel")
	if err != nil {
		t.Fatalf("cancel error: %v", err)
	}
	if response.Status != model.StatusCancelled {
		t.Fatalf("expected cancelled status, got %s", response.Status)
	}
	if response.ScheduledFor != nil {
		t.Fatalf("expected scheduled time cleared on cancellation")
	}

	stored, fetchErr := model.GetNotificationByID(tenantContext(), database, testTenantID, "notif-cancel")
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

	if _, err := serviceInstance.CancelNotification(tenantContext(), "notif-sent"); !errors.Is(err, ErrNotificationNotEditable) {
		t.Fatalf("expected ErrNotificationNotEditable, got %v", err)
	}
}

func TestEmailSenderForTenantUsesRuntimeCredentials(t *testing.T) {
	t.Helper()

	serviceInstance := &notificationServiceImpl{
		logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		config:       config.Config{ConnectionTimeoutSec: 5, OperationTimeoutSec: 5},
		emailSenders: make(map[string]EmailSender),
		smsSenders:   make(map[string]SmsSender),
	}

	alphaRuntime := tenant.RuntimeConfig{
		Tenant: tenant.Tenant{ID: "tenant-alpha"},
		Email: tenant.EmailCredentials{
			Host:        "smtp.alpha.example",
			Port:        2525,
			Username:    "alpha-user",
			Password:    "alpha-pass",
			FromAddress: "noreply@alpha.example",
		},
	}
	sender, err := serviceInstance.emailSenderForTenant(alphaRuntime)
	if err != nil {
		t.Fatalf("email sender resolve error: %v", err)
	}
	smtpSender, ok := sender.(*SMTPEmailSender)
	if !ok {
		t.Fatalf("expected SMTPEmailSender, got %T", sender)
	}
	if smtpSender.Config.Host != "smtp.alpha.example" || smtpSender.Config.Username != "alpha-user" {
		t.Fatalf("smtp config mismatch: %+v", smtpSender.Config)
	}
	cached, err := serviceInstance.emailSenderForTenant(alphaRuntime)
	if err != nil {
		t.Fatalf("cached sender error: %v", err)
	}
	if cached != sender {
		t.Fatalf("expected cached sender reuse")
	}

	bravoRuntime := tenant.RuntimeConfig{
		Tenant: tenant.Tenant{ID: "tenant-bravo"},
		Email: tenant.EmailCredentials{
			Host:        "smtp.bravo.example",
			Port:        465,
			Username:    "bravo-user",
			Password:    "bravo-pass",
			FromAddress: "noreply@bravo.example",
		},
	}
	otherSender, err := serviceInstance.emailSenderForTenant(bravoRuntime)
	if err != nil {
		t.Fatalf("second sender error: %v", err)
	}
	if otherSender == sender {
		t.Fatalf("expected distinct sender instances for different tenants")
	}
}

func TestSmsSenderForTenantUsesRuntimeCredentials(t *testing.T) {
	t.Helper()

	serviceInstance := &notificationServiceImpl{
		logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		config:       config.Config{ConnectionTimeoutSec: 5, OperationTimeoutSec: 5},
		emailSenders: make(map[string]EmailSender),
		smsSenders:   make(map[string]SmsSender),
	}

	bravoRuntime := tenant.RuntimeConfig{
		Tenant: tenant.Tenant{ID: "tenant-bravo"},
		SMS: &tenant.SMSCredentials{
			AccountSID: "AC123",
			AuthToken:  "token",
			FromNumber: "+15550001111",
		},
	}
	sender, err := serviceInstance.smsSenderForTenant(bravoRuntime)
	if err != nil {
		t.Fatalf("sms sender error: %v", err)
	}
	twilioSender, ok := sender.(*TwilioSmsSender)
	if !ok {
		t.Fatalf("expected TwilioSmsSender, got %T", sender)
	}
	if twilioSender.FromNumber != "+15550001111" || twilioSender.AccountSID != "AC123" {
		t.Fatalf("twilio sender mismatch: %+v", twilioSender)
	}
	cached, err := serviceInstance.smsSenderForTenant(bravoRuntime)
	if err != nil {
		t.Fatalf("cached sms sender error: %v", err)
	}
	if cached != sender {
		t.Fatalf("expected cached sms sender")
	}
}

func newNotificationServiceForDomainTests(database *gorm.DB) *notificationServiceImpl {
	return &notificationServiceImpl{
		database:           database,
		logger:             slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		defaultEmailSender: &stubEmailSender{},
		defaultSmsSender:   &stubSmsSender{},
		maxRetries:         3,
		retryIntervalSec:   1,
		emailSenders:       make(map[string]EmailSender),
		smsSenders:         make(map[string]SmsSender),
	}
}

func insertNotificationRecord(t *testing.T, database *gorm.DB, record model.Notification) {
	t.Helper()
	if record.TenantID == "" {
		record.TenantID = testTenantID
	}

	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := model.CreateNotification(tenantContext(), database, &record); err != nil {
		t.Fatalf("create notification error: %v", err)
	}
}
