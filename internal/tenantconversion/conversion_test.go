package tenantconversion

import (
	"bytes"
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/tyemirov/pinguin/internal/db"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
)

func TestConvertProductionShapeFixture(t *testing.T) {
	database := newLegacyDatabase(t)
	keeper := conversionKeeper(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedLegacyState(t, database, keeper, now)
	credentialID := uuid.NewString()
	digest := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	secondCredentialID := uuid.NewString()
	source := SourceConfig{Tenants: []SourceTenant{
		{ID: "tenant-alpha", DisplayName: "Alpha", SupportEmail: "support@alpha.example", EmailProfile: SourceEmailProfile{Host: "smtp.alpha.example", Port: 587, Username: "alpha-user", Password: "alpha-pass", FromAddress: "sender@alpha.example"}, SMSProfile: &SourceSMSProfile{AccountSID: "AC-alpha", AuthToken: "alpha-token", FromNumber: "+15550000001"}},
		{ID: "tenant-delete", DisplayName: "Deleted", EmailProfile: SourceEmailProfile{Host: "smtp.delete.example", Port: 587, Username: "delete-user", Password: "delete-pass", FromAddress: "sender@delete.example"}},
	}}
	mapping := Mapping{
		Owner: OwnerMapping{Email: productionOwnerEmail, UserID: "tauth-user-alpha"},
		Tenants: []TenantMapping{
			{SourceTenantID: "tenant-alpha", APICredentialID: credentialID, APICredentialDigest: digest},
			{SourceTenantID: "tenant-delete", APICredentialID: secondCredentialID, APICredentialDigest: digest},
		},
		SMTPSenderDomains: []SenderDomainMapping{
			{ID: 1, Disposition: dispositionRetain, TargetSourceTenantID: "tenant-alpha"},
			{ID: 2, Disposition: dispositionDelete},
		},
		SMTPIdentities: []IdentityMapping{
			{ID: "identity-alpha", Disposition: dispositionRetain, TargetSourceTenantID: "tenant-alpha"},
			{ID: "identity-delete", Disposition: dispositionDelete},
		},
	}

	result, conversionErr := Convert(context.Background(), database, keeper, source, mapping)
	if conversionErr != nil {
		t.Fatalf("convert production fixture: %v", conversionErr)
	}
	if result != (Result{Tenants: 2, Notifications: 2, Attachments: 1, SMTPSenderDomains: 1, SMTPIdentities: 1, ForwardingRoutes: 1}) {
		t.Fatalf("unexpected conversion result %+v", result)
	}
	if schemaErr := db.ValidateManagedSchema(database); schemaErr != nil {
		t.Fatalf("validate converted schema: %v", schemaErr)
	}
	var managedTenant tenant.Tenant
	if lookupErr := database.Where(&tenant.Tenant{OwnerUserID: "tauth-user-alpha", DisplayName: "Alpha"}).First(&managedTenant).Error; lookupErr != nil {
		t.Fatalf("load managed tenant: %v", lookupErr)
	}
	if _, parseErr := tenant.NewTenantID(managedTenant.ID); parseErr != nil || managedTenant.ID == "tenant-alpha" {
		t.Fatalf("expected generated tenant UUID, got %q", managedTenant.ID)
	}
	repository := tenant.NewRepository(database, keeper)
	runtimeConfig, runtimeErr := repository.ResolveByID(context.Background(), managedTenant.ID)
	if runtimeErr != nil {
		t.Fatalf("resolve converted runtime: %v", runtimeErr)
	}
	if runtimeConfig.Email.Password != "alpha-pass" || runtimeConfig.SMS == nil || runtimeConfig.SMS.AuthToken != "alpha-token" {
		t.Fatalf("unexpected converted providers %+v", runtimeConfig)
	}
	var credential tenant.APICredential
	if lookupErr := database.Where(&tenant.APICredential{TenantID: managedTenant.ID}).First(&credential).Error; lookupErr != nil {
		t.Fatalf("load API credential: %v", lookupErr)
	}
	if credential.ID != credentialID || string(credential.SecretDigest) == "alpha-pass" {
		t.Fatalf("unexpected credential metadata id=%s", credential.ID)
	}
	var notification model.Notification
	if lookupErr := database.Where(&model.Notification{NotificationID: "notification-alpha", TenantID: managedTenant.ID}).First(&notification).Error; lookupErr != nil {
		t.Fatalf("load retained notification: %v", lookupErr)
	}
	var domain smtpidentity.SenderDomain
	if lookupErr := database.Where(&smtpidentity.SenderDomain{ID: 1, TenantID: managedTenant.ID}).First(&domain).Error; lookupErr != nil {
		t.Fatalf("load retained SMTP domain: %v", lookupErr)
	}
	var identity smtpidentity.Identity
	if lookupErr := database.Preload("ForwardTo").Where(&smtpidentity.Identity{ID: "identity-alpha", TenantID: managedTenant.ID}).First(&identity).Error; lookupErr != nil {
		t.Fatalf("load retained SMTP identity: %v", lookupErr)
	}
	if len(identity.ForwardTo) != 1 {
		t.Fatalf("expected retained forwarding route, got %+v", identity.ForwardTo)
	}
	for _, obsoleteTable := range []string{"tenant_domains", "tenant_admins", conversionTableName("tenants")} {
		if database.Migrator().HasTable(obsoleteTable) {
			t.Fatalf("obsolete table remains: %s", obsoleteTable)
		}
	}
}

func TestConvertAssignsAllSourceTenantsToProductionOwner(t *testing.T) {
	database := newLegacyDatabase(t)
	keeper := conversionKeeper(t)
	seedLegacyState(t, database, keeper, time.Now().UTC().Truncate(time.Second))
	digest := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
	source := SourceConfig{Tenants: []SourceTenant{
		{ID: "tenant-alpha", DisplayName: "Alpha", SupportEmail: "support@alpha.example", EmailProfile: SourceEmailProfile{Host: "smtp.alpha.example", Port: 587, Username: "alpha-user", Password: "alpha-pass", FromAddress: "sender@alpha.example"}},
		{ID: "tenant-delete", DisplayName: "Second", SupportEmail: "support@second.example", EmailProfile: SourceEmailProfile{Host: "smtp.second.example", Port: 587, Username: "second-user", Password: "second-pass", FromAddress: "sender@second.example"}},
	}}
	mapping := Mapping{
		Owner: OwnerMapping{Email: productionOwnerEmail, UserID: "google:production-owner"},
		Tenants: []TenantMapping{
			{SourceTenantID: "tenant-alpha", APICredentialID: uuid.NewString(), APICredentialDigest: digest},
			{SourceTenantID: "tenant-delete", APICredentialID: uuid.NewString(), APICredentialDigest: digest},
		},
		SMTPSenderDomains: []SenderDomainMapping{
			{ID: 1, Disposition: dispositionRetain, TargetSourceTenantID: "tenant-alpha"},
			{ID: 2, Disposition: dispositionRetain, TargetSourceTenantID: "tenant-delete"},
		},
		SMTPIdentities: []IdentityMapping{
			{ID: "identity-alpha", Disposition: dispositionRetain, TargetSourceTenantID: "tenant-alpha"},
			{ID: "identity-delete", Disposition: dispositionRetain, TargetSourceTenantID: "tenant-delete"},
		},
	}

	result, conversionErr := Convert(context.Background(), database, keeper, source, mapping)
	if conversionErr != nil {
		t.Fatalf("convert all source tenants: %v", conversionErr)
	}
	if result != (Result{Tenants: 2, Notifications: 2, Attachments: 1, SMTPSenderDomains: 2, SMTPIdentities: 2, ForwardingRoutes: 2}) {
		t.Fatalf("unexpected conversion result %+v", result)
	}
	var ownerTenantCount int64
	if countErr := database.Model(&tenant.Tenant{}).Where(&tenant.Tenant{OwnerUserID: "google:production-owner"}).Count(&ownerTenantCount).Error; countErr != nil || ownerTenantCount != 2 {
		t.Fatalf("production owner tenant count=%d err=%v", ownerTenantCount, countErr)
	}
}

func TestConvertRejectsIncompleteInputsBeforeSchemaChanges(t *testing.T) {
	database := newLegacyDatabase(t)
	keeper := conversionKeeper(t)
	seedLegacyState(t, database, keeper, time.Now().UTC())
	source := SourceConfig{Tenants: []SourceTenant{{ID: "tenant-alpha"}, {ID: "tenant-delete"}}}
	_, conversionErr := Convert(context.Background(), database, keeper, source, Mapping{})
	if conversionErr == nil {
		t.Fatal("expected incomplete mapping rejection")
	}
	if !database.Migrator().HasTable(&legacyTenantDomain{}) || database.Migrator().HasTable(&tenant.APICredential{}) {
		t.Fatal("conversion changed schema before validation completed")
	}
}

func TestStrictConversionYAML(t *testing.T) {
	source, sourceErr := DecodeSource([]byte("tenants:\n  - id: tenant-alpha\n    displayName: Alpha\n    supportEmail: ''\n    enabled: true\n    domains: []\n    admins: []\n    emailProfile:\n      host: smtp.example\n      port: 587\n      username: user\n      password: pass\n      fromAddress: sender@example.com\n"))
	if sourceErr != nil || len(source.Tenants) != 1 {
		t.Fatalf("decode source: %+v err=%v", source, sourceErr)
	}
	if _, sourceErr := DecodeSource([]byte("tenants: []\nunknown: true\n")); sourceErr == nil {
		t.Fatal("expected unknown source key rejection")
	}
	mapping, mappingErr := DecodeMapping([]byte("owner:\n  email: temirov@gmail.com\n  userId: google:subject\ntenants: []\nsmtpSenderDomains: []\nsmtpIdentities: []\n"))
	if mappingErr != nil || mapping.Tenants == nil || mapping.Owner.Email != productionOwnerEmail {
		t.Fatalf("decode mapping: %+v err=%v", mapping, mappingErr)
	}
	if _, mappingErr := DecodeMapping([]byte("tenants: []\nunsupported: true\n")); mappingErr == nil {
		t.Fatal("expected unknown mapping key rejection")
	}
	if _, mappingErr := DecodeMapping([]byte("owner:\n  email: temirov@gmail.com\n  userId: google:subject\ntenants:\n  - sourceTenantId: tenant-alpha\n    disposition: retain\n    apiCredentialId: 00000000-0000-4000-8000-000000000001\n    apiCredentialDigest: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\nsmtpSenderDomains: []\nsmtpIdentities: []\n")); mappingErr == nil {
		t.Fatal("expected obsolete tenant disposition rejection")
	}
}

func newLegacyDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, openErr := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	if openErr != nil {
		t.Fatalf("open legacy database: %v", openErr)
	}
	if migrateErr := database.AutoMigrate(
		&legacyTenant{}, &legacyTenantDomain{}, &legacyTenantAdmin{}, &legacyEmailProfile{}, &legacySMSProfile{},
		&model.Notification{}, &model.NotificationAttachment{}, &legacySenderDomain{}, &legacyIdentity{}, &smtpidentity.ForwardRecipient{},
	); migrateErr != nil {
		t.Fatalf("create legacy schema: %v", migrateErr)
	}
	return database
}

