package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/pinguin/pkg/scheduler"
	"gorm.io/gorm"
)

type notificationRetryStore struct {
	database   *gorm.DB
	tenantRepo *tenant.Repository
}

func newNotificationRetryStore(database *gorm.DB, tenantRepo *tenant.Repository) *notificationRetryStore {
	return &notificationRetryStore{database: database, tenantRepo: tenantRepo}
}

func (store *notificationRetryStore) PendingJobs(ctx context.Context, maxRetries int, now time.Time) ([]scheduler.Job, error) {
	if store.tenantRepo == nil {
		return store.pendingJobsAll(ctx, maxRetries, now)
	}
	tenants, err := store.tenantRepo.ListActiveTenants(ctx)
	if err != nil {
		return nil, err
	}
	var jobs []scheduler.Job
	for _, tenantRecord := range tenants {
		records, err := model.GetQueuedOrFailedNotifications(ctx, store.database, tenantRecord.ID, maxRetries, now)
		if err != nil {
			return nil, err
		}
		for index := range records {
			record := records[index]
			jobs = append(jobs, scheduler.Job{
				ID:              record.NotificationID,
				ScheduledFor:    record.ScheduledFor,
				RetryCount:      record.RetryCount,
				LastAttemptedAt: record.LastAttemptedAt,
				Payload:         &records[index],
			})
		}
	}
	return jobs, nil
}

func (store *notificationRetryStore) pendingJobsAll(ctx context.Context, maxRetries int, now time.Time) ([]scheduler.Job, error) {
	var notifications []model.Notification
	err := store.database.WithContext(ctx).
		Preload("Attachments").
		Where("(status = ? OR status = ? OR status = ?) AND retry_count < ? AND (scheduled_for IS NULL OR scheduled_for <= ?)",
			model.StatusQueued, model.StatusErrored, model.StatusFailed, maxRetries, now).
		Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	jobs := make([]scheduler.Job, 0, len(notifications))
	for index := range notifications {
		record := notifications[index]
		jobs = append(jobs, scheduler.Job{
			ID:              record.NotificationID,
			ScheduledFor:    record.ScheduledFor,
			RetryCount:      record.RetryCount,
			LastAttemptedAt: record.LastAttemptedAt,
			Payload:         &notifications[index],
		})
	}
	return jobs, nil
}

func (store *notificationRetryStore) ApplyAttemptResult(ctx context.Context, job scheduler.Job, update scheduler.AttemptUpdate) error {
	record, err := store.notificationFromJob(job)
	if err != nil {
		return err
	}
	canonicalStatus := model.CanonicalStatus(model.NotificationStatus(update.Status))
	if canonicalStatus == "" {
		canonicalStatus = model.StatusErrored
	}
	record.Status = canonicalStatus
	record.ProviderMessageID = update.ProviderMessageID
	record.RetryCount = update.RetryCount
	record.LastAttemptedAt = update.LastAttemptedAt
	record.UpdatedAt = update.LastAttemptedAt
	return model.SaveNotification(ctx, store.database, record)
}

func (store *notificationRetryStore) notificationFromJob(job scheduler.Job) (*model.Notification, error) {
	if job.Payload == nil {
		return nil, fmt.Errorf("missing notification payload for job %s", job.ID)
	}
	notificationRecord, ok := job.Payload.(*model.Notification)
	if !ok {
		return nil, fmt.Errorf("invalid notification payload type for job %s", job.ID)
	}
	return notificationRecord, nil
}

type notificationDispatcher struct {
	serviceInstance *notificationServiceImpl
}

func newNotificationDispatcher(serviceInstance *notificationServiceImpl) *notificationDispatcher {
	return &notificationDispatcher{serviceInstance: serviceInstance}
}

func (dispatcher *notificationDispatcher) Attempt(ctx context.Context, job scheduler.Job) (scheduler.DispatchResult, error) {
	notificationRecord, err := dispatcher.recordFromJob(job)
	if err != nil {
		return scheduler.DispatchResult{}, err
	}
	runtimeCfg, runtimeErr := dispatcher.serviceInstance.runtimeForTenantID(ctx, notificationRecord.TenantID)
	if runtimeErr != nil {
		dispatcher.serviceInstance.logger.Error("Failed to resolve tenant runtime for retry", "tenant_id", notificationRecord.TenantID, "error", runtimeErr)
		return scheduler.DispatchResult{Status: string(model.StatusErrored)}, runtimeErr
	}

	switch notificationRecord.NotificationType {
	case model.NotificationEmail:
		emailSender, senderErr := dispatcher.serviceInstance.emailSenderForTenant(runtimeCfg)
		if senderErr != nil {
			return scheduler.DispatchResult{Status: string(model.StatusErrored)}, senderErr
		}
		emailAttachments := model.ToEmailAttachments(notificationRecord.Attachments)
		sendErr := emailSender.SendEmail(ctx, notificationRecord.Recipient, notificationRecord.Subject, notificationRecord.Message, emailAttachments)
		if sendErr != nil {
			return scheduler.DispatchResult{}, sendErr
		}
		return scheduler.DispatchResult{Status: string(model.StatusSent)}, nil
	case model.NotificationSMS:
		smsSender, senderErr := dispatcher.serviceInstance.smsSenderForTenant(runtimeCfg)
		if senderErr != nil {
			dispatcher.serviceInstance.logger.Warn("Skipping SMS retry because delivery is disabled", "notification_id", notificationRecord.NotificationID)
			return scheduler.DispatchResult{Status: string(model.StatusErrored)}, senderErr
		}
		providerMessageID, sendErr := smsSender.SendSms(ctx, notificationRecord.Recipient, notificationRecord.Message)
		if sendErr != nil {
			return scheduler.DispatchResult{}, sendErr
		}
		return scheduler.DispatchResult{
			Status:            string(model.StatusSent),
			ProviderMessageID: providerMessageID,
		}, nil
	default:
		dispatcher.serviceInstance.logger.Error("Unsupported notification type during retry", "notification_id", notificationRecord.NotificationID)
		return scheduler.DispatchResult{Status: string(model.StatusErrored)}, fmt.Errorf("unsupported notification type: %s", notificationRecord.NotificationType)
	}
}

func (dispatcher *notificationDispatcher) recordFromJob(job scheduler.Job) (*model.Notification, error) {
	notificationRecord, ok := job.Payload.(*model.Notification)
	if !ok || notificationRecord == nil {
		return nil, errors.New("notification payload missing from job")
	}
	return notificationRecord, nil
}
