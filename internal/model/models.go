package model

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
)

const (
	notificationTenantIDColumn       = "tenant_id"
	notificationIDColumn             = "id"
	notificationNotificationIDColumn = "notification_id"
	notificationTypeColumn           = "notification_type"
	notificationRecipientColumn      = "recipient"
	notificationSubjectColumn        = "subject"
	notificationMessageColumn        = "message"
	notificationStatusColumn         = "status"
	notificationRetryCountColumn     = "retry_count"
	notificationScheduledForColumn   = "scheduled_for"
	notificationCreatedAtColumn      = "created_at"
	defaultNotificationListLimit     = 50
	maxNotificationListLimit         = 100
	maxNotificationSearchLength      = 200
)

var (
	ErrNotificationNotFound      = errors.New("notification not found")
	ErrInvalidNotificationCursor = errors.New("invalid notification list cursor")
	ErrInvalidNotificationLimit  = errors.New("invalid notification list limit")
	ErrInvalidNotificationSearch = errors.New("invalid notification search query")
)

func CanonicalStatus(status NotificationStatus) NotificationStatus {
	switch status {
	case StatusQueued, StatusSent, StatusErrored, StatusCancelled, StatusUnknown:
		return status
	default:
		return ""
	}
}

// NotificationListFilters constrain List operations (e.g., by status).
type NotificationListFilters struct {
	Statuses    []NotificationStatus
	SearchQuery NotificationSearchQuery
}

// NotificationSearchQuery is a validated optional list-search query.
type NotificationSearchQuery struct {
	value string
}

// NewNotificationSearchQuery trims and validates a list-search query.
func NewNotificationSearchQuery(rawValue string) (NotificationSearchQuery, error) {
	normalized := strings.TrimSpace(rawValue)
	if utf8.RuneCountInString(normalized) > maxNotificationSearchLength {
		return NotificationSearchQuery{}, fmt.Errorf("%w: max length is %d", ErrInvalidNotificationSearch, maxNotificationSearchLength)
	}
	return NotificationSearchQuery{value: normalized}, nil
}

// Value returns the normalized search string.
func (query NotificationSearchQuery) Value() string {
	return query.value
}

// IsZero reports whether the query should be ignored.
func (query NotificationSearchQuery) IsZero() bool {
	return query.value == ""
}

// NotificationListCursor identifies the last record emitted by a page.
type NotificationListCursor struct {
	createdAt time.Time
	id        uint
}

type notificationListCursorPayload struct {
	CreatedAt string `json:"created_at"`
	ID        uint   `json:"id"`
}

// NewNotificationListCursor constructs a keyset cursor from a persisted row.
func NewNotificationListCursor(createdAt time.Time, id uint) (NotificationListCursor, error) {
	if id == 0 {
		return NotificationListCursor{}, fmt.Errorf("%w: id is required", ErrInvalidNotificationCursor)
	}
	return NotificationListCursor{createdAt: createdAt.UTC(), id: id}, nil
}

// ParseNotificationListCursor decodes a cursor previously returned by Encode.
func ParseNotificationListCursor(rawValue string) (*NotificationListCursor, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return nil, nil
	}
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(normalized)
	if decodeErr != nil {
		return nil, fmt.Errorf("%w: decode", ErrInvalidNotificationCursor)
	}
	var payload notificationListCursorPayload
	if unmarshalErr := json.Unmarshal(decoded, &payload); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: payload", ErrInvalidNotificationCursor)
	}
	createdAt, parseErr := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if parseErr != nil {
		return nil, fmt.Errorf("%w: created_at", ErrInvalidNotificationCursor)
	}
	cursor, cursorErr := NewNotificationListCursor(createdAt, payload.ID)
	if cursorErr != nil {
		return nil, cursorErr
	}
	return &cursor, nil
}

// Encode serializes the cursor for HTTP clients.
func (cursor NotificationListCursor) Encode() string {
	payload := notificationListCursorPayload{
		CreatedAt: cursor.createdAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.id,
	}
	encoded, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// CreatedAt returns the cursor timestamp.
func (cursor NotificationListCursor) CreatedAt() time.Time {
	return cursor.createdAt
}

// ID returns the cursor row id.
func (cursor NotificationListCursor) ID() uint {
	return cursor.id
}

// NotificationListPageRequest contains validated pagination inputs.
type NotificationListPageRequest struct {
	limit  int
	cursor *NotificationListCursor
}

// NewNotificationListPageRequest constructs pagination inputs.
func NewNotificationListPageRequest(limit int, cursor *NotificationListCursor) (NotificationListPageRequest, error) {
	if limit < 1 || limit > maxNotificationListLimit {
		return NotificationListPageRequest{}, fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidNotificationLimit, maxNotificationListLimit)
	}
	var cursorCopy *NotificationListCursor
	if cursor != nil {
		clonedCursor := *cursor
		cursorCopy = &clonedCursor
	}
	return NotificationListPageRequest{limit: limit, cursor: cursorCopy}, nil
}

// DefaultNotificationListPageRequest returns the standard first page request.
func DefaultNotificationListPageRequest() NotificationListPageRequest {
	return NotificationListPageRequest{limit: defaultNotificationListLimit}
}

// Limit returns the validated row limit.
func (request NotificationListPageRequest) Limit() int {
	return request.limit
}

// Cursor returns a copy of the cursor.
func (request NotificationListPageRequest) Cursor() *NotificationListCursor {
	if request.cursor == nil {
		return nil
	}
	cursorCopy := *request.cursor
	return &cursorCopy
}

