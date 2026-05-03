package tenant

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestBootstrapFromFileCreatesTenantRecords(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	cfg := sampleBootstrapConfig()
	configPath := writeBootstrapFile(t, cfg)

	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, configPath); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}

	var tenantCount int64
	if err := dbInstance.Model(&Tenant{}).Count(&tenantCount).Error; err != nil {
		t.Fatalf("count tenants: %v", err)
	}
	if tenantCount != 1 {
		t.Fatalf("expected 1 tenant, got %d", tenantCount)
	}

	var domainCount int64
	if err := dbInstance.Model(&TenantDomain{}).Count(&domainCount).Error; err != nil {
		t.Fatalf("count domains: %v", err)
	}
	if domainCount != 2 {
		t.Fatalf("expected 2 domains, got %d", domainCount)
	}

	var emailProfile EmailProfile
	if err := dbInstance.First(&emailProfile).Error; err != nil {
		t.Fatalf("fetch email profile: %v", err)
	}
	if len(emailProfile.UsernameCipher) == 0 || len(emailProfile.PasswordCipher) == 0 {
		t.Fatalf("expected encrypted credentials")
	}
}

func TestBootstrapFromFileRejectsEmptyList(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	path := writeBootstrapFile(t, BootstrapConfig{})

	err := BootstrapFromFile(context.Background(), dbInstance, keeper, path)
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestBootstrapFromYamlFile(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	cfg := sampleBootstrapConfig()
	path := writeBootstrapFile(t, cfg)

	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, path); err != nil {
		t.Fatalf("bootstrap yaml error: %v", err)
	}

	var tenantCount int64
	if err := dbInstance.Model(&Tenant{}).Count(&tenantCount).Error; err != nil {
		t.Fatalf("count tenants: %v", err)
	}
	if tenantCount != 1 {
		t.Fatalf("expected 1 tenant, got %d", tenantCount)
	}
}

func TestBootstrapReassignsDomain(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	domainHost := "ps.mprlab.com"

	initialConfig := BootstrapConfig{
		Tenants: []BootstrapTenant{
			bootstrapTenantSpec("tenant-one", []string{domainHost}),
		},
	}
	if err := Bootstrap(context.Background(), dbInstance, keeper, initialConfig); err != nil {
		t.Fatalf("bootstrap initial config: %v", err)
	}

	updatedConfig := BootstrapConfig{
		Tenants: []BootstrapTenant{
			bootstrapTenantSpec("tenant-two", []string{domainHost}),
		},
	}
	if err := Bootstrap(context.Background(), dbInstance, keeper, updatedConfig); err != nil {
		t.Fatalf("bootstrap updated config: %v", err)
	}

	var domain TenantDomain
	if err := dbInstance.Where(&TenantDomain{Host: domainHost}).First(&domain).Error; err != nil {
		t.Fatalf("fetch domain: %v", err)
	}
	if domain.TenantID != "tenant-two" {
		t.Fatalf("expected domain tenant to be tenant-two, got %s", domain.TenantID)
	}

	var domainCount int64
	if err := dbInstance.Model(&TenantDomain{}).Count(&domainCount).Error; err != nil {
		t.Fatalf("count domains: %v", err)
	}
	if domainCount != 1 {
		t.Fatalf("expected 1 domain, got %d", domainCount)
	}
}

func TestBootstrapRejectsDuplicateDomains(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	domainHost := "ps.mprlab.com"
	cfg := BootstrapConfig{
		Tenants: []BootstrapTenant{
			bootstrapTenantSpec("tenant-one", []string{domainHost}),
			bootstrapTenantSpec("tenant-two", []string{domainHost}),
		},
	}

	err := Bootstrap(context.Background(), dbInstance, keeper, cfg)
	if err == nil {
		t.Fatalf("expected duplicate domain error")
	}
}

