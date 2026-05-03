package model

import (
	"context"
	"errors"
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

func TestCanonicalStatusCoversAllStatuses(t *testing.T) {
	testCases := map[NotificationStatus]NotificationStatus{
		StatusQueued:    StatusQueued,
		StatusSent:      StatusSent,
		StatusErrored:   StatusErrored,
		StatusCancelled: StatusCancelled,
		StatusUnknown:   StatusUnknown,
		StatusFailed:    StatusErrored,
		"invalid":       "",
	}
	for input, expected := range testCases {
		if got := CanonicalStatus(input); got != expected {
			t.Fatalf("canonical status for %s: want %s got %s", input, expected, got)
		}
	}
	if (NotificationListFilters{}).NormalizedStatuses() != nil {
		t.Fatalf("expected nil normalized statuses for empty filters")
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

func TestNewNotificationResponseDefaultsUnknownStatus(t *testing.T) {
	response := NewNotificationResponse(Notification{
		NotificationID:   "notif-unknown",
		Status:           "not-real",
		NotificationType: NotificationEmail,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	})
	if response.Status != StatusUnknown {
		t.Fatalf("expected unknown status, got %s", response.Status)
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
	fetchedDirect, directFetchError := GetNotificationByID(ctx, database, modelTestTenantID, "queued-now")
	if directFetchError != nil || fetchedDirect.NotificationID != "queued-now" {
		t.Fatalf("direct fetch notification=%+v error=%v", fetchedDirect, directFetchError)
	}
	fetchedDirect.Status = StatusSent
	if saveError := SaveNotification(ctx, database, fetchedDirect); saveError != nil {
		t.Fatalf("save notification: %v", saveError)
	}

	listed, listError := ListNotifications(ctx, database, modelTestTenantID, NotificationListFilters{})
	if listError != nil {
		t.Fatalf("list notifications: %v", listError)
	}
	if len(listed) != 3 {
		t.Fatalf("expected three tenant notifications, got %d", len(listed))
	}
	listedErrored, listErroredError := ListNotifications(ctx, database, modelTestTenantID, NotificationListFilters{Statuses: []NotificationStatus{StatusErrored}})
	if listErroredError != nil {
		t.Fatalf("list errored notifications: %v", listErroredError)
	}
	if len(listedErrored) != 1 || listedErrored[0].Status != StatusFailed {
		t.Fatalf("expected legacy failed record through errored filter, got %+v", listedErrored)
	}
	allNotifications, listAllError := ListNotificationsAll(ctx, database, NotificationListFilters{Statuses: []NotificationStatus{StatusSent}})
	if listAllError != nil {
		t.Fatalf("list all notifications: %v", listAllError)
	}
	if len(allNotifications) != 1 || allNotifications[0].NotificationID != "queued-now" {
		t.Fatalf("unexpected list all result %+v", allNotifications)
	}
	allErroredNotifications, listAllErroredError := ListNotificationsAll(ctx, database, NotificationListFilters{Statuses: []NotificationStatus{StatusErrored}})
	if listAllErroredError != nil {
		t.Fatalf("list all errored notifications: %v", listAllErroredError)
	}
	if len(allErroredNotifications) != 1 || allErroredNotifications[0].Status != StatusFailed {
		t.Fatalf("expected failed record through all errored filter, got %+v", allErroredNotifications)
	}

	_, missingError := MustGetNotificationByID(ctx, database, modelTestTenantID, "missing")
	if !errors.Is(missingError, ErrNotificationNotFound) {
		t.Fatalf("expected missing error, got %v", missingError)
	}
}

func TestDatabaseHelpersReturnStorageErrors(t *testing.T) {
	database := openModelTestDatabase(t)
	ctx := context.Background()
	closeModelDatabase(t, database)

	if err := CreateNotification(ctx, database, &Notification{}); err == nil {
		t.Fatalf("expected create storage error")
	}
	if _, err := GetNotificationByID(ctx, database, modelTestTenantID, "notif"); err == nil {
		t.Fatalf("expected get storage error")
	}
	if err := SaveNotification(ctx, database, &Notification{}); err == nil {
		t.Fatalf("expected save storage error")
	}
	if _, err := GetQueuedOrFailedNotifications(ctx, database, modelTestTenantID, 3, time.Now().UTC()); err == nil {
		t.Fatalf("expected pending storage error")
	}
	if _, err := ListNotifications(ctx, database, modelTestTenantID, NotificationListFilters{}); err == nil {
		t.Fatalf("expected list storage error")
	}
	if _, err := ListNotificationsAll(ctx, database, NotificationListFilters{}); err == nil {
		t.Fatalf("expected list all storage error")
	}
	if _, err := MustGetNotificationByID(ctx, database, modelTestTenantID, "notif"); err == nil || errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("expected wrapped storage error, got %v", err)
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

func closeModelDatabase(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	if closeErr := sqlDatabase.Close(); closeErr != nil {
		t.Fatalf("close database: %v", closeErr)
	}
}
