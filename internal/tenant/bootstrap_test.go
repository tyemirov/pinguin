package tenant

import (
	"context"
	"testing"
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
	if err := dbInstance.Where("host = ?", domainHost).First(&domain).Error; err != nil {
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

func bootstrapTenantSpec(tenantID string, domainHosts []string) BootstrapTenant {
	return BootstrapTenant{
		ID:           tenantID,
		DisplayName:  tenantID,
		SupportEmail: "support@" + tenantID + ".example",
		Enabled:      ptrBool(true),
		Domains:      domainHosts,
		Admins:       BootstrapAdmins{"admin@" + tenantID + ".example"},
		EmailProfile: BootstrapEmailProfile{
			Host:        "smtp." + tenantID + ".example",
			Port:        587,
			Username:    "smtp-user-" + tenantID,
			Password:    "smtp-pass-" + tenantID,
			FromAddress: "noreply@" + tenantID + ".example",
		},
	}
}
