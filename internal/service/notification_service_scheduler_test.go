package service

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/utils/scheduler"
	"gorm.io/gorm"
	"log/slog"
)

func TestSendNotificationRespectsSchedule(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name                    string
		scheduledOffset         *time.Duration
		expectedStatus          model.NotificationStatus
		expectImmediateDispatch bool
	}{
		{
			name:                    "ImmediateSendWithoutSchedule",
			scheduledOffset:         nil,
			expectedStatus:          model.StatusSent,
			expectImmediateDispatch: true,
		},
		{
			name:                    "ImmediateSendForPastSchedule",
			scheduledOffset:         durationPointer(-1 * time.Minute),
			expectedStatus:          model.StatusSent,
			expectImmediateDispatch: true,
		},
		{
			name:                    "QueuedForFutureSchedule",
			scheduledOffset:         durationPointer(2 * time.Minute),
			expectedStatus:          model.StatusQueued,
			expectImmediateDispatch: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			database := openIsolatedDatabase(t)
			emailSender := &stubEmailSender{}
			smsSender := &stubSmsSender{}

			serviceInstance := newNotificationServiceWithSendersForSchedulerTests(database, emailSender, smsSender)
			serviceInstance.maxRetries = 5

			var scheduledFor *time.Time
			if testCase.scheduledOffset != nil {
				scheduledTime := time.Now().UTC().Add(*testCase.scheduledOffset)
				scheduledFor = &scheduledTime
			}

			request := mustNotificationRequest(
				t,
				model.NotificationEmail,
				"user@example.com",
				"Subject",
				"Body",
				scheduledFor,
				nil,
			)

			response, responseError := serviceInstance.SendNotification(tenantContext(), request)
			if responseError != nil {
				t.Fatalf("SendNotification error: %v", responseError)
			}

			if response.Status != testCase.expectedStatus {
				t.Fatalf("unexpected status %s", response.Status)
			}

			if testCase.expectImmediateDispatch && emailSender.callCount != 1 {
				t.Fatalf("expected immediate dispatch")
			}
			if !testCase.expectImmediateDispatch && emailSender.callCount != 0 {
				t.Fatalf("unexpected immediate dispatch")
			}

			if testCase.scheduledOffset == nil && response.ScheduledFor != nil {
				t.Fatalf("expected nil scheduledFor in response")
			}
			if testCase.scheduledOffset != nil {
				if response.ScheduledFor == nil {
					t.Fatalf("expected scheduledFor value in response")
				}
			}
		})
	}
}

func TestRetryWorkerRespectsSchedule(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	smsSender := &stubSmsSender{}
	serviceInstance := newNotificationServiceWithSendersForSchedulerTests(database, emailSender, smsSender)
	serviceInstance.maxRetries = 5

	now := time.Now().UTC()
	future := now.Add(5 * time.Minute)
	scheduledNotification := model.Notification{
		TenantID:         testTenantID,
		NotificationID:   "notif-scheduled",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "Body",
		Status:           model.StatusQueued,
		ScheduledFor:     &future,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if createErr := model.CreateNotification(tenantContext(), database, &scheduledNotification); createErr != nil {
		t.Fatalf("create notification error: %v", createErr)
	}

	clock := &adjustableClock{now: now}
	worker := newRetryWorkerForTest(t, serviceInstance, clock)
	worker.RunOnce(tenantContext())
	if emailSender.callCount != 0 {
		t.Fatalf("expected no dispatch before schedule")
	}

	past := now.Add(-1 * time.Minute)
	scheduledNotification.ScheduledFor = &past
	scheduledNotification.Status = model.StatusQueued
	if saveErr := model.SaveNotification(tenantContext(), database, &scheduledNotification); saveErr != nil {
		t.Fatalf("save notification error: %v", saveErr)
	}

	clock.now = now.Add(30 * time.Minute)
	worker.RunOnce(tenantContext())
	if emailSender.callCount != 1 {
		t.Fatalf("expected dispatch after schedule")
	}

	persisted, fetchErr := model.GetNotificationByID(tenantContext(), database, testTenantID, "notif-scheduled")
	if fetchErr != nil {
		t.Fatalf("fetch notification error: %v", fetchErr)
	}
	if persisted.Status != model.StatusSent {
		t.Fatalf("expected status sent, got %s", persisted.Status)
	}
	if persisted.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", persisted.RetryCount)
	}
	if persisted.LastAttemptedAt.IsZero() {
		t.Fatalf("expected last attempted timestamp")
	}
}

