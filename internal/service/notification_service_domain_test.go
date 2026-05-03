package service

import (
	"context"
	"errors"
	"io"
	"strings"
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

func TestGetNotificationStatusReturnsStoredRecord(t *testing.T) {
	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)
	now := time.Now().UTC()
	insertNotificationRecord(t, database, model.Notification{
		NotificationID:   "notif-status",
		NotificationType: model.NotificationEmail,
		Recipient:        "status@example.com",
		Message:          "status",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	})

	response, err := serviceInstance.GetNotificationStatus(tenantContext(), "notif-status")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if response.NotificationID != "notif-status" || response.Status != model.StatusQueued {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestListNotificationsAllReturnsRecordsAcrossTenants(t *testing.T) {
	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)
	now := time.Now().UTC()
	insertNotificationRecord(t, database, model.Notification{
		TenantID:         testTenantID,
		NotificationID:   "notif-alpha",
		NotificationType: model.NotificationEmail,
		Recipient:        "alpha@example.com",
		Message:          "alpha",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	insertNotificationRecord(t, database, model.Notification{
		TenantID:         "tenant-other",
		NotificationID:   "notif-bravo",
		NotificationType: model.NotificationEmail,
		Recipient:        "bravo@example.com",
		Message:          "bravo",
		Status:           model.StatusSent,
		CreatedAt:        now.Add(time.Second),
		UpdatedAt:        now.Add(time.Second),
	})

	responses, err := serviceInstance.ListNotificationsAll(context.Background(), model.NotificationListFilters{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("expected two responses, got %d", len(responses))
	}
}

func TestNotificationServiceRequiresTenantContext(t *testing.T) {
	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)
	request := mustNotificationRequest(t, model.NotificationEmail, "user@example.com", "Subject", "Body", nil, nil)

	if _, err := serviceInstance.SendNotification(context.Background(), request); !errors.Is(err, ErrMissingTenantContext) {
		t.Fatalf("expected missing tenant on send, got %v", err)
	}
	if _, err := serviceInstance.GetNotificationStatus(context.Background(), "notif"); !errors.Is(err, ErrMissingTenantContext) {
		t.Fatalf("expected missing tenant on get, got %v", err)
	}
	if _, err := serviceInstance.ListNotifications(context.Background(), model.NotificationListFilters{}); !errors.Is(err, ErrMissingTenantContext) {
		t.Fatalf("expected missing tenant on list, got %v", err)
	}
	if _, err := serviceInstance.RescheduleNotification(context.Background(), "notif", time.Now()); !errors.Is(err, ErrMissingTenantContext) {
		t.Fatalf("expected missing tenant on reschedule, got %v", err)
	}
	if _, err := serviceInstance.CancelNotification(context.Background(), "notif"); !errors.Is(err, ErrMissingTenantContext) {
		t.Fatalf("expected missing tenant on cancel, got %v", err)
	}
}

func TestNotificationServicePropagatesStorageErrors(t *testing.T) {
	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceForDomainTests(database)
	request := mustNotificationRequest(t, model.NotificationEmail, "user@example.com", "Subject", "Body", nil, nil)
	closeDatabase(t, database)

	if _, err := serviceInstance.SendNotification(tenantContext(), request); err == nil {
		t.Fatalf("expected send storage error")
	}
	if _, err := serviceInstance.GetNotificationStatus(tenantContext(), "missing"); err == nil {
		t.Fatalf("expected get storage error")
	}
	if _, err := serviceInstance.ListNotifications(tenantContext(), model.NotificationListFilters{}); err == nil {
		t.Fatalf("expected list storage error")
	}
	if _, err := serviceInstance.ListNotificationsAll(context.Background(), model.NotificationListFilters{}); err == nil {
		t.Fatalf("expected list all storage error")
	}
	if _, err := serviceInstance.RescheduleNotification(tenantContext(), "missing", time.Now()); err == nil {
		t.Fatalf("expected reschedule storage error")
	}
	if _, err := serviceInstance.CancelNotification(tenantContext(), "missing"); err == nil {
		t.Fatalf("expected cancel storage error")
	}
}

func TestSendNotificationReturnsEmailSenderResolutionError(t *testing.T) {
	database := openIsolatedDatabase(t)
	serviceInstance := &notificationServiceImpl{
		database:     database,
		logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		config:       config.Config{ConnectionTimeoutSec: 5, OperationTimeoutSec: 5},
		emailSenders: make(map[string]EmailSender),
		smsSenders:   make(map[string]SmsSender),
	}
	request := mustNotificationRequest(t, model.NotificationEmail, "user@example.com", "Subject", "Body", nil, nil)
	runtimeCtx := tenant.WithRuntime(context.Background(), tenant.RuntimeConfig{Tenant: tenant.Tenant{ID: "tenant-empty-email"}})
	if _, err := serviceInstance.SendNotification(runtimeCtx, request); err == nil || !strings.Contains(err.Error(), "email credentials unavailable") {
		t.Fatalf("expected email sender resolution error, got %v", err)
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

func TestRescheduleAndCancelPropagateSaveErrors(t *testing.T) {
	testCases := []struct {
		name string
		call func(*notificationServiceImpl, time.Time) error
	}{
		{
			name: "reschedule",
			call: func(serviceInstance *notificationServiceImpl, now time.Time) error {
				_, err := serviceInstance.RescheduleNotification(tenantContext(), "notif-edit", now.Add(time.Hour))
				return err
			},
		},
		{
			name: "cancel",
			call: func(serviceInstance *notificationServiceImpl, now time.Time) error {
				_, err := serviceInstance.CancelNotification(tenantContext(), "notif-edit")
				return err
			},
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			database := openIsolatedDatabase(t)
			serviceInstance := newNotificationServiceForDomainTests(database)
			now := time.Now().UTC()
			insertNotificationRecord(t, database, model.Notification{
				NotificationID:   "notif-edit",
				NotificationType: model.NotificationEmail,
				Recipient:        "edit@example.com",
				Message:          "queued",
				Status:           model.StatusQueued,
				CreatedAt:        now,
				UpdatedAt:        now,
			})
			registerNotificationUpdateError(t, database)
			if err := testCase.call(serviceInstance, now); err == nil {
				t.Fatalf("expected save error")
			}
		})
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

func TestNotificationServiceUsesInjectedDefaultsAndRuntimeFallbacks(t *testing.T) {
	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	smsSender := &stubSmsSender{}
	serviceInterface := NewNotificationServiceWithSenders(
		database,
		slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		config.Config{MaxRetries: 3, RetryIntervalSec: 1},
		nil,
		emailSender,
		smsSender,
	)
	serviceInstance := serviceInterface.(*notificationServiceImpl)
	if serviceInstance.defaultEmailSender != emailSender || serviceInstance.defaultSmsSender != smsSender {
		t.Fatalf("expected injected default senders")
	}
	runtimeCfg, err := serviceInstance.runtimeForTenantID(context.Background(), "tenant-from-payload")
	if err != nil {
		t.Fatalf("runtime fallback: %v", err)
	}
	if runtimeCfg.Tenant.ID != "tenant-from-payload" {
		t.Fatalf("unexpected fallback runtime %+v", runtimeCfg)
	}
}

func TestRuntimeForTenantIDValidation(t *testing.T) {
	bareService := &notificationServiceImpl{}
	if _, err := bareService.runtimeForTenantID(context.Background(), ""); !errors.Is(err, ErrMissingTenantContext) {
		t.Fatalf("expected missing tenant for blank id, got %v", err)
	}
	if _, err := bareService.runtimeForTenantID(context.Background(), "tenant"); !errors.Is(err, ErrMissingTenantContext) {
		t.Fatalf("expected missing tenant without default senders, got %v", err)
	}
	serviceInstance := newNotificationServiceForDomainTests(openIsolatedDatabase(t))
	if _, err := serviceInstance.runtimeForTenantID(tenantContext(), "other-tenant"); err == nil || !strings.Contains(err.Error(), "tenant mismatch") {
		t.Fatalf("expected tenant mismatch, got %v", err)
	}
	runtimeCfg, err := serviceInstance.runtimeForTenantID(tenantContext(), testTenantID)
	if err != nil {
		t.Fatalf("expected runtime from context: %v", err)
	}
	if runtimeCfg.Tenant.ID != testTenantID {
		t.Fatalf("unexpected runtime tenant %s", runtimeCfg.Tenant.ID)
	}

	database := openIsolatedDatabase(t)
	if err := database.AutoMigrate(&tenant.Tenant{}, &tenant.TenantDomain{}, &tenant.EmailProfile{}, &tenant.SMSProfile{}); err != nil {
		t.Fatalf("tenant migration: %v", err)
	}
	keeper, err := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("secret keeper: %v", err)
	}
	if err := tenant.Bootstrap(context.Background(), database, keeper, tenant.BootstrapConfig{
		Tenants: []tenant.BootstrapTenant{
			{
				ID:           "tenant-repo",
				DisplayName:  "Repo Tenant",
				SupportEmail: "support@example.com",
				Enabled:      ptrBool(true),
				Domains:      []string{"repo.example"},
				EmailProfile: tenant.BootstrapEmailProfile{
					Host:        "smtp.example.com",
					Port:        587,
					Username:    "smtp-user",
					Password:    "smtp-pass",
					FromAddress: "from@example.com",
				},
			},
		},
	}); err != nil {
		t.Fatalf("bootstrap tenant repo: %v", err)
	}
	repoService := &notificationServiceImpl{tenantRepo: tenant.NewRepository(database, keeper)}
	repoRuntime, err := repoService.runtimeForTenantID(context.Background(), "tenant-repo")
	if err != nil {
		t.Fatalf("tenant repo runtime: %v", err)
	}
	if repoRuntime.Tenant.ID != "tenant-repo" {
		t.Fatalf("unexpected tenant repo runtime %+v", repoRuntime)
	}
}

func TestSenderResolutionErrors(t *testing.T) {
	serviceInstance := &notificationServiceImpl{
		logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		config:       config.Config{ConnectionTimeoutSec: 5, OperationTimeoutSec: 5},
		emailSenders: make(map[string]EmailSender),
		smsSenders:   make(map[string]SmsSender),
	}
	if _, err := serviceInstance.emailSenderForTenant(tenant.RuntimeConfig{Tenant: tenant.Tenant{ID: "tenant-empty"}}); err == nil {
		t.Fatalf("expected missing email credentials error")
	}
	if _, err := serviceInstance.smsSenderForTenant(tenant.RuntimeConfig{Tenant: tenant.Tenant{ID: "tenant-empty"}}); !errors.Is(err, ErrSMSDisabled) {
		t.Fatalf("expected sms disabled, got %v", err)
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

func closeDatabase(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	if closeErr := sqlDatabase.Close(); closeErr != nil {
		t.Fatalf("close database: %v", closeErr)
	}
}

func registerNotificationUpdateError(t *testing.T, database *gorm.DB) {
	t.Helper()
	callbackName := "pinguin:force_notification_update_error"
	if err := database.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Notification" {
			tx.AddError(errors.New("forced notification update failure"))
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
}

func ptrBool(value bool) *bool {
	return &value
}
