package tenant

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm/logger"
)

func ptrBool(value bool) *bool {
	return &value
}

type queryCounter struct {
	logger.Interface
	mutex   sync.Mutex
	queries int
}

func newQueryCounter() *queryCounter {
	baseLogger := logger.New(log.New(io.Discard, "", log.LstdFlags), logger.Config{
		LogLevel: logger.Silent,
	})
	return &queryCounter{Interface: baseLogger}
}

func (counter *queryCounter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	counter.mutex.Lock()
	counter.queries++
	counter.mutex.Unlock()
	counter.Interface.Trace(ctx, begin, fc, err)
}

func (counter *queryCounter) Count() int {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()
	return counter.queries
}

func (counter *queryCounter) Reset() {
	counter.mutex.Lock()
	counter.queries = 0
	counter.mutex.Unlock()
}

func TestRepositoryResolveByHost(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	configPath := writeBootstrapFile(t, sampleBootstrapConfig())
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, configPath); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}

	repo := NewRepository(dbInstance, keeper)
	runtimeCfg, err := repo.ResolveByHost(context.Background(), "portal.alpha.example")
	if err != nil {
		t.Fatalf("resolve host error: %v", err)
	}
	if runtimeCfg.Tenant.ID != "tenant-one" {
		t.Fatalf("unexpected tenant id %q", runtimeCfg.Tenant.ID)
	}
	if runtimeCfg.Email.Username != "smtp-user" || runtimeCfg.Email.Password != "smtp-pass" {
		t.Fatalf("SMTP credentials not decrypted correctly")
	}
	if runtimeCfg.SMS == nil || runtimeCfg.SMS.AccountSID != "AC123" {
		t.Fatalf("expected SMS credentials")
	}
	if len(runtimeCfg.Admins) != 2 {
		t.Fatalf("expected 2 admins, got %d", len(runtimeCfg.Admins))
	}
}

func TestRepositoryResolveByHostCachesRuntimeConfig(t *testing.T) {
	t.Helper()
	counter := newQueryCounter()
	dbInstance := newTestDatabaseWithLogger(t, counter)
	keeper := newTestSecretKeeper(t)
	configPath := writeBootstrapFile(t, sampleBootstrapConfig())
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, configPath); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}
	repo := NewRepository(dbInstance, keeper)

	counter.Reset()
	if _, err := repo.ResolveByHost(context.Background(), "portal.alpha.example"); err != nil {
		t.Fatalf("resolve host error: %v", err)
	}
	if firstQueries := counter.Count(); firstQueries == 0 {
		t.Fatalf("expected database queries during first resolve")
	}

	counter.Reset()
	if _, err := repo.ResolveByHost(context.Background(), "portal.alpha.example"); err != nil {
		t.Fatalf("resolve host error: %v", err)
	}
	if cachedQueries := counter.Count(); cachedQueries != 0 {
		t.Fatalf("expected cached resolve without database queries, got %d", cachedQueries)
	}
}

func TestRepositoryResolveByIDCachesRuntimeConfig(t *testing.T) {
	t.Helper()
	counter := newQueryCounter()
	dbInstance := newTestDatabaseWithLogger(t, counter)
	keeper := newTestSecretKeeper(t)
	configPath := writeBootstrapFile(t, sampleBootstrapConfig())
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, configPath); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}
	repo := NewRepository(dbInstance, keeper)

	counter.Reset()
	if _, err := repo.ResolveByID(context.Background(), "tenant-one"); err != nil {
		t.Fatalf("resolve by id error: %v", err)
	}
	if firstQueries := counter.Count(); firstQueries == 0 {
		t.Fatalf("expected database queries during first resolve by id")
	}

	counter.Reset()
	if _, err := repo.ResolveByID(context.Background(), "tenant-one"); err != nil {
		t.Fatalf("resolve by id error: %v", err)
	}
	if cachedQueries := counter.Count(); cachedQueries != 0 {
		t.Fatalf("expected cached resolve by id to avoid database queries, got %d", cachedQueries)
	}
}

func TestRepositoryResolveByIDRejectsEmpty(t *testing.T) {
	t.Helper()
	counter := newQueryCounter()
	dbInstance := newTestDatabaseWithLogger(t, counter)
	keeper := newTestSecretKeeper(t)
	repo := NewRepository(dbInstance, keeper)

	counter.Reset()
	_, err := repo.ResolveByID(context.Background(), "   ")
	if err == nil {
		t.Fatalf("expected error for empty tenant id")
	}
	if !errors.Is(err, ErrInvalidTenantID) {
		t.Fatalf("expected ErrInvalidTenantID, got %v", err)
	}
	if cachedQueries := counter.Count(); cachedQueries != 0 {
		t.Fatalf("expected validation to short-circuit without queries, got %d", cachedQueries)
	}
}

