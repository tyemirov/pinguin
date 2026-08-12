package service

import (
	"context"
	"errors"
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

type staticTenantRuntimeResolver struct {
	runtime tenant.RuntimeConfig
	err     error
}

func (resolver staticTenantRuntimeResolver) ResolveByID(_ context.Context, tenantID string) (tenant.RuntimeConfig, error) {
	if resolver.err != nil {
		return tenant.RuntimeConfig{}, resolver.err
	}
	if resolver.runtime.Tenant.ID != tenantID {
		return tenant.RuntimeConfig{}, tenant.ErrTenantNotFound
	}
	return resolver.runtime, nil
}

func testTenantRuntimeResolver() staticTenantRuntimeResolver {
	return staticTenantRuntimeResolver{runtime: tenant.RuntimeConfig{Tenant: tenant.Tenant{ID: testTenantID}}}
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
		tenantRepo:         testTenantRuntimeResolver(),
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
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		tenantRepo: testTenantRuntimeResolver(),
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
		tenantRepo:       testTenantRuntimeResolver(),
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
		{ID: "tenant-retry-1"},
		{ID: "tenant-retry-2"},
		{ID: "tenant-retry-3"},
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
	store := newNotificationRetryStore(database)

	jobs, err := store.PendingJobs(context.Background(), 5, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("pending jobs error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected jobs for all three tenants, got %d", len(jobs))
	}
	tenantSet := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		record, ok := job.Payload.(*model.Notification)
		if !ok {
			t.Fatalf("unexpected payload type %T", job.Payload)
		}
		tenantSet[record.TenantID] = struct{}{}
	}
	for _, tenantID := range []string{tenants[0].ID, tenants[1].ID, tenants[2].ID} {
		if _, ok := tenantSet[tenantID]; !ok {
			t.Fatalf("expected tenant %s in job payloads", tenantID)
		}
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

	store := newNotificationRetryStore(database)
	jobs, err := store.PendingJobs(context.Background(), 5, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("pending jobs error: %v", err)
	}
	if len(jobs) != expectedJobs {
		t.Fatalf("expected %d jobs, got %d", expectedJobs, len(jobs))
	}
}

func TestNotificationRetryStoreReportsStorageAndPayloadErrors(t *testing.T) {
	now := time.Now().UTC()
	allDatabase := openIsolatedDatabase(t)
	allStore := newNotificationRetryStore(allDatabase)
	closeDatabase(t, allDatabase)
	if _, err := allStore.PendingJobs(context.Background(), 3, now); err == nil {
		t.Fatalf("expected pending jobs storage error without tenant repo")
	}

	activeDatabase := openIsolatedDatabase(t)
	if err := activeDatabase.AutoMigrate(&tenant.Tenant{}); err != nil {
		t.Fatalf("tenant migration error: %v", err)
	}
	activeStore := newNotificationRetryStore(activeDatabase)
	closeDatabase(t, activeDatabase)
	if _, err := activeStore.PendingJobs(context.Background(), 3, now); err == nil {
		t.Fatalf("expected pending jobs storage error with tenant repo")
	}

	store := newNotificationRetryStore(openIsolatedDatabase(t))
	if err := store.ApplyAttemptResult(context.Background(), scheduler.Job{ID: "missing"}, scheduler.AttemptUpdate{}); err == nil {
		t.Fatalf("expected missing payload error")
	}
	if err := store.ApplyAttemptResult(context.Background(), scheduler.Job{ID: "wrong", Payload: "not a notification"}, scheduler.AttemptUpdate{}); err == nil {
		t.Fatalf("expected invalid payload type error")
	}
}

func TestNotificationRetryStoreCanonicalizesUnknownAttemptStatus(t *testing.T) {
	database := openIsolatedDatabase(t)
	store := newNotificationRetryStore(database)
	now := time.Now().UTC()
	record := &model.Notification{
		TenantID:         testTenantID,
		NotificationID:   "notif-unknown-status",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "Body",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if createErr := model.CreateNotification(tenantContext(), database, record); createErr != nil {
		t.Fatalf("create notification: %v", createErr)
	}
	attemptedAt := now.Add(time.Minute)
	if err := store.ApplyAttemptResult(context.Background(), scheduler.Job{ID: record.NotificationID, Payload: record}, scheduler.AttemptUpdate{
		Status:            "not-a-status",
		ProviderMessageID: "provider-1",
		RetryCount:        2,
		LastAttemptedAt:   attemptedAt,
	}); err != nil {
		t.Fatalf("apply attempt result: %v", err)
	}
	updated, fetchErr := model.GetNotificationByID(tenantContext(), database, testTenantID, record.NotificationID)
	if fetchErr != nil {
		t.Fatalf("fetch updated notification: %v", fetchErr)
	}
	if updated.Status != model.StatusErrored || updated.ProviderMessageID != "provider-1" || updated.RetryCount != 2 {
		t.Fatalf("unexpected updated notification %+v", updated)
	}
}

func TestNotificationDispatcherReportsPayloadRuntimeAndSendFailures(t *testing.T) {
	bareService := &notificationServiceImpl{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	dispatcher := newNotificationDispatcher(bareService)
	if _, err := dispatcher.Attempt(context.Background(), scheduler.Job{}); err == nil {
		t.Fatalf("expected missing payload error")
	}
	if _, err := dispatcher.Attempt(context.Background(), scheduler.Job{Payload: "bad"}); err == nil {
		t.Fatalf("expected invalid payload error")
	}
	runtimeResult, runtimeErr := dispatcher.Attempt(context.Background(), scheduler.Job{Payload: &model.Notification{
		NotificationID:   "notif-no-runtime",
		TenantID:         "tenant-no-runtime",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "Body",
	}})
	if !errors.Is(runtimeErr, ErrMissingTenantContext) || runtimeResult.Status != string(model.StatusErrored) {
		t.Fatalf("expected runtime error result, got result=%+v err=%v", runtimeResult, runtimeErr)
	}

	emailErr := errors.New("email retry failed")
	emailService := &notificationServiceImpl{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		defaultEmailSender: &stubEmailSender{err: emailErr},
		tenantRepo:         testTenantRuntimeResolver(),
	}
	emailResult, err := newNotificationDispatcher(emailService).Attempt(tenantContext(), scheduler.Job{Payload: &model.Notification{
		TenantID:         testTenantID,
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Subject:          "Subject",
		Message:          "Body",
	}})
	if !errors.Is(err, emailErr) || emailResult.Status != "" {
		t.Fatalf("expected email send error without status, got result=%+v err=%v", emailResult, err)
	}

	unavailableEmailService := &notificationServiceImpl{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		tenantRepo: testTenantRuntimeResolver(),
	}
	unavailableEmailResult, unavailableEmailErr := newNotificationDispatcher(unavailableEmailService).Attempt(
		tenant.WithRuntime(context.Background(), tenant.RuntimeConfig{Tenant: tenant.Tenant{ID: testTenantID}}),
		scheduler.Job{Payload: &model.Notification{
			TenantID:         testTenantID,
			NotificationType: model.NotificationEmail,
			Recipient:        "user@example.com",
			Subject:          "Subject",
			Message:          "Body",
		}},
	)
	if unavailableEmailErr == nil || unavailableEmailResult.Status != string(model.StatusErrored) {
		t.Fatalf("expected unavailable email sender error, got result=%+v err=%v", unavailableEmailResult, unavailableEmailErr)
	}

	smsErr := errors.New("sms retry failed")
	smsService := &notificationServiceImpl{
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		defaultSmsSender: &stubSmsSender{err: smsErr},
		tenantRepo:       testTenantRuntimeResolver(),
	}
	smsResult, err := newNotificationDispatcher(smsService).Attempt(tenantContext(), scheduler.Job{Payload: &model.Notification{
		TenantID:         testTenantID,
		NotificationType: model.NotificationSMS,
		Recipient:        "+1222",
		Message:          "Body",
	}})
	if !errors.Is(err, smsErr) || smsResult.Status != "" {
		t.Fatalf("expected sms send error without status, got result=%+v err=%v", smsResult, err)
	}

	unsupportedService := &notificationServiceImpl{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		defaultEmailSender: &testEmailSender{},
		defaultSmsSender:   &testSmsSender{},
		tenantRepo:         testTenantRuntimeResolver(),
	}
	unsupportedResult, unsupportedErr := newNotificationDispatcher(unsupportedService).Attempt(tenantContext(), scheduler.Job{Payload: &model.Notification{
		TenantID:         testTenantID,
		NotificationType: "push",
		Recipient:        "user@example.com",
		Message:          "Body",
	}})
	if unsupportedErr == nil || unsupportedResult.Status != string(model.StatusErrored) {
		t.Fatalf("expected unsupported notification error, got result=%+v err=%v", unsupportedResult, unsupportedErr)
	}
}
