package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/pkg/scheduler"
	"gorm.io/gorm"
)

// NotificationService defines the external interface for processing notifications.
type NotificationService interface {
	// SendNotification immediately dispatches the notification and stores it.
	SendNotification(ctx context.Context, request model.NotificationRequest) (model.NotificationResponse, error)
	// GetNotificationStatus retrieves the stored notification status.
	GetNotificationStatus(ctx context.Context, notificationID string) (model.NotificationResponse, error)
	// ListNotifications returns stored notifications honoring the provided filters.
	ListNotifications(ctx context.Context, filters model.NotificationListFilters) ([]model.NotificationResponse, error)
	// RescheduleNotification updates the scheduled send time for a queued notification.
	RescheduleNotification(ctx context.Context, notificationID string, scheduledFor time.Time) (model.NotificationResponse, error)
	// CancelNotification transitions a queued notification to cancelled so workers skip it.
	CancelNotification(ctx context.Context, notificationID string) (model.NotificationResponse, error)
	// StartRetryWorker begins a background worker that processes retries with exponential backoff.
	StartRetryWorker(ctx context.Context)
}

var (
	ErrSMSDisabled             = errors.New("sms delivery disabled: missing Twilio credentials")
	ErrScheduleInPast          = errors.New("notification schedule must be in the future")
	ErrNotificationNotEditable = errors.New("notification must be queued before editing")
)

const (
	maxAttachmentCount           = 10
	maxAttachmentSizeBytes       = 5 * 1024 * 1024  // 5 MiB per file
	maxTotalAttachmentSizeBytes  = 25 * 1024 * 1024 // 25 MiB aggregate cap
	defaultAttachmentContentType = "application/octet-stream"
)

type notificationServiceImpl struct {
	database         *gorm.DB
	logger           *slog.Logger
	emailSender      EmailSender
	smsSender        SmsSender
	maxRetries       int
	retryIntervalSec int
	smsEnabled       bool
}

// NewNotificationService creates a NotificationService backed by SMTP/Twilio senders.
func NewNotificationService(db *gorm.DB, logger *slog.Logger, cfg config.Config) NotificationService {
	return NewNotificationServiceWithSenders(db, logger, cfg, nil, nil)
}

// NewNotificationServiceWithSenders allows callers (primarily tests) to provide custom senders.
func NewNotificationServiceWithSenders(
	db *gorm.DB,
	logger *slog.Logger,
	cfg config.Config,
	emailSender EmailSender,
	smsSender SmsSender,
) NotificationService {
	if emailSender == nil {
		emailSender = NewSMTPEmailSender(SMTPConfig{
			Host:        cfg.SMTPHost,
			Port:        fmt.Sprintf("%d", cfg.SMTPPort),
			Username:    cfg.SMTPUsername,
			Password:    cfg.SMTPPassword,
			FromAddress: cfg.FromEmail,
			Timeouts:    cfg,
		}, logger)
	}

	var resolvedSmsSender SmsSender
	var smsEnabled bool
	switch {
	case smsSender != nil:
		resolvedSmsSender = smsSender
		smsEnabled = true
	case cfg.TwilioConfigured():
		resolvedSmsSender = NewTwilioSmsSender(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber, logger, cfg)
		smsEnabled = true
	default:
		logger.Warn("SMS notifications disabled: missing Twilio credentials")
	}

	return &notificationServiceImpl{
		database:         db,
		logger:           logger,
		emailSender:      emailSender,
		smsSender:        resolvedSmsSender,
		maxRetries:       cfg.MaxRetries,
		retryIntervalSec: cfg.RetryIntervalSec,
		smsEnabled:       smsEnabled,
	}
}

