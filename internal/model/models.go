package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// NotificationType enumerations: "email" or "sms".
type NotificationType string
type NotificationStatus string

const (
	NotificationEmail NotificationType = "email"
	NotificationSMS   NotificationType = "sms"
)

// EmailAttachment carries attachment metadata used across domain layers.
type EmailAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

// Status constants used for the Notification model.
const (
	StatusQueued    NotificationStatus = "queued"
	StatusSent      NotificationStatus = "sent"
	StatusErrored   NotificationStatus = "errored"
	StatusCancelled NotificationStatus = "cancelled"
	StatusUnknown   NotificationStatus = "unknown"
	StatusFailed    NotificationStatus = "failed" // legacy value kept for previously persisted rows
)

var ErrNotificationNotFound = errors.New("notification not found")

func CanonicalStatus(status NotificationStatus) NotificationStatus {
	switch status {
	case StatusQueued, StatusSent, StatusErrored, StatusCancelled, StatusUnknown:
		return status
	case StatusFailed:
		return StatusErrored
	default:
		return ""
	}
}

// NotificationListFilters constrain List operations (e.g., by status).
type NotificationListFilters struct {
	Statuses []NotificationStatus
}

// NormalizedStatuses removes duplicates and legacy aliases while preserving order.
func (filters NotificationListFilters) NormalizedStatuses() []NotificationStatus {
	if len(filters.Statuses) == 0 {
		return nil
	}
	seen := make(map[NotificationStatus]struct{}, len(filters.Statuses))
	var normalized []NotificationStatus
	for _, status := range filters.Statuses {
		canonical := CanonicalStatus(status)
		if canonical == "" {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}
	return normalized
}

// Notification is our main model in the DB, with GORM & JSON tags.
// You can return this directly via JSON or create a separate struct if you like.
type Notification struct {
	ID                uint                     `json:"-" gorm:"primaryKey"`
	TenantID          string                   `json:"tenant_id" gorm:"index"`
	NotificationID    string                   `json:"notification_id" gorm:"index:idx_tenant_notification,unique"`
	NotificationType  NotificationType         `json:"notification_type"`
	Recipient         string                   `json:"recipient"`
	Subject           string                   `json:"subject,omitempty"`
	Message           string                   `json:"message"`
	ProviderMessageID string                   `json:"provider_message_id"`
	Status            NotificationStatus       `json:"status"`
	RetryCount        int                      `json:"retry_count"`
	LastAttemptedAt   time.Time                `json:"last_attempted_at"`
	ScheduledFor      *time.Time               `json:"scheduled_for"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
	Attachments       []NotificationAttachment `json:"attachments,omitempty" gorm:"foreignKey:NotificationID,TenantID;references:NotificationID,TenantID;constraint:OnDelete:CASCADE"`
}

// NotificationAttachment persists attachment payloads per notification.
type NotificationAttachment struct {
	ID             uint      `json:"-" gorm:"primaryKey"`
	TenantID       string    `json:"tenant_id" gorm:"index"`
	NotificationID string    `json:"notification_id" gorm:"index"`
	Filename       string    `json:"filename"`
	ContentType    string    `json:"content_type"`
	Data           []byte    `json:"data" gorm:"type:blob"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NotificationRequest represents a validated request payload.
type NotificationRequest struct {
	notificationType NotificationType
	recipient        string
	subject          string
	message          string
	scheduledFor     *time.Time
	attachments      []EmailAttachment
}

// NotificationResponse is what you'll return to the client.
// You could also return the Notification itself, but some prefer a separate shape.
type NotificationResponse struct {
	NotificationID    string             `json:"notification_id"`
	TenantID          string             `json:"tenant_id"`
	NotificationType  NotificationType   `json:"notification_type"`
	Recipient         string             `json:"recipient"`
	Subject           string             `json:"subject,omitempty"`
	Message           string             `json:"message"`
	Status            NotificationStatus `json:"status"`
	ProviderMessageID string             `json:"provider_message_id"`
	RetryCount        int                `json:"retry_count"`
	ScheduledFor      *time.Time         `json:"scheduled_for,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	Attachments       []EmailAttachment  `json:"attachments,omitempty"`
}

// NewNotification constructs a ready-to-insert DB Notification from a request, defaulting status=queued.
func NewNotification(notificationID string, tenantID string, req NotificationRequest) Notification {
	now := time.Now().UTC()
	var scheduledFor *time.Time
	if req.scheduledFor != nil {
		normalizedScheduled := req.scheduledFor.UTC()
		scheduledFor = &normalizedScheduled
	}
	return Notification{
		TenantID:         tenantID,
		NotificationID:   notificationID,
		NotificationType: req.notificationType,
		Recipient:        req.recipient,
		Subject:          req.subject,
		Message:          req.message,
		Status:           StatusQueued,
		ScheduledFor:     scheduledFor,
		CreatedAt:        now,
		UpdatedAt:        now,
		Attachments:      convertEmailAttachments(tenantID, notificationID, req.attachments),
	}
}

// NewNotificationResponse translates a DB Notification to a response shape.
func NewNotificationResponse(n Notification) NotificationResponse {
	var scheduledFor *time.Time
	if n.ScheduledFor != nil {
		normalizedScheduled := n.ScheduledFor.UTC()
		scheduledFor = &normalizedScheduled
	}
	status := CanonicalStatus(n.Status)
	if status == "" {
		status = StatusUnknown
	}
	return NotificationResponse{
		NotificationID:    n.NotificationID,
		TenantID:          n.TenantID,
		NotificationType:  n.NotificationType,
		Recipient:         n.Recipient,
		Subject:           n.Subject,
		Message:           n.Message,
		Status:            status,
		ProviderMessageID: n.ProviderMessageID,
		RetryCount:        n.RetryCount,
		ScheduledFor:      scheduledFor,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
		Attachments:       ToEmailAttachments(n.Attachments),
	}
}

// ====================== DB CRUD METHODS ====================== //

func CreateNotification(ctx context.Context, db *gorm.DB, n *Notification) error {
	return db.WithContext(ctx).Create(n).Error
}

func GetNotificationByID(ctx context.Context, db *gorm.DB, tenantID string, notificationID string) (*Notification, error) {
	var notif Notification
	err := db.WithContext(ctx).
		Preload("Attachments").
		Where("tenant_id = ? AND notification_id = ?", tenantID, notificationID).
		First(&notif).Error
	if err != nil {
		return nil, err
	}
	return &notif, nil
}

func SaveNotification(ctx context.Context, db *gorm.DB, n *Notification) error {
	return db.WithContext(ctx).Save(n).Error
}

func GetQueuedOrFailedNotifications(ctx context.Context, db *gorm.DB, tenantID string, maxRetries int, currentTime time.Time) ([]Notification, error) {
	var notifications []Notification
	err := db.WithContext(ctx).
		Preload("Attachments").
		Where("tenant_id = ? AND (status = ? OR status = ? OR status = ?) AND retry_count < ? AND (scheduled_for IS NULL OR scheduled_for <= ?)",
			tenantID, StatusQueued, StatusErrored, StatusFailed, maxRetries, currentTime).
		Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

func ListNotifications(ctx context.Context, db *gorm.DB, tenantID string, filters NotificationListFilters) ([]Notification, error) {
	query := db.WithContext(ctx).Preload("Attachments").Order("created_at DESC").Where("tenant_id = ?", tenantID)
	statuses := filters.NormalizedStatuses()
	if len(statuses) > 0 {
		statusStrings := make([]string, 0, len(statuses))
		for _, status := range statuses {
			statusStrings = append(statusStrings, string(status))
			if status == StatusErrored {
				statusStrings = append(statusStrings, string(StatusFailed))
			}
		}
		query = query.Where("status IN ?", statusStrings)
	}
	var notifications []Notification
	if err := query.Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func MustGetNotificationByID(ctx context.Context, db *gorm.DB, tenantID string, notificationID string) (*Notification, error) {
	n, err := GetNotificationByID(ctx, db, tenantID, notificationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("%w: %s", ErrNotificationNotFound, notificationID)
		}
		return nil, fmt.Errorf("get_notification_by_id: %w", err)
	}
	return n, nil
}

func convertEmailAttachments(tenantID string, notificationID string, attachments []EmailAttachment) []NotificationAttachment {
	if len(attachments) == 0 {
		return nil
	}
	converted := make([]NotificationAttachment, 0, len(attachments))
	for _, att := range attachments {
		clonedData := make([]byte, len(att.Data))
		copy(clonedData, att.Data)
		converted = append(converted, NotificationAttachment{
			TenantID:       tenantID,
			NotificationID: notificationID,
			Filename:       att.Filename,
			ContentType:    att.ContentType,
			Data:           clonedData,
		})
	}
	return converted
}

// ToEmailAttachments translates stored attachments to the domain shape.
func ToEmailAttachments(stored []NotificationAttachment) []EmailAttachment {
	if len(stored) == 0 {
		return nil
	}
	result := make([]EmailAttachment, 0, len(stored))
	for _, att := range stored {
		clonedData := make([]byte, len(att.Data))
		copy(clonedData, att.Data)
		result = append(result, EmailAttachment{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Data:        clonedData,
		})
	}
	return result
}
