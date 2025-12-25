package model

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const modelTestTenantID = "tenant-model"

func TestNotificationListFiltersNormalizeStatuses(t *testing.T) {
	t.Helper()

	filters := NotificationListFilters{
		Statuses: []NotificationStatus{
			StatusQueued,
			NotificationStatus("ignored"),
			StatusFailed,
			StatusCancelled,
			StatusFailed,
		},
	}

	normalized := filters.NormalizedStatuses()
	expected := []NotificationStatus{StatusQueued, StatusErrored, StatusCancelled}
	if len(normalized) != len(expected) {
		t.Fatalf("expected %d statuses, got %d", len(expected), len(normalized))
	}
	for index, status := range normalized {
		if status != expected[index] {
			t.Fatalf("status mismatch at %d: want %s got %s", index, expected[index], status)
		}
	}
}

func TestNewNotificationConstructsQueuedRecord(t *testing.T) {
	t.Helper()

	scheduledTime := time.Now().UTC().Add(10 * time.Minute)

	testCases := []struct {
		name           string
		scheduledInput *time.Time
	}{
		{name: "WithoutSchedule", scheduledInput: nil},
		{name: "WithSchedule", scheduledInput: &scheduledTime},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			request, requestErr := NewNotificationRequest(
				NotificationEmail,
				"user@example.com",
				"Greetings",
				"Body",
				testCase.scheduledInput,
				nil,
			)
			if requestErr != nil {
				t.Fatalf("notification request error: %v", requestErr)
			}

			record := NewNotification("notif-1", modelTestTenantID, request)
			if record.Status != StatusQueued {
				t.Fatalf("expected queued status, got %s", record.Status)
			}
			if record.NotificationType != NotificationEmail {
				t.Fatalf("unexpected type %s", record.NotificationType)
			}

			if testCase.scheduledInput == nil && record.ScheduledFor != nil {
				t.Fatalf("expected nil scheduled time")
			}
			if testCase.scheduledInput != nil {
				if record.ScheduledFor == nil {
					t.Fatalf("expected scheduled time to be set")
				}
				if !record.ScheduledFor.Equal(testCase.scheduledInput.UTC()) {
					t.Fatalf("scheduled time mismatch")
				}
			}
		})
	}
}

func TestNewNotificationCopiesAttachments(t *testing.T) {
	t.Helper()

	request, requestErr := NewNotificationRequest(
		NotificationEmail,
		"user@example.com",
		"",
		"Body",
		nil,
		[]EmailAttachment{
			{
				Filename:    "report.pdf",
				ContentType: "application/pdf",
				Data:        []byte{0x01, 0x02},
			},
		},
	)
	if requestErr != nil {
		t.Fatalf("notification request error: %v", requestErr)
	}

	record := NewNotification("notif-attachments", modelTestTenantID, request)
	if len(record.Attachments) != 1 {
		t.Fatalf("expected attachment to be copied")
	}
	if record.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("unexpected filename %s", record.Attachments[0].Filename)
	}

	response := NewNotificationResponse(record)
	if len(response.Attachments) != 1 {
		t.Fatalf("expected attachment in response")
	}
	if response.Attachments[0].ContentType != "application/pdf" {
		t.Fatalf("unexpected content type %s", response.Attachments[0].ContentType)
	}
}

func TestNewNotificationResponseCopiesScheduledTime(t *testing.T) {
	t.Helper()

	scheduledTime := time.Now().UTC().Add(5 * time.Minute)
	response := NewNotificationResponse(Notification{
		TenantID:         modelTestTenantID,
		NotificationID:   "notif-1",
		NotificationType: NotificationSMS,
		Recipient:        "+15550000000",
		Message:          "Ping",
		Status:           StatusQueued,
		ScheduledFor:     &scheduledTime,
		CreatedAt:        scheduledTime,
		UpdatedAt:        scheduledTime,
	})

	if response.NotificationID != "notif-1" {
		t.Fatalf("unexpected notification id %s", response.NotificationID)
	}
	if response.ScheduledFor == nil {
		t.Fatalf("expected scheduled time copy")
	}
	if !response.ScheduledFor.Equal(scheduledTime) {
		t.Fatalf("scheduled time mismatch")
	}
}

func TestDatabaseHelpersFilterAndRetrieve(t *testing.T) {
	t.Helper()

	database := openModelTestDatabase(t)
	ctx := context.Background()

	baseNotification := Notification{
		TenantID:         modelTestTenantID,
		NotificationType: NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "Body",
		Status:           StatusQueued,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	notifications := []Notification{
		mergeNotification(baseNotification, Notification{NotificationID: "queued-now"}),
		mergeNotification(baseNotification, Notification{NotificationID: "failed", Status: StatusFailed, RetryCount: 1}),
		mergeNotification(baseNotification, Notification{NotificationID: "scheduled-future", ScheduledFor: timePointer(time.Now().UTC().Add(30 * time.Minute))}),
	}

	for index := range notifications {
		if createError := CreateNotification(ctx, database, &notifications[index]); createError != nil {
			t.Fatalf("create notification error: %v", createError)
		}
	}

	pending, pendingError := GetQueuedOrFailedNotifications(ctx, database, modelTestTenantID, 5, time.Now().UTC())
	if pendingError != nil {
		t.Fatalf("pending retrieval error: %v", pendingError)
	}

	if len(pending) != 2 {
		t.Fatalf("expected two pending notifications, got %d", len(pending))
	}

	fetched, fetchError := MustGetNotificationByID(ctx, database, modelTestTenantID, "queued-now")
	if fetchError != nil {
		t.Fatalf("fetch notification error: %v", fetchError)
	}
	if fetched.NotificationID != "queued-now" {
		t.Fatalf("unexpected fetched id %s", fetched.NotificationID)
	}

	_, missingError := MustGetNotificationByID(ctx, database, modelTestTenantID, "missing")
	if missingError == nil {
		t.Fatalf("expected missing error")
	}
}

func openModelTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	databaseName := time.Now().UTC().Format("20060102150405.000000000")
	database, openError := gorm.Open(sqlite.Open("file:"+databaseName+"?mode=memory&cache=shared"), &gorm.Config{})
	if openError != nil {
		t.Fatalf("open database error: %v", openError)
	}
	if migrateError := database.AutoMigrate(&Notification{}, &NotificationAttachment{}); migrateError != nil {
		t.Fatalf("migration error: %v", migrateError)
	}
	return database
}

func mergeNotification(base Notification, override Notification) Notification {
	result := base
	if override.NotificationID != "" {
		result.NotificationID = override.NotificationID
	}
	if override.Status != "" {
		result.Status = override.Status
	}
	if override.RetryCount != 0 {
		result.RetryCount = override.RetryCount
	}
	if override.ScheduledFor != nil {
		result.ScheduledFor = override.ScheduledFor
	}
	return result
}

func timePointer(value time.Time) *time.Time {
	copiedValue := value
	return &copiedValue
}
