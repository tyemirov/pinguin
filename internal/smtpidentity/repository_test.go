package smtpidentity

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryCreateAuthenticateRotateAndDelete(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@example.com")

	identity, password, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if password == "" {
		t.Fatalf("expected generated password")
	}
	if identity.EmailAddress != "alice@example.com" || identity.Username == "" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	if len(identity.ForwardTo) != 1 || identity.ForwardTo[0] != "owner@example.com" {
		t.Fatalf("unexpected forwarding recipients: %+v", identity.ForwardTo)
	}
	retrievedIdentity, retrievedPassword, credentialsErr := repository.Credentials(context.Background(), identity.ID)
	if credentialsErr != nil {
		t.Fatalf("retrieve identity credentials: %v", credentialsErr)
	}
	if retrievedIdentity.ID != identity.ID || retrievedPassword != password {
		t.Fatalf("unexpected retrieved credentials identity=%+v password=%s", retrievedIdentity, retrievedPassword)
	}
	var storedIdentity Identity
	if fetchErr := database.Where(&Identity{ID: identity.ID}).First(&storedIdentity).Error; fetchErr != nil {
		t.Fatalf("fetch stored identity: %v", fetchErr)
	}
	if len(storedIdentity.PasswordCipher) == 0 || bytes.Contains(storedIdentity.PasswordCipher, []byte(password)) {
		t.Fatalf("expected encrypted stored password, got %q", string(storedIdentity.PasswordCipher))
	}

	authenticated, authErr := repository.Authenticate(context.Background(), identity.Username, password)
	if authErr != nil {
		t.Fatalf("authenticate identity: %v", authErr)
	}
	if authenticated.EmailAddress.String() != "alice@example.com" {
		t.Fatalf("unexpected authenticated sender %s", authenticated.EmailAddress.String())
	}

	rotatedIdentity, rotatedPassword, rotateErr := repository.Rotate(context.Background(), identity.ID)
	if rotateErr != nil {
		t.Fatalf("rotate identity: %v", rotateErr)
	}
	if rotatedPassword == password || rotatedIdentity.Username == identity.Username {
		t.Fatalf("expected rotated credentials to change")
	}
	if _, oldAuthErr := repository.Authenticate(context.Background(), identity.Username, password); !errors.Is(oldAuthErr, ErrAuthenticationFailed) {
		t.Fatalf("expected old credentials to fail, got %v", oldAuthErr)
	}
	_, retrievedRotatedPassword, rotatedCredentialsErr := repository.Credentials(context.Background(), identity.ID)
	if rotatedCredentialsErr != nil {
		t.Fatalf("retrieve rotated credentials: %v", rotatedCredentialsErr)
	}
	if retrievedRotatedPassword != rotatedPassword {
		t.Fatalf("expected retrievable rotated password, got %s", retrievedRotatedPassword)
	}
	if _, newAuthErr := repository.Authenticate(context.Background(), rotatedIdentity.Username, rotatedPassword); newAuthErr != nil {
		t.Fatalf("expected rotated credentials to authenticate: %v", newAuthErr)
	}

	if deleteErr := repository.Delete(context.Background(), identity.ID); deleteErr != nil {
		t.Fatalf("delete identity: %v", deleteErr)
	}
	if _, deletedAuthErr := repository.Authenticate(context.Background(), rotatedIdentity.Username, rotatedPassword); !errors.Is(deletedAuthErr, ErrAuthenticationFailed) {
		t.Fatalf("expected deleted identity auth failure, got %v", deletedAuthErr)
	}
}

func TestRepositoryAuthenticateDoesNotRestoreIdentityDeletedDuringAuth(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@example.com")

	identity, password, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}

	deleteDuringAuth := true
	var deleteErr error
	repository.clockFunc = func() time.Time {
		now := time.Now().UTC()
		if deleteDuringAuth {
			deleteDuringAuth = false
			deleteErr = database.Model(&Identity{}).
				Where(&Identity{ID: identity.ID}).
				Updates(map[string]interface{}{
					statusColumn:    IdentityStatusDeleted,
					updatedAtColumn: now,
				}).Error
		}
		return now
	}

	_, authErr := repository.Authenticate(context.Background(), identity.Username, password)
	if deleteErr != nil {
		t.Fatalf("delete during auth: %v", deleteErr)
	}
	if !errors.Is(authErr, ErrAuthenticationFailed) {
		t.Fatalf("expected auth failure after concurrent delete, got %v", authErr)
	}

	var storedIdentity Identity
	if fetchErr := database.Where(&Identity{ID: identity.ID}).First(&storedIdentity).Error; fetchErr != nil {
		t.Fatalf("fetch identity: %v", fetchErr)
	}
	if storedIdentity.Status != IdentityStatusDeleted {
		t.Fatalf("expected identity to remain deleted, got %s", storedIdentity.Status)
	}
}

func TestRepositoryRejectsAddressOutsideSenderDomains(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@other.example")

	_, _, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
	if !errors.Is(createErr, ErrSenderDomainNotAllowed) {
		t.Fatalf("expected sender domain error, got %v", createErr)
	}
}

