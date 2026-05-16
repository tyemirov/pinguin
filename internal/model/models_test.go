package model

import (
	"context"
	"errors"
	"strings"
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
			StatusErrored,
			StatusCancelled,
			StatusErrored,
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

func TestNotificationSearchQueryValidation(t *testing.T) {
	query, err := NewNotificationSearchQuery("  body needle  ")
	if err != nil {
		t.Fatalf("search query: %v", err)
	}
	if query.Value() != "body needle" {
		t.Fatalf("unexpected query value %q", query.Value())
	}
	emptyQuery, emptyErr := NewNotificationSearchQuery("  ")
	if emptyErr != nil {
		t.Fatalf("empty query: %v", emptyErr)
	}
	if !emptyQuery.IsZero() {
		t.Fatalf("expected empty query to be zero")
	}
	if _, longErr := NewNotificationSearchQuery(strings.Repeat("a", 201)); !errors.Is(longErr, ErrInvalidNotificationSearch) {
		t.Fatalf("expected invalid search error, got %v", longErr)
	}
}

func TestNotificationListCursorRoundTripAndValidation(t *testing.T) {
	createdAt := time.Date(2030, 1, 2, 3, 4, 5, 6, time.UTC)
	if _, err := NewNotificationListCursor(createdAt, 0); !errors.Is(err, ErrInvalidNotificationCursor) {
		t.Fatalf("expected invalid cursor id, got %v", err)
	}
	cursor, err := NewNotificationListCursor(createdAt, 42)
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	encoded := cursor.Encode()
	parsed, parseErr := ParseNotificationListCursor(encoded)
	if parseErr != nil {
		t.Fatalf("parse cursor: %v", parseErr)
	}
	if parsed == nil || parsed.ID() != 42 || !parsed.CreatedAt().Equal(createdAt) {
		t.Fatalf("unexpected parsed cursor %+v", parsed)
	}
	empty, emptyErr := ParseNotificationListCursor(" ")
	if emptyErr != nil || empty != nil {
		t.Fatalf("expected empty cursor to parse as nil, got cursor=%+v err=%v", empty, emptyErr)
	}
	for name, rawCursor := range map[string]string{
		"decode":     "!!!",
		"payload":    "bm90LWpzb24",
		"created_at": "eyJjcmVhdGVkX2F0Ijoibm90LXRpbWUiLCJpZCI6MX0",
		"id":         "eyJjcmVhdGVkX2F0IjoiMjAzMC0wMS0wMlQwMzowNDowNVoiLCJpZCI6MH0",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseNotificationListCursor(rawCursor); !errors.Is(err, ErrInvalidNotificationCursor) {
				t.Fatalf("expected invalid cursor error, got %v", err)
			}
		})
	}
}

