package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	sqliteBusyTimeoutMilliseconds = 10000
	sqliteJournalMode             = "WAL"
	sqlitePragmaQueryKey          = "_pragma"
)

func InitDB(dbPath string, logger *slog.Logger) (*gorm.DB, error) {
	logger.Info("Initializing SQLite DB", "path", dbPath)

	directory := filepath.Dir(dbPath)
	if directory != "." && directory != "" {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory failed: %w", err)
		}
	}

	gormLogger := &slogGormLogger{logger: logger}
	database, err := gorm.Open(sqlite.Open(sqliteDSN(dbPath)), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}

	if err := migrateDatabaseSchema(database); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return database, nil
}

func sqliteDSN(dbPath string) string {
	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	return fmt.Sprintf(
		"%s%s%s=busy_timeout(%d)&%s=journal_mode(%s)",
		dbPath,
		separator,
		sqlitePragmaQueryKey,
		sqliteBusyTimeoutMilliseconds,
		sqlitePragmaQueryKey,
		sqliteJournalMode,
	)
}

var migrateDatabaseSchema = func(database *gorm.DB) error {
	tables, tableErr := database.Migrator().GetTables()
	if tableErr != nil {
		return fmt.Errorf("inspect database schema: %w", tableErr)
	}
	applicationTables := make(map[string]struct{})
	for _, table := range tables {
		if table != "sqlite_sequence" {
			applicationTables[table] = struct{}{}
		}
	}
	if len(applicationTables) > 0 {
		return validateDatabaseSchema(database, applicationTables)
	}
	return database.AutoMigrate(
		&model.Notification{},
		&model.NotificationAttachment{},
		&tenant.Tenant{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&tenant.APICredential{},
		&tenant.IdempotencyRecord{},
		&smtpidentity.SenderDomain{},
		&smtpidentity.Identity{},
		&smtpidentity.ForwardRecipient{},
	)
}

// ValidateManagedSchema verifies that a database contains only the current managed schema.
func ValidateManagedSchema(database *gorm.DB) error {
	tables, tableErr := database.Migrator().GetTables()
	if tableErr != nil {
		return fmt.Errorf("inspect database schema: %w", tableErr)
	}
	applicationTables := make(map[string]struct{})
	for _, tableName := range tables {
		if tableName != "sqlite_sequence" {
			applicationTables[tableName] = struct{}{}
		}
	}
	return validateDatabaseSchema(database, applicationTables)
}

func validateDatabaseSchema(database *gorm.DB, actualTables map[string]struct{}) error {
	models := []interface{}{
		&model.Notification{},
		&model.NotificationAttachment{},
		&tenant.Tenant{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&tenant.APICredential{},
		&tenant.IdempotencyRecord{},
		&smtpidentity.SenderDomain{},
		&smtpidentity.Identity{},
		&smtpidentity.ForwardRecipient{},
	}
	return validateDatabaseSchemaWithModels(database, database.Migrator(), actualTables, models)
}

func validateDatabaseSchemaWithModels(database *gorm.DB, migrator gorm.Migrator, actualTables map[string]struct{}, models []interface{}) error {
	expectedTables := make(map[string]struct{}, len(models))
	for _, schemaModel := range models {
		statement := &gorm.Statement{DB: database}
		if parseErr := statement.Parse(schemaModel); parseErr != nil {
			return fmt.Errorf("parse managed schema: %w", parseErr)
		}
		expectedTables[statement.Schema.Table] = struct{}{}
		if _, exists := actualTables[statement.Schema.Table]; !exists {
			return fmt.Errorf("database schema is not current: missing table %s", statement.Schema.Table)
		}
		columnTypes, columnErr := migrator.ColumnTypes(schemaModel)
		if columnErr != nil {
			return fmt.Errorf("inspect table %s: %w", statement.Schema.Table, columnErr)
		}
		actualColumns := make(map[string]struct{}, len(columnTypes))
		for _, columnType := range columnTypes {
			actualColumns[columnType.Name()] = struct{}{}
		}
		if len(actualColumns) != len(statement.Schema.DBNames) {
			return fmt.Errorf("database schema is not current: table %s has %d columns, expected %d", statement.Schema.Table, len(actualColumns), len(statement.Schema.DBNames))
		}
		for _, expectedColumn := range statement.Schema.DBNames {
			if _, exists := actualColumns[expectedColumn]; !exists {
				return fmt.Errorf("database schema is not current: table %s is missing column %s", statement.Schema.Table, expectedColumn)
			}
		}
		declaredIndexes := statement.Schema.ParseIndexes()
		expectedIndexes := make(map[string]struct{}, len(declaredIndexes))
		for _, index := range declaredIndexes {
			expectedIndexes[index.Name] = struct{}{}
		}
		actualIndexes, indexErr := migrator.GetIndexes(schemaModel)
		if indexErr != nil {
			return fmt.Errorf("inspect indexes for table %s: %w", statement.Schema.Table, indexErr)
		}
		actualIndexNames := make(map[string]struct{}, len(actualIndexes))
		for _, index := range actualIndexes {
			if !strings.HasPrefix(index.Name(), "sqlite_autoindex_") {
				actualIndexNames[index.Name()] = struct{}{}
			}
		}
		if len(actualIndexNames) != len(expectedIndexes) {
			return fmt.Errorf("database schema is not current: table %s has %d indexes, expected %d", statement.Schema.Table, len(actualIndexNames), len(expectedIndexes))
		}
		for indexName := range expectedIndexes {
			if _, exists := actualIndexNames[indexName]; !exists {
				return fmt.Errorf("database schema is not current: table %s is missing index %s", statement.Schema.Table, indexName)
			}
		}
	}
	if len(actualTables) != len(expectedTables) {
		return fmt.Errorf("database schema is not current: found %d application tables, expected %d", len(actualTables), len(expectedTables))
	}
	return nil
}

type slogGormLogger struct {
	logger *slog.Logger
}

var _ logger.Interface = (*slogGormLogger)(nil)

func (l *slogGormLogger) LogMode(_ logger.LogLevel) logger.Interface {
	return l
}

func (l *slogGormLogger) Info(_ context.Context, msg string, data ...interface{}) {
	l.logger.Info(msg, data...)
}

func (l *slogGormLogger) Warn(_ context.Context, msg string, data ...interface{}) {
	l.logger.Warn(msg, data...)
}

func (l *slogGormLogger) Error(_ context.Context, msg string, data ...interface{}) {
	l.logger.Error(msg, data...)
}

func (l *slogGormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	_, rows := fc()
	elapsed := time.Since(begin)

	if err != nil && err != gorm.ErrRecordNotFound {
		l.logger.Error("database_query_failed",
			"error", err,
			"rows", rows,
			"elapsed", elapsed,
		)
	}
}