func TestRepositoryVerifiedSenderDomainScopeControlsIdentityUse(t *testing.T) {
	repository, _ := newIdentityRepository(t)
	scope := AccessScope{OwnerEmail: "member@example.com"}
	otherScope := AccessScope{OwnerEmail: "other@example.com"}
	domain, domainErr := repository.CreateSenderDomainForScope(context.Background(), scope, "Customer.Example")
	if domainErr != nil {
		t.Fatalf("create sender domain: %v", domainErr)
	}
	if domain.Status != SenderDomainStatusPending || domain.OwnerEmail != "member@example.com" {
		t.Fatalf("unexpected pending sender domain: %+v", domain)
	}

	address := mustAddress(t, "alice@customer.example")
	if _, _, createErr := repository.CreateForScope(context.Background(), scope, address, defaultForwardRecipients(t)); !errors.Is(createErr, ErrSenderDomainNotAllowed) {
		t.Fatalf("expected unverified sender domain to block identity create, got %v", createErr)
	}
	verifiedAt := time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC)
	if _, updateErr := repository.UpdateSenderDomainStatusForScope(context.Background(), scope, domain.ID, SenderDomainStatusVerified, verifiedAt); updateErr != nil {
		t.Fatalf("verify sender domain: %v", updateErr)
	}
	identity, password, createErr := repository.CreateForScope(context.Background(), scope, address, defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity after verification: %v", createErr)
	}
	visibleIdentities, listErr := repository.ListForScope(context.Background(), scope)
	if listErr != nil {
		t.Fatalf("list owner identities: %v", listErr)
	}
	if len(visibleIdentities) != 1 || visibleIdentities[0].ID != identity.ID {
		t.Fatalf("expected owner to see created identity, got %+v", visibleIdentities)
	}
	otherVisibleIdentities, otherListErr := repository.ListForScope(context.Background(), otherScope)
	if otherListErr != nil {
		t.Fatalf("list other identities: %v", otherListErr)
	}
	if len(otherVisibleIdentities) != 0 {
		t.Fatalf("expected other owner isolation, got %+v", otherVisibleIdentities)
	}
	if _, _, credentialsErr := repository.CredentialsForScope(context.Background(), otherScope, identity.ID); !errors.Is(credentialsErr, ErrIdentityNotFound) {
		t.Fatalf("expected other owner credentials lookup to be hidden, got %v", credentialsErr)
	}
	if _, authErr := repository.Authenticate(context.Background(), identity.Username, password); authErr != nil {
		t.Fatalf("authenticate verified identity: %v", authErr)
	}
	if _, updateErr := repository.UpdateSenderDomainStatusForScope(context.Background(), scope, domain.ID, SenderDomainStatusPending, verifiedAt.Add(time.Hour)); updateErr != nil {
		t.Fatalf("unverify sender domain: %v", updateErr)
	}
	if _, authErr := repository.Authenticate(context.Background(), identity.Username, password); !errors.Is(authErr, ErrAuthenticationFailed) {
		t.Fatalf("expected pending sender domain to disable SMTP auth, got %v", authErr)
	}
	if _, _, exists, resolveErr := repository.ResolveForwarding(context.Background(), address); resolveErr != nil || exists {
		t.Fatalf("expected pending sender domain to disable forwarding route, exists=%v err=%v", exists, resolveErr)
	}
}

func TestRepositoryRejectsSenderDomainClaimedByAnotherOwner(t *testing.T) {
	repository, _ := newIdentityRepository(t)
	if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example"); domainErr != nil {
		t.Fatalf("create sender domain: %v", domainErr)
	}
	if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "other@example.com"}, "customer.example"); !errors.Is(domainErr, ErrSenderDomainExists) {
		t.Fatalf("expected sender domain ownership conflict, got %v", domainErr)
	}
}

func TestRepositorySenderDomainSetupErrorPaths(t *testing.T) {
	t.Run("idempotent owner create fills missing token", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		if err := database.Create(&SenderDomain{OwnerEmail: "member@example.com", Domain: "customer.example", Status: SenderDomainStatusPending}).Error; err != nil {
			t.Fatalf("seed sender domain: %v", err)
		}
		domain, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example")
		if domainErr != nil {
			t.Fatalf("idempotent domain create: %v", domainErr)
		}
		if domain.VerificationToken == "" {
			t.Fatalf("expected missing token to be filled")
		}
	})
	t.Run("admin can read existing owner domain", func(t *testing.T) {
		repository, _ := newIdentityRepository(t)
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example"); domainErr != nil {
			t.Fatalf("create sender domain: %v", domainErr)
		}
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "admin@example.com", Admin: true}, "customer.example"); domainErr != nil {
			t.Fatalf("admin read sender domain: %v", domainErr)
		}
	})
	t.Run("invalid domain", func(t *testing.T) {
		repository, _ := newIdentityRepository(t)
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "bad domain"); !errors.Is(domainErr, ErrInvalidSenderDomain) {
			t.Fatalf("expected invalid domain, got %v", domainErr)
		}
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, " "); !errors.Is(domainErr, ErrInvalidSenderDomain) {
			t.Fatalf("expected blank domain, got %v", domainErr)
		}
	})
	t.Run("random token failure", func(t *testing.T) {
		repository, _ := newIdentityRepository(t)
		repository.random = identityFailingReader{err: io.ErrUnexpectedEOF}
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example"); !errors.Is(domainErr, io.ErrUnexpectedEOF) {
			t.Fatalf("expected token failure, got %v", domainErr)
		}
	})
	t.Run("create storage failure", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		if dropErr := database.Migrator().DropTable(&SenderDomain{}); dropErr != nil {
			t.Fatalf("drop sender domains: %v", dropErr)
		}
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example"); domainErr == nil || !strings.Contains(domainErr.Error(), "lookup") {
			t.Fatalf("expected lookup storage failure, got %v", domainErr)
		}
	})
	t.Run("create save failure", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		registerSenderDomainCreateError(t, database)
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example"); domainErr == nil || !strings.Contains(domainErr.Error(), "create") {
			t.Fatalf("expected create storage failure, got %v", domainErr)
		}
	})
	t.Run("missing token save failure", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		if err := database.Create(&SenderDomain{OwnerEmail: "member@example.com", Domain: "customer.example", Status: SenderDomainStatusPending}).Error; err != nil {
			t.Fatalf("seed sender domain: %v", err)
		}
		registerSenderDomainUpdateError(t, database)
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example"); domainErr == nil || !strings.Contains(domainErr.Error(), "token") {
			t.Fatalf("expected token storage failure, got %v", domainErr)
		}
	})
	t.Run("missing token random failure", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		if err := database.Create(&SenderDomain{OwnerEmail: "member@example.com", Domain: "customer.example", Status: SenderDomainStatusPending}).Error; err != nil {
			t.Fatalf("seed sender domain: %v", err)
		}
		repository.random = identityFailingReader{err: io.ErrUnexpectedEOF}
		if _, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example"); !errors.Is(domainErr, io.ErrUnexpectedEOF) {
			t.Fatalf("expected missing token random failure, got %v", domainErr)
		}
	})
	t.Run("list and lookup storage failures", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		closeIdentityDatabase(t, database)
		if _, listErr := repository.ListSenderDomainsForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}); listErr == nil {
			t.Fatalf("expected list storage failure")
		}
		if _, lookupErr := repository.RequireSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, 1); lookupErr == nil {
			t.Fatalf("expected lookup storage failure")
		}
	})
	t.Run("not found and update fetch failure", func(t *testing.T) {
		repository, _ := newIdentityRepository(t)
		if _, lookupErr := repository.RequireSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, 404); !errors.Is(lookupErr, ErrSenderDomainNotFound) {
			t.Fatalf("expected missing sender domain, got %v", lookupErr)
		}
		if _, updateErr := repository.UpdateSenderDomainStatusForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, 404, SenderDomainStatusVerified, time.Now().UTC()); !errors.Is(updateErr, ErrSenderDomainNotFound) {
			t.Fatalf("expected missing update sender domain, got %v", updateErr)
		}
	})
	t.Run("update save failure", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		domain, domainErr := repository.CreateSenderDomainForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, "customer.example")
		if domainErr != nil {
			t.Fatalf("create sender domain: %v", domainErr)
		}
		registerSenderDomainUpdateError(t, database)
		if _, updateErr := repository.UpdateSenderDomainStatusForScope(context.Background(), AccessScope{OwnerEmail: "member@example.com"}, domain.ID, SenderDomainStatusVerified, time.Now().UTC()); updateErr == nil || !strings.Contains(updateErr.Error(), "status") {
			t.Fatalf("expected status save failure, got %v", updateErr)
		}
	})
}