func TestNotificationListPageRequestValidation(t *testing.T) {
	defaultRequest := DefaultNotificationListPageRequest()
	if defaultRequest.Limit() != 50 {
		t.Fatalf("unexpected default limit %d", defaultRequest.Limit())
	}
	if defaultRequest.Cursor() != nil {
		t.Fatalf("expected default cursor to be nil")
	}
	cursor, cursorErr := NewNotificationListCursor(time.Now().UTC(), 7)
	if cursorErr != nil {
		t.Fatalf("cursor: %v", cursorErr)
	}
	request, requestErr := NewNotificationListPageRequest(25, &cursor)
	if requestErr != nil {
		t.Fatalf("page request: %v", requestErr)
	}
	if request.Limit() != 25 || request.Cursor() == nil || request.Cursor().ID() != 7 {
		t.Fatalf("unexpected page request %+v", request)
	}
	for _, limit := range []int{0, 101} {
		if _, err := NewNotificationListPageRequest(limit, nil); !errors.Is(err, ErrInvalidNotificationLimit) {
			t.Fatalf("expected invalid limit for %d, got %v", limit, err)
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
		mergeNotification(baseNotification, Notification{NotificationID: "errored", Status: StatusErrored, RetryCount: 1}),
		mergeNotification(baseNotification, Notification{NotificationID: "scheduled-future", ScheduledFor: timePointer(time.Now().UTC().Add(30 * time.Minute))}),
	}

	for index := range notifications {
		if createError := CreateNotification(ctx, database, &notifications[index]); createError != nil {
			t.Fatalf("create notification error: %v", createError)
		}
	}

	pending, pendingError := GetPendingRetryNotifications(ctx, database, modelTestTenantID, 5, time.Now().UTC())
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
	if len(listedErrored) != 1 || listedErrored[0].Status != StatusErrored {
		t.Fatalf("expected errored record through errored filter, got %+v", listedErrored)
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
	if len(allErroredNotifications) != 1 || allErroredNotifications[0].Status != StatusErrored {
		t.Fatalf("expected errored record through all errored filter, got %+v", allErroredNotifications)
	}

	_, missingError := MustGetNotificationByID(ctx, database, modelTestTenantID, "missing")
	if !errors.Is(missingError, ErrNotificationNotFound) {
		t.Fatalf("expected missing error, got %v", missingError)
	}
}

func TestListNotificationsPageSearchesAndPaginates(t *testing.T) {
	t.Helper()

	database := openModelTestDatabase(t)
	ctx := context.Background()
	now := time.Now().UTC()
	records := []Notification{
		{
			TenantID:         modelTestTenantID,
			NotificationID:   "notif-oldest",
			NotificationType: NotificationEmail,
			Recipient:        "oldest@example.com",
			Subject:          "Oldest",
			Message:          "launch body",
			Status:           StatusQueued,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			TenantID:         modelTestTenantID,
			NotificationID:   "notif-middle",
			NotificationType: NotificationEmail,
			Recipient:        "middle@example.com",
			Subject:          "Middle",
			Message:          "launch body",
			Status:           StatusQueued,
			CreatedAt:        now.Add(time.Second),
			UpdatedAt:        now.Add(time.Second),
		},
		{
			TenantID:         modelTestTenantID,
			NotificationID:   "notif-newest",
			NotificationType: NotificationEmail,
			Recipient:        "newest@example.com",
			Subject:          "Newest",
			Message:          "launch body",
			Status:           StatusQueued,
			CreatedAt:        now.Add(2 * time.Second),
			UpdatedAt:        now.Add(2 * time.Second),
		},
		{
			TenantID:         modelTestTenantID,
			NotificationID:   "notif-errored",
			NotificationType: NotificationEmail,
			Recipient:        "errored@example.com",
			Subject:          "Errored",
			Message:          "other body",
			Status:           StatusErrored,
			CreatedAt:        now.Add(3 * time.Second),
			UpdatedAt:        now.Add(3 * time.Second),
		},
	}
	for index := range records {
		if err := CreateNotification(ctx, database, &records[index]); err != nil {
			t.Fatalf("create notification: %v", err)
		}
	}
	searchQuery, searchErr := NewNotificationSearchQuery("launch")
	if searchErr != nil {
		t.Fatalf("search query: %v", searchErr)
	}
	firstRequest, firstRequestErr := NewNotificationListPageRequest(2, nil)
	if firstRequestErr != nil {
		t.Fatalf("first request: %v", firstRequestErr)
	}
	firstPage, firstPageErr := ListNotificationsPage(ctx, database, modelTestTenantID, NotificationListFilters{SearchQuery: searchQuery}, firstRequest)
	if firstPageErr != nil {
		t.Fatalf("first page: %v", firstPageErr)
	}
	if len(firstPage.Notifications) != 2 {
		t.Fatalf("expected two first-page records, got %d", len(firstPage.Notifications))
	}
	if firstPage.Notifications[0].NotificationID != "notif-newest" || firstPage.Notifications[1].NotificationID != "notif-middle" {
		t.Fatalf("unexpected first page %+v", firstPage.Notifications)
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("expected next cursor")
	}
	cursor, cursorErr := ParseNotificationListCursor(firstPage.NextCursor)
	if cursorErr != nil {
		t.Fatalf("parse cursor: %v", cursorErr)
	}
	secondRequest, secondRequestErr := NewNotificationListPageRequest(2, cursor)
	if secondRequestErr != nil {
		t.Fatalf("second request: %v", secondRequestErr)
	}
	secondPage, secondPageErr := ListNotificationsPage(ctx, database, modelTestTenantID, NotificationListFilters{SearchQuery: searchQuery}, secondRequest)
	if secondPageErr != nil {
		t.Fatalf("second page: %v", secondPageErr)
	}
	if len(secondPage.Notifications) != 1 || secondPage.Notifications[0].NotificationID != "notif-oldest" {
		t.Fatalf("unexpected second page %+v", secondPage.Notifications)
	}
	if secondPage.NextCursor != "" {
		t.Fatalf("expected empty next cursor")
	}
	statusSearch, statusSearchErr := NewNotificationSearchQuery("errored")
	if statusSearchErr != nil {
		t.Fatalf("status search: %v", statusSearchErr)
	}
	statusPage, statusPageErr := ListNotificationsPage(ctx, database, modelTestTenantID, NotificationListFilters{SearchQuery: statusSearch}, DefaultNotificationListPageRequest())
	if statusPageErr != nil {
		t.Fatalf("status page: %v", statusPageErr)
	}
	if len(statusPage.Notifications) != 1 || statusPage.Notifications[0].NotificationID != "notif-errored" {
		t.Fatalf("expected errored record through errored search, got %+v", statusPage.Notifications)
	}
}

func TestNotificationPageFromRecordsRejectsInvalidCursorRecord(t *testing.T) {
	_, err := notificationPageFromRecords([]Notification{
		{ID: 0, CreatedAt: time.Now().UTC()},
		{ID: 1, CreatedAt: time.Now().UTC().Add(-time.Second)},
	}, 1)
	if !errors.Is(err, ErrInvalidNotificationCursor) {
		t.Fatalf("expected invalid cursor error, got %v", err)
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
	if _, err := GetPendingRetryNotifications(ctx, database, modelTestTenantID, 3, time.Now().UTC()); err == nil {
		t.Fatalf("expected pending storage error")
	}
	if _, err := ListNotifications(ctx, database, modelTestTenantID, NotificationListFilters{}); err == nil {
		t.Fatalf("expected list storage error")
	}
	if _, err := ListNotificationsPage(ctx, database, modelTestTenantID, NotificationListFilters{}, DefaultNotificationListPageRequest()); err == nil {
		t.Fatalf("expected list page storage error")
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
