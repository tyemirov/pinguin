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
		t.Fatalf("expected one-time password")
	}
	if identity.EmailAddress != "alice@example.com" || identity.Username == "" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	if len(identity.ForwardTo) != 1 || identity.ForwardTo[0] != "owner@example.com" {
		t.Fatalf("unexpected forwarding recipients: %+v", identity.ForwardTo)
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
}

func TestRepositoryCreateReportsCredentialFailures(t *testing.T) {
	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{name: "username token", reader: identityFailingReader{err: io.ErrUnexpectedEOF}},
		{name: "password token", reader: io.MultiReader(bytes.NewReader(make([]byte, credentialUsernameBytes)), identityFailingReader{err: io.ErrUnexpectedEOF})},
		{name: "salt", reader: io.MultiReader(bytes.NewReader(make([]byte, credentialUsernameBytes+credentialPasswordBytes)), identityFailingReader{err: io.ErrUnexpectedEOF})},
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
		repository.random = bytes.NewReader(make([]byte, credentialUsernameBytes+credentialPasswordBytes+credentialSaltBytes))
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
	database, databaseErr := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "identity.db")), &gorm.Config{})
	if databaseErr != nil {
		t.Fatalf("open database: %v", databaseErr)
	}
	if migrateErr := database.AutoMigrate(&SenderDomain{}, &Identity{}, &ForwardRecipient{}); migrateErr != nil {
		t.Fatalf("migrate database: %v", migrateErr)
	}
	repository, repositoryErr := NewRepository(database, strings.Repeat("a", 64))
	if repositoryErr != nil {
		t.Fatalf("new repository: %v", repositoryErr)
	}
	return repository, database
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