func conversionKeeper(t *testing.T) *tenant.SecretKeeper {
	t.Helper()
	keeper, keeperErr := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if keeperErr != nil {
		t.Fatalf("new secret keeper: %v", keeperErr)
	}
	return keeper
}

func seedLegacyState(t *testing.T, database *gorm.DB, keeper *tenant.SecretKeeper, now time.Time) {
	t.Helper()
	for _, tenantRow := range []legacyTenant{
		{ID: "tenant-alpha", DisplayName: "Alpha", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "tenant-delete", DisplayName: "Delete", Status: "active", CreatedAt: now, UpdatedAt: now},
	} {
		if createErr := database.Create(&tenantRow).Error; createErr != nil {
			t.Fatalf("seed tenant: %v", createErr)
		}
	}
	usernameCipher, _ := keeper.Encrypt("old-user")
	passwordCipher, _ := keeper.Encrypt("old-pass")
	for _, sourceID := range []string{"tenant-alpha", "tenant-delete"} {
		if createErr := database.Create(&legacyTenantDomain{TenantID: sourceID, Host: sourceID + ".example", IsDefault: true, CreatedAt: now, UpdatedAt: now}).Error; createErr != nil {
			t.Fatalf("seed domain: %v", createErr)
		}
		if createErr := database.Create(&legacyTenantAdmin{TenantID: sourceID, Email: sourceID + "@example.com", CreatedAt: now, UpdatedAt: now}).Error; createErr != nil {
			t.Fatalf("seed admin: %v", createErr)
		}
		if createErr := database.Create(&legacyEmailProfile{ID: uuid.NewString(), TenantID: sourceID, Host: "old.example", Port: 587, UsernameCipher: usernameCipher, PasswordCipher: passwordCipher, FromAddress: "old@example.com", IsDefault: true, CreatedAt: now, UpdatedAt: now}).Error; createErr != nil {
			t.Fatalf("seed email profile: %v", createErr)
		}
	}
	notifications := []model.Notification{
		{TenantID: "tenant-alpha", NotificationID: "notification-alpha", NotificationType: model.NotificationEmail, Recipient: "alpha@example.com", Message: "alpha", Status: model.StatusSent, CreatedAt: now, UpdatedAt: now},
		{TenantID: "tenant-delete", NotificationID: "notification-delete", NotificationType: model.NotificationEmail, Recipient: "delete@example.com", Message: "delete", Status: model.StatusSent, CreatedAt: now, UpdatedAt: now},
	}
	for index := range notifications {
		if createErr := database.Create(&notifications[index]).Error; createErr != nil {
			t.Fatalf("seed notification: %v", createErr)
		}
	}
	if createErr := database.Create(&model.NotificationAttachment{TenantID: "tenant-alpha", NotificationID: "notification-alpha", Filename: "fixture.txt", ContentType: "text/plain", Data: []byte("fixture"), CreatedAt: now, UpdatedAt: now}).Error; createErr != nil {
		t.Fatalf("seed attachment: %v", createErr)
	}
	for _, domain := range []legacySenderDomain{
		{ID: 1, OwnerEmail: "alpha@example.com", Domain: "alpha.example", Status: smtpidentity.SenderDomainStatusVerified, VerificationToken: "alpha-token", CreatedAt: now, UpdatedAt: now},
		{ID: 2, OwnerEmail: "delete@example.com", Domain: "delete.example", Status: smtpidentity.SenderDomainStatusVerified, VerificationToken: "delete-token", CreatedAt: now, UpdatedAt: now},
	} {
		if createErr := database.Create(&domain).Error; createErr != nil {
			t.Fatalf("seed SMTP domain: %v", createErr)
		}
	}
	for _, identity := range []legacyIdentity{
		{ID: "identity-alpha", OwnerEmail: "alpha@example.com", EmailAddress: "sender@alpha.example", Username: "smtp-alpha", PasswordSalt: []byte("salt-alpha"), PasswordDigest: []byte("digest-alpha"), PasswordCipher: []byte("cipher-alpha"), Status: smtpidentity.IdentityStatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "identity-delete", OwnerEmail: "delete@example.com", EmailAddress: "sender@delete.example", Username: "smtp-delete", PasswordSalt: []byte("salt-delete"), PasswordDigest: []byte("digest-delete"), PasswordCipher: []byte("cipher-delete"), Status: smtpidentity.IdentityStatusActive, CreatedAt: now, UpdatedAt: now},
	} {
		if createErr := database.Create(&identity).Error; createErr != nil {
			t.Fatalf("seed SMTP identity: %v", createErr)
		}
	}
	for _, route := range []smtpidentity.ForwardRecipient{
		{ID: "route-alpha", IdentityID: "identity-alpha", EmailAddress: "owner@alpha.example", CreatedAt: now, UpdatedAt: now},
		{ID: "route-delete", IdentityID: "identity-delete", EmailAddress: "owner@delete.example", CreatedAt: now, UpdatedAt: now},
	} {
		if createErr := database.Create(&route).Error; createErr != nil {
			t.Fatalf("seed forwarding route: %v", createErr)
		}
	}
}