func TestRepositoryCreatePreservesSenderDomainStorageFailure(t *testing.T) {
	repository, database := newIdentityRepository(t)
	sqlDatabase, sqlErr := database.DB()
	if sqlErr != nil {
		t.Fatalf("database handle: %v", sqlErr)
	}
	if closeErr := sqlDatabase.Close(); closeErr != nil {
		t.Fatalf("close database: %v", closeErr)
	}

	_, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
	if createErr == nil {
		t.Fatalf("expected storage failure")
	}
	if errors.Is(createErr, ErrSenderDomainNotAllowed) {
		t.Fatalf("expected storage failure to remain distinct from sender-domain policy, got %v", createErr)
	}
}

func TestRepositoryRequiresSenderDomainStorageForSMTPUse(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@example.com")
	identity, password, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if dropErr := database.Migrator().DropTable(&SenderDomain{}); dropErr != nil {
		t.Fatalf("drop sender domains: %v", dropErr)
	}
	if _, authErr := repository.Authenticate(context.Background(), identity.Username, password); authErr == nil || errors.Is(authErr, ErrAuthenticationFailed) {
		t.Fatalf("expected sender domain storage auth failure, got %v", authErr)
	}
	if _, _, exists, resolveErr := repository.ResolveForwarding(context.Background(), address); resolveErr == nil || exists {
		t.Fatalf("expected sender domain storage forwarding failure, exists=%v err=%v", exists, resolveErr)
	}
}

func TestRepositoryRejectsDuplicateActiveIdentity(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@example.com")

	if _, _, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t)); createErr != nil {
		t.Fatalf("create first identity: %v", createErr)
	}
	if _, _, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t)); !errors.Is(createErr, ErrIdentityExists) {
		t.Fatalf("expected duplicate identity error, got %v", createErr)
	}
}

func TestRepositoryReportsInvalidInitialization(t *testing.T) {
	if _, err := NewRepository(nil, strings.Repeat("a", 64)); err == nil {
		t.Fatalf("expected nil database error")
	}
	_, database := newIdentityRepository(t)
	if _, err := NewRepository(database, "short"); err == nil {
		t.Fatalf("expected invalid key error")
	}
	if _, err := NewRepository(database, strings.Repeat("a", 62)); err == nil {
		t.Fatalf("expected invalid decoded key length error")
	}
}

func TestRepositoryRotateReportsMissingIdentityAsNotFound(t *testing.T) {
	repository, _ := newIdentityRepository(t)

	_, _, rotateErr := repository.Rotate(context.Background(), "missing-identity")
	if !errors.Is(rotateErr, ErrIdentityNotFound) {
		t.Fatalf("expected identity not found, got %v", rotateErr)
	}
	_, _, credentialsErr := repository.Credentials(context.Background(), "missing-identity")
	if !errors.Is(credentialsErr, ErrIdentityNotFound) {
		t.Fatalf("expected credentials identity not found, got %v", credentialsErr)
	}
}

func TestRepositoryRotatePreservesIdentityLookupStorageFailure(t *testing.T) {
	repository, database := newIdentityRepository(t)
	sqlDatabase, sqlErr := database.DB()
	if sqlErr != nil {
		t.Fatalf("database handle: %v", sqlErr)
	}
	if closeErr := sqlDatabase.Close(); closeErr != nil {
		t.Fatalf("close database: %v", closeErr)
	}

	_, _, rotateErr := repository.Rotate(context.Background(), "identity-one")
	if rotateErr == nil {
		t.Fatalf("expected storage failure")
	}
	if errors.Is(rotateErr, ErrIdentityNotFound) {
		t.Fatalf("expected storage failure to remain distinct from not found, got %v", rotateErr)
	}
}

func TestRepositoryListNeverReturnsPasswords(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@example.com")
	identity, password, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}

	identities, listErr := repository.List(context.Background())
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

func TestRepositoryListReportsStorageFailure(t *testing.T) {
	repository, database := newIdentityRepository(t)
	closeIdentityDatabase(t, database)

	if _, listErr := repository.List(context.Background()); listErr == nil {
		t.Fatalf("expected list storage failure")
	}
}

func TestRepositoryReactivatesDeletedIdentity(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@example.com")
	identity, password, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if deleteErr := repository.Delete(context.Background(), identity.ID); deleteErr != nil {
		t.Fatalf("delete identity: %v", deleteErr)
	}

	reactivated, newPassword, reactivateErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
	if reactivateErr != nil {
		t.Fatalf("reactivate identity: %v", reactivateErr)
	}
	if reactivated.ID != identity.ID {
		t.Fatalf("expected identity id to be reused, got %s", reactivated.ID)
	}
	if newPassword == password || reactivated.Username == identity.Username {
		t.Fatalf("expected reactivated credentials to rotate")
	}
	if _, authErr := repository.Authenticate(context.Background(), reactivated.Username, newPassword); authErr != nil {
		t.Fatalf("authenticate reactivated identity: %v", authErr)
	}
}

