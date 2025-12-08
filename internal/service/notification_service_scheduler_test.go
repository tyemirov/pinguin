package service

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/pkg/scheduler"
	"gorm.io/driver/sqlite"
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

			serviceInstance := &notificationServiceImpl{
				database:         database,
				logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
				emailSender:      emailSender,
				smsSender:        smsSender,
				maxRetries:       5,
				retryIntervalSec: 1,
				smsEnabled:       true,
			}

			var scheduledFor *time.Time
			if testCase.scheduledOffset != nil {
				scheduledTime := time.Now().UTC().Add(*testCase.scheduledOffset)
				scheduledFor = &scheduledTime
			}

			request := model.NotificationRequest{
				NotificationType: model.NotificationEmail,
				Recipient:        "user@example.com",
				Subject:          "Subject",
				Message:          "Body",
				ScheduledFor:     scheduledFor,
			}

			response, responseError := serviceInstance.SendNotification(context.Background(), request)
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

func TestSendNotificationRejectsUnsupportedTypes(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name            string
		scheduledOffset *time.Duration
	}{
		{
			name:            "ImmediateUnsupportedType",
			scheduledOffset: nil,
		},
		{
			name:            "ScheduledUnsupportedType",
			scheduledOffset: durationPointer(2 * time.Minute),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			database := openIsolatedDatabase(t)
			emailSender := &stubEmailSender{}
			smsSender := &stubSmsSender{}

			serviceInstance := &notificationServiceImpl{
				database:         database,
				logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
				emailSender:      emailSender,
				smsSender:        smsSender,
				maxRetries:       5,
				retryIntervalSec: 1,
				smsEnabled:       true,
			}

			var scheduledFor *time.Time
			if testCase.scheduledOffset != nil {
				scheduledTime := time.Now().UTC().Add(*testCase.scheduledOffset)
				scheduledFor = &scheduledTime
			}

			_, responseError := serviceInstance.SendNotification(context.Background(), model.NotificationRequest{
				NotificationType: model.NotificationType("push"),
				Recipient:        "user@example.com",
				Subject:          "Subject",
				Message:          "Body",
				ScheduledFor:     scheduledFor,
			})
			if responseError == nil {
				t.Fatalf("expected unsupported type error")
			}

			if emailSender.callCount != 0 {
				t.Fatalf("unexpected email dispatch attempts")
			}
			if smsSender.callCount != 0 {
				t.Fatalf("unexpected sms dispatch attempts")
			}

			pendingNotifications, pendingError := model.GetQueuedOrFailedNotifications(context.Background(), database, 5, time.Now().UTC())
			if pendingError != nil {
				t.Fatalf("pending notifications error: %v", pendingError)
			}
			if len(pendingNotifications) != 0 {
				t.Fatalf("unexpected stored notifications")
			}
		})
	}
}

func TestRetryWorkerRespectsSchedule(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	smsSender := &stubSmsSender{}
	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      emailSender,
		smsSender:        smsSender,
		maxRetries:       5,
		retryIntervalSec: 1,
		smsEnabled:       true,
	}

	now := time.Now().UTC()
	future := now.Add(5 * time.Minute)
	scheduledNotification := model.Notification{
		NotificationID:   "notif-scheduled",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "Body",
		Status:           model.StatusQueued,
		ScheduledFor:     &future,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if createErr := model.CreateNotification(context.Background(), database, &scheduledNotification); createErr != nil {
		t.Fatalf("create notification error: %v", createErr)
	}

	clock := &adjustableClock{now: now}
	worker := newRetryWorkerForTest(t, serviceInstance, clock)
	worker.RunOnce(context.Background())
	if emailSender.callCount != 0 {
		t.Fatalf("expected no dispatch before schedule")
	}

	past := now.Add(-1 * time.Minute)
	scheduledNotification.ScheduledFor = &past
	scheduledNotification.Status = model.StatusQueued
	if saveErr := model.SaveNotification(context.Background(), database, &scheduledNotification); saveErr != nil {
		t.Fatalf("save notification error: %v", saveErr)
	}

	clock.now = now.Add(30 * time.Minute)
	worker.RunOnce(context.Background())
	if emailSender.callCount != 1 {
		t.Fatalf("expected dispatch after schedule")
	}

	persisted, fetchErr := model.GetNotificationByID(context.Background(), database, "notif-scheduled")
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

func TestSendNotificationValidatesRequiredFields(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	smsSender := &stubSmsSender{}

	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      emailSender,
		smsSender:        smsSender,
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       true,
	}

	_, sendError := serviceInstance.SendNotification(context.Background(), model.NotificationRequest{
		NotificationType: model.NotificationSMS,
		Recipient:        "",
		Message:          "Body",
	})
	if sendError == nil {
		t.Fatalf("expected validation error")
	}

	_, fetchError := model.GetQueuedOrFailedNotifications(context.Background(), database, 3, time.Now().UTC())
	if fetchError != nil {
		t.Fatalf("fetch pending error: %v", fetchError)
	}

	if emailSender.callCount != 0 || smsSender.callCount != 0 {
		t.Fatalf("unexpected dispatch attempts")
	}
}

