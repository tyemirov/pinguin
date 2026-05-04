package tenant

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
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
}

func TestRepositoryResolveByHostValidationAndLookupErrors(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	repo := NewRepository(dbInstance, keeper)

	if _, err := repo.ResolveByHost(context.Background(), "   "); err == nil {
		t.Fatalf("expected empty host error")
	}
	if _, err := repo.ResolveByHost(context.Background(), "missing.example"); err == nil {
		t.Fatalf("expected missing domain error")
	}
	if err := dbInstance.Create(&TenantDomain{Host: "orphan.example", TenantID: "missing-tenant"}).Error; err != nil {
		t.Fatalf("seed orphan domain: %v", err)
	}
	if _, err := repo.ResolveByHost(context.Background(), "orphan.example"); err == nil || !strings.Contains(err.Error(), "missing-tenant") {
		t.Fatalf("expected runtime error for orphan domain, got %v", err)
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
	if runtimeCfg.SMS != nil {
		runtimeCfg.SMS.AuthToken = "tampered"
	}

	cachedCfg, err := repo.ResolveByID(context.Background(), "tenant-one")
	if err != nil {
		t.Fatalf("resolve by id error: %v", err)
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

func TestRepositoryListActiveTenantsOrdersByDisplayName(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	cfg := BootstrapConfig{
		Tenants: []BootstrapTenant{
			bootstrapTenantSpec("tenant-z", []string{"z.example"}),
			bootstrapTenantSpec("tenant-a", []string{"a.example"}),
		},
	}
	cfg.Tenants[0].DisplayName = "Zulu"
	cfg.Tenants[1].DisplayName = "Alpha"
	if err := Bootstrap(context.Background(), dbInstance, keeper, cfg); err != nil {
		t.Fatalf("bootstrap tenants: %v", err)
	}

	tenants, err := NewRepository(dbInstance, keeper).ListActiveTenants(context.Background())
	if err != nil {
		t.Fatalf("list active tenants: %v", err)
	}
	if len(tenants) != 2 || tenants[0].DisplayName != "Alpha" || tenants[1].DisplayName != "Zulu" {
		t.Fatalf("expected display-name ordering, got %+v", tenants)
	}
}

func TestRepositoryListActiveTenantsByDomain(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	cfg := sampleBootstrapConfig()
	cfg.Tenants = append(cfg.Tenants,
		BootstrapTenant{
			ID:           "tenant-two",
			DisplayName:  "Beta",
			SupportEmail: "support@beta.example",
			Enabled:      ptrBool(true),
			Domains:      []string{"beta.example"},
			EmailProfile: BootstrapEmailProfile{
				Host:        "smtp.beta.example",
				Port:        25,
				Username:    "beta-user",
				Password:    "beta-pass",
				FromAddress: "noreply@beta.example",
			},
		},
		BootstrapTenant{
			ID:           "tenant-three",
			DisplayName:  "Gamma",
			SupportEmail: "support@gamma.example",
			Enabled:      ptrBool(false),
			Domains:      []string{"gamma.example"},
			EmailProfile: BootstrapEmailProfile{
				Host:        "smtp.gamma.example",
				Port:        25,
				Username:    "gamma-user",
				Password:    "gamma-pass",
				FromAddress: "noreply@gamma.example",
			},
		},
	)
	if err := Bootstrap(context.Background(), dbInstance, keeper, cfg); err != nil {
		t.Fatalf("bootstrap tenants: %v", err)
	}

	repo := NewRepository(dbInstance, keeper)
	tenants, err := repo.ListActiveTenantsByDomain(context.Background(), " PORTAL.ALPHA.EXAMPLE:443 ")
	if err != nil {
		t.Fatalf("list active tenants by domain: %v", err)
	}
	if len(tenants) != 1 || tenants[0].ID != "tenant-one" {
		t.Fatalf("expected tenant-one, got %+v", tenants)
	}
	suspendedTenants, suspendedErr := repo.ListActiveTenantsByDomain(context.Background(), "gamma.example")
	if suspendedErr != nil {
		t.Fatalf("list suspended domain tenants: %v", suspendedErr)
	}
	if len(suspendedTenants) != 0 {
		t.Fatalf("expected suspended tenant domain to be hidden, got %+v", suspendedTenants)
	}
	missingTenants, missingErr := repo.ListActiveTenantsByDomain(context.Background(), "missing.example")
	if missingErr != nil {
		t.Fatalf("list missing domain tenants: %v", missingErr)
	}
	if len(missingTenants) != 0 {
		t.Fatalf("expected no missing domain tenants, got %+v", missingTenants)
	}
}

func TestRepositoryListActiveTenantsByDomainReportsStorageFailure(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	repo := NewRepository(dbInstance, newTestSecretKeeper(t))
	closeTenantDatabase(t, dbInstance)
	if _, err := repo.ListActiveTenantsByDomain(context.Background(), "alpha.example"); err == nil {
		t.Fatalf("expected list active tenants by domain storage error")
	}
}

func TestRepositoryListActiveTenantsReportsStorageFailure(t *testing.T) {
	dbInstance := newTestDatabase(t)
	repo := NewRepository(dbInstance, newTestSecretKeeper(t))
	closeTenantDatabase(t, dbInstance)
	if _, err := repo.ListActiveTenants(context.Background()); err == nil {
		t.Fatalf("expected list active tenants storage error")
	}
}

func TestRepositoryLoadRuntimeErrors(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	repo := NewRepository(dbInstance, keeper)

	if _, err := repo.ResolveByID(context.Background(), "missing"); err == nil {
		t.Fatalf("expected missing tenant error")
	}

	if err := dbInstance.Create(&Tenant{
		ID:          "without-email",
		DisplayName: "Without Email",
		Status:      TenantStatusActive,
	}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if _, err := repo.ResolveByID(context.Background(), "without-email"); err == nil {
		t.Fatalf("expected missing email profile error")
	}

	usernameCipher, err := keeper.Encrypt("user")
	if err != nil {
		t.Fatalf("encrypt username: %v", err)
	}
	if err := dbInstance.Create(&EmailProfile{
		ID:             "email-bad-password",
		TenantID:       "without-email",
		Host:           "smtp.example",
		Port:           587,
		UsernameCipher: usernameCipher,
		PasswordCipher: []byte("not-a-ciphertext"),
		FromAddress:    "from@example.com",
		IsDefault:      true,
	}).Error; err != nil {
		t.Fatalf("create email profile: %v", err)
	}
	if _, err := repo.ResolveByID(context.Background(), "without-email"); err == nil {
		t.Fatalf("expected decrypt error")
	}
}

func TestRepositoryLoadRuntimeReportsSMSAndEmailDecryptErrors(t *testing.T) {
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	repo := NewRepository(dbInstance, keeper)
	if err := dbInstance.Create(&Tenant{
		ID:          "tenant-runtime-errors",
		DisplayName: "Runtime Errors",
		Status:      TenantStatusActive,
	}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	usernameCipher, err := keeper.Encrypt("smtp-user")
	if err != nil {
		t.Fatalf("encrypt username: %v", err)
	}
	passwordCipher, err := keeper.Encrypt("smtp-pass")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	if err := dbInstance.Create(&EmailProfile{
		ID:             "email-runtime-errors",
		TenantID:       "tenant-runtime-errors",
		Host:           "smtp.example",
		Port:           587,
		UsernameCipher: usernameCipher,
		PasswordCipher: passwordCipher,
		FromAddress:    "from@example.com",
		IsDefault:      true,
	}).Error; err != nil {
		t.Fatalf("create email profile: %v", err)
	}

	if err := dbInstance.Create(&SMSProfile{
		ID:               "sms-bad-account",
		TenantID:         "tenant-runtime-errors",
		AccountSIDCipher: []byte("bad-account"),
		AuthTokenCipher:  []byte("bad-token"),
		FromNumber:       "+15550001111",
		IsDefault:        true,
	}).Error; err != nil {
		t.Fatalf("create sms profile: %v", err)
	}
	if _, err := repo.ResolveByID(context.Background(), "tenant-runtime-errors"); err == nil {
		t.Fatalf("expected sms account decrypt error")
	}

	goodAccountCipher, err := keeper.Encrypt("AC123")
	if err != nil {
		t.Fatalf("encrypt account: %v", err)
	}
	var smsProfile SMSProfile
	if err := dbInstance.Where(&SMSProfile{ID: "sms-bad-account"}).First(&smsProfile).Error; err != nil {
		t.Fatalf("fetch sms profile: %v", err)
	}
	smsProfile.AccountSIDCipher = goodAccountCipher
	smsProfile.AuthTokenCipher = []byte("bad-token")
	if err := dbInstance.Save(&smsProfile).Error; err != nil {
		t.Fatalf("update sms profile: %v", err)
	}
	if _, err := repo.ResolveByID(context.Background(), "tenant-runtime-errors"); err == nil {
		t.Fatalf("expected sms token decrypt error")
	}

	if err := dbInstance.Where(&SMSProfile{ID: "sms-bad-account"}).Delete(&SMSProfile{}).Error; err != nil {
		t.Fatalf("delete sms profile: %v", err)
	}
	if err := dbInstance.Model(&EmailProfile{}).
		Where(&EmailProfile{ID: "email-runtime-errors"}).
		Update("username_cipher", []byte("bad-username")).Error; err != nil {
		t.Fatalf("update email username: %v", err)
	}
	if _, err := repo.ResolveByID(context.Background(), "tenant-runtime-errors"); err == nil {
		t.Fatalf("expected email username decrypt error")
	}
}

func TestRepositoryLoadRuntimeReportsSMSQueryFailure(t *testing.T) {
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	if err := Bootstrap(context.Background(), dbInstance, keeper, sampleBootstrapConfig()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	repo := NewRepository(dbInstance, keeper)
	if err := dbInstance.Migrator().DropTable(&SMSProfile{}); err != nil {
		t.Fatalf("drop sms profiles: %v", err)
	}
	if _, err := repo.ResolveByID(context.Background(), "tenant-one"); err == nil || !strings.Contains(err.Error(), "sms profile") {
		t.Fatalf("expected sms query failure, got %v", err)
	}
}

func TestRepositoryCacheHelpersIgnoreEmptyKeys(t *testing.T) {
	t.Helper()
	repo := NewRepository(newTestDatabase(t), newTestSecretKeeper(t))
	repo.cacheRuntimeConfig("", RuntimeConfig{Tenant: Tenant{ID: "ignored"}})
	if _, ok := repo.cachedRuntimeConfig(""); ok {
		t.Fatalf("expected empty runtime cache key to be ignored")
	}
	repo.cacheTenantID("", "tenant")
	repo.cacheTenantID("host.example", "")
	if _, ok := repo.cachedTenantID(""); ok {
		t.Fatalf("expected empty host cache key to be ignored")
	}
	if _, ok := repo.cachedTenantID("host.example"); ok {
		t.Fatalf("expected empty tenant id cache value to be ignored")
	}
}

func TestNormalizeHost(t *testing.T) {
	t.Helper()
	testCases := map[string]string{
		" Example.COM ":       "example.com",
		"example.com:8080":    "example.com",
		" ":                   "",
		"sub.example.com:443": "sub.example.com",
	}
	for input, expected := range testCases {
		if actual := normalizeHost(input); actual != expected {
			t.Fatalf("normalizeHost(%q) = %q, expected %q", input, actual, expected)
		}
	}
}

func closeTenantDatabase(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	if closeErr := sqlDatabase.Close(); closeErr != nil {
		t.Fatalf("close database: %v", closeErr)
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