func TestRepositoryRuntimeCacheIsolation(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	configPath := writeBootstrapFile(t, sampleBootstrapConfig())
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, configPath); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}
	repo := NewRepository(dbInstance, keeper)

	runtimeCfg, err := repo.ResolveByID(context.Background(), "tenant-one")
	if err != nil {
		t.Fatalf("resolve by id error: %v", err)
	}
	runtimeCfg.Admins["mutated@example.com"] = struct{}{}
	if runtimeCfg.SMS != nil {
		runtimeCfg.SMS.AuthToken = "tampered"
	}

	cachedCfg, err := repo.ResolveByID(context.Background(), "tenant-one")
	if err != nil {
		t.Fatalf("resolve by id error: %v", err)
	}
	if len(cachedCfg.Admins) != 2 {
		t.Fatalf("expected cached admin map to remain unchanged, got %d entries", len(cachedCfg.Admins))
	}
	if _, exists := cachedCfg.Admins["mutated@example.com"]; exists {
		t.Fatalf("cached admin map should not retain caller mutations")
	}
	if cachedCfg.SMS != nil && cachedCfg.SMS.AuthToken == "tampered" {
		t.Fatalf("cached SMS credentials should not retain caller mutations")
	}
}

func TestRepositoryCachesInvalidateAfterBootstrap(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	cfg := sampleBootstrapConfig()
	path := writeBootstrapFile(t, cfg)
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, path); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}
	repo := NewRepository(dbInstance, keeper)
	if _, err := repo.ResolveByHost(context.Background(), "portal.alpha.example"); err != nil {
		t.Fatalf("resolve host error: %v", err)
	}

	cfg.Tenants[0].Admins = BootstrapAdmins{"rotated@alpha.example"}
	cfg.Tenants[0].EmailProfile.Password = "new-smtp-password"
	cfg.Tenants[0].SMSProfile = &BootstrapSMSProfile{
		AccountSID: "AC999",
		AuthToken:  "sms-rotated",
		FromNumber: "+19999999999",
	}
	updatedPath := writeBootstrapFile(t, cfg)
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, updatedPath); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}

	refreshedCfg, err := repo.ResolveByHost(context.Background(), "portal.alpha.example")
	if err != nil {
		t.Fatalf("resolve after bootstrap error: %v", err)
	}
	if len(refreshedCfg.Admins) != 1 {
		t.Fatalf("expected refreshed admin list, got %d entries", len(refreshedCfg.Admins))
	}
	if refreshedCfg.Email.Password != "new-smtp-password" {
		t.Fatalf("expected refreshed SMTP password")
	}
	if refreshedCfg.SMS == nil || refreshedCfg.SMS.AuthToken != "sms-rotated" {
		t.Fatalf("expected refreshed SMS credentials")
	}
}

func TestRepositoryListActiveTenants(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	cfg := sampleBootstrapConfig()
	cfg.Tenants = append(cfg.Tenants, BootstrapTenant{
		ID:           "tenant-two",
		DisplayName:  "Beta",
		SupportEmail: "support@beta.example",
		Enabled:      ptrBool(false),
		Domains:      []string{"beta.example"},
		Admins:       BootstrapAdmins{},
		Identity: BootstrapIdentity{
			GoogleClientID: "google-beta",
			TAuthBaseURL:   "https://tauth.beta.example",
		},
		EmailProfile: BootstrapEmailProfile{
			Host:        "smtp.beta.example",
			Port:        25,
			Username:    "beta-user",
			Password:    "beta-pass",
			FromAddress: "noreply@beta.example",
		},
	})
	configPath := writeBootstrapFile(t, cfg)
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, configPath); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}

	repo := NewRepository(dbInstance, keeper)
	tenants, err := repo.ListActiveTenants(context.Background())
	if err != nil {
		t.Fatalf("list active tenants error: %v", err)
	}
	if len(tenants) != 1 || tenants[0].ID != "tenant-one" {
		t.Fatalf("expected only active tenant, got %+v", tenants)
	}
}

func TestRuntimeContextHelpers(t *testing.T) {
	t.Helper()
	cfg := RuntimeConfig{
		Tenant: Tenant{ID: "tenant-ctx"},
	}
	ctx := context.Background()
	ctx = WithRuntime(ctx, cfg)
	result, ok := RuntimeFromContext(ctx)
	if !ok {
		t.Fatalf("expected runtime config")
	}
	if result.Tenant.ID != "tenant-ctx" {
		t.Fatalf("unexpected tenant in context")
	}
}
