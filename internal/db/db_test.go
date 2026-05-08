package db

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
)

const dbTestTenantID = "tenant-db"

func TestInitDBCreatesSchema(t *testing.T) {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "pinguin.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	database, initError := InitDB(databasePath, logger)
	if initError != nil {
		t.Fatalf("init db error: %v", initError)
	}

	notification := model.Notification{
		TenantID:         dbTestTenantID,
		NotificationID:   "db-test",
		NotificationType: model.NotificationEmail,
		Recipient:        "user@example.com",
		Message:          "Body",
		Status:           model.StatusQueued,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	ctx := context.Background()
	if createError := database.WithContext(ctx).Create(&notification).Error; createError != nil {
		t.Fatalf("create notification error: %v", createError)
	}

	fetched, fetchError := model.GetNotificationByID(ctx, database, dbTestTenantID, "db-test")
	if fetchError != nil {
		t.Fatalf("fetch notification error: %v", fetchError)
	}
	if fetched.NotificationID != "db-test" {
		t.Fatalf("unexpected notification id %s", fetched.NotificationID)
	}

	tables := []interface{}{
		&tenant.Tenant{},
		&tenant.TenantDomain{},
		&tenant.TenantAdmin{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&smtpidentity.SenderDomain{},
		&smtpidentity.Identity{},
		&smtpidentity.ForwardRecipient{},
	}
	for _, table := range tables {
		if exists := database.Migrator().HasTable(table); !exists {
			t.Fatalf("expected tenant table for %T", table)
		}
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

func TestInitDBReportsDirectoryCreationFailure(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	blockingFile := filepath.Join(tempDir, "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("block"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	_, initError := InitDB(filepath.Join(blockingFile, "pinguin.db"), logger)
	if initError == nil {
		t.Fatalf("expected directory creation error")
	}
}

func TestInitDBReportsOpenOrMigrationFailure(t *testing.T) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	_, initError := InitDB(t.TempDir(), logger)
	if initError == nil {
		t.Fatalf("expected sqlite open or migration error for directory path")
	}

	originalMigrate := migrateDatabaseSchema
	t.Cleanup(func() { migrateDatabaseSchema = originalMigrate })
	migrationErr := errors.New("migration blocked")
	migrateDatabaseSchema = func(*gorm.DB) error {
		return migrationErr
	}
	_, initError = InitDB(filepath.Join(t.TempDir(), "pinguin.db"), logger)
	if !errors.Is(initError, migrationErr) {
		t.Fatalf("expected migration error, got %v", initError)
	}
}

func TestSlogGormLoggerImplementsMethods(t *testing.T) {
	t.Helper()

	logger := &slogGormLogger{logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))}
	if logger.LogMode(0) != logger {
		t.Fatalf("expected LogMode to return same logger")
	}
	logger.Info(context.Background(), "info", "key", "value")
	logger.Warn(context.Background(), "warn", "key", "value")
	logger.Error(context.Background(), "error", "key", "value")
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "select 1", 1
	}, nil)
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "select missing", 0
	}, gorm.ErrRecordNotFound)
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "select broken", 0
	}, errors.New("query failed"))
}
