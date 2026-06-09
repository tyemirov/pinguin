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
	return database.AutoMigrate(
		&model.Notification{},
		&model.NotificationAttachment{},
		&tenant.Tenant{},
		&tenant.TenantDomain{},
		&tenant.TenantAdmin{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&smtpidentity.SenderDomain{},
		&smtpidentity.Identity{},
		&smtpidentity.ForwardRecipient{},
	)
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