func TestRepositoryForwardingLifecycle(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "support@example.com")
	firstOwner := mustAddress(t, "owner@example.com")
	secondOwner := mustAddress(t, "maria@example.com")
	identity, _, createErr := repository.Create(context.Background(), address, []Address{secondOwner, firstOwner})
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if strings.Join(identity.ForwardTo, ",") != "maria@example.com,owner@example.com" {
		t.Fatalf("expected sorted forwarding recipients, got %+v", identity.ForwardTo)
	}

	routeAddress, recipients, exists, resolveErr := repository.ResolveForwarding(context.Background(), address)
	if resolveErr != nil || !exists || routeAddress.String() != address.String() {
		t.Fatalf("expected forwarding route, exists=%v address=%s err=%v", exists, routeAddress.String(), resolveErr)
	}
	if len(recipients) != 2 || recipients[0].String() != "maria@example.com" || recipients[1].String() != "owner@example.com" {
		t.Fatalf("unexpected resolved recipients: %+v", recipients)
	}

	updated, updateErr := repository.UpdateForwarding(context.Background(), identity.ID, []Address{firstOwner})
	if updateErr != nil {
		t.Fatalf("update forwarding: %v", updateErr)
	}
	if strings.Join(updated.ForwardTo, ",") != "owner@example.com" {
		t.Fatalf("unexpected updated forwarding recipients: %+v", updated.ForwardTo)
	}
	if _, recipients, exists, resolveErr := repository.ResolveForwarding(context.Background(), address); resolveErr != nil || !exists || len(recipients) != 1 {
		t.Fatalf("expected updated forwarding route, exists=%v recipients=%+v err=%v", exists, recipients, resolveErr)
	}

	if deleteErr := repository.Delete(context.Background(), identity.ID); deleteErr != nil {
		t.Fatalf("delete identity: %v", deleteErr)
	}
	if _, _, exists, resolveErr := repository.ResolveForwarding(context.Background(), address); resolveErr != nil || exists {
		t.Fatalf("expected deleted identity to stop resolving, exists=%v err=%v", exists, resolveErr)
	}
}

func TestRepositoryRejectsInvalidForwardingRecipients(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	address := mustAddress(t, "alice@example.com")
	owner := mustAddress(t, "owner@example.com")
	if _, _, createErr := repository.Create(context.Background(), address, nil); !errors.Is(createErr, ErrForwardRecipientsRequired) {
		t.Fatalf("expected create missing forwarding error, got %v", createErr)
	}
	if _, _, createErr := repository.Create(context.Background(), address, []Address{{}}); !errors.Is(createErr, ErrForwardRecipientsRequired) {
		t.Fatalf("expected create empty forwarding error, got %v", createErr)
	}
	if _, _, createErr := repository.Create(context.Background(), address, []Address{owner, owner}); !errors.Is(createErr, ErrForwardRecipientDuplicate) {
		t.Fatalf("expected create duplicate forwarding error, got %v", createErr)
	}
	if _, _, createErr := repository.Create(context.Background(), address, []Address{address}); !errors.Is(createErr, ErrForwardRecipientSelf) {
		t.Fatalf("expected create self forwarding error, got %v", createErr)
	}
	identity, _, createErr := repository.Create(context.Background(), address, []Address{owner})
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if _, updateErr := repository.UpdateForwarding(context.Background(), identity.ID, nil); !errors.Is(updateErr, ErrForwardRecipientsRequired) {
		t.Fatalf("expected update missing forwarding error, got %v", updateErr)
	}
	if _, updateErr := repository.UpdateForwarding(context.Background(), identity.ID, []Address{owner, owner}); !errors.Is(updateErr, ErrForwardRecipientDuplicate) {
		t.Fatalf("expected update duplicate forwarding error, got %v", updateErr)
	}
	if _, updateErr := repository.UpdateForwarding(context.Background(), identity.ID, []Address{address}); !errors.Is(updateErr, ErrForwardRecipientSelf) {
		t.Fatalf("expected update self forwarding error, got %v", updateErr)
	}
}

func TestRepositoryCreateReportsCredentialFailures(t *testing.T) {
	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{name: "username token", reader: identityFailingReader{err: io.ErrUnexpectedEOF}},
		{name: "password token", reader: io.MultiReader(bytes.NewReader(make([]byte, credentialUsernameBytes)), identityFailingReader{err: io.ErrUnexpectedEOF})},
		{name: "salt", reader: io.MultiReader(bytes.NewReader(make([]byte, credentialUsernameBytes+credentialPasswordBytes)), identityFailingReader{err: io.ErrUnexpectedEOF})},
		{name: "password cipher nonce", reader: io.MultiReader(bytes.NewReader(make([]byte, credentialUsernameBytes+credentialPasswordBytes+credentialSaltBytes)), identityFailingReader{err: io.ErrUnexpectedEOF})},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			repository, database := newIdentityRepository(t)
			seedSenderDomain(t, database, "example.com")
			repository.random = testCase.reader
			_, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
			if createErr == nil || !errors.Is(createErr, io.ErrUnexpectedEOF) {
				t.Fatalf("expected credential read failure, got %v", createErr)
			}
		})
	}
	t.Run("password cipher key", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		repository.key = []byte("short")
		_, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr == nil || !strings.Contains(createErr.Error(), "password cipher") {
			t.Fatalf("expected password cipher failure, got %v", createErr)
		}
	})
}

