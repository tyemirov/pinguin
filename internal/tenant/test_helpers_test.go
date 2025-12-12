package tenant

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	return newTestDatabaseWithLogger(t, nil)
}

func newTestDatabaseWithLogger(t *testing.T, customLogger logger.Interface) *gorm.DB {
	t.Helper()
	config := &gorm.Config{}
	if customLogger != nil {
		config.Logger = customLogger
	}
	dbInstance, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), config)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbInstance.AutoMigrate(
		&Tenant{},
		&TenantDomain{},
		&TenantMember{},
		&TenantIdentity{},
		&EmailProfile{},
		&SMSProfile{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return dbInstance
}

func writeBootstrapFile(t *testing.T, cfg BootstrapConfig) string {
	t.Helper()
	payload, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal bootstrap yaml: %v", err)
	}
	path := filepath.Join(t.TempDir(), "tenants.yml")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write bootstrap yaml file: %v", err)
	}
	return path
}

func sampleBootstrapConfig() BootstrapConfig {
	return BootstrapConfig{
		Tenants: []BootstrapTenant{
			{
				ID:           "tenant-one",
				Slug:         "alpha",
				DisplayName:  "Alpha Corp",
				SupportEmail: "support@alpha.example",
				Status:       string(TenantStatusActive),
				Domains:      []string{"alpha.example", "portal.alpha.example"},
				Admins:       BootstrapAdmins{"admin@alpha.example", "viewer@alpha.example"},
				Identity: BootstrapIdentity{
					GoogleClientID: "google-alpha",
					TAuthBaseURL:   "https://tauth.alpha.example",
				},
				EmailProfile: BootstrapEmailProfile{
					Host:        "smtp.alpha.example",
					Port:        587,
					Username:    "smtp-user",
					Password:    "smtp-pass",
					FromAddress: "noreply@alpha.example",
				},
				SMSProfile: &BootstrapSMSProfile{
					AccountSID: "AC123",
					AuthToken:  "sms-secret",
					FromNumber: "+10000000000",
				},
			},
		},
	}
}