func TestSendNotificationRejectsSmsWhenSenderDisabled(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	serviceInstance := newNotificationServiceWithSendersForSchedulerTests(database, emailSender, nil)
	serviceInstance.maxRetries = 3

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic when sms sender disabled: %v", recovered)
		}
	}()

	request := mustNotificationRequest(
		t,
		model.NotificationSMS,
		"+15555555555",
		"",
		"Body",
		nil,
		nil,
	)
	_, sendError := serviceInstance.SendNotification(tenantContextWithoutSMS(), request)
	if sendError == nil {
		t.Fatalf("expected error when sms sender is disabled")
	}

	var notificationCount int64
	if countError := database.WithContext(tenantContextWithoutSMS()).Model(&model.Notification{}).Count(&notificationCount).Error; countError != nil {
		t.Fatalf("count notifications error: %v", countError)
	}
	if notificationCount != 0 {
		t.Fatalf("expected zero notifications stored, got %d", notificationCount)
	}
}

func TestSendNotificationPersistsAttachments(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	serviceInstance := newNotificationServiceWithSendersForSchedulerTests(database, emailSender, &stubSmsSender{})
	serviceInstance.maxRetries = 3

	attachment := model.EmailAttachment{
		Filename:    "hello.txt",
		ContentType: "text/plain",
		Data:        []byte("hi"),
	}

	request := mustNotificationRequest(
		t,
		model.NotificationEmail,
		"user@example.com",
		"Subject",
		"Body",
		nil,
		[]model.EmailAttachment{attachment},
	)
	response, err := serviceInstance.SendNotification(tenantContext(), request)
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	if emailSender.callCount != 1 {
		t.Fatalf("expected immediate email dispatch")
	}
	if len(emailSender.receivedAttachments) != 1 || len(emailSender.receivedAttachments[0]) != 1 {
		t.Fatalf("expected attachment to be forwarded to sender")
	}

	saved, fetchErr := model.GetNotificationByID(tenantContext(), database, testTenantID, response.NotificationID)
	if fetchErr != nil {
		t.Fatalf("fetch error: %v", fetchErr)
	}
	if len(saved.Attachments) != 1 {
		t.Fatalf("expected one stored attachment, got %d", len(saved.Attachments))
	}
	if saved.Attachments[0].Filename != attachment.Filename {
		t.Fatalf("attachment filename mismatch")
	}
}