func TestRepositoryCreateReportsIdentityStorageFailures(t *testing.T) {
	t.Run("find existing", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		if dropErr := database.Migrator().DropTable(&Identity{}); dropErr != nil {
			t.Fatalf("drop identities: %v", dropErr)
		}
		_, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr == nil || !strings.Contains(createErr.Error(), "find existing") {
			t.Fatalf("expected find existing storage failure, got %v", createErr)
		}
	})

	t.Run("create", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		username := "smtp_" + base64.RawURLEncoding.EncodeToString(make([]byte, credentialUsernameBytes))
		if err := database.Create(&Identity{
			ID:           "existing-identity",
			EmailAddress: "existing@example.com",
			Username:     username,
			Status:       IdentityStatusActive,
		}).Error; err != nil {
			t.Fatalf("seed identity: %v", err)
		}
		repository.random = bytes.NewReader(make([]byte, credentialUsernameBytes+credentialPasswordBytes+credentialSaltBytes+credentialNonceBytes))
		_, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr == nil || !strings.Contains(createErr.Error(), "smtp identity create") {
			t.Fatalf("expected create storage failure, got %v", createErr)
		}
	})

	t.Run("create forwarding recipients", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		if dropErr := database.Migrator().DropTable(&ForwardRecipient{}); dropErr != nil {
			t.Fatalf("drop forwarding recipients: %v", dropErr)
		}
		_, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr == nil || !strings.Contains(createErr.Error(), "forwarding recipients") {
			t.Fatalf("expected forwarding recipient storage failure, got %v", createErr)
		}
	})

	t.Run("reactivate", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		address := mustAddress(t, "alice@example.com")
		identity, _, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		if deleteErr := repository.Delete(context.Background(), identity.ID); deleteErr != nil {
			t.Fatalf("delete identity: %v", deleteErr)
		}
		registerIdentityUpdateError(t, database)
		_, _, reactivateErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
		if reactivateErr == nil || !strings.Contains(reactivateErr.Error(), "reactivate") {
			t.Fatalf("expected reactivate storage failure, got %v", reactivateErr)
		}
	})

	t.Run("reactivate reset forwarding", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		address := mustAddress(t, "alice@example.com")
		identity, _, createErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		if deleteErr := repository.Delete(context.Background(), identity.ID); deleteErr != nil {
			t.Fatalf("delete identity: %v", deleteErr)
		}
		if dropErr := database.Migrator().DropTable(&ForwardRecipient{}); dropErr != nil {
			t.Fatalf("drop forwarding recipients: %v", dropErr)
		}
		_, _, reactivateErr := repository.Create(context.Background(), address, defaultForwardRecipients(t))
		if reactivateErr == nil || !strings.Contains(reactivateErr.Error(), "reset forwarding") {
			t.Fatalf("expected reset forwarding storage failure, got %v", reactivateErr)
		}
	})
}

func TestRepositoryUpdateForwardingReportsStorageFailures(t *testing.T) {
	t.Run("stored address", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		if err := database.Create(&Identity{
			ID:           "invalid-address-identity",
			EmailAddress: "bad address",
			Username:     "smtp-invalid-address",
			Status:       IdentityStatusActive,
		}).Error; err != nil {
			t.Fatalf("seed identity: %v", err)
		}
		if _, updateErr := repository.UpdateForwarding(context.Background(), "invalid-address-identity", defaultForwardRecipients(t)); updateErr == nil || !strings.Contains(updateErr.Error(), "stored address") {
			t.Fatalf("expected stored address update failure, got %v", updateErr)
		}
	})

	t.Run("lookup", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		closeIdentityDatabase(t, database)
		if _, updateErr := repository.UpdateForwarding(context.Background(), "identity", defaultForwardRecipients(t)); updateErr == nil {
			t.Fatalf("expected lookup storage failure")
		}
	})

	t.Run("reset", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		if dropErr := database.Migrator().DropTable(&ForwardRecipient{}); dropErr != nil {
			t.Fatalf("drop forwarding recipients: %v", dropErr)
		}
		if _, updateErr := repository.UpdateForwarding(context.Background(), identity.ID, defaultForwardRecipients(t)); updateErr == nil || !strings.Contains(updateErr.Error(), "reset") {
			t.Fatalf("expected reset storage failure, got %v", updateErr)
		}
	})

	t.Run("timestamp", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		registerIdentityUpdateError(t, database)
		if _, updateErr := repository.UpdateForwarding(context.Background(), identity.ID, []Address{mustAddress(t, "maria@example.com")}); updateErr == nil || !strings.Contains(updateErr.Error(), "timestamp") {
			t.Fatalf("expected timestamp storage failure, got %v", updateErr)
		}
	})

	t.Run("create recipients", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		registerForwardRecipientCreateError(t, database)
		if _, updateErr := repository.UpdateForwarding(context.Background(), identity.ID, []Address{mustAddress(t, "maria@example.com")}); updateErr == nil || !strings.Contains(updateErr.Error(), "forwarding recipients") {
			t.Fatalf("expected forwarding recipient storage failure, got %v", updateErr)
		}
	})
}

func TestRepositoryResolveForwardingReportsStorageFailures(t *testing.T) {
	repository, database := newIdentityRepository(t)
	closeIdentityDatabase(t, database)
	if _, _, _, resolveErr := repository.ResolveForwarding(context.Background(), mustAddress(t, "alice@example.com")); resolveErr == nil {
		t.Fatalf("expected resolve storage failure")
	}
}

func TestRepositoryResolveForwardingReportsInvalidStoredRecipient(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if resetErr := database.Where(&ForwardRecipient{IdentityID: identity.ID}).Delete(&ForwardRecipient{}).Error; resetErr != nil {
		t.Fatalf("reset forwarding recipients: %v", resetErr)
	}
	if seedErr := database.Create(&ForwardRecipient{
		ID:           "bad-recipient",
		IdentityID:   identity.ID,
		EmailAddress: "bad address",
	}).Error; seedErr != nil {
		t.Fatalf("seed invalid recipient: %v", seedErr)
	}
	if _, _, _, resolveErr := repository.ResolveForwarding(context.Background(), mustAddress(t, "alice@example.com")); resolveErr == nil || !strings.Contains(resolveErr.Error(), "stored recipient") {
		t.Fatalf("expected invalid stored recipient error, got %v", resolveErr)
	}
}