func TestBootstrapFromFileReportsReadAndParseErrors(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)

	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Fatalf("expected read error")
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid.yml")
	if err := os.WriteFile(invalidPath, []byte("tenants: ["), 0o600); err != nil {
		t.Fatalf("write invalid yaml: %v", err)
	}
	if err := BootstrapFromFile(context.Background(), dbInstance, keeper, invalidPath); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestBootstrapRejectsDisabledStatusAndMissingDomains(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	disabled := false

	err := Bootstrap(context.Background(), dbInstance, keeper, BootstrapConfig{
		Tenants: []BootstrapTenant{
			bootstrapTenantSpec("disabled", []string{"disabled.example"}),
		},
	})
	if err != nil {
		t.Fatalf("control bootstrap should succeed: %v", err)
	}

	err = Bootstrap(context.Background(), dbInstance, keeper, BootstrapConfig{
		Tenants: []BootstrapTenant{
			func() BootstrapTenant {
				spec := bootstrapTenantSpec("disabled", []string{"disabled.example"})
				spec.Enabled = &disabled
				return spec
			}(),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "no enabled tenants") {
		t.Fatalf("expected disabled tenants error, got %v", err)
	}

	err = Bootstrap(context.Background(), dbInstance, keeper, BootstrapConfig{
		Tenants: []BootstrapTenant{
			func() BootstrapTenant {
				spec := bootstrapTenantSpec("legacy-status", []string{"status.example"})
				spec.Status = "active"
				return spec
			}(),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "status is no longer supported") {
		t.Fatalf("expected legacy status error, got %v", err)
	}

	err = Bootstrap(context.Background(), dbInstance, keeper, BootstrapConfig{
		Tenants: []BootstrapTenant{
			bootstrapTenantSpec("missing-domain", []string{"   "}),
		},
	})
	if err == nil || !strings.Contains(err.Error(), bootstrapMissingDomainCode) {
		t.Fatalf("expected missing domain error, got %v", err)
	}
}

func TestBootstrapGeneratesTenantIDAndRemovesSMSProfile(t *testing.T) {
	t.Helper()
	dbInstance := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	initial := sampleBootstrapConfig()
	if err := Bootstrap(context.Background(), dbInstance, keeper, initial); err != nil {
		t.Fatalf("initial bootstrap: %v", err)
	}

	updated := initial
	updated.Tenants[0].SMSProfile = nil
	if err := Bootstrap(context.Background(), dbInstance, keeper, updated); err != nil {
		t.Fatalf("updated bootstrap without sms: %v", err)
	}
	var smsCount int64
	if err := dbInstance.Model(&SMSProfile{}).Where(&SMSProfile{TenantID: "tenant-one"}).Count(&smsCount).Error; err != nil {
		t.Fatalf("count sms profiles: %v", err)
	}
	if smsCount != 0 {
		t.Fatalf("expected SMS profile removal, got %d", smsCount)
	}

	anonymous := bootstrapTenantSpec("", []string{"generated.example"})
	if err := Bootstrap(context.Background(), dbInstance, keeper, BootstrapConfig{Tenants: []BootstrapTenant{anonymous}}); err != nil {
		t.Fatalf("bootstrap anonymous tenant: %v", err)
	}
	var tenantCount int64
	if err := dbInstance.Model(&Tenant{}).Where(clause.Neq{Column: clause.Column{Name: "id"}, Value: ""}).Count(&tenantCount).Error; err != nil {
		t.Fatalf("count generated tenants: %v", err)
	}
	if tenantCount == 0 {
		t.Fatalf("expected generated tenant id")
	}
}

func TestNormalizeDomainHostsSkipsBlankValues(t *testing.T) {
	t.Helper()
	hosts := normalizeDomainHosts([]string{" Alpha.Example ", " ", "BETA.example"})
	if len(hosts) != 2 || hosts[0] != "alpha.example" || hosts[1] != "beta.example" {
		t.Fatalf("unexpected normalized hosts %v", hosts)
	}
}

func TestBootstrapReportsStorageAndCredentialFailures(t *testing.T) {
	testCases := []struct {
		name       string
		config     BootstrapConfig
		keeper     *SecretKeeper
		beforeCall func(*testing.T, *gorm.DB)
		wantErr    string
	}{
		{
			name: "reset domains",
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "delete", "TenantDomain", errors.New("reset failed"))
			},
			wantErr: bootstrapDomainResetCode,
		},
		{
			name: "tenant upsert",
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "create", "Tenant", errors.New("tenant create failed"))
			},
			wantErr: "upsert tenant tenant-one",
		},
		{
			name: "domain create",
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "create", "TenantDomain", errors.New("domain create failed"))
			},
			wantErr: "domain alpha.example",
		},
		{
			name: "domain lookup",
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "query", "TenantDomain", errors.New("domain query failed"))
			},
			wantErr: "domain alpha.example",
		},
		{
			name:    "email username encrypt",
			keeper:  &SecretKeeper{key: []byte("short")},
			wantErr: "init cipher",
		},
		{
			name: "email password encrypt",
			keeper: func() *SecretKeeper {
				keeper := newTestSecretKeeper(t)
				keeper.random = io.MultiReader(bytes.NewReader(make([]byte, 12)), failingReader{err: io.ErrUnexpectedEOF})
				return keeper
			}(),
			wantErr: "nonce",
		},
		{
			name: "email delete",
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "delete", "EmailProfile", errors.New("email delete failed"))
			},
			wantErr: "email delete failed",
		},
		{
			name: "email create",
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "create", "EmailProfile", errors.New("email create failed"))
			},
			wantErr: "email profile",
		},
		{
			name: "sms account encrypt",
			config: func() BootstrapConfig {
				cfg := sampleBootstrapConfig()
				cfg.Tenants[0].SMSProfile = &BootstrapSMSProfile{AccountSID: "AC123", AuthToken: "token", FromNumber: "+15550001111"}
				return cfg
			}(),
			keeper: func() *SecretKeeper {
				keeper := newTestSecretKeeper(t)
				keeper.random = io.MultiReader(
					bytes.NewReader(make([]byte, 12+12)),
					failingReader{err: io.ErrUnexpectedEOF},
				)
				return keeper
			}(),
			wantErr: "nonce",
		},
		{
			name: "sms token encrypt",
			config: func() BootstrapConfig {
				cfg := sampleBootstrapConfig()
				cfg.Tenants[0].SMSProfile = &BootstrapSMSProfile{AccountSID: "AC123", AuthToken: "token", FromNumber: "+15550001111"}
				return cfg
			}(),
			keeper: func() *SecretKeeper {
				keeper := newTestSecretKeeper(t)
				keeper.random = io.MultiReader(
					bytes.NewReader(make([]byte, 12+12+12)),
					failingReader{err: io.ErrUnexpectedEOF},
				)
				return keeper
			}(),
			wantErr: "nonce",
		},
		{
			name: "sms delete",
			config: func() BootstrapConfig {
				cfg := sampleBootstrapConfig()
				cfg.Tenants[0].SMSProfile = &BootstrapSMSProfile{AccountSID: "AC123", AuthToken: "token", FromNumber: "+15550001111"}
				return cfg
			}(),
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "delete", "SMSProfile", errors.New("sms delete failed"))
			},
			wantErr: "sms delete failed",
		},
		{
			name: "sms create",
			config: func() BootstrapConfig {
				cfg := sampleBootstrapConfig()
				cfg.Tenants[0].SMSProfile = &BootstrapSMSProfile{AccountSID: "AC123", AuthToken: "token", FromNumber: "+15550001111"}
				return cfg
			}(),
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "create", "SMSProfile", errors.New("sms create failed"))
			},
			wantErr: "sms profile",
		},
		{
			name: "sms delete without profile",
			config: func() BootstrapConfig {
				cfg := sampleBootstrapConfig()
				cfg.Tenants[0].SMSProfile = nil
				return cfg
			}(),
			beforeCall: func(t *testing.T, database *gorm.DB) {
				registerTenantCallbackError(t, database, "delete", "SMSProfile", errors.New("sms delete failed"))
			},
			wantErr: "sms delete failed",
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			database := newTestDatabase(t)
			keeper := testCase.keeper
			if keeper == nil {
				keeper = newTestSecretKeeper(t)
			}
			cfg := testCase.config
			if len(cfg.Tenants) == 0 {
				cfg = sampleBootstrapConfig()
			}
			if testCase.beforeCall != nil {
				testCase.beforeCall(t, database)
			}
			err := Bootstrap(context.Background(), database, keeper, cfg)
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("expected %q error, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestUpsertTenantReportsExistingDomainConflict(t *testing.T) {
	database := newTestDatabase(t)
	keeper := newTestSecretKeeper(t)
	if err := database.Create(&Tenant{ID: "tenant-existing", Status: TenantStatusActive}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := database.Create(&TenantDomain{TenantID: "tenant-existing", Host: "conflict.example", IsDefault: true}).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	err := upsertTenant(context.Background(), database, keeper, bootstrapTenantSpec("tenant-new", []string{"conflict.example"}))
	if err == nil || !strings.Contains(err.Error(), bootstrapDomainConflictCode) {
		t.Fatalf("expected domain conflict, got %v", err)
	}
}

func registerTenantCallbackError(t *testing.T, database *gorm.DB, callbackType string, schemaName string, callbackErr error) {
	t.Helper()
	callbackName := "pinguin:force_" + callbackType + "_" + schemaName + "_error"
	addError := func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == schemaName {
			tx.AddError(callbackErr)
		}
	}
	switch callbackType {
	case "create":
		if err := database.Callback().Create().Before("gorm:create").Register(callbackName, addError); err != nil {
			t.Fatalf("register create callback: %v", err)
		}
	case "delete":
		if err := database.Callback().Delete().Before("gorm:delete").Register(callbackName, addError); err != nil {
			t.Fatalf("register delete callback: %v", err)
		}
	case "query":
		if err := database.Callback().Query().Before("gorm:query").Register(callbackName, addError); err != nil {
			t.Fatalf("register query callback: %v", err)
		}
	default:
		t.Fatalf("unsupported callback type %s", callbackType)
	}
}

func bootstrapTenantSpec(tenantID string, domainHosts []string) BootstrapTenant {
	return BootstrapTenant{
		ID:           tenantID,
		DisplayName:  tenantID,
		SupportEmail: "support@" + tenantID + ".example",
		Enabled:      ptrBool(true),
		Domains:      domainHosts,
		EmailProfile: BootstrapEmailProfile{
			Host:        "smtp." + tenantID + ".example",
			Port:        587,
			Username:    "smtp-user-" + tenantID,
			Password:    "smtp-pass-" + tenantID,
			FromAddress: "noreply@" + tenantID + ".example",
		},
	}
}
