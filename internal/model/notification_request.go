package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxNotificationAttachmentCount       = 10
	maxNotificationAttachmentSizeBytes   = 5 * 1024 * 1024
	maxNotificationAttachmentsTotalBytes = 25 * 1024 * 1024
	defaultAttachmentContentType         = "application/octet-stream"
	attachmentIndexTemplate              = "attachment %d"
	attachmentFilenameTemplate           = "attachment %q"
	attachmentMaxTemplate                = "max %d"
	wrapWithIndexTemplate                = "%w: " + attachmentIndexTemplate
	wrapWithFilenameTemplate             = "%w: " + attachmentFilenameTemplate
	wrapWithMaxTemplate                  = "%w: " + attachmentMaxTemplate
)

var (
	// ErrNotificationRecipientRequired indicates the recipient is missing.
	ErrNotificationRecipientRequired = errors.New("notification.request.recipient_required")
	// ErrNotificationMessageRequired indicates the message is missing.
	ErrNotificationMessageRequired = errors.New("notification.request.message_required")
	// ErrNotificationTypeUnsupported indicates the notification type is unsupported.
	ErrNotificationTypeUnsupported = errors.New("notification.request.invalid_type")
	// ErrNotificationAttachmentsNotAllowed indicates attachments were provided for a non-email notification.
	ErrNotificationAttachmentsNotAllowed = errors.New("notification.request.attachments_not_allowed")
	// ErrNotificationAttachmentsTooMany indicates the attachment count exceeds limits.
	ErrNotificationAttachmentsTooMany = errors.New("notification.request.attachments_count_exceeded")
	// ErrNotificationAttachmentFilenameRequired indicates an attachment filename is missing.
	ErrNotificationAttachmentFilenameRequired = errors.New("notification.request.attachment_filename_required")
	// ErrNotificationAttachmentDataRequired indicates an attachment payload is empty.
	ErrNotificationAttachmentDataRequired = errors.New("notification.request.attachment_data_required")
	// ErrNotificationAttachmentTooLarge indicates an attachment exceeds the per-file size limit.
	ErrNotificationAttachmentTooLarge = errors.New("notification.request.attachment_size_exceeded")
	// ErrNotificationAttachmentsTooLarge indicates attachments exceed the total size limit.
	ErrNotificationAttachmentsTooLarge = errors.New("notification.request.attachments_total_size_exceeded")
)

// NewNotificationRequest validates and normalizes a notification request payload.
func NewNotificationRequest(notificationType NotificationType, recipient string, subject string, message string, scheduledFor *time.Time, attachments []EmailAttachment) (NotificationRequest, error) {
	normalizedRecipient := strings.TrimSpace(recipient)
	if normalizedRecipient == "" {
		return NotificationRequest{}, ErrNotificationRecipientRequired
	}
	normalizedMessage := strings.TrimSpace(message)
	if normalizedMessage == "" {
		return NotificationRequest{}, ErrNotificationMessageRequired
	}
	if !isSupportedNotificationType(notificationType) {
		return NotificationRequest{}, ErrNotificationTypeUnsupported
	}
	normalizedAttachments, err := normalizeNotificationAttachments(notificationType, attachments)
	if err != nil {
		return NotificationRequest{}, err
	}
	var normalizedSchedule *time.Time
	if scheduledFor != nil {
		scheduleCopy := scheduledFor.UTC()
		normalizedSchedule = &scheduleCopy
	}
	return NotificationRequest{
		notificationType: notificationType,
		recipient:        normalizedRecipient,
		subject:          strings.TrimSpace(subject),
		message:          message,
		scheduledFor:     normalizedSchedule,
		attachments:      normalizedAttachments,
	}, nil
}

// NotificationType returns the request notification type.
func (request NotificationRequest) NotificationType() NotificationType {
	return request.notificationType
}

// Recipient returns the request recipient.
func (request NotificationRequest) Recipient() string {
	return request.recipient
}

// Subject returns the request subject.
func (request NotificationRequest) Subject() string {
	return request.subject
}

// Message returns the request message.
func (request NotificationRequest) Message() string {
	return request.message
}

// ScheduledFor returns the scheduled time in UTC, when present.
func (request NotificationRequest) ScheduledFor() *time.Time {
	if request.scheduledFor == nil {
		return nil
	}
	scheduleCopy := request.scheduledFor.UTC()
	return &scheduleCopy
}

// Attachments returns a copy of the normalized attachments.
func (request NotificationRequest) Attachments() []EmailAttachment {
	return cloneEmailAttachments(request.attachments)
}

func isSupportedNotificationType(notificationType NotificationType) bool {
	switch notificationType {
	case NotificationEmail, NotificationSMS:
		return true
	default:
		return false
	}
}

func normalizeNotificationAttachments(notificationType NotificationType, attachments []EmailAttachment) ([]EmailAttachment, error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	if notificationType != NotificationEmail {
		return nil, ErrNotificationAttachmentsNotAllowed
	}
	if len(attachments) > maxNotificationAttachmentCount {
		return nil, fmt.Errorf(wrapWithMaxTemplate, ErrNotificationAttachmentsTooMany, maxNotificationAttachmentCount)
	}

	totalSize := 0
	normalized := make([]EmailAttachment, 0, len(attachments))
	for attachmentIndex, attachment := range attachments {
		filename := strings.TrimSpace(attachment.Filename)
		if filename == "" {
			return nil, fmt.Errorf(wrapWithIndexTemplate, ErrNotificationAttachmentFilenameRequired, attachmentIndex+1)
		}
		dataCopy := append([]byte(nil), attachment.Data...)
		payloadSize := len(dataCopy)
		if payloadSize == 0 {
			return nil, fmt.Errorf(wrapWithFilenameTemplate, ErrNotificationAttachmentDataRequired, filename)
		}
		if payloadSize > maxNotificationAttachmentSizeBytes {
			return nil, fmt.Errorf(wrapWithFilenameTemplate, ErrNotificationAttachmentTooLarge, filename)
		}
		totalSize += payloadSize

		contentType := strings.TrimSpace(attachment.ContentType)
		if contentType == "" {
			contentType = defaultAttachmentContentType
		}
		normalized = append(normalized, EmailAttachment{
			Filename:    filename,
			ContentType: contentType,
			Data:        dataCopy,
		})
	}

	if totalSize > maxNotificationAttachmentsTotalBytes {
		return nil, fmt.Errorf(wrapWithMaxTemplate, ErrNotificationAttachmentsTooLarge, maxNotificationAttachmentsTotalBytes)
	}
	return normalized, nil
}

func cloneEmailAttachments(attachments []EmailAttachment) []EmailAttachment {
	if len(attachments) == 0 {
		return nil
	}
	cloned := make([]EmailAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		dataCopy := append([]byte(nil), attachment.Data...)
		cloned = append(cloned, EmailAttachment{
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			Data:        dataCopy,
		})
	}
	return cloned
}