func TestRepositoryResolveForwardingIgnoresIdentitiesWithoutRecipients(t *testing.T) {
	repository, database := newIdentityRepository(t)
	record := Identity{
		ID:           "identity-no-forwarding",
		EmailAddress: "alice@example.com",
		Username:     "smtp-no-forwarding",
		Status:       IdentityStatusActive,
	}
	if seedErr := database.Create(&record).Error; seedErr != nil {
		t.Fatalf("seed identity: %v", seedErr)
	}
	if _, _, exists, resolveErr := repository.ResolveForwarding(context.Background(), mustAddress(t, "alice@example.com")); resolveErr != nil || exists {
		t.Fatalf("expected no forwarding route, exists=%v err=%v", exists, resolveErr)
	}
}

func TestRepositoryRotateReportsCredentialAndSaveFailures(t *testing.T) {
	t.Run("credential", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		repository.random = identityFailingReader{err: io.ErrUnexpectedEOF}
		_, _, rotateErr := repository.Rotate(context.Background(), identity.ID)
		if rotateErr == nil || !errors.Is(rotateErr, io.ErrUnexpectedEOF) {
			t.Fatalf("expected credential failure, got %v", rotateErr)
		}
	})

	t.Run("save", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		registerIdentityUpdateError(t, database)
		_, _, rotateErr := repository.Rotate(context.Background(), identity.ID)
		if rotateErr == nil || !strings.Contains(rotateErr.Error(), "rotate") {
			t.Fatalf("expected rotate storage failure, got %v", rotateErr)
		}
	})
}

func TestMigrateStoredCredentialPasswordsPopulatesInvalidRows(t *testing.T) {
	database := newIdentityDatabase(t)
	repository, repositoryErr := NewRepository(database, strings.Repeat("a", 64))
	if repositoryErr != nil {
		t.Fatalf("new repository: %v", repositoryErr)
	}
	seedSenderDomain(t, database, "example.com")

	validIdentity, validPassword, validCreateErr := repository.Create(context.Background(), mustAddress(t, "valid@example.com"), defaultForwardRecipients(t))
	if validCreateErr != nil {
		t.Fatalf("create valid identity: %v", validCreateErr)
	}
	missingCipherID := "missing-cipher-identity"
	if err := database.Create(&Identity{
		ID:           missingCipherID,
		EmailAddress: "missing@example.com",
		Username:     "smtp-missing",
		Status:       IdentityStatusActive,
	}).Error; err != nil {
		t.Fatalf("seed missing cipher identity: %v", err)
	}
	if err := database.Create(&ForwardRecipient{ID: missingCipherID + "-forward", IdentityID: missingCipherID, EmailAddress: "owner@example.com"}).Error; err != nil {
		t.Fatalf("seed forwarding recipient for %s: %v", missingCipherID, err)
	}

	if migrateErr := MigrateStoredCredentialPasswords(context.Background(), database, strings.Repeat("a", 64)); migrateErr != nil {
		t.Fatalf("migrate stored credential passwords: %v", migrateErr)
	}
	migratedRepository, migratedRepositoryErr := NewRepository(database, strings.Repeat("a", 64))
	if migratedRepositoryErr != nil {
		t.Fatalf("new migrated repository: %v", migratedRepositoryErr)
	}
	identities, listErr := migratedRepository.List(context.Background())
	if listErr != nil {
		t.Fatalf("list identities after migration: %v", listErr)
	}
	if len(identities) != 2 {
		t.Fatalf("expected all active identities after migration, got %+v", identities)
	}
	identitiesByID := make(map[string]PublicIdentity, len(identities))
	for _, identity := range identities {
		identitiesByID[identity.ID] = identity
	}
	if identitiesByID[validIdentity.ID].Username != validIdentity.Username {
		t.Fatalf("expected valid identity to keep existing username")
	}
	if _, validPasswordAfterMigration, credentialsErr := migratedRepository.Credentials(context.Background(), validIdentity.ID); credentialsErr != nil || validPasswordAfterMigration != validPassword {
		t.Fatalf("expected valid credentials to remain unchanged password=%q err=%v", validPasswordAfterMigration, credentialsErr)
	}
	identity, password, credentialsErr := migratedRepository.Credentials(context.Background(), missingCipherID)
	if credentialsErr != nil {
		t.Fatalf("retrieve migrated credentials for %s: %v", missingCipherID, credentialsErr)
	}
	if password == "" || identity.Username == "" || identity.Username == "smtp-missing" {
		t.Fatalf("expected migrated credentials for %s, identity=%+v password=%q", missingCipherID, identity, password)
	}
	if len(identitiesByID[missingCipherID].ForwardTo) != 1 || identitiesByID[missingCipherID].ForwardTo[0] != "owner@example.com" {
		t.Fatalf("expected forwarding recipients to remain for %s, got %+v", missingCipherID, identitiesByID[missingCipherID].ForwardTo)
	}
	if _, authErr := migratedRepository.Authenticate(context.Background(), identity.Username, password); authErr != nil {
		t.Fatalf("expected migrated credentials to authenticate for %s: %v", missingCipherID, authErr)
	}
}

