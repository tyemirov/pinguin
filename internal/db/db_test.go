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

	"github.com/tyemirov/pingwin/internal/model"
	"github.com/tyemirov/pingwin/internal/smtpidentity"
	"github.com/tyemirov/pingwin/internal/tenant"
	"gorm.io/gorm"
)

const dbTestTenantID = "tenant-db"

type extraApplicationTable struct {
	ID uint `gorm:"primaryKey"`
}

func (extraApplicationTable) TableName() string { return "extra_application_table" }

type tenantWithExtraColumn struct {
	tenant.Tenant
	Unexpected string
}

func (tenantWithExtraColumn) TableName() string { return "tenants" }

type failingSchemaMigrator struct {
	gorm.Migrator
	columnErr error
	indexErr  error
}

func (migrator failingSchemaMigrator) ColumnTypes(interface{}) ([]gorm.ColumnType, error) {
	if migrator.columnErr != nil {
		return nil, migrator.columnErr
	}
	return migrator.Migrator.ColumnTypes(&model.Notification{})
}

func (migrator failingSchemaMigrator) GetIndexes(interface{}) ([]gorm.Index, error) {
	if migrator.indexErr != nil {
		return nil, migrator.indexErr
	}
	return migrator.Migrator.GetIndexes(&model.Notification{})
}