func (serviceInstance *notificationServiceImpl) SendNotification(ctx context.Context, request model.NotificationRequest) (model.NotificationResponse, error) {
	if request.Recipient == "" || request.Message == "" {
		serviceInstance.logger.Error("Missing required fields", "recipient", request.Recipient, "message", request.Message)
		return model.NotificationResponse{}, fmt.Errorf("missing required fields: recipient or message")
	}

	switch request.NotificationType {
	case model.NotificationEmail, model.NotificationSMS:
	default:
		serviceInstance.logger.Error("Unsupported notification type", "type", request.NotificationType)
		return model.NotificationResponse{}, fmt.Errorf("unsupported notification type: %s", request.NotificationType)
	}

	if request.NotificationType == model.NotificationSMS && !serviceInstance.smsEnabled {
		serviceInstance.logger.Warn("SMS notification rejected because delivery is disabled", "recipient", request.Recipient)
		return model.NotificationResponse{}, ErrSMSDisabled
	}

	normalizedAttachments, attachmentsErr := normalizeAttachments(request.NotificationType, request.Attachments)
	if attachmentsErr != nil {
		serviceInstance.logger.Error("Attachment validation failed", "error", attachmentsErr)
		return model.NotificationResponse{}, attachmentsErr
	}
	request.Attachments = normalizedAttachments

	notificationID := fmt.Sprintf("notif-%d", time.Now().UnixNano())
	newNotification := model.NewNotification(notificationID, request)

	currentTime := time.Now().UTC()

	shouldAttemptImmediateSend := true
	if request.ScheduledFor != nil && request.ScheduledFor.After(currentTime) {
		shouldAttemptImmediateSend = false
	}

	var dispatchError error
	if shouldAttemptImmediateSend {
		switch newNotification.NotificationType {
		case model.NotificationEmail:
			dispatchError = serviceInstance.emailSender.SendEmail(ctx, newNotification.Recipient, newNotification.Subject, newNotification.Message, request.Attachments)
			if dispatchError == nil {
				newNotification.Status = model.StatusSent
				newNotification.LastAttemptedAt = currentTime
				// When using SMTP no provider message ID is returned.
			}
		case model.NotificationSMS:
			if serviceInstance.smsSender == nil {
				dispatchError = ErrSMSDisabled
				break
			}
			var providerMessageID string
			providerMessageID, dispatchError = serviceInstance.smsSender.SendSms(ctx, newNotification.Recipient, newNotification.Message)
			if dispatchError == nil {
				newNotification.Status = model.StatusSent
				newNotification.ProviderMessageID = providerMessageID
				newNotification.LastAttemptedAt = currentTime
			}
		}
		if dispatchError != nil {
			serviceInstance.logger.Error("Immediate dispatch failed", "error", dispatchError)
			newNotification.Status = model.StatusErrored
			newNotification.LastAttemptedAt = currentTime
		}
	}

	if err := model.CreateNotification(ctx, serviceInstance.database, &newNotification); err != nil {
		serviceInstance.logger.Error("Failed to store notification", "error", err)
		return model.NotificationResponse{}, err
	}
	serviceInstance.logger.Info(
		"notification_persisted",
		"notification_id", newNotification.NotificationID,
		"notification_type", newNotification.NotificationType,
		"status", newNotification.Status,
	)
	return model.NewNotificationResponse(newNotification), nil
}

func (serviceInstance *notificationServiceImpl) GetNotificationStatus(ctx context.Context, notificationID string) (model.NotificationResponse, error) {
	if notificationID == "" {
		serviceInstance.logger.Error("Missing notification_id")
		return model.NotificationResponse{}, fmt.Errorf("missing notification_id")
	}
	notificationRecord, retrievalError := model.MustGetNotificationByID(ctx, serviceInstance.database, notificationID)
	if retrievalError != nil {
		serviceInstance.logger.Error("Failed to retrieve notification", "error", retrievalError)
		return model.NotificationResponse{}, retrievalError
	}
	return model.NewNotificationResponse(*notificationRecord), nil
}

func (serviceInstance *notificationServiceImpl) ListNotifications(ctx context.Context, filters model.NotificationListFilters) ([]model.NotificationResponse, error) {
	records, err := model.ListNotifications(ctx, serviceInstance.database, filters)
	if err != nil {
		serviceInstance.logger.Error("Failed to list notifications", "error", err)
		return nil, err
	}
	responses := make([]model.NotificationResponse, 0, len(records))
	for _, record := range records {
		responses = append(responses, model.NewNotificationResponse(record))
	}
	return responses, nil
}

func (serviceInstance *notificationServiceImpl) RescheduleNotification(ctx context.Context, notificationID string, scheduledFor time.Time) (model.NotificationResponse, error) {
	trimmedID := strings.TrimSpace(notificationID)
	if trimmedID == "" {
		return model.NotificationResponse{}, fmt.Errorf("missing notification_id")
	}
	normalizedSchedule := scheduledFor.UTC()
	if normalizedSchedule.Before(time.Now().UTC()) {
		serviceInstance.logger.Warn("Rejecting reschedule because schedule is in the past", "notification_id", trimmedID, "scheduled_for", normalizedSchedule)
		return model.NotificationResponse{}, ErrScheduleInPast
	}
	existingNotification, fetchErr := model.MustGetNotificationByID(ctx, serviceInstance.database, trimmedID)
	if fetchErr != nil {
		serviceInstance.logger.Error("Failed to fetch notification for reschedule", "notification_id", trimmedID, "error", fetchErr)
		return model.NotificationResponse{}, fetchErr
	}
	if existingNotification.Status != model.StatusQueued {
		serviceInstance.logger.Warn("Rejecting reschedule because notification is not queued", "notification_id", trimmedID, "status", existingNotification.Status)
		return model.NotificationResponse{}, ErrNotificationNotEditable
	}
	scheduleCopy := normalizedSchedule
	existingNotification.ScheduledFor = &scheduleCopy
	existingNotification.UpdatedAt = time.Now().UTC()
	if saveErr := model.SaveNotification(ctx, serviceInstance.database, existingNotification); saveErr != nil {
		serviceInstance.logger.Error("Failed to reschedule notification", "notification_id", trimmedID, "error", saveErr)
		return model.NotificationResponse{}, saveErr
	}
	return model.NewNotificationResponse(*existingNotification), nil
}

