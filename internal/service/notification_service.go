package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/utils/scheduler"
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
	ErrNotificationNotEditable = errors.New("notification must be queued before editing")
	ErrMissingTenantContext    = errors.New("tenant context missing")
)

type notificationServiceImpl struct {
	database           *gorm.DB
	logger             *slog.Logger
	tenantRepo         *tenant.Repository
	config             config.Config
	defaultEmailSender EmailSender
	defaultSmsSender   SmsSender
	maxRetries         int
	retryIntervalSec   int
	senderMutex        sync.RWMutex
	emailSenders       map[string]EmailSender
	smsSenders         map[string]SmsSender
}

// NewNotificationService creates a NotificationService backed by SMTP/Twilio senders.
func NewNotificationService(db *gorm.DB, logger *slog.Logger, cfg config.Config, tenantRepo *tenant.Repository) NotificationService {
	return NewNotificationServiceWithSenders(db, logger, cfg, tenantRepo, nil, nil)
}

// NewNotificationServiceWithSenders allows callers (primarily tests) to provide custom senders.
func NewNotificationServiceWithSenders(
	db *gorm.DB,
	logger *slog.Logger,
	cfg config.Config,
	tenantRepo *tenant.Repository,
	emailSender EmailSender,
	smsSender SmsSender,
) NotificationService {
	var defaultEmailSender EmailSender
	var defaultSmsSender SmsSender

	if emailSender != nil {
		defaultEmailSender = emailSender
	} else if tenantRepo == nil {
		emailSender = NewSMTPEmailSender(SMTPConfig{
			Host:        cfg.SMTPHost,
			Port:        fmt.Sprintf("%d", cfg.SMTPPort),
			Username:    cfg.SMTPUsername,
			Password:    cfg.SMTPPassword,
			FromAddress: cfg.FromEmail,
			Timeouts:    cfg,
		}, logger)
		defaultEmailSender = emailSender
	}

	switch {
	case smsSender != nil:
		defaultSmsSender = smsSender
	case tenantRepo == nil && cfg.TwilioConfigured():
		defaultSmsSender = NewTwilioSmsSender(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber, logger, cfg)
	case tenantRepo == nil:
		logger.Warn("SMS notifications disabled: missing Twilio credentials")
	}

	return &notificationServiceImpl{
		database:           db,
		logger:             logger,
		tenantRepo:         tenantRepo,
		config:             cfg,
		defaultEmailSender: defaultEmailSender,
		defaultSmsSender:   defaultSmsSender,
		maxRetries:         cfg.MaxRetries,
		retryIntervalSec:   cfg.RetryIntervalSec,
		emailSenders:       make(map[string]EmailSender),
		smsSenders:         make(map[string]SmsSender),
	}
}