func TestInitDBCreatesSchema(t *testing.T) {
	t.Helper()

	database := newMemoryDatabase(t)

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
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&tenant.APICredential{},
		&tenant.IdempotencyRecord{},
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

func TestValidateManagedSchema(t *testing.T) {
	t.Run("current schema", func(t *testing.T) {
		database := newMemoryDatabase(t)
		if err := migrateDatabaseSchema(database); err != nil {
			t.Fatalf("inspect current schema: %v", err)
		}
		if err := ValidateManagedSchema(database); err != nil {
			t.Fatalf("validate current schema: %v", err)
		}
	})
	t.Run("migration table inspection failure", func(t *testing.T) {
		database := newMemoryDatabase(t)
		sqlDatabase, err := database.DB()
		if err != nil {
			t.Fatalf("database handle: %v", err)
		}
		if err := sqlDatabase.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
		if err := migrateDatabaseSchema(database); err == nil || !strings.Contains(err.Error(), "inspect database schema") {
			t.Fatalf("migration table inspection error = %v", err)
		}
	})
	t.Run("table inspection failure", func(t *testing.T) {
		database := newMemoryDatabase(t)
		sqlDatabase, err := database.DB()
		if err != nil {
			t.Fatalf("database handle: %v", err)
		}
		if err := sqlDatabase.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
		if err := ValidateManagedSchema(database); err == nil || !strings.Contains(err.Error(), "inspect database schema") {
			t.Fatalf("table inspection error = %v", err)
		}
	})
	t.Run("missing table", func(t *testing.T) {
		database := newMemoryDatabase(t)
		if err := database.Migrator().DropTable(&tenant.Tenant{}); err != nil {
			t.Fatalf("drop tenant table: %v", err)
		}
		if err := ValidateManagedSchema(database); err == nil || !strings.Contains(err.Error(), "missing table tenants") {
			t.Fatalf("missing table error = %v", err)
		}
	})
	t.Run("extra table", func(t *testing.T) {
		database := newMemoryDatabase(t)
		if err := database.Migrator().CreateTable(&extraApplicationTable{}); err != nil {
			t.Fatalf("create extra table: %v", err)
		}
		if err := ValidateManagedSchema(database); err == nil || !strings.Contains(err.Error(), "application tables") {
			t.Fatalf("extra table error = %v", err)
		}
	})
	t.Run("extra column", func(t *testing.T) {
		database := newMemoryDatabase(t)
		if err := database.AutoMigrate(&tenantWithExtraColumn{}); err != nil {
			t.Fatalf("add extra tenant column: %v", err)
		}
		if err := ValidateManagedSchema(database); err == nil || !strings.Contains(err.Error(), "columns, expected") {
			t.Fatalf("extra column error = %v", err)
		}
	})
	t.Run("missing expected column", func(t *testing.T) {
		database := newMemoryDatabase(t)
		if err := database.Migrator().RenameColumn(&tenant.Tenant{}, "support_email", "unexpected"); err != nil {
			t.Fatalf("rename tenant column: %v", err)
		}
		if err := ValidateManagedSchema(database); err == nil || !strings.Contains(err.Error(), "missing column support_email") {
			t.Fatalf("missing column error = %v", err)
		}
	})
	t.Run("missing index count", func(t *testing.T) {
		database := newMemoryDatabase(t)
		if err := database.Migrator().DropIndex(&tenant.Tenant{}, "idx_tenants_owner_user_id"); err != nil {
			t.Fatalf("drop tenant index: %v", err)
		}
		if err := ValidateManagedSchema(database); err == nil || !strings.Contains(err.Error(), "indexes, expected") {
			t.Fatalf("index count error = %v", err)
		}
	})
	t.Run("missing expected index", func(t *testing.T) {
		database := newMemoryDatabase(t)
		if err := database.Migrator().RenameIndex(&tenant.Tenant{}, "idx_tenants_owner_user_id", "idx_unexpected"); err != nil {
			t.Fatalf("rename tenant index: %v", err)
		}
		if err := ValidateManagedSchema(database); err == nil || !strings.Contains(err.Error(), "missing index idx_tenants_owner_user_id") {
			t.Fatalf("missing index error = %v", err)
		}
	})
	t.Run("schema parse failure", func(t *testing.T) {
		database := newMemoryDatabase(t)
		err := validateDatabaseSchemaWithModels(database, database.Migrator(), map[string]struct{}{}, []interface{}{make(chan int)})
		if err == nil || !strings.Contains(err.Error(), "parse managed schema") {
			t.Fatalf("schema parse error = %v", err)
		}
	})
	t.Run("column inspection failure", func(t *testing.T) {
		database := newMemoryDatabase(t)
		err := validateDatabaseSchemaWithModels(
			database,
			failingSchemaMigrator{Migrator: database.Migrator(), columnErr: errors.New("column inspection blocked")},
			map[string]struct{}{"notifications": {}},
			[]interface{}{&model.Notification{}},
		)
		if err == nil || !strings.Contains(err.Error(), "inspect table notifications") {
			t.Fatalf("column inspection error = %v", err)
		}
	})
	t.Run("index inspection failure", func(t *testing.T) {
		database := newMemoryDatabase(t)
		err := validateDatabaseSchemaWithModels(
			database,
			failingSchemaMigrator{Migrator: database.Migrator(), indexErr: errors.New("index inspection blocked")},
			map[string]struct{}{"notifications": {}},
			[]interface{}{&model.Notification{}},
		)
		if err == nil || !strings.Contains(err.Error(), "inspect indexes for table notifications") {
			t.Fatalf("index inspection error = %v", err)
		}
	})
}

func TestSQLiteDSNAppendsPragmas(t *testing.T) {
	t.Helper()

	plainDSN := sqliteDSN("pingwin.db")
	if plainDSN != "pingwin.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)" {
		t.Fatalf("unexpected SQLite driver configuration %s", plainDSN)
	}

	existingQueryDSN := sqliteDSN("file:pingwin.db?cache=shared")
	if existingQueryDSN != "file:pingwin.db?cache=shared&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)" {
		t.Fatalf("unexpected SQLite driver configuration %s", existingQueryDSN)
	}
}

func TestInitDBCreatesMissingDirectories(t *testing.T) {
	t.Helper()

	baseDirectory := t.TempDir()
	databasePath := filepath.Join(baseDirectory, "nested", "pingwin.db")
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

	_, initError := InitDB(filepath.Join(blockingFile, "pingwin.db"), logger)
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
	_, initError = InitDB(filepath.Join(t.TempDir(), "pingwin.db"), logger)
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
		return "successful query with sensitive data", 1
	}, nil)
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "missing record with sensitive data", 0
	}, gorm.ErrRecordNotFound)
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "failed query with sensitive data", 0
	}, errors.New("query failed"))
}

func TestSlogGormLoggerOmitsInterpolatedSQLValues(t *testing.T) {
	t.Helper()

	var logBuffer bytes.Buffer
	logger := &slogGormLogger{logger: slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{}))}
	logger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "driver-generated statement with smtp_sensitive_username", 0
	}, errors.New("query failed"))

	output := logBuffer.String()
	if strings.Contains(output, "smtp_sensitive_username") {
		t.Fatalf("expected log output to omit SQL values, got %q", output)
	}
	if strings.Contains(output, "driver-generated statement") {
		t.Fatalf("expected log output to omit raw SQL, got %q", output)
	}
	if !strings.Contains(output, "query failed") {
		t.Fatalf("expected log output to include error, got %q", output)
	}
	if !strings.Contains(output, "rows=0") {
		t.Fatalf("expected log output to include row count, got %q", output)
	}
}

func newMemoryDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := InitDB(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("initialize in-memory database: %v", err)
	}
	handle, err := database.DB()
	if err != nil {
		t.Fatalf("get database lifecycle handle: %v", err)
	}
	handle.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	return database
}