func TestMigrateStoredCredentialPasswordsReportsFailures(t *testing.T) {
	t.Run("nil database", func(t *testing.T) {
		migrateErr := MigrateStoredCredentialPasswords(context.Background(), nil, strings.Repeat("a", 64))
		if migrateErr == nil || !strings.Contains(migrateErr.Error(), "database is required") {
			t.Fatalf("expected nil database failure, got %v", migrateErr)
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		database := newIdentityDatabase(t)
		migrateErr := MigrateStoredCredentialPasswords(context.Background(), database, "short")
		if migrateErr == nil || !strings.Contains(migrateErr.Error(), "invalid master key") {
			t.Fatalf("expected invalid key failure, got %v", migrateErr)
		}
	})

	t.Run("list", func(t *testing.T) {
		database := newIdentityDatabase(t)
		closeIdentityDatabase(t, database)
		migrateErr := MigrateStoredCredentialPasswords(context.Background(), database, strings.Repeat("a", 64))
		if migrateErr == nil || !strings.Contains(migrateErr.Error(), "list identities") {
			t.Fatalf("expected migration list failure, got %v", migrateErr)
		}
	})

	t.Run("generate credentials", func(t *testing.T) {
		database := newIdentityDatabase(t)
		if err := database.Create(&Identity{
			ID:           "invalid-identity",
			EmailAddress: "invalid@example.com",
			Username:     "smtp-invalid",
			Status:       IdentityStatusActive,
		}).Error; err != nil {
			t.Fatalf("seed invalid identity: %v", err)
		}
		repository := &Repository{
			db:        database,
			key:       bytes.Repeat([]byte{0xaa}, 32),
			random:    identityFailingReader{err: io.ErrUnexpectedEOF},
			clockFunc: func() time.Time { return time.Now().UTC() },
		}
		migrateErr := repository.migrateStoredCredentialPasswords(context.Background())
		if migrateErr == nil || !strings.Contains(migrateErr.Error(), "generate credentials") || !errors.Is(migrateErr, io.ErrUnexpectedEOF) {
			t.Fatalf("expected credential generation failure, got %v", migrateErr)
		}
	})

	t.Run("update identity", func(t *testing.T) {
		database := newIdentityDatabase(t)
		if err := database.Create(&Identity{
			ID:           "invalid-identity",
			EmailAddress: "invalid@example.com",
			Username:     "smtp-invalid",
			Status:       IdentityStatusActive,
		}).Error; err != nil {
			t.Fatalf("seed invalid identity: %v", err)
		}
		registerIdentityUpdateError(t, database)
		migrateErr := MigrateStoredCredentialPasswords(context.Background(), database, strings.Repeat("a", 64))
		if migrateErr == nil || !strings.Contains(migrateErr.Error(), "update identity") {
			t.Fatalf("expected identity update failure, got %v", migrateErr)
		}
	})
}

func TestRepositoryCredentialsReportsStoredPasswordFailures(t *testing.T) {
	t.Run("missing ciphertext", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		if err := database.Create(&Identity{
			ID:           "missing-cipher-identity",
			EmailAddress: "alice@example.com",
			Username:     "smtp-missing-cipher",
			Status:       IdentityStatusActive,
		}).Error; err != nil {
			t.Fatalf("seed missing cipher identity: %v", err)
		}
		_, _, credentialsErr := repository.Credentials(context.Background(), "missing-cipher-identity")
		if credentialsErr == nil || !strings.Contains(credentialsErr.Error(), "ciphertext too short") {
			t.Fatalf("expected missing ciphertext error, got %v", credentialsErr)
		}
	})

	t.Run("short ciphertext", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		if err := database.Create(&Identity{
			ID:             "short-cipher-identity",
			EmailAddress:   "alice@example.com",
			Username:       "smtp-short-cipher",
			PasswordCipher: []byte("short"),
			Status:         IdentityStatusActive,
		}).Error; err != nil {
			t.Fatalf("seed short cipher identity: %v", err)
		}
		_, _, credentialsErr := repository.Credentials(context.Background(), "short-cipher-identity")
		if credentialsErr == nil || !strings.Contains(credentialsErr.Error(), "ciphertext too short") {
			t.Fatalf("expected short ciphertext error, got %v", credentialsErr)
		}
	})

	t.Run("bad key", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		repository.key = []byte("short")
		_, _, credentialsErr := repository.Credentials(context.Background(), identity.ID)
		if credentialsErr == nil || !strings.Contains(credentialsErr.Error(), "invalid key size") {
			t.Fatalf("expected invalid key error, got %v", credentialsErr)
		}
	})

	t.Run("bad ciphertext", func(t *testing.T) {
		repository, database := newIdentityRepository(t)
		seedSenderDomain(t, database, "example.com")
		identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
		if createErr != nil {
			t.Fatalf("create identity: %v", createErr)
		}
		var stored Identity
		if err := database.Where(&Identity{ID: identity.ID}).First(&stored).Error; err != nil {
			t.Fatalf("fetch identity: %v", err)
		}
		stored.PasswordCipher[len(stored.PasswordCipher)-1] ^= 1
		if err := database.Model(&Identity{}).Where(&Identity{ID: identity.ID}).Update("password_cipher", stored.PasswordCipher).Error; err != nil {
			t.Fatalf("corrupt password cipher: %v", err)
		}
		_, _, credentialsErr := repository.Credentials(context.Background(), identity.ID)
		if credentialsErr == nil || !strings.Contains(credentialsErr.Error(), "message authentication failed") {
			t.Fatalf("expected bad ciphertext error, got %v", credentialsErr)
		}
	})
}

func TestRepositoryCredentialsDoNotMutateInvalidStoredRows(t *testing.T) {
	repository, database := newIdentityRepository(t)
	missingCipherID := "missing-cipher-identity"
	if err := database.Create(&Identity{
		ID:           missingCipherID,
		EmailAddress: "missing@example.com",
		Username:     "smtp-missing",
		Status:       IdentityStatusActive,
	}).Error; err != nil {
		t.Fatalf("seed missing cipher identity: %v", err)
	}
	seedSenderDomain(t, database, "example.com")
	corruptIdentity, _, createErr := repository.Create(context.Background(), mustAddress(t, "corrupt@example.com"), defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create corrupt candidate identity: %v", createErr)
	}
	var storedCorruptIdentity Identity
	if err := database.Where(&Identity{ID: corruptIdentity.ID}).First(&storedCorruptIdentity).Error; err != nil {
		t.Fatalf("fetch corrupt candidate identity: %v", err)
	}
	storedCorruptIdentity.PasswordCipher[len(storedCorruptIdentity.PasswordCipher)-1] ^= 1
	if err := database.Model(&Identity{}).Where(&Identity{ID: corruptIdentity.ID}).Update("password_cipher", storedCorruptIdentity.PasswordCipher).Error; err != nil {
		t.Fatalf("corrupt password cipher: %v", err)
	}
	for _, identityID := range []string{missingCipherID, corruptIdentity.ID} {
		_, _, credentialsErr := repository.Credentials(context.Background(), identityID)
		if credentialsErr == nil {
			t.Fatalf("expected invalid credentials error for %s", identityID)
		}
	}
	for _, identityID := range []string{missingCipherID, corruptIdentity.ID} {
		var count int64
		if err := database.Model(&Identity{}).Where(&Identity{ID: identityID}).Count(&count).Error; err != nil {
			t.Fatalf("count identity %s: %v", identityID, err)
		}
		if count != 1 {
			t.Fatalf("expected invalid identity %s to remain for migration, got %d", identityID, count)
		}
	}
}