func TestRetryWorkerMarksSmsDisabledAsFailed(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := newNotificationServiceWithSendersForSchedulerTests(database, &stubEmailSender{}, nil)
	serviceInstance.maxRetries = 3

	now := time.Now().UTC()
	smsNotification := model.Notification{
		TenantID:         testTenantID,
		NotificationID:   "notif-sms-disabled",
		NotificationType: model.NotificationSMS,
		Recipient:        "+15555555555",
		Message:          "Body",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if createErr := model.CreateNotification(tenantContext(), database, &smsNotification); createErr != nil {
		t.Fatalf("create notification error: %v", createErr)
	}

	clock := &adjustableClock{now: now}
	worker := newRetryWorkerForTest(t, serviceInstance, clock)
	worker.RunOnce(tenantContextWithoutSMS())

	updated, fetchErr := model.GetNotificationByID(tenantContext(), database, testTenantID, "notif-sms-disabled")
	if fetchErr != nil {
		t.Fatalf("fetch notification error: %v", fetchErr)
	}
	if updated.Status != model.StatusErrored {
		t.Fatalf("expected status errored, got %s", updated.Status)
	}
	if updated.RetryCount != 1 {
		t.Fatalf("expected retry count increment, got %d", updated.RetryCount)
	}
}

func TestRetryWorkerDispatchesStoredAttachments(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	serviceInstance := newNotificationServiceWithSendersForSchedulerTests(database, emailSender, &stubSmsSender{})
	serviceInstance.maxRetries = 3

	future := time.Now().UTC().Add(5 * time.Minute)
	request := mustNotificationRequest(
		t,
		model.NotificationEmail,
		"user@example.com",
		"Subject",
		"Body",
		&future,
		[]model.EmailAttachment{
			{
				Filename:    "data.txt",
				ContentType: "text/plain",
				Data:        []byte("content"),
			},
		},
	)
	response, err := serviceInstance.SendNotification(tenantContext(), request)
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	stored, fetchErr := model.GetNotificationByID(tenantContext(), database, testTenantID, response.NotificationID)
	if fetchErr != nil {
		t.Fatalf("fetch error: %v", fetchErr)
	}
	past := time.Now().UTC().Add(-1 * time.Minute)
	stored.ScheduledFor = &past
	stored.Status = model.StatusQueued
	if saveErr := model.SaveNotification(tenantContext(), database, stored); saveErr != nil {
		t.Fatalf("save error: %v", saveErr)
	}

	clock := &adjustableClock{now: time.Now().UTC()}
	worker := newRetryWorkerForTest(t, serviceInstance, clock)
	worker.RunOnce(tenantContext())
	if emailSender.callCount != 1 {
		t.Fatalf("expected retry to dispatch email")
	}
	if len(emailSender.receivedAttachments) != 1 || len(emailSender.receivedAttachments[0]) != 1 {
		t.Fatalf("expected attachments to flow through retry")
	}
}

type adjustableClock struct {
	now time.Time
}

func (clock *adjustableClock) Now() time.Time {
	return clock.now
}

func newRetryWorkerForTest(t *testing.T, serviceInstance *notificationServiceImpl, clock scheduler.Clock) *scheduler.Worker {
	t.Helper()

	worker, err := scheduler.NewWorker(scheduler.Config{
		Repository:    newNotificationRetryStore(serviceInstance.database, nil),
		Dispatcher:    newNotificationDispatcher(serviceInstance),
		Logger:        serviceInstance.logger,
		Interval:      time.Duration(serviceInstance.retryIntervalSec) * time.Second,
		MaxRetries:    serviceInstance.maxRetries,
		SuccessStatus: string(model.StatusSent),
		FailureStatus: string(model.StatusErrored),
		Clock:         clock,
	})
	if err != nil {
		t.Fatalf("worker init error: %v", err)
	}
	return worker
}

type stubEmailSender struct {
	callCount           int
	receivedAttachments [][]model.EmailAttachment
}

func (sender *stubEmailSender) SendEmail(_ context.Context, _ string, _ string, _ string, attachments []model.EmailAttachment) error {
	sender.callCount++
	cloned := make([]model.EmailAttachment, len(attachments))
	copy(cloned, attachments)
	sender.receivedAttachments = append(sender.receivedAttachments, cloned)
	return nil
}

type stubSmsSender struct {
	callCount int
}

func (sender *stubSmsSender) SendSms(_ context.Context, _ string, _ string) (string, error) {
	sender.callCount++
	return "queued", nil
}

func mustNotificationRequest(
	testHandle *testing.T,
	notificationType model.NotificationType,
	recipient string,
	subject string,
	message string,
	scheduledFor *time.Time,
	attachments []model.EmailAttachment,
) model.NotificationRequest {
	testHandle.Helper()

	request, requestErr := model.NewNotificationRequest(
		notificationType,
		recipient,
		subject,
		message,
		scheduledFor,
		attachments,
	)
	if requestErr != nil {
		testHandle.Fatalf("notification request error: %v", requestErr)
	}
	return request
}

func openIsolatedDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	databaseName := time.Now().UTC().Format("20060102150405.000000000")
	database, openError := gorm.Open(sqlite.Open("file:"+databaseName+"?mode=memory&cache=shared"), &gorm.Config{})
	if openError != nil {
		t.Fatalf("sqlite open error: %v", openError)
	}
	if migrateError := database.AutoMigrate(&model.Notification{}, &model.NotificationAttachment{}); migrateError != nil {
		t.Fatalf("migration error: %v", migrateError)
	}
	return database
}

func durationPointer(value time.Duration) *time.Duration {
	return &value
}

func newNotificationServiceWithSendersForSchedulerTests(database *gorm.DB, emailSender EmailSender, smsSender SmsSender) *notificationServiceImpl {
	return &notificationServiceImpl{
		database:           database,
		logger:             slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		defaultEmailSender: emailSender,
		defaultSmsSender:   smsSender,
		maxRetries:         5,
		retryIntervalSec:   1,
		emailSenders:       make(map[string]EmailSender),
		smsSenders:         make(map[string]SmsSender),
	}
}
