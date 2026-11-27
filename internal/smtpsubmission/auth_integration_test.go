//go:build integration

package smtpsubmission

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pingwin/internal/smtpidentity"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestIntegrationSMTPSubmissionRetriesAuthenticationAfterSQLiteReadLock(t *testing.T) {
	database, repository, identity, password := newSMTPAuthIdentity(t, "_txlock=exclusive")
	fixture := newSMTPIntegrationFixture(t, func(config *Config) {
		config.Authenticator = repository
		config.AuthFailureLimit = 2
	})
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")
	client.send(t, "AUTH PLAIN "+plainAuthPayload(identity.Username, "wrong-password"))
	client.expectCode(t, "535")

	lock := database.Begin()
	if lock.Error != nil {
		t.Fatalf("acquire exclusive SQLite lock: %v", lock.Error)
	}
	defer lock.Rollback()

	for attempt := 0; attempt < 3; attempt++ {
		client.send(t, "AUTH PLAIN "+plainAuthPayload(identity.Username, password))
		client.expectCode(t, "454 4.7.0 ")
	}
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "530")

	if err := lock.Rollback().Error; err != nil {
		t.Fatalf("release exclusive SQLite lock: %v", err)
	}
	client.send(t, "AUTH PLAIN "+plainAuthPayload(identity.Username, password))
	client.expectCode(t, "235")
}

func TestIntegrationSMTPSubmissionAuthenticatesWhileSQLiteWriterIsLocked(t *testing.T) {
	database, repository, identity, password := newSMTPAuthIdentity(t, "_pragma=journal_mode(WAL)")
	type usageResult struct {
		usedAt time.Time
		err    error
	}
	usage := make(chan usageResult, 2)
	fixture := newSMTPIntegrationFixture(t, func(config *Config) {
		config.Authenticator = repository
		config.IdentityUsageRecorder = identityUsageRecorderFunc(func(ctx context.Context, authenticated smtpidentity.AuthenticatedIdentity, usedAt time.Time) error {
			err := repository.RecordIdentityUse(ctx, authenticated, usedAt)
			usage <- usageResult{usedAt: usedAt, err: err}
			return err
		})
	})
	client := fixture.dial(t)
	defer client.close()
	client.expectCode(t, "220")

	lock := database.Begin()
	if lock.Error != nil {
		t.Fatalf("start SQLite writer: %v", lock.Error)
	}
	defer lock.Rollback()
	if err := lock.Model(&smtpidentity.Identity{}).
		Where(&smtpidentity.Identity{ID: identity.ID}).
		Update("updated_at", time.Now().UTC()).Error; err != nil {
		t.Fatalf("hold SQLite writer lock: %v", err)
	}

	client.send(t, "AUTH PLAIN "+plainAuthPayload(identity.Username, password))
	client.expectCode(t, "235")
	client.send(t, "MAIL FROM:<alice@example.com>")
	client.expectCode(t, "250")
	failedUsage := receiveWithin(t, usage)
	if failedUsage.err == nil || !strings.Contains(failedUsage.err.Error(), "SQLITE_BUSY") {
		t.Fatalf("expected SQLite contention while writing use, got %v", failedUsage.err)
	}

	if err := lock.Rollback().Error; err != nil {
		t.Fatalf("release SQLite writer lock: %v", err)
	}
	nextClient := fixture.dial(t)
	defer nextClient.close()
	nextClient.expectCode(t, "220")
	nextClient.send(t, "AUTH PLAIN "+plainAuthPayload(identity.Username, password))
	nextClient.expectCode(t, "235")

	recordedUsage := receiveWithin(t, usage)
	if recordedUsage.err != nil {
		t.Fatalf("record use after lock release: %v", recordedUsage.err)
	}
	var stored smtpidentity.Identity
	if err := database.Where(&smtpidentity.Identity{ID: identity.ID}).First(&stored).Error; err != nil {
		t.Fatalf("read recorded usage: %v", err)
	}
	if stored.LastUsedAt == nil || !stored.LastUsedAt.Equal(recordedUsage.usedAt) {
		t.Fatalf("expected usage timestamp %v, got %v", recordedUsage.usedAt, stored.LastUsedAt)
	}
}