func (serviceInstance *notificationServiceImpl) SendNotification(ctx context.Context, request model.NotificationRequest) (model.NotificationResponse, error) {
	runtimeCfg, err := serviceInstance.requireTenant(ctx)
	if err != nil {
		return model.NotificationResponse{}, err
	}
	recipient := request.Recipient()
	subject := request.Subject()
	message := request.Message()
	attachments := request.Attachments()
	scheduledFor := request.ScheduledFor()

	notificationID := fmt.Sprintf("notif-%d", time.Now().UnixNano())
	newNotification := model.NewNotification(notificationID, runtimeCfg.Tenant.ID, request)

	currentTime := time.Now().UTC()

	shouldAttemptImmediateSend := true
	if scheduledFor != nil && scheduledFor.After(currentTime) {
		shouldAttemptImmediateSend = false
	}

	var dispatchError error
	if shouldAttemptImmediateSend {
		switch newNotification.NotificationType {
		case model.NotificationEmail:
			var emailSender EmailSender
			emailSender, err = serviceInstance.emailSenderForTenant(runtimeCfg)
			if err != nil {
				serviceInstance.logger.Error("Email sender unavailable", "tenant_id", runtimeCfg.Tenant.ID, "error", err)
				return model.NotificationResponse{}, err
			}
			dispatchError = emailSender.SendEmail(ctx, recipient, subject, message, attachments)
			if dispatchError == nil {
				newNotification.Status = model.StatusSent
				newNotification.LastAttemptedAt = currentTime
				// When using SMTP no provider message ID is returned.
			}
		case model.NotificationSMS:
			var smsSender SmsSender
			smsSender, err = serviceInstance.smsSenderForTenant(runtimeCfg)
			if err != nil {
				serviceInstance.logger.Warn("SMS sender unavailable", "tenant_id", runtimeCfg.Tenant.ID, "error", err)
				return model.NotificationResponse{}, err
			}
			var providerMessageID string
			providerMessageID, dispatchError = smsSender.SendSms(ctx, recipient, message)
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
	runtimeCfg, err := serviceInstance.requireTenant(ctx)
	if err != nil {
		return model.NotificationResponse{}, err
	}
	notificationRecord, retrievalError := model.MustGetNotificationByID(ctx, serviceInstance.database, runtimeCfg.Tenant.ID, notificationID)
	if retrievalError != nil {
		serviceInstance.logger.Error("Failed to retrieve notification", "error", retrievalError)
		return model.NotificationResponse{}, retrievalError
	}
	return model.NewNotificationResponse(*notificationRecord), nil
}

func (serviceInstance *notificationServiceImpl) ListNotifications(ctx context.Context, filters model.NotificationListFilters) ([]model.NotificationResponse, error) {
	runtimeCfg, err := serviceInstance.requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	records, err := model.ListNotifications(ctx, serviceInstance.database, runtimeCfg.Tenant.ID, filters)
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
	runtimeCfg, err := serviceInstance.requireTenant(ctx)
	if err != nil {
		return model.NotificationResponse{}, err
	}
	normalizedSchedule := scheduledFor.UTC()
	existingNotification, fetchErr := model.MustGetNotificationByID(ctx, serviceInstance.database, runtimeCfg.Tenant.ID, notificationID)
	if fetchErr != nil {
		serviceInstance.logger.Error("Failed to fetch notification for reschedule", "notification_id", notificationID, "error", fetchErr)
		return model.NotificationResponse{}, fetchErr
	}
	if existingNotification.Status != model.StatusQueued {
		serviceInstance.logger.Warn("Rejecting reschedule because notification is not queued", "notification_id", notificationID, "status", existingNotification.Status)
		return model.NotificationResponse{}, ErrNotificationNotEditable
	}
	scheduleCopy := normalizedSchedule
	existingNotification.ScheduledFor = &scheduleCopy
	existingNotification.UpdatedAt = time.Now().UTC()
	if saveErr := model.SaveNotification(ctx, serviceInstance.database, existingNotification); saveErr != nil {
		serviceInstance.logger.Error("Failed to reschedule notification", "notification_id", notificationID, "error", saveErr)
		return model.NotificationResponse{}, saveErr
	}
	return model.NewNotificationResponse(*existingNotification), nil
}

func (serviceInstance *notificationServiceImpl) CancelNotification(ctx context.Context, notificationID string) (model.NotificationResponse, error) {
	runtimeCfg, err := serviceInstance.requireTenant(ctx)
	if err != nil {
		return model.NotificationResponse{}, err
	}
	existingNotification, fetchErr := model.MustGetNotificationByID(ctx, serviceInstance.database, runtimeCfg.Tenant.ID, notificationID)
	if fetchErr != nil {
		serviceInstance.logger.Error("Failed to fetch notification for cancellation", "notification_id", notificationID, "error", fetchErr)
		return model.NotificationResponse{}, fetchErr
	}
	if existingNotification.Status != model.StatusQueued {
		serviceInstance.logger.Warn("Rejecting cancellation because notification is not queued", "notification_id", notificationID, "status", existingNotification.Status)
		return model.NotificationResponse{}, ErrNotificationNotEditable
	}
	existingNotification.Status = model.StatusCancelled
	existingNotification.ScheduledFor = nil
	existingNotification.UpdatedAt = time.Now().UTC()
	if saveErr := model.SaveNotification(ctx, serviceInstance.database, existingNotification); saveErr != nil {
		serviceInstance.logger.Error("Failed to cancel notification", "notification_id", notificationID, "error", saveErr)
		return model.NotificationResponse{}, saveErr
	}
	return model.NewNotificationResponse(*existingNotification), nil
}

func (serviceInstance *notificationServiceImpl) StartRetryWorker(ctx context.Context) {
	worker, workerErr := scheduler.NewWorker(scheduler.Config{
		Repository:    newNotificationRetryStore(serviceInstance.database, serviceInstance.tenantRepo),
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

func (serviceInstance *notificationServiceImpl) requireTenant(ctx context.Context) (tenant.RuntimeConfig, error) {
	runtimeCfg, ok := tenant.RuntimeFromContext(ctx)
	if !ok {
		return tenant.RuntimeConfig{}, ErrMissingTenantContext
	}
	return runtimeCfg, nil
}

func (serviceInstance *notificationServiceImpl) emailSenderForTenant(runtimeCfg tenant.RuntimeConfig) (EmailSender, error) {
	if serviceInstance.defaultEmailSender != nil {
		return serviceInstance.defaultEmailSender, nil
	}
	if runtimeCfg.Email.Host == "" || runtimeCfg.Email.Username == "" || runtimeCfg.Email.Password == "" || runtimeCfg.Email.FromAddress == "" {
		return nil, fmt.Errorf("email credentials unavailable for tenant %s", runtimeCfg.Tenant.ID)
	}
	serviceInstance.senderMutex.RLock()
	cached := serviceInstance.emailSenders[runtimeCfg.Tenant.ID]
	serviceInstance.senderMutex.RUnlock()
	if cached != nil {
		return cached, nil
	}
	smtpSender := NewSMTPEmailSender(SMTPConfig{
		Host:        runtimeCfg.Email.Host,
		Port:        strconv.Itoa(runtimeCfg.Email.Port),
		Username:    runtimeCfg.Email.Username,
		Password:    runtimeCfg.Email.Password,
		FromAddress: runtimeCfg.Email.FromAddress,
		Timeouts:    serviceInstance.config,
	}, serviceInstance.logger)
	serviceInstance.senderMutex.Lock()
	defer serviceInstance.senderMutex.Unlock()
	if existing := serviceInstance.emailSenders[runtimeCfg.Tenant.ID]; existing != nil {
		return existing, nil
	}
	serviceInstance.emailSenders[runtimeCfg.Tenant.ID] = smtpSender
	return smtpSender, nil
}

func (serviceInstance *notificationServiceImpl) smsSenderForTenant(runtimeCfg tenant.RuntimeConfig) (SmsSender, error) {
	if serviceInstance.defaultSmsSender != nil {
		return serviceInstance.defaultSmsSender, nil
	}
	if runtimeCfg.SMS == nil || runtimeCfg.SMS.AccountSID == "" || runtimeCfg.SMS.AuthToken == "" || runtimeCfg.SMS.FromNumber == "" {
		return nil, ErrSMSDisabled
	}
	serviceInstance.senderMutex.RLock()
	cached := serviceInstance.smsSenders[runtimeCfg.Tenant.ID]
	serviceInstance.senderMutex.RUnlock()
	if cached != nil {
		return cached, nil
	}
	smsSender := NewTwilioSmsSender(runtimeCfg.SMS.AccountSID, runtimeCfg.SMS.AuthToken, runtimeCfg.SMS.FromNumber, serviceInstance.logger, serviceInstance.config)
	serviceInstance.senderMutex.Lock()
	defer serviceInstance.senderMutex.Unlock()
	if existing := serviceInstance.smsSenders[runtimeCfg.Tenant.ID]; existing != nil {
		return existing, nil
	}
	serviceInstance.smsSenders[runtimeCfg.Tenant.ID] = smsSender
	return smsSender, nil
}

func (serviceInstance *notificationServiceImpl) runtimeForTenantID(ctx context.Context, tenantID string) (tenant.RuntimeConfig, error) {
	if tenantID == "" {
		return tenant.RuntimeConfig{}, ErrMissingTenantContext
	}
	if serviceInstance.tenantRepo != nil {
		return serviceInstance.tenantRepo.ResolveByID(ctx, tenantID)
	}
	runtimeCfg, ok := tenant.RuntimeFromContext(ctx)
	if !ok {
		if serviceInstance.defaultEmailSender != nil || serviceInstance.defaultSmsSender != nil {
			return tenant.RuntimeConfig{
				Tenant: tenant.Tenant{ID: tenantID},
			}, nil
		}
		return tenant.RuntimeConfig{}, ErrMissingTenantContext
	}
	if runtimeCfg.Tenant.ID != tenantID {
		return tenant.RuntimeConfig{}, fmt.Errorf("tenant mismatch: context=%s payload=%s", runtimeCfg.Tenant.ID, tenantID)
	}
	return runtimeCfg, nil
}
