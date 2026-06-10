package db

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

func TestInitDBConfiguresSQLiteContentionSettings(t *testing.T) {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "pinguin.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	database, initError := InitDB(databasePath, logger)
	if initError != nil {
		t.Fatalf("init db error: %v", initError)
	}

	var journalMode string
	sqlDB, sqlDBError := database.DB()
	if sqlDBError != nil {
		t.Fatalf("retrieve sql db error: %v", sqlDBError)
	}
	if queryError := sqlDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode); queryError != nil {
		t.Fatalf("query journal mode: %v", queryError)
	}
	if !strings.EqualFold(journalMode, sqliteJournalMode) {
		t.Fatalf("expected journal mode %s, got %s", sqliteJournalMode, journalMode)
	}

	var busyTimeoutMilliseconds int
	if queryError := sqlDB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeoutMilliseconds); queryError != nil {
		t.Fatalf("query busy timeout: %v", queryError)
	}
	if busyTimeoutMilliseconds != sqliteBusyTimeoutMilliseconds {
		t.Fatalf("expected busy timeout %d, got %d", sqliteBusyTimeoutMilliseconds, busyTimeoutMilliseconds)
	}
}

func TestSQLiteDSNAppendsPragmas(t *testing.T) {
	t.Helper()

	plainDSN := sqliteDSN("pinguin.db")
	if !strings.Contains(plainDSN, "pinguin.db?") {
		t.Fatalf("expected query separator in %s", plainDSN)
	}
	if !strings.Contains(plainDSN, "busy_timeout(10000)") {
		t.Fatalf("expected busy timeout pragma in %s", plainDSN)
	}
	if !strings.Contains(plainDSN, "journal_mode(WAL)") {
		t.Fatalf("expected journal mode pragma in %s", plainDSN)
	}

	existingQueryDSN := sqliteDSN("file:pinguin.db?cache=shared")
	if !strings.Contains(existingQueryDSN, "cache=shared&") {
		t.Fatalf("expected existing query separator in %s", existingQueryDSN)
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

func TestSlogGormLoggerOmitsInterpolatedSQLValues(t *testing.T) {
	t.Helper()

	var logBuffer bytes.Buffer
	logger := &slogGormLogger{logger: slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{}))}
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return `SELECT * FROM identities WHERE username = "smtp_sensitive_username"`, 0
	}, errors.New("query failed"))

	output := logBuffer.String()
	if strings.Contains(output, "smtp_sensitive_username") {
		t.Fatalf("expected log output to omit SQL values, got %q", output)
	}
	if strings.Contains(output, "SELECT * FROM identities") {
		t.Fatalf("expected log output to omit raw SQL, got %q", output)
	}
	if !strings.Contains(output, "query failed") {
		t.Fatalf("expected log output to include error, got %q", output)
	}
	if !strings.Contains(output, "rows=0") {
		t.Fatalf("expected log output to include row count, got %q", output)
	}
}
