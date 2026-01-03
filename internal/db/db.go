package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}

	if err := database.AutoMigrate(
		&model.Notification{},
		&model.NotificationAttachment{},
		&tenant.Tenant{},
		&tenant.TenantDomain{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
	); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return database, nil
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
	sql, rows := fc()
	elapsed := time.Since(begin)

	if err != nil && err != gorm.ErrRecordNotFound {
		l.logger.Error("Trace error",
			"error", err,
			"sql", sql,
			"rows", rows,
			"elapsed", elapsed,
		)
	}
}
