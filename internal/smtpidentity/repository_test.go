package smtpidentity

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
)

func TestRepositoryCreateAuthenticateRotateAndDelete(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "tenant-one", "example.com")
	address := mustAddress(t, "alice@example.com")

	identity, password, createErr := repository.Create(context.Background(), "tenant-one", address)
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if password == "" {
		t.Fatalf("expected one-time password")
	}
	if identity.EmailAddress != "alice@example.com" || identity.Username == "" {
		t.Fatalf("unexpected identity: %+v", identity)
	}

	authenticated, authErr := repository.Authenticate(context.Background(), identity.Username, password)
	if authErr != nil {
		t.Fatalf("authenticate identity: %v", authErr)
	}
	if authenticated.EmailAddress.String() != "alice@example.com" {
		t.Fatalf("unexpected authenticated sender %s", authenticated.EmailAddress.String())
	}

	rotatedIdentity, rotatedPassword, rotateErr := repository.Rotate(context.Background(), "tenant-one", identity.ID)
	if rotateErr != nil {
		t.Fatalf("rotate identity: %v", rotateErr)
	}
	if rotatedPassword == password || rotatedIdentity.Username == identity.Username {
		t.Fatalf("expected rotated credentials to change")
	}
	if _, oldAuthErr := repository.Authenticate(context.Background(), identity.Username, password); !errors.Is(oldAuthErr, ErrAuthenticationFailed) {
		t.Fatalf("expected old credentials to fail, got %v", oldAuthErr)
	}
	if _, newAuthErr := repository.Authenticate(context.Background(), rotatedIdentity.Username, rotatedPassword); newAuthErr != nil {
		t.Fatalf("expected rotated credentials to authenticate: %v", newAuthErr)
	}

	if deleteErr := repository.Delete(context.Background(), "tenant-one", identity.ID); deleteErr != nil {
		t.Fatalf("delete identity: %v", deleteErr)
	}
	if _, deletedAuthErr := repository.Authenticate(context.Background(), rotatedIdentity.Username, rotatedPassword); !errors.Is(deletedAuthErr, ErrAuthenticationFailed) {
		t.Fatalf("expected deleted identity auth failure, got %v", deletedAuthErr)
	}
}

func TestRepositoryRejectsAddressOutsideSenderDomains(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "tenant-one", "example.com")
	address := mustAddress(t, "alice@other.example")

	_, _, createErr := repository.Create(context.Background(), "tenant-one", address)
	if !errors.Is(createErr, ErrSenderDomainNotAllowed) {
		t.Fatalf("expected sender domain error, got %v", createErr)
	}
}

func TestRepositoryRejectsDuplicateActiveIdentity(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "tenant-one", "example.com")
	address := mustAddress(t, "alice@example.com")

	if _, _, createErr := repository.Create(context.Background(), "tenant-one", address); createErr != nil {
		t.Fatalf("create first identity: %v", createErr)
	}
	if _, _, createErr := repository.Create(context.Background(), "tenant-one", address); !errors.Is(createErr, ErrIdentityExists) {
		t.Fatalf("expected duplicate identity error, got %v", createErr)
	}
}

func TestRepositoryListNeverReturnsPasswords(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "tenant-one", "example.com")
	address := mustAddress(t, "alice@example.com")
	identity, password, createErr := repository.Create(context.Background(), "tenant-one", address)
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}

	identities, listErr := repository.List(context.Background(), "tenant-one")
	if listErr != nil {
		t.Fatalf("list identities: %v", listErr)
	}
	if len(identities) != 1 {
		t.Fatalf("expected one identity, got %d", len(identities))
	}
	if identities[0].ID != identity.ID || identities[0].Username == "" {
		t.Fatalf("unexpected listed identity: %+v", identities[0])
	}
	if strings.Contains(fmtPublicIdentity(identities[0]), password) {
		t.Fatalf("listed identity leaked password")
	}
}

func newIdentityRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	database, databaseErr := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "identity.db")), &gorm.Config{})
	if databaseErr != nil {
		t.Fatalf("open database: %v", databaseErr)
	}
	if migrateErr := database.AutoMigrate(&tenant.SenderDomain{}, &Identity{}); migrateErr != nil {
		t.Fatalf("migrate database: %v", migrateErr)
	}
	repository, repositoryErr := NewRepository(database, strings.Repeat("a", 64))
	if repositoryErr != nil {
		t.Fatalf("new repository: %v", repositoryErr)
	}
	return repository, database
}

func seedSenderDomain(t *testing.T, database *gorm.DB, tenantID string, domain string) {
	t.Helper()
	if err := database.Create(&tenant.SenderDomain{TenantID: tenantID, Domain: domain}).Error; err != nil {
		t.Fatalf("seed sender domain: %v", err)
	}
}

func mustAddress(t *testing.T, rawAddress string) Address {
	t.Helper()
	address, addressErr := NewAddress(rawAddress)
	if addressErr != nil {
		t.Fatalf("new address: %v", addressErr)
	}
	return address
}

func fmtPublicIdentity(identity PublicIdentity) string {
	return identity.ID + identity.TenantID + identity.EmailAddress + identity.Username + identity.Status
}
