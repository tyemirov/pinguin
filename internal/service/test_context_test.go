package service

import (
	"context"

	"github.com/tyemirov/pinguin/internal/tenant"
)

const testTenantID = "tenant-service"

func tenantContext() context.Context {
	return tenant.WithRuntime(context.Background(), baseRuntimeConfig())
}

func tenantContextWithoutSMS() context.Context {
	cfg := baseRuntimeConfig()
	cfg.SMS = nil
	return tenant.WithRuntime(context.Background(), cfg)
}

func baseRuntimeConfig() tenant.RuntimeConfig {
	return tenant.RuntimeConfig{
		Tenant: tenant.Tenant{
			ID: testTenantID,
		},
		Identity: tenant.TenantIdentity{
			ViewScope: tenant.ViewScopeTenant,
		},
		Email: tenant.EmailCredentials{
			Host:        "smtp.test",
			Port:        587,
			Username:    "smtp-user",
			Password:    "smtp-pass",
			FromAddress: "noreply@test",
		},
		SMS: &tenant.SMSCredentials{
			AccountSID: "AC123",
			AuthToken:  "sms-secret",
			FromNumber: "+15550000000",
		},
	}
}
