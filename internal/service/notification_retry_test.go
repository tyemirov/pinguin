package service

import (
	"context"
	"io"
	"testing"

	"log/slog"

	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/pkg/scheduler"
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
		emailSender: emailSender,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	dispatcher := newNotificationDispatcher(serviceInstance)
	job := scheduler.Job{
		Payload: &model.Notification{
			NotificationType: model.NotificationEmail,
			Recipient:        "user@example.com",
			Subject:          "Hello",
			Message:          "Body",
		},
	}
	result, err := dispatcher.Attempt(context.Background(), job)
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
		smsEnabled: false,
	}
	dispatcher := newNotificationDispatcher(serviceInstance)
	job := scheduler.Job{
		Payload: &model.Notification{
			NotificationType: model.NotificationSMS,
			Recipient:        "+1222",
			Message:          "Body",
		},
	}
	result, err := dispatcher.Attempt(context.Background(), job)
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
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		smsSender:  sender,
		smsEnabled: true,
	}
	dispatcher := newNotificationDispatcher(serviceInstance)
	job := scheduler.Job{
		Payload: &model.Notification{
			NotificationType: model.NotificationSMS,
			Recipient:        "+1333",
			Message:          "Body",
		},
	}
	result, err := dispatcher.Attempt(context.Background(), job)
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
