package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/utils/scheduler"
)

type testEmailSender struct {
	called bool
}

func (sender *testEmailSender) SendEmail(context.Context, string, string, string, []model.EmailAttachment) error {
	sender.called = true
	return nil
}

type testSmsSender struct {
	response string
	err      error
	called   bool
}

func (sender *testSmsSender) SendSms(context.Context, string, string) (string, error) {
	sender.called = true
	return sender.response, sender.err
}

func TestNotificationDispatcherEmail(t *testing.T) {
	emailSender := &testEmailSender{}
	serviceInstance := &notificationServiceImpl{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		defaultEmailSender: emailSender,
	}
	dispatcher := newNotificationDispatcher(serviceInstance)
	job := scheduler.Job{
		Payload: &model.Notification{
			TenantID:         testTenantID,
			NotificationType: model.NotificationEmail,
			Recipient:        "user@example.com",
			Subject:          "Hello",
			Message:          "Body",
		},
	}
	result, err := dispatcher.Attempt(tenantContext(), job)
	if err != nil {
		t.Fatalf("Attempt returned error: %v", err)
	}
	if !emailSender.called {
		t.Fatalf("expected email sender to be invoked")
	}
	if result.Status != string(model.StatusSent) {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

func TestNotificationDispatcherSMSDisabled(t *testing.T) {
	serviceInstance := &notificationServiceImpl{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	dispatcher := newNotificationDispatcher(serviceInstance)
	job := scheduler.Job{
		Payload: &model.Notification{
			TenantID:         testTenantID,
			NotificationType: model.NotificationSMS,
			Recipient:        "+1222",
			Message:          "Body",
		},
	}
	result, err := dispatcher.Attempt(tenantContextWithoutSMS(), job)
	if err != ErrSMSDisabled {
		t.Fatalf("expected ErrSMSDisabled, got %v", err)
	}
	if result.Status != string(model.StatusErrored) {
		t.Fatalf("unexpected status %q", result.Status)
	}
}

func TestNotificationDispatcherSMSSuccess(t *testing.T) {
	sender := &testSmsSender{response: "sid-123"}
	serviceInstance := &notificationServiceImpl{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		defaultSmsSender: sender,
	}
	dispatcher := newNotificationDispatcher(serviceInstance)
	job := scheduler.Job{
		Payload: &model.Notification{
			TenantID:         testTenantID,
			NotificationType: model.NotificationSMS,
			Recipient:        "+1333",
			Message:          "Body",
		},
	}
	result, err := dispatcher.Attempt(tenantContext(), job)
	if err != nil {
		t.Fatalf("Attempt returned error: %v", err)
	}
	if !sender.called {
		t.Fatalf("expected sms sender to be invoked")
	}
	if result.Status != string(model.StatusSent) {
		t.Fatalf("unexpected status %q", result.Status)
	}
	if result.ProviderMessageID != sender.response {
		t.Fatalf("expected provider message ID %q, got %q", sender.response, result.ProviderMessageID)
	}
}

func TestNotificationRetryStoreFetchesJobsPerTenant(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	if err := database.AutoMigrate(&tenant.Tenant{}); err != nil {
		t.Fatalf("tenant migration error: %v", err)
	}
	tenants := []tenant.Tenant{
		{ID: "tenant-retry-1", Status: tenant.TenantStatusActive},
		{ID: "tenant-retry-2", Status: tenant.TenantStatusActive},
		{ID: "tenant-suspended", Status: tenant.TenantStatusSuspended},
	}
	for _, tenantRow := range tenants {
		if err := database.WithContext(context.Background()).Create(&tenantRow).Error; err != nil {
			t.Fatalf("create tenant error: %v", err)
		}
	}
	now := time.Now().UTC()
	records := []model.Notification{
		{
			TenantID:         tenants[0].ID,
			NotificationID:   "notif-retry-1",
			NotificationType: model.NotificationEmail,
			Recipient:        "one@example.com",
			Message:          "Body",
			Status:           model.StatusQueued,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			TenantID:         tenants[1].ID,
			NotificationID:   "notif-retry-2",
			NotificationType: model.NotificationEmail,
			Recipient:        "two@example.com",
			Message:          "Body",
			Status:           model.StatusErrored,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			TenantID:         tenants[2].ID,
			NotificationID:   "notif-retry-ignored",
			NotificationType: model.NotificationEmail,
			Recipient:        "ignored@example.com",
			Message:          "Body",
			Status:           model.StatusQueued,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	for index := range records {
		if err := model.CreateNotification(context.Background(), database, &records[index]); err != nil {
			t.Fatalf("create notification error: %v", err)
		}
	}
	repository := tenant.NewRepository(database, nil)
	store := newNotificationRetryStore(database, repository)

	jobs, err := store.PendingJobs(context.Background(), 5, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("pending jobs error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected jobs for two active tenants, got %d", len(jobs))
	}
	tenantSet := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		record, ok := job.Payload.(*model.Notification)
		if !ok {
			t.Fatalf("unexpected payload type %T", job.Payload)
		}
		tenantSet[record.TenantID] = struct{}{}
	}
	for _, tenantID := range []string{tenants[0].ID, tenants[1].ID} {
		if _, ok := tenantSet[tenantID]; !ok {
			t.Fatalf("expected tenant %s in job payloads", tenantID)
		}
	}
	if _, ok := tenantSet[tenants[2].ID]; ok {
		t.Fatalf("suspended tenant should not contribute jobs")
	}
}

func TestNotificationRetryStoreWithoutTenantRepository(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	now := time.Now().UTC()
	expectedJobs := 3
	for index := 0; index < expectedJobs; index++ {
		record := model.Notification{
			TenantID:         fmt.Sprintf("tenant-fallback-%d", index),
			NotificationID:   fmt.Sprintf("notif-fallback-%d", index),
			NotificationType: model.NotificationEmail,
			Recipient:        fmt.Sprintf("user-%d@example.com", index),
			Message:          "Body",
			Status:           model.StatusQueued,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := model.CreateNotification(context.Background(), database, &record); err != nil {
			t.Fatalf("create notification error: %v", err)
		}
	}

	store := newNotificationRetryStore(database, nil)
	jobs, err := store.PendingJobs(context.Background(), 5, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("pending jobs error: %v", err)
	}
	if len(jobs) != expectedJobs {
		t.Fatalf("expected %d jobs, got %d", expectedJobs, len(jobs))
	}
}