func TestRepositoryDeleteReportsSaveFailure(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	identity, _, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	registerIdentityUpdateError(t, database)

	deleteErr := repository.Delete(context.Background(), identity.ID)
	if deleteErr == nil || !strings.Contains(deleteErr.Error(), "delete") {
		t.Fatalf("expected delete storage failure, got %v", deleteErr)
	}
}

func TestRepositoryAuthenticateRejectsInvalidAndReportsStoredDataFailures(t *testing.T) {
	repository, database := newIdentityRepository(t)
	seedSenderDomain(t, database, "example.com")
	identity, password, createErr := repository.Create(context.Background(), mustAddress(t, "alice@example.com"), defaultForwardRecipients(t))
	if createErr != nil {
		t.Fatalf("create identity: %v", createErr)
	}
	if _, authErr := repository.Authenticate(context.Background(), " ", password); !errors.Is(authErr, ErrAuthenticationFailed) {
		t.Fatalf("expected blank username auth failure, got %v", authErr)
	}
	if _, authErr := repository.Authenticate(context.Background(), identity.Username, " "); !errors.Is(authErr, ErrAuthenticationFailed) {
		t.Fatalf("expected blank password auth failure, got %v", authErr)
	}
	if _, authErr := repository.Authenticate(context.Background(), identity.Username, "wrong"); !errors.Is(authErr, ErrAuthenticationFailed) {
		t.Fatalf("expected digest auth failure, got %v", authErr)
	}

	registerIdentityUpdateError(t, database)
	if _, authErr := repository.Authenticate(context.Background(), identity.Username, password); authErr == nil || !strings.Contains(authErr.Error(), "mark used") {
		t.Fatalf("expected mark-used storage failure, got %v", authErr)
	}
}

func TestRepositoryAuthenticateReportsInvalidStoredAddress(t *testing.T) {
	repository, database := newIdentityRepository(t)
	password := "pgsmtp_password"
	salt := []byte("fixed-salt")
	record := Identity{
		ID:             "identity-invalid-address",
		EmailAddress:   "Alice <alice@example.com>",
		Username:       "smtp-invalid-address",
		PasswordSalt:   salt,
		PasswordDigest: repository.digest(salt, password),
		Status:         IdentityStatusActive,
	}
	if err := database.Create(&record).Error; err != nil {
		t.Fatalf("seed invalid stored identity: %v", err)
	}

	_, authErr := repository.Authenticate(context.Background(), record.Username, password)
	if authErr == nil || !strings.Contains(authErr.Error(), "stored address") {
		t.Fatalf("expected stored address error, got %v", authErr)
	}
}

func newIdentityRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	database := newIdentityDatabase(t)
	repository, repositoryErr := NewRepository(database, strings.Repeat("a", 64))
	if repositoryErr != nil {
		t.Fatalf("new repository: %v", repositoryErr)
	}
	return repository, database
}

func newIdentityDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, databaseErr := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "identity.db")), &gorm.Config{})
	if databaseErr != nil {
		t.Fatalf("open database: %v", databaseErr)
	}
	if migrateErr := database.AutoMigrate(&SenderDomain{}, &Identity{}, &ForwardRecipient{}); migrateErr != nil {
		t.Fatalf("migrate database: %v", migrateErr)
	}
	return database
}

func closeIdentityDatabase(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDatabase, sqlErr := database.DB()
	if sqlErr != nil {
		t.Fatalf("database handle: %v", sqlErr)
	}
	if closeErr := sqlDatabase.Close(); closeErr != nil {
		t.Fatalf("close database: %v", closeErr)
	}
}

func registerIdentityUpdateError(t *testing.T, database *gorm.DB) {
	t.Helper()
	callbackName := "pinguin:force_identity_update_error"
	if err := database.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		tx.AddError(errors.New("forced identity update failure"))
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Callback().Update().Remove(callbackName); err != nil {
			t.Fatalf("remove update callback: %v", err)
		}
	})
}

func registerSenderDomainUpdateError(t *testing.T, database *gorm.DB) {
	t.Helper()
	callbackName := "pinguin:force_sender_domain_update_error"
	if err := database.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*SenderDomain); ok {
			tx.AddError(errors.New("forced sender domain update failure"))
		}
	}); err != nil {
		t.Fatalf("register sender domain update callback: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Callback().Update().Remove(callbackName); err != nil {
			t.Fatalf("remove sender domain update callback: %v", err)
		}
	})
}

func registerSenderDomainCreateError(t *testing.T, database *gorm.DB) {
	t.Helper()
	callbackName := "pinguin:force_sender_domain_create_error"
	if err := database.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*SenderDomain); ok {
			tx.AddError(errors.New("forced sender domain create failure"))
		}
	}); err != nil {
		t.Fatalf("register sender domain create callback: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Callback().Create().Remove(callbackName); err != nil {
			t.Fatalf("remove sender domain create callback: %v", err)
		}
	})
}

func registerForwardRecipientCreateError(t *testing.T, database *gorm.DB) {
	t.Helper()
	callbackName := "pinguin:force_forward_recipient_create_error"
	if err := database.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*ForwardRecipient); ok {
			tx.AddError(errors.New("forced forward recipient create failure"))
		}
		if _, ok := tx.Statement.Dest.(*[]ForwardRecipient); ok {
			tx.AddError(errors.New("forced forward recipient create failure"))
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Callback().Create().Remove(callbackName); err != nil {
			t.Fatalf("remove create callback: %v", err)
		}
	})
}

type identityFailingReader struct {
	err error
}

func (reader identityFailingReader) Read([]byte) (int, error) {
	return 0, reader.err
}

func seedSenderDomain(t *testing.T, database *gorm.DB, domain string) {
	t.Helper()
	if err := database.Create(&SenderDomain{Domain: domain}).Error; err != nil {
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

func defaultForwardRecipients(t *testing.T) []Address {
	t.Helper()
	return []Address{mustAddress(t, "owner@example.com")}
}

func fmtPublicIdentity(identity PublicIdentity) string {
	return identity.ID + identity.EmailAddress + identity.Username + strings.Join(identity.ForwardTo, ",") + identity.Status
}