func (serviceInstance *notificationServiceImpl) CancelNotification(ctx context.Context, notificationID string) (model.NotificationResponse, error) {
	trimmedID := strings.TrimSpace(notificationID)
	if trimmedID == "" {
		return model.NotificationResponse{}, fmt.Errorf("missing notification_id")
	}
	existingNotification, fetchErr := model.MustGetNotificationByID(ctx, serviceInstance.database, trimmedID)
	if fetchErr != nil {
		serviceInstance.logger.Error("Failed to fetch notification for cancellation", "notification_id", trimmedID, "error", fetchErr)
		return model.NotificationResponse{}, fetchErr
	}
	if existingNotification.Status != model.StatusQueued {
		serviceInstance.logger.Warn("Rejecting cancellation because notification is not queued", "notification_id", trimmedID, "status", existingNotification.Status)
		return model.NotificationResponse{}, ErrNotificationNotEditable
	}
	existingNotification.Status = model.StatusCancelled
	existingNotification.ScheduledFor = nil
	existingNotification.UpdatedAt = time.Now().UTC()
	if saveErr := model.SaveNotification(ctx, serviceInstance.database, existingNotification); saveErr != nil {
		serviceInstance.logger.Error("Failed to cancel notification", "notification_id", trimmedID, "error", saveErr)
		return model.NotificationResponse{}, saveErr
	}
	return model.NewNotificationResponse(*existingNotification), nil
}

func (serviceInstance *notificationServiceImpl) StartRetryWorker(ctx context.Context) {
	worker, workerErr := scheduler.NewWorker(scheduler.Config{
		Repository:    newNotificationRetryStore(serviceInstance.database),
		Dispatcher:    newNotificationDispatcher(serviceInstance),
		Logger:        serviceInstance.logger,
		Interval:      time.Duration(serviceInstance.retryIntervalSec) * time.Second,
		MaxRetries:    serviceInstance.maxRetries,
		SuccessStatus: string(model.StatusSent),
		FailureStatus: string(model.StatusErrored),
	})
	if workerErr != nil {
		serviceInstance.logger.Error("Failed to initialize retry worker", "error", workerErr)
		return
	}
	worker.Run(ctx)
}

func normalizeAttachments(notificationType model.NotificationType, attachments []model.EmailAttachment) ([]model.EmailAttachment, error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	if notificationType != model.NotificationEmail {
		return nil, fmt.Errorf("attachments supported only for email notifications")
	}
	if len(attachments) > maxAttachmentCount {
		return nil, fmt.Errorf("too many attachments: max %d", maxAttachmentCount)
	}

	totalSize := 0
	normalized := make([]model.EmailAttachment, 0, len(attachments))
	for idx, attachment := range attachments {
		filename := strings.TrimSpace(attachment.Filename)
		if filename == "" {
			return nil, fmt.Errorf("attachment %d missing filename", idx+1)
		}
		dataCopy := append([]byte(nil), attachment.Data...)
		payloadSize := len(dataCopy)
		if payloadSize == 0 {
			return nil, fmt.Errorf("attachment %q has empty data", filename)
		}
		if payloadSize > maxAttachmentSizeBytes {
			return nil, fmt.Errorf("attachment %q exceeds %d bytes", filename, maxAttachmentSizeBytes)
		}
		totalSize += payloadSize

		contentType := strings.TrimSpace(attachment.ContentType)
		if contentType == "" {
			contentType = defaultAttachmentContentType
		}
		normalized = append(normalized, model.EmailAttachment{
			Filename:    filename,
			ContentType: contentType,
			Data:        dataCopy,
		})
	}

	if totalSize > maxTotalAttachmentSizeBytes {
		return nil, fmt.Errorf("attachments exceed total limit of %d bytes", maxTotalAttachmentSizeBytes)
	}
	return normalized, nil
}
