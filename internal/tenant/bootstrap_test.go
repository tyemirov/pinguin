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