func TestIntegrationSMTPSubmissionReturns454WhenAuthorizationReadFails(t *testing.T) {
	testCases := []struct {
		name  string
		match func(*gorm.Statement) bool
	}{
		{name: "sender domain", match: func(statement *gorm.Statement) bool {
			_, isDomain := statement.Dest.(*smtpidentity.SenderDomain)
			return isDomain
		}},
		{name: "active credential", match: func(statement *gorm.Statement) bool {
			_, isIdentity := statement.Dest.(*smtpidentity.Identity)
			return isIdentity && len(statement.Selects) > 0
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database, repository, identity, password := newSMTPAuthIdentity(t, "_pragma=journal_mode(WAL)")
			err := database.Callback().Query().Before("gorm:query").Register("pingwin:auth_read_error", func(tx *gorm.DB) {
				if testCase.match(tx.Statement) {
					tx.AddError(errors.New("storage temporarily unavailable"))
				}
			})
			if err != nil {
				t.Fatalf("register auth read failure: %v", err)
			}
			fixture := newSMTPIntegrationFixture(t, func(config *Config) {
				config.Authenticator = repository
			})
			client := fixture.dial(t)
			defer client.close()
			client.expectCode(t, "220")
			client.send(t, "AUTH PLAIN "+plainAuthPayload(identity.Username, password))
			client.expectCode(t, "454 4.7.0 Temporary authentication failure")
			client.send(t, "MAIL FROM:<alice@example.com>")
			client.expectCode(t, "530")
		})
	}
}

func TestIntegrationSMTPSubmissionStartsImplicitTLSListener(t *testing.T) {
	server := newBareSMTPSubmissionServer(t)
	server.config.TLSListenAddr = "127.0.0.1:0"
	server.config.TLSConfig = testTLSConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("start implicit TLS listener: %v", err)
	}
}

func newSMTPIntegrationFixture(t *testing.T, mutate func(*Config)) *smtpServerFixture {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	dialer := &net.Dialer{}
	return serveSMTPTestFixture(t, true, nil, mutate, listener, func(ctx context.Context) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", listener.Addr().String())
	})
}

func newSMTPAuthIdentity(t *testing.T, sqliteOptions string) (*gorm.DB, *smtpidentity.Repository, smtpidentity.PublicIdentity, string) {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "auth.db")
	database, err := gorm.Open(sqlite.Open(databasePath+"?_pragma=busy_timeout(0)&"+sqliteOptions), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open GORM database: %v", err)
	}
	handle, err := database.DB()
	if err != nil {
		t.Fatalf("get database lifecycle handle: %v", err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	if err := database.AutoMigrate(&smtpidentity.SenderDomain{}, &smtpidentity.Identity{}, &smtpidentity.ForwardRecipient{}); err != nil {
		t.Fatalf("migrate SMTP identity models: %v", err)
	}
	scope := smtpidentity.AccessScope{TenantID: "test-tenant"}
	if err := database.Create(&smtpidentity.SenderDomain{
		TenantID: scope.TenantID,
		Domain:   "example.com",
		Status:   smtpidentity.SenderDomainStatusVerified,
	}).Error; err != nil {
		t.Fatalf("create verified sender domain: %v", err)
	}
	repository, err := smtpidentity.NewRepository(database, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("create identity repository: %v", err)
	}
	identity, password, err := repository.CreateForScope(
		context.Background(),
		scope,
		mustSMTPSubmissionAddress(t, "alice@example.com"),
		[]smtpidentity.Address{mustSMTPSubmissionAddress(t, "owner@example.com")},
	)
	if err != nil {
		t.Fatalf("create SMTP identity: %v", err)
	}
	return database, repository, identity, password
}
