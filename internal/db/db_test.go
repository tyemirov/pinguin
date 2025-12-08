package db

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
)

func TestInitDBCreatesSchema(t *testing.T) {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "pinguin.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	database, initError := InitDB(databasePath, logger)
	if initError != nil {
		t.Fatalf("init db error: %v", initError)
	}

	notification := model.Notification{
		NotificationID:   "db-test",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "Body",
		Status:           model.StatusQueued,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if createError := database.WithContext(context.Background()).Create(&notification).Error; createError != nil {
		t.Fatalf("create notification error: %v", createError)
	}

	fetched, fetchError := model.GetNotificationByID(context.Background(), database, "db-test")
	if fetchError != nil {
		t.Fatalf("fetch notification error: %v", fetchError)
	}
	if fetched.NotificationID != "db-test" {
		t.Fatalf("unexpected notification id %s", fetched.NotificationID)
	}
}

func TestInitDBCreatesMissingDirectories(t *testing.T) {
	t.Helper()

	baseDirectory := t.TempDir()
	databasePath := filepath.Join(baseDirectory, "nested", "pinguin.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	database, initError := InitDB(databasePath, logger)
	if initError != nil {
		t.Fatalf("init db error: %v", initError)
	}

	fileInfo, statError := os.Stat(filepath.Dir(databasePath))
	if statError != nil {
		t.Fatalf("stat directory error: %v", statError)
	}
	if !fileInfo.IsDir() {
		t.Fatalf("expected directory at %s", filepath.Dir(databasePath))
	}

	sqlDB, sqlDBError := database.DB()
	if sqlDBError != nil {
		t.Fatalf("retrieve sql db error: %v", sqlDBError)
	}
	if closeError := sqlDB.Close(); closeError != nil {
		t.Fatalf("close sql db error: %v", closeError)
	}
}