// NotificationListPage is a persisted notification page.
type NotificationListPage struct {
	Notifications []Notification
	NextCursor    string
}

// NotificationListResponsePage is a client-facing notification page.
type NotificationListResponsePage struct {
	Notifications []NotificationResponse
	NextCursor    string
}

// NormalizedStatuses removes duplicates while preserving order.
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
		Where(&Notification{TenantID: tenantID, NotificationID: notificationID}).
		First(&notif).Error
	if err != nil {
		return nil, err
	}
	return &notif, nil
}

func SaveNotification(ctx context.Context, db *gorm.DB, n *Notification) error {
	return db.WithContext(ctx).Save(n).Error
}

func GetPendingRetryNotifications(ctx context.Context, db *gorm.DB, tenantID string, maxRetries int, currentTime time.Time) ([]Notification, error) {
	var notifications []Notification
	tenantIDColumn := clause.Column{Name: notificationTenantIDColumn}
	statusColumn := clause.Column{Name: notificationStatusColumn}
	retryCountColumn := clause.Column{Name: notificationRetryCountColumn}
	scheduledForColumn := clause.Column{Name: notificationScheduledForColumn}
	statusValues := []interface{}{StatusQueued, StatusErrored}
	err := db.WithContext(ctx).
		Preload("Attachments").
		Where(clause.And(
			clause.Eq{Column: tenantIDColumn, Value: tenantID},
			clause.IN{Column: statusColumn, Values: statusValues},
			clause.Lt{Column: retryCountColumn, Value: maxRetries},
			clause.Or(
				clause.Eq{Column: scheduledForColumn, Value: nil},
				clause.Lte{Column: scheduledForColumn, Value: currentTime},
			),
		)).
		Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

func ListNotifications(ctx context.Context, db *gorm.DB, tenantID string, filters NotificationListFilters) ([]Notification, error) {
	query := notificationListQuery(ctx, db, filters).
		Where(&Notification{TenantID: tenantID})
	var notifications []Notification
	if err := query.Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func ListNotificationsPage(ctx context.Context, db *gorm.DB, tenantID string, filters NotificationListFilters, pageRequest NotificationListPageRequest) (NotificationListPage, error) {
	query := notificationListQuery(ctx, db, filters).
		Where(&Notification{TenantID: tenantID})
	if cursor := pageRequest.Cursor(); cursor != nil {
		query = query.Where(notificationCursorCondition(*cursor))
	}
	var notifications []Notification
	if err := query.Limit(pageRequest.Limit() + 1).Find(&notifications).Error; err != nil {
		return NotificationListPage{}, err
	}
	return notificationPageFromRecords(notifications, pageRequest.Limit())
}

func ListNotificationsAll(ctx context.Context, db *gorm.DB, filters NotificationListFilters) ([]Notification, error) {
	query := notificationListQuery(ctx, db, filters)
	var notifications []Notification
	if err := query.Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func notificationListQuery(ctx context.Context, db *gorm.DB, filters NotificationListFilters) *gorm.DB {
	query := db.WithContext(ctx).
		Preload("Attachments").
		Order(clause.OrderByColumn{Column: clause.Column{Name: notificationCreatedAtColumn}, Desc: true}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: notificationIDColumn}, Desc: true})
	statuses := filters.NormalizedStatuses()
	if len(statuses) > 0 {
		statusValues := make([]interface{}, 0, len(statuses))
		for _, status := range statuses {
			statusValues = append(statusValues, status)
		}
		query = query.Where(clause.IN{Column: clause.Column{Name: notificationStatusColumn}, Values: statusValues})
	}
	if !filters.SearchQuery.IsZero() {
		query = query.Where(notificationSearchCondition(filters.SearchQuery))
	}
	return query
}

func notificationSearchCondition(query NotificationSearchQuery) clause.Expression {
	value := query.Value()
	pattern := "%" + value + "%"
	columns := []string{
		notificationNotificationIDColumn,
		notificationTenantIDColumn,
		notificationTypeColumn,
		notificationStatusColumn,
		notificationRecipientColumn,
		notificationSubjectColumn,
		notificationMessageColumn,
	}
	expressions := make([]clause.Expression, 0, len(columns)+1)
	for _, columnName := range columns {
		expressions = append(expressions, clause.Like{Column: clause.Column{Name: columnName}, Value: pattern})
	}
	return clause.Or(expressions...)
}

func notificationCursorCondition(cursor NotificationListCursor) clause.Expression {
	createdAtColumn := clause.Column{Name: notificationCreatedAtColumn}
	idColumn := clause.Column{Name: notificationIDColumn}
	return clause.Or(
		clause.Lt{Column: createdAtColumn, Value: cursor.CreatedAt()},
		clause.And(
			clause.Eq{Column: createdAtColumn, Value: cursor.CreatedAt()},
			clause.Lt{Column: idColumn, Value: cursor.ID()},
		),
	)
}

func notificationPageFromRecords(records []Notification, limit int) (NotificationListPage, error) {
	if len(records) <= limit {
		return NotificationListPage{Notifications: records}, nil
	}
	pageRecords := records[:limit]
	lastRecord := pageRecords[len(pageRecords)-1]
	cursor, cursorErr := NewNotificationListCursor(lastRecord.CreatedAt, lastRecord.ID)
	if cursorErr != nil {
		return NotificationListPage{}, cursorErr
	}
	return NotificationListPage{
		Notifications: pageRecords,
		NextCursor:    cursor.Encode(),
	}, nil
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
