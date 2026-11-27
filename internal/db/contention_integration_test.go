//go:build integration

package db

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tyemirov/pingwin/internal/model"
	"gorm.io/gorm"
)

func TestIntegrationInitDBReadersRemainAvailableDuringPendingWrite(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "pingwin.db")
	writer := newContentionDatabase(t, databasePath)
	reader := newContentionDatabase(t, databasePath)
	record := model.Notification{
		TenantID:       dbTestTenantID,
		NotificationID: "concurrent-reader",
		Message:        "committed",
	}
	if err := writer.Create(&record).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	err := writer.Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Model(&record).Update("message", "pending").Error; err != nil {
			return err
		}
		var fetched model.Notification
		if err := reader.Where(&model.Notification{NotificationID: record.NotificationID}).First(&fetched).Error; err != nil {
			return err
		}
		if fetched.Message != "committed" {
			t.Errorf("reader saw uncommitted message %q", fetched.Message)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read while writer transaction is active: %v", err)
	}
	if _, err := os.Stat(databasePath + "-wal"); err != nil {
		t.Fatalf("expected active driver-managed write-ahead log: %v", err)
	}
}

func TestIntegrationInitDBWaitsForWriterContention(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "pingwin.db")
	firstWriter := newContentionDatabase(t, databasePath)
	secondWriter := newContentionDatabase(t, databasePath)
	record := model.Notification{
		TenantID:       dbTestTenantID,
		NotificationID: "concurrent-writer",
		Message:        "original",
	}
	if err := firstWriter.Create(&record).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	transaction := firstWriter.Begin()
	if transaction.Error != nil {
		t.Fatalf("start first writer: %v", transaction.Error)
	}
	t.Cleanup(func() { _ = transaction.Rollback().Error })
	if err := transaction.Model(&record).Update("message", "first writer").Error; err != nil {
		t.Fatalf("hold first writer lock: %v", err)
	}

	started := make(chan struct{}, 1)
	const callbackName = "pingwin:writer_contention_started"
	if err := secondWriter.Callback().Update().Before("gorm:update").Register(callbackName, func(*gorm.DB) {
		started <- struct{}{}
	}); err != nil {
		t.Fatalf("register writer start signal: %v", err)
	}
	t.Cleanup(func() {
		if err := secondWriter.Callback().Update().Remove(callbackName); err != nil {
			t.Errorf("remove writer start signal: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		result <- secondWriter.WithContext(ctx).
			Model(&model.Notification{}).
			Where(&model.Notification{TenantID: dbTestTenantID, NotificationID: record.NotificationID}).
			Update("message", "second writer").Error
	}()
	t.Cleanup(func() {
		cancel()
		_ = transaction.Rollback().Error
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("second writer did not stop")
		}
	})
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("second writer did not reach its update")
	}
	select {
	case err := <-result:
		t.Fatalf("second writer completed while the first lock was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := transaction.Commit().Error; err != nil {
		t.Fatalf("release first writer lock: %v", err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("complete second write after lock release: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("second writer did not complete after lock release")
	}
	var stored model.Notification
	if err := secondWriter.Where(&model.Notification{NotificationID: record.NotificationID}).First(&stored).Error; err != nil {
		t.Fatalf("read second writer result: %v", err)
	}
	if stored.Message != "second writer" {
		t.Fatalf("expected second writer update, got %q", stored.Message)
	}
}

func newContentionDatabase(t *testing.T, path string) *gorm.DB {
	t.Helper()
	database, err := InitDB(path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("initialize contention database: %v", err)
	}
	handle, err := database.DB()
	if err != nil {
		t.Fatalf("get contention database lifecycle handle: %v", err)
	}
	handle.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Errorf("close contention database: %v", err)
		}
	})
	return database
}