func TestSendNotificationRejectsUnsupportedTypeForScheduledRequests(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	smsSender := &stubSmsSender{}

	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      emailSender,
		smsSender:        smsSender,
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       true,
	}

	scheduledFor := time.Now().UTC().Add(5 * time.Minute)
	_, sendError := serviceInstance.SendNotification(context.Background(), model.NotificationRequest{
		NotificationType: model.NotificationType("push"),
		Recipient:        "user@example.com",
		Message:          "Body",
		ScheduledFor:     &scheduledFor,
	})

	if sendError == nil {
		t.Fatalf("expected unsupported type error")
	}

	if emailSender.callCount != 0 || smsSender.callCount != 0 {
		t.Fatalf("unexpected dispatch attempts")
	}

	var notificationCount int64
	if countError := database.WithContext(context.Background()).Model(&model.Notification{}).Count(&notificationCount).Error; countError != nil {
		t.Fatalf("count notifications error: %v", countError)
	}
	if notificationCount != 0 {
		t.Fatalf("expected zero stored notifications, got %d", notificationCount)
	}
}

func TestSendNotificationRejectsSmsWhenSenderDisabled(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      &stubEmailSender{},
		smsSender:        nil,
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       false,
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic when sms sender disabled: %v", recovered)
		}
	}()

	_, sendError := serviceInstance.SendNotification(context.Background(), model.NotificationRequest{
		NotificationType: model.NotificationSMS,
		Recipient:        "+15555555555",
		Message:          "Body",
	})
	if sendError == nil {
		t.Fatalf("expected error when sms sender is disabled")
	}

	var notificationCount int64
	if countError := database.WithContext(context.Background()).Model(&model.Notification{}).Count(&notificationCount).Error; countError != nil {
		t.Fatalf("count notifications error: %v", countError)
	}
	if notificationCount != 0 {
		t.Fatalf("expected zero notifications stored, got %d", notificationCount)
	}
}

func TestSendNotificationRejectsAttachmentsForSms(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      &stubEmailSender{},
		smsSender:        &stubSmsSender{},
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       true,
	}

	_, err := serviceInstance.SendNotification(context.Background(), model.NotificationRequest{
		NotificationType: model.NotificationSMS,
		Recipient:        "+15555550100",
		Message:          "Body",
		Attachments: []model.EmailAttachment{
			{
				Filename:    "secret.txt",
				ContentType: "text/plain",
				Data:        []byte("data"),
			},
		},
	})
	if err == nil {
		t.Fatalf("expected attachment rejection for sms")
	}
}

func TestSendNotificationPersistsAttachments(t *testing.T) {
	t.Helper()

	database := openIsolatedDatabase(t)
	emailSender := &stubEmailSender{}
	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      emailSender,
		smsSender:        &stubSmsSender{},
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       true,
	}

	attachment := model.EmailAttachment{
		Filename:    "hello.txt",
		ContentType: "text/plain",
		Data:        []byte("hi"),
	}

	response, err := serviceInstance.SendNotification(context.Background(), model.NotificationRequest{
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Subject:          "Subject",
		Message:          "Body",
		Attachments:      []model.EmailAttachment{attachment},
	})
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	if emailSender.callCount != 1 {
		t.Fatalf("expected immediate email dispatch")
	}
	if len(emailSender.receivedAttachments) != 1 || len(emailSender.receivedAttachments[0]) != 1 {
		t.Fatalf("expected attachment to be forwarded to sender")
	}

	saved, fetchErr := model.GetNotificationByID(context.Background(), database, response.NotificationID)
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
	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      &stubEmailSender{},
		smsSender:        nil,
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       false,
	}

	now := time.Now().UTC()
	smsNotification := model.Notification{
		NotificationID:   "notif-sms-disabled",
		NotificationType: model.NotificationSMS,
		Recipient:        "+15555555555",
		Message:          "Body",
		Status:           model.StatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if createErr := model.CreateNotification(context.Background(), database, &smsNotification); createErr != nil {
		t.Fatalf("create notification error: %v", createErr)
	}

	clock := &adjustableClock{now: now}
	worker := newRetryWorkerForTest(t, serviceInstance, clock)
	worker.RunOnce(context.Background())

	updated, fetchErr := model.GetNotificationByID(context.Background(), database, "notif-sms-disabled")
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
	serviceInstance := &notificationServiceImpl{
		database:         database,
		logger:           slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		emailSender:      emailSender,
		smsSender:        &stubSmsSender{},
		maxRetries:       3,
		retryIntervalSec: 1,
		smsEnabled:       true,
	}

	future := time.Now().UTC().Add(5 * time.Minute)
	response, err := serviceInstance.SendNotification(context.Background(), model.NotificationRequest{
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Subject:          "Subject",
		Message:          "Body",
		ScheduledFor:     &future,
		Attachments: []model.EmailAttachment{
			{
				Filename:    "data.txt",
				ContentType: "text/plain",
				Data:        []byte("content"),
			},
		},
	})
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	stored, fetchErr := model.GetNotificationByID(context.Background(), database, response.NotificationID)
	if fetchErr != nil {
		t.Fatalf("fetch error: %v", fetchErr)
	}
	past := time.Now().UTC().Add(-1 * time.Minute)
	stored.ScheduledFor = &past
	stored.Status = model.StatusQueued
	if saveErr := model.SaveNotification(context.Background(), database, stored); saveErr != nil {
		t.Fatalf("save error: %v", saveErr)
	}

	clock := &adjustableClock{now: time.Now().UTC()}
	worker := newRetryWorkerForTest(t, serviceInstance, clock)
	worker.RunOnce(context.Background())
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
		Repository:    newNotificationRetryStore(serviceInstance.database),
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
