package tenant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	repositoryOwnerID       = "owner-user"
	repositoryOtherOwnerID  = "other-owner"
	repositoryTenantID      = "11111111-1111-4111-8111-111111111111"
	repositorySecondID      = "22222222-2222-4222-8222-222222222222"
	repositoryCredentialID  = "33333333-3333-4333-8333-333333333333"
	repositoryCredentialID2 = "44444444-4444-4444-8444-444444444444"
)

func TestManagedTenantValueTypes(t *testing.T) {
	owner, err := NewOwnerUserID(" owner-user ")
	if err != nil || owner.String() != repositoryOwnerID {
		t.Fatalf("owner = %q, %v", owner, err)
	}
	if _, err := NewOwnerUserID(" "); !errors.Is(err, ErrInvalidOwnerUserID) {
		t.Fatalf("expected invalid owner, got %v", err)
	}
	tenantID, err := NewTenantID(" " + repositoryTenantID + " ")
	if err != nil || tenantID.String() != repositoryTenantID {
		t.Fatalf("tenant id = %q, %v", tenantID, err)
	}
	for _, value := range []string{"bad"} {
		if _, err := NewTenantID(value); !errors.Is(err, ErrInvalidTenantID) {
			t.Fatalf("expected invalid tenant id for %q, got %v", value, err)
		}
	}
	displayName, err := NewDisplayName(" Example Tenant ")
	if err != nil || displayName.String() != "Example Tenant" {
		t.Fatalf("display name = %q, %v", displayName, err)
	}
	for _, value := range []string{"", strings.Repeat("x", maxDisplayNameLength+1)} {
		if _, err := NewDisplayName(value); !errors.Is(err, ErrInvalidDisplayName) {
			t.Fatalf("expected invalid display name, got %v", err)
		}
	}
	blankSupport, err := NewSupportEmail(" ")
	if err != nil || blankSupport.String() != "" {
		t.Fatalf("blank support email = %q, %v", blankSupport, err)
	}
	support, err := NewSupportEmail("support@example.com")
	if err != nil || support.String() != "support@example.com" {
		t.Fatalf("support email = %q, %v", support, err)
	}
	for _, value := range []string{"invalid", strings.Repeat("x", maxSupportEmailLength+1) + "@example.com"} {
		if _, err := NewSupportEmail(value); !errors.Is(err, ErrInvalidSupportEmail) {
			t.Fatalf("expected invalid support email, got %v", err)
		}
	}
	credentialID, err := NewCredentialID(repositoryCredentialID)
	if err != nil || credentialID.String() != repositoryCredentialID || credentialID.DisplayPrefix() != "pgn_1_33333333" {
		t.Fatalf("credential id = %q, %v", credentialID, err)
	}
	if _, err := NewCredentialID("bad"); !errors.Is(err, ErrInvalidCredentialID) {
		t.Fatalf("expected invalid credential id, got %v", err)
	}
	if _, err := NewRequestDigest([]byte{1}); !errors.Is(err, ErrInvalidCredentialDigest) {
		t.Fatalf("expected invalid request digest, got %v", err)
	}
	requestDigest, err := NewRequestDigest(bytes.Repeat([]byte{1}, sha256.Size))
	if err != nil || len(requestDigest.Bytes()) != sha256.Size {
		t.Fatalf("request digest = %v, %v", requestDigest, err)
	}
	if _, err := NewCredentialDigest([]byte{1}); !errors.Is(err, ErrInvalidCredentialDigest) {
		t.Fatalf("expected invalid credential digest, got %v", err)
	}
	credentialDigest, err := NewCredentialDigest(bytes.Repeat([]byte{2}, sha256.Size))
	if err != nil || len(credentialDigest.Bytes()) != sha256.Size {
		t.Fatalf("credential digest = %v, %v", credentialDigest, err)
	}
	parsedDigest, err := ParseCredentialDigest(credentialDigest.String())
	if err != nil || parsedDigest != credentialDigest {
		t.Fatalf("parsed digest = %v, %v", parsedDigest, err)
	}
	for _, value := range []string{"bad", base64.RawURLEncoding.EncodeToString([]byte{1})} {
		if _, err := ParseCredentialDigest(value); !errors.Is(err, ErrInvalidCredentialDigest) {
			t.Fatalf("expected invalid parsed digest, got %v", err)
		}
	}
	secret := bytes.Repeat([]byte{3}, apiSecretByteCount)
	rawAPIKey := "pgn_1_" + repositoryCredentialID + "_" + base64.RawURLEncoding.EncodeToString(secret)
	apiKey, err := ParseAPIKey(rawAPIKey)
	if err != nil || apiKey.CredentialID().String() != repositoryCredentialID || len(apiKey.Digest().Bytes()) != sha256.Size {
		t.Fatalf("api key = %+v, %v", apiKey, err)
	}
	for _, value := range []string{"bad", "pgn_1_bad_" + base64.RawURLEncoding.EncodeToString(secret), "pgn_1_" + repositoryCredentialID + "_bad", "pgn_1_" + repositoryCredentialID + "_" + base64.RawURLEncoding.EncodeToString([]byte{1})} {
		if _, err := ParseAPIKey(value); !errors.Is(err, ErrInvalidAPIKey) {
			t.Fatalf("expected invalid API key for %q, got %v", value, err)
		}
	}
	email, err := NewEmailProfileInput(" smtp.example.com ", 587, " user ", " password ", "sender@example.com")
	if err != nil || email.Host != "smtp.example.com" || email.Username != "user" || email.Password != "password" {
		t.Fatalf("email profile = %+v, %v", email, err)
	}
	for _, input := range []EmailProfileInput{{Port: 587, Username: "u", Password: "p", FromAddress: "sender@example.com"}, {Host: "h", Port: 0, Username: "u", Password: "p", FromAddress: "sender@example.com"}, {Host: "h", Port: 65536, Username: "u", Password: "p", FromAddress: "sender@example.com"}, {Host: "h", Port: 1, Password: "p", FromAddress: "sender@example.com"}, {Host: "h", Port: 1, Username: "u", FromAddress: "sender@example.com"}, {Host: "h", Port: 1, Username: "u", Password: "p", FromAddress: "bad"}} {
		if _, err := NewEmailProfileInput(input.Host, input.Port, input.Username, input.Password, input.FromAddress); !errors.Is(err, ErrInvalidEmailProfile) {
			t.Fatalf("expected invalid email profile, got %v", err)
		}
	}
	sms, err := NewSMSProfileInput(" sid ", " token ", " +15551234567 ")
	if err != nil || sms.AccountSID != "sid" || sms.AuthToken != "token" || sms.FromNumber != "+15551234567" {
		t.Fatalf("sms profile = %+v, %v", sms, err)
	}
	for _, input := range []SMSProfileInput{{AuthToken: "t", FromNumber: "+1"}, {AccountSID: "s", FromNumber: "+1"}, {AccountSID: "s", AuthToken: "t", FromNumber: "1"}} {
		if _, err := NewSMSProfileInput(input.AccountSID, input.AuthToken, input.FromNumber); !errors.Is(err, ErrInvalidSMSProfile) {
			t.Fatalf("expected invalid SMS profile, got %v", err)
		}
	}
	runtimeConfig := RuntimeConfig{Tenant: Tenant{ID: repositoryTenantID}, SMS: &SMSCredentials{AccountSID: "sid"}}
	ctx := WithRuntime(context.Background(), runtimeConfig)
	resolved, ok := RuntimeFromContext(ctx)
	if !ok || resolved.Tenant.ID != repositoryTenantID {
		t.Fatalf("runtime context = %+v, %t", resolved, ok)
	}
	if _, ok := RuntimeFromContext(context.Background()); ok {
		t.Fatal("unexpected runtime in empty context")
	}
}

func TestManagedTenantRepositoryLifecycle(t *testing.T) {
	repository, database := newRepositoryTestFixture(t)
	created, rawAPIKey := createRepositoryTenant(t, repository, true, repositoryTenantID, repositoryCredentialID, "first-key")
	if created.Repeated || created.Resource.ID != repositoryTenantID || created.Resource.SMSProfile == nil {
		t.Fatalf("created tenant = %+v", created)
	}
	repeated, _ := createRepositoryTenant(t, repository, true, repositoryTenantID, repositoryCredentialID, "first-key")
	if !repeated.Repeated || repeated.Resource.ID != created.Resource.ID {
		t.Fatalf("repeated create = %+v", repeated)
	}
	conflictingInput := repositoryCreateInput(t, true, repositoryCredentialID)
	conflictingInput.DisplayName, _ = NewDisplayName("Different Tenant")
	conflictingDigest, _ := NewRequestDigest(bytes.Repeat([]byte{10}, sha256.Size))
	if _, err := repository.Create(context.Background(), conflictingInput, "first-key", conflictingDigest); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	createRepositoryTenant(t, repository, false, repositorySecondID, repositoryCredentialID2, "second-key")
	owner, _ := NewOwnerUserID(repositoryOwnerID)
	otherOwner, _ := NewOwnerUserID(repositoryOtherOwnerID)
	firstID, _ := NewTenantID(repositoryTenantID)
	listed, err := repository.ListOwned(context.Background(), owner)
	if err != nil || len(listed) != 2 || listed[0].DisplayName != "Managed Tenant" {
		t.Fatalf("listed tenants = %+v, %v", listed, err)
	}
	resource, err := repository.GetOwned(context.Background(), owner, firstID)
	if err != nil || resource.ID != repositoryTenantID {
		t.Fatalf("get owned = %+v, %v", resource, err)
	}
	if _, err := repository.GetOwned(context.Background(), otherOwner, firstID); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("expected foreign tenant to be hidden, got %v", err)
	}
	runtimeConfig, err := repository.ResolveByID(context.Background(), repositoryTenantID)
	if err != nil || runtimeConfig.Email.Password != "smtp-password" || runtimeConfig.SMS == nil || runtimeConfig.SMS.AuthToken != "auth-token" {
		t.Fatalf("runtime config = %+v, %v", runtimeConfig, err)
	}
	runtimeConfig.SMS.AuthToken = "mutated"
	cached, err := repository.ResolveByID(context.Background(), repositoryTenantID)
	if err != nil || cached.SMS.AuthToken != "auth-token" {
		t.Fatalf("cached runtime clone = %+v, %v", cached, err)
	}
	name, _ := NewDisplayName("Updated Tenant")
	support, _ := NewSupportEmail("updated@example.com")
	updated, err := repository.UpdateMetadata(context.Background(), owner, firstID, 1, MetadataInput{DisplayName: name, SupportEmail: support})
	if err != nil || updated.Version != 2 || updated.DisplayName != "Updated Tenant" {
		t.Fatalf("updated metadata = %+v, %v", updated, err)
	}
	if _, err := repository.UpdateMetadata(context.Background(), owner, firstID, 1, MetadataInput{DisplayName: name, SupportEmail: support}); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected stale metadata update, got %v", err)
	}
	if _, err := repository.UpdateMetadata(context.Background(), otherOwner, firstID, 2, MetadataInput{DisplayName: name, SupportEmail: support}); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("expected foreign metadata update, got %v", err)
	}
	replacementEmail, _ := NewEmailProfileInput("smtp.updated.example", 2525, "new-user", "new-password", "updated-sender@example.com")
	emailProfile, err := repository.ReplaceEmailProfile(context.Background(), owner, firstID, 1, replacementEmail)
	if err != nil || emailProfile.Version != 2 || emailProfile.Host != "smtp.updated.example" {
		t.Fatalf("replaced email = %+v, %v", emailProfile, err)
	}
	if _, err := repository.ReplaceEmailProfile(context.Background(), owner, firstID, 1, replacementEmail); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected stale email replace, got %v", err)
	}
	patchedHost, patchedUser, patchedPassword, patchedFrom := "smtp.patched.example", "patched-user", "patched-password", "patched@example.com"
	patchedPort := 465
	emailProfile, err = repository.PatchEmailProfile(context.Background(), owner, firstID, 2, EmailProfilePatch{Host: &patchedHost, Port: &patchedPort, Username: &patchedUser, Password: &patchedPassword, FromAddress: &patchedFrom})
	if err != nil || emailProfile.Version != 3 || emailProfile.Port != 465 {
		t.Fatalf("patched email = %+v, %v", emailProfile, err)
	}
	if _, err := repository.PatchEmailProfile(context.Background(), owner, firstID, 2, EmailProfilePatch{}); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected stale email patch, got %v", err)
	}
	invalidHost := ""
	if _, err := repository.PatchEmailProfile(context.Background(), owner, firstID, 3, EmailProfilePatch{Host: &invalidHost}); !errors.Is(err, ErrInvalidEmailProfile) {
		t.Fatalf("expected invalid email patch, got %v", err)
	}
	replacementSMS, _ := NewSMSProfileInput("new-sid", "new-token", "+15550000001")
	smsProfile, err := repository.ReplaceSMSProfile(context.Background(), owner, firstID, 1, replacementSMS)
	if err != nil || smsProfile.Version != 2 {
		t.Fatalf("replaced SMS = %+v, %v", smsProfile, err)
	}
	patchedSID, patchedToken, patchedNumber := "patched-sid", "patched-token", "+15550000002"
	smsProfile, err = repository.PatchSMSProfile(context.Background(), owner, firstID, 2, SMSProfilePatch{AccountSID: &patchedSID, AuthToken: &patchedToken, FromNumber: &patchedNumber})
	if err != nil || smsProfile.Version != 3 || smsProfile.FromNumber != patchedNumber {
		t.Fatalf("patched SMS = %+v, %v", smsProfile, err)
	}
	if _, err := repository.PatchSMSProfile(context.Background(), owner, firstID, 2, SMSProfilePatch{}); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected stale SMS patch, got %v", err)
	}
	invalidNumber := "invalid"
	if _, err := repository.PatchSMSProfile(context.Background(), owner, firstID, 3, SMSProfilePatch{FromNumber: &invalidNumber}); !errors.Is(err, ErrInvalidSMSProfile) {
		t.Fatalf("expected invalid SMS patch, got %v", err)
	}
	secondID, _ := NewTenantID(repositorySecondID)
	createdSMS, err := repository.ReplaceSMSProfile(context.Background(), owner, secondID, 0, replacementSMS)
	if err != nil || createdSMS.Version != 1 {
		t.Fatalf("created SMS = %+v, %v", createdSMS, err)
	}
	if _, err := repository.ReplaceSMSProfile(context.Background(), owner, secondID, 0, replacementSMS); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected stale SMS create, got %v", err)
	}
	deleteTenantSMS(t, database, repositorySecondID)
	if _, err := repository.ReplaceSMSProfile(context.Background(), owner, secondID, 1, replacementSMS); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected missing SMS version precondition, got %v", err)
	}
	credential, err := repository.GetCredential(context.Background(), owner, firstID)
	if err != nil || credential.ID != repositoryCredentialID {
		t.Fatalf("credential = %+v, %v", credential, err)
	}
	newCredentialID, _ := NewCredentialID("55555555-5555-4555-8555-555555555555")
	newDigest, _ := NewCredentialDigest(bytes.Repeat([]byte{6}, sha256.Size))
	rotated, err := repository.RotateCredential(context.Background(), owner, firstID, 1, newCredentialID, newDigest)
	if err != nil || rotated.ID != newCredentialID.String() || rotated.Version != 2 {
		t.Fatalf("rotated credential = %+v, %v", rotated, err)
	}
	repeatedRotation, err := repository.RotateCredential(context.Background(), owner, firstID, 1, newCredentialID, newDigest)
	if err != nil || repeatedRotation.ID != rotated.ID {
		t.Fatalf("repeated rotation = %+v, %v", repeatedRotation, err)
	}
	otherCredentialID, _ := NewCredentialID("66666666-6666-4666-8666-666666666666")
	if _, err := repository.RotateCredential(context.Background(), owner, firstID, 1, otherCredentialID, newDigest); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected stale credential rotation, got %v", err)
	}
	apiKey, _ := ParseAPIKey(rawAPIKey)
	if _, err := repository.AuthenticateAPIKey(context.Background(), apiKey); !errors.Is(err, ErrCredentialAuthentication) {
		t.Fatalf("rotated key must fail, got %v", err)
	}
	rotatedAPIKey := APIKey{credentialID: newCredentialID, digest: newDigest}
	authenticated, err := repository.AuthenticateAPIKey(context.Background(), rotatedAPIKey)
	if err != nil || authenticated.Tenant.ID != repositoryTenantID {
		t.Fatalf("authenticated runtime = %+v, %v", authenticated, err)
	}
	credential, err = repository.GetCredential(context.Background(), owner, firstID)
	if err != nil || credential.LastUsedAt == nil {
		t.Fatalf("credential last use = %+v, %v", credential, err)
	}
	identifiers, err := repository.ListTenantIDs(context.Background())
	if err != nil || len(identifiers) != 2 || identifiers[0] != repositoryTenantID || identifiers[1] != repositorySecondID {
		t.Fatalf("tenant ids = %+v, %v", identifiers, err)
	}
	seedTenantOwnedRecords(t, database, repositoryTenantID)
	if err := repository.Delete(context.Background(), owner, firstID, 1); !errors.Is(err, ErrVersionPrecondition) {
		t.Fatalf("expected stale delete, got %v", err)
	}
	if err := repository.Delete(context.Background(), otherOwner, firstID, 2); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("expected foreign delete, got %v", err)
	}
	if err := repository.Delete(context.Background(), owner, firstID, 2); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	if _, err := repository.GetOwned(context.Background(), owner, firstID); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("expected deleted tenant to disappear, got %v", err)
	}
	for _, modelValue := range []interface{}{&smtpidentity.Identity{}, &smtpidentity.SenderDomain{}, &model.Notification{}, &model.NotificationAttachment{}, &EmailProfile{}, &SMSProfile{}, &APICredential{}, &IdempotencyRecord{}} {
		var count int64
		if err := database.Model(modelValue).Where(clause.Eq{Column: clause.Column{Name: "tenant_id"}, Value: repositoryTenantID}).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("expected deleted owned model %T count 0, got %d, %v", modelValue, count, err)
		}
	}
	var recipientCount int64
	if err := database.Model(&smtpidentity.ForwardRecipient{}).Count(&recipientCount).Error; err != nil || recipientCount != 0 {
		t.Fatalf("expected deleted forwarding recipients, got %d, %v", recipientCount, err)
	}
}

func TestManagedTenantRepositoryFailures(t *testing.T) {
	t.Run("default constructors", func(t *testing.T) {
		database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "default.db")), &gorm.Config{})
		if err != nil {
			t.Fatalf("open database: %v", err)
		}
		keeper, _ := NewSecretKeeper(strings.Repeat("a", 64))
		repository := NewRepository(database, keeper)
		if repository.clock().IsZero() || repository.newID() == "" {
			t.Fatal("expected default clock and identifier generator")
		}
	})

	t.Run("create stage errors", func(t *testing.T) {
		testCases := []struct {
			name      string
			operation string
			table     string
			prepare   func(*Repository)
		}{
			{name: "idempotency lookup", operation: "query", table: "idempotency_records"},
			{name: "tenant insert", operation: "create", table: "tenants"},
			{name: "email insert", operation: "create", table: "email_profiles"},
			{name: "sms insert", operation: "create", table: "sms_profiles"},
			{name: "credential insert", operation: "create", table: "api_credentials"},
			{name: "idempotency insert", operation: "create", table: "idempotency_records"},
			{name: "email encryption", prepare: func(repository *Repository) {
				repository.keeper = &SecretKeeper{key: []byte{1}, random: bytes.NewReader(nil)}
			}},
			{name: "sms encryption", prepare: func(repository *Repository) {
				repository.keeper = &SecretKeeper{key: bytes.Repeat([]byte{1}, 32), random: &failingTenantReader{failAt: 3}}
			}},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				if testCase.operation != "" {
					registerTenantFailure(t, database, testCase.operation, testCase.table, 1)
				}
				if testCase.prepare != nil {
					testCase.prepare(repository)
				}
				input := repositoryCreateInput(t, true, repositoryCredentialID)
				requestDigest, _ := NewRequestDigest(bytes.Repeat([]byte{1}, sha256.Size))
				if _, err := repository.Create(context.Background(), input, testCase.name, requestDigest); err == nil {
					t.Fatal("expected create failure")
				}
			})
		}
	})

	t.Run("corrupt repeated create response", func(t *testing.T) {
		repository, database := newRepositoryTestFixture(t)
		createRepositoryTenant(t, repository, false, repositoryTenantID, repositoryCredentialID, "corrupt-repeat")
		if err := database.Model(&IdempotencyRecord{}).Where(&IdempotencyRecord{RequestKey: "corrupt-repeat"}).Update("response_body", []byte("not-json")).Error; err != nil {
			t.Fatalf("corrupt response: %v", err)
		}
		input := repositoryCreateInput(t, false, repositoryCredentialID)
		digest, _ := NewRequestDigest(bytes.Repeat([]byte{byte(len("corrupt-repeat"))}, sha256.Size))
		if _, err := repository.Create(context.Background(), input, "corrupt-repeat", digest); err == nil {
			t.Fatal("expected repeated response decode failure")
		}
	})

	t.Run("read and metadata errors", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			operation string
			table     string
			invoke    func(*Repository, OwnerUserID, TenantID) error
		}{
			{name: "list query", operation: "query", table: "tenants", invoke: func(repository *Repository, owner OwnerUserID, _ TenantID) error {
				_, err := repository.ListOwned(context.Background(), owner)
				return err
			}},
			{name: "list resource", operation: "query", table: "email_profiles", invoke: func(repository *Repository, owner OwnerUserID, _ TenantID) error {
				_, err := repository.ListOwned(context.Background(), owner)
				return err
			}},
			{name: "metadata update", operation: "update", table: "tenants", invoke: updateMetadataFailureCall},
			{name: "metadata result lookup", operation: "query", table: "tenants", invoke: updateMetadataFailureCall},
			{name: "metadata resource load", operation: "query", table: "email_profiles", invoke: updateMetadataFailureCall},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				createRepositoryTenant(t, repository, false, repositoryTenantID, repositoryCredentialID, "read-error")
				registerTenantFailure(t, database, testCase.operation, testCase.table, 1)
				owner, _ := NewOwnerUserID(repositoryOwnerID)
				tenantID, _ := NewTenantID(repositoryTenantID)
				if err := testCase.invoke(repository, owner, tenantID); err == nil {
					t.Fatal("expected repository failure")
				}
			})
		}
	})

	t.Run("metadata zero row", func(t *testing.T) {
		repository, database := newRepositoryTestFixture(t)
		createRepositoryTenant(t, repository, false, repositoryTenantID, repositoryCredentialID, "metadata-zero")
		registerTenantZeroRows(t, database, "update", "tenants")
		owner, _ := NewOwnerUserID(repositoryOwnerID)
		tenantID, _ := NewTenantID(repositoryTenantID)
		if err := updateMetadataFailureCall(repository, owner, tenantID); !errors.Is(err, ErrVersionPrecondition) {
			t.Fatalf("expected version precondition, got %v", err)
		}
	})

	t.Run("delete stage errors", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			operation string
			table     string
			seed      bool
		}{
			{name: "lock", operation: "update", table: "tenants"},
			{name: "identity list", operation: "query", table: "identities"},
			{name: "forwarding delete", operation: "delete", table: "forward_recipients", seed: true},
			{name: "owned delete", operation: "delete", table: "identities"},
			{name: "tenant delete", operation: "delete", table: "tenants"},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				createRepositoryTenant(t, repository, false, repositoryTenantID, repositoryCredentialID, "delete-error")
				if testCase.seed {
					seedTenantOwnedRecords(t, database, repositoryTenantID)
				}
				registerTenantFailure(t, database, testCase.operation, testCase.table, 1)
				owner, _ := NewOwnerUserID(repositoryOwnerID)
				tenantID, _ := NewTenantID(repositoryTenantID)
				if err := repository.Delete(context.Background(), owner, tenantID, 1); err == nil {
					t.Fatal("expected delete failure")
				}
			})
		}
	})

	t.Run("profile and credential authorization", func(t *testing.T) {
		repository, _ := newRepositoryTestFixture(t)
		createRepositoryTenant(t, repository, true, repositoryTenantID, repositoryCredentialID, "authorization")
		otherOwner, _ := NewOwnerUserID(repositoryOtherOwnerID)
		tenantID, _ := NewTenantID(repositoryTenantID)
		emailInput, _ := NewEmailProfileInput("smtp.example", 25, "u", "p", "sender@example.com")
		smsInput, _ := NewSMSProfileInput("sid", "token", "+1")
		credentialID, _ := NewCredentialID(repositoryCredentialID2)
		digest, _ := NewCredentialDigest(bytes.Repeat([]byte{1}, sha256.Size))
		calls := []func() error{
			func() error {
				_, err := repository.ReplaceEmailProfile(context.Background(), otherOwner, tenantID, 1, emailInput)
				return err
			},
			func() error {
				_, err := repository.PatchEmailProfile(context.Background(), otherOwner, tenantID, 1, EmailProfilePatch{})
				return err
			},
			func() error {
				_, err := repository.ReplaceSMSProfile(context.Background(), otherOwner, tenantID, 1, smsInput)
				return err
			},
			func() error {
				_, err := repository.PatchSMSProfile(context.Background(), otherOwner, tenantID, 1, SMSProfilePatch{})
				return err
			},
			func() error {
				_, err := repository.GetCredential(context.Background(), otherOwner, tenantID)
				return err
			},
			func() error {
				_, err := repository.RotateCredential(context.Background(), otherOwner, tenantID, 1, credentialID, digest)
				return err
			},
		}
		for index, call := range calls {
			if err := call(); !errors.Is(err, ErrTenantNotFound) {
				t.Fatalf("foreign call %d = %v", index, err)
			}
		}
	})

	t.Run("email profile errors", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			operation string
			table     string
			zeroRows  bool
			prepare   func(*Repository, *gorm.DB)
			patch     bool
		}{
			{name: "replace lookup", operation: "query", table: "email_profiles"},
			{name: "replace username encryption", prepare: func(repository *Repository, _ *gorm.DB) { repository.keeper = &SecretKeeper{key: []byte{1}} }},
			{name: "replace password encryption", prepare: func(repository *Repository, _ *gorm.DB) { repository.keeper.random = &failingTenantReader{failAt: 2} }},
			{name: "replace save", operation: "update", table: "email_profiles"},
			{name: "replace zero rows", operation: "update", table: "email_profiles", zeroRows: true},
			{name: "patch lookup", operation: "query", table: "email_profiles", patch: true},
			{name: "patch username decrypt", patch: true, prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &EmailProfile{}, "username_cipher")
			}},
			{name: "patch password decrypt", patch: true, prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &EmailProfile{}, "password_cipher")
			}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				createRepositoryTenant(t, repository, false, repositoryTenantID, repositoryCredentialID, "email-error")
				if testCase.prepare != nil {
					testCase.prepare(repository, database)
				}
				if testCase.operation != "" {
					if testCase.zeroRows {
						registerTenantZeroRows(t, database, testCase.operation, testCase.table)
					} else {
						registerTenantFailure(t, database, testCase.operation, testCase.table, 1)
					}
				}
				owner, _ := NewOwnerUserID(repositoryOwnerID)
				tenantID, _ := NewTenantID(repositoryTenantID)
				var err error
				if testCase.patch {
					_, err = repository.PatchEmailProfile(context.Background(), owner, tenantID, 1, EmailProfilePatch{})
				} else {
					input, _ := NewEmailProfileInput("smtp.new", 25, "new-user", "new-password", "new@example.com")
					_, err = repository.ReplaceEmailProfile(context.Background(), owner, tenantID, 1, input)
				}
				if err == nil {
					t.Fatal("expected email profile failure")
				}
			})
		}
	})

	t.Run("sms profile errors", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			operation string
			table     string
			zeroRows  bool
			prepare   func(*Repository, *gorm.DB)
			patch     bool
			create    bool
		}{
			{name: "replace lookup", operation: "query", table: "sms_profiles"},
			{name: "replace encryption", prepare: func(repository *Repository, _ *gorm.DB) { repository.keeper = &SecretKeeper{key: []byte{1}} }},
			{name: "replace create", operation: "create", table: "sms_profiles", create: true, prepare: func(_ *Repository, database *gorm.DB) { deleteTenantSMS(t, database, repositoryTenantID) }},
			{name: "replace save", operation: "update", table: "sms_profiles"},
			{name: "replace zero rows", operation: "update", table: "sms_profiles", zeroRows: true},
			{name: "patch lookup", operation: "query", table: "sms_profiles", patch: true},
			{name: "patch account decrypt", patch: true, prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &SMSProfile{}, "account_sid_cipher")
			}},
			{name: "patch token decrypt", patch: true, prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &SMSProfile{}, "auth_token_cipher")
			}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				createRepositoryTenant(t, repository, true, repositoryTenantID, repositoryCredentialID, "sms-error")
				if testCase.prepare != nil {
					testCase.prepare(repository, database)
				}
				if testCase.operation != "" {
					if testCase.zeroRows {
						registerTenantZeroRows(t, database, testCase.operation, testCase.table)
					} else {
						registerTenantFailure(t, database, testCase.operation, testCase.table, 1)
					}
				}
				owner, _ := NewOwnerUserID(repositoryOwnerID)
				tenantID, _ := NewTenantID(repositoryTenantID)
				var err error
				if testCase.patch {
					_, err = repository.PatchSMSProfile(context.Background(), owner, tenantID, 1, SMSProfilePatch{})
				} else {
					input, _ := NewSMSProfileInput("new-sid", "new-token", "+15550000000")
					expectedVersion := uint64(1)
					if testCase.create {
						expectedVersion = 0
					}
					_, err = repository.ReplaceSMSProfile(context.Background(), owner, tenantID, expectedVersion, input)
				}
				if err == nil {
					t.Fatal("expected SMS profile failure")
				}
			})
		}
	})

	t.Run("credential errors", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			operation string
			table     string
			zeroRows  bool
			rotate    bool
		}{
			{name: "get lookup", operation: "query", table: "api_credentials"},
			{name: "rotate lookup", operation: "query", table: "api_credentials", rotate: true},
			{name: "rotate save", operation: "update", table: "api_credentials", rotate: true},
			{name: "rotate zero rows", operation: "update", table: "api_credentials", rotate: true, zeroRows: true},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				createRepositoryTenant(t, repository, false, repositoryTenantID, repositoryCredentialID, "credential-error")
				if testCase.zeroRows {
					registerTenantZeroRows(t, database, testCase.operation, testCase.table)
				} else {
					registerTenantFailure(t, database, testCase.operation, testCase.table, 1)
				}
				owner, _ := NewOwnerUserID(repositoryOwnerID)
				tenantID, _ := NewTenantID(repositoryTenantID)
				var err error
				if testCase.rotate {
					credentialID, _ := NewCredentialID(repositoryCredentialID2)
					digest, _ := NewCredentialDigest(bytes.Repeat([]byte{2}, sha256.Size))
					_, err = repository.RotateCredential(context.Background(), owner, tenantID, 1, credentialID, digest)
				} else {
					_, err = repository.GetCredential(context.Background(), owner, tenantID)
				}
				if err == nil {
					t.Fatal("expected credential failure")
				}
			})
		}
	})

	t.Run("authentication and runtime errors", func(t *testing.T) {
		for _, testCase := range []struct {
			name      string
			operation string
			table     string
			prepare   func(*Repository, *gorm.DB)
			invoke    func(*Repository) error
		}{
			{name: "authenticate last use", operation: "update", table: "api_credentials", invoke: authenticateStoredCredential},
			{name: "invalid tenant id", invoke: func(repository *Repository) error {
				_, err := repository.ResolveByID(context.Background(), "bad")
				return err
			}},
			{name: "tenant lookup", operation: "query", table: "tenants", invoke: resolveStoredTenant},
			{name: "email lookup", operation: "query", table: "email_profiles", invoke: resolveStoredTenant},
			{name: "username decrypt", prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &EmailProfile{}, "username_cipher")
			}, invoke: resolveStoredTenant},
			{name: "password decrypt", prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &EmailProfile{}, "password_cipher")
			}, invoke: resolveStoredTenant},
			{name: "sms account decrypt", prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &SMSProfile{}, "account_sid_cipher")
			}, invoke: resolveStoredTenant},
			{name: "sms token decrypt", prepare: func(_ *Repository, database *gorm.DB) {
				corruptTenantColumn(t, database, &SMSProfile{}, "auth_token_cipher")
			}, invoke: resolveStoredTenant},
			{name: "sms lookup", operation: "query", table: "sms_profiles", invoke: resolveStoredTenant},
			{name: "list ids", operation: "query", table: "tenants", invoke: func(repository *Repository) error {
				_, err := repository.ListTenantIDs(context.Background())
				return err
			}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				createRepositoryTenant(t, repository, true, repositoryTenantID, repositoryCredentialID, "runtime-error")
				if testCase.prepare != nil {
					testCase.prepare(repository, database)
				}
				if testCase.operation != "" {
					registerTenantFailure(t, database, testCase.operation, testCase.table, 1)
				}
				if err := testCase.invoke(repository); err == nil {
					t.Fatal("expected runtime failure")
				}
			})
		}
	})

	t.Run("resource shape errors", func(t *testing.T) {
		for _, table := range []string{"email_profiles", "sms_profiles", "api_credentials"} {
			t.Run(table, func(t *testing.T) {
				repository, database := newRepositoryTestFixture(t)
				createRepositoryTenant(t, repository, true, repositoryTenantID, repositoryCredentialID, "resource-error")
				registerTenantFailure(t, database, "query", table, 1)
				owner, _ := NewOwnerUserID(repositoryOwnerID)
				tenantID, _ := NewTenantID(repositoryTenantID)
				if _, err := repository.GetOwned(context.Background(), owner, tenantID); err == nil {
					t.Fatal("expected resource load failure")
				}
			})
		}
	})

	t.Run("ownership storage error", func(t *testing.T) {
		repository, database := newRepositoryTestFixture(t)
		createRepositoryTenant(t, repository, false, repositoryTenantID, repositoryCredentialID, "ownership-error")
		registerTenantFailure(t, database, "query", "tenants", 1)
		owner, _ := NewOwnerUserID(repositoryOwnerID)
		tenantID, _ := NewTenantID(repositoryTenantID)
		if _, err := repository.GetOwned(context.Background(), owner, tenantID); err == nil {
			t.Fatal("expected ownership lookup failure")
		}
	})

	t.Run("direct profile encryption errors", func(t *testing.T) {
		repository := &Repository{keeper: &SecretKeeper{key: bytes.Repeat([]byte{1}, 32), random: &failingTenantReader{failAt: 2}}, newID: func() string { return "id" }}
		emailInput, _ := NewEmailProfileInput("host", 25, "user", "password", "sender@example.com")
		if _, err := repository.newEmailProfile(repositoryTenantID, emailInput, time.Now()); err == nil {
			t.Fatal("expected email password encryption failure")
		}
		repository.keeper.random = &failingTenantReader{failAt: 1}
		if _, err := repository.newEmailProfile(repositoryTenantID, emailInput, time.Now()); err == nil {
			t.Fatal("expected email username encryption failure")
		}
		smsInput, _ := NewSMSProfileInput("sid", "token", "+1")
		repository.keeper.random = &failingTenantReader{failAt: 2}
		if _, err := repository.newSMSProfile(repositoryTenantID, smsInput, time.Now()); err == nil {
			t.Fatal("expected SMS token encryption failure")
		}
		repository.keeper.random = &failingTenantReader{failAt: 1}
		if _, err := repository.newSMSProfile(repositoryTenantID, smsInput, time.Now()); err == nil {
			t.Fatal("expected SMS account encryption failure")
		}
	})
}

func updateMetadataFailureCall(repository *Repository, owner OwnerUserID, tenantID TenantID) error {
	name, _ := NewDisplayName("Updated")
	support, _ := NewSupportEmail("")
	_, err := repository.UpdateMetadata(context.Background(), owner, tenantID, 1, MetadataInput{DisplayName: name, SupportEmail: support})
	return err
}

func authenticateStoredCredential(repository *Repository) error {
	credentialID, _ := NewCredentialID(repositoryCredentialID)
	digest, _ := NewCredentialDigest(bytes.Repeat([]byte{4}, sha256.Size))
	_, err := repository.AuthenticateAPIKey(context.Background(), APIKey{credentialID: credentialID, digest: digest})
	return err
}

func resolveStoredTenant(repository *Repository) error {
	_, err := repository.ResolveByID(context.Background(), repositoryTenantID)
	return err
}

type failingTenantReader struct {
	calls  int
	failAt int
}

func (reader *failingTenantReader) Read(target []byte) (int, error) {
	reader.calls++
	if reader.calls >= reader.failAt {
		return 0, errors.New("random failed")
	}
	for index := range target {
		target[index] = byte(index + 1)
	}
	return len(target), nil
}

func registerTenantFailure(t *testing.T, database *gorm.DB, operation, table string, occurrence int) {
	t.Helper()
	name := "tenant-test-failure-" + operation + "-" + table
	seen := 0
	callback := func(transaction *gorm.DB) {
		if transaction.Statement.Table != table {
			return
		}
		seen++
		if seen == occurrence {
			transaction.AddError(errors.New("injected database failure"))
		}
	}
	var err error
	switch operation {
	case "query":
		err = database.Callback().Query().Before("gorm:query").Register(name, callback)
		t.Cleanup(func() { _ = database.Callback().Query().Remove(name) })
	case "create":
		err = database.Callback().Create().Before("gorm:create").Register(name, callback)
		t.Cleanup(func() { _ = database.Callback().Create().Remove(name) })
	case "update":
		err = database.Callback().Update().Before("gorm:update").Register(name, callback)
		t.Cleanup(func() { _ = database.Callback().Update().Remove(name) })
	case "delete":
		err = database.Callback().Delete().Before("gorm:delete").Register(name, callback)
		t.Cleanup(func() { _ = database.Callback().Delete().Remove(name) })
	default:
		t.Fatalf("unknown callback operation %q", operation)
	}
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}
}

func registerTenantZeroRows(t *testing.T, database *gorm.DB, operation, table string) {
	t.Helper()
	name := "tenant-test-zero-" + operation + "-" + table
	callback := func(transaction *gorm.DB) {
		if transaction.Statement.Table == table {
			transaction.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.Eq{Column: clause.Column{Name: "id"}, Value: "never"}}})
		}
	}
	if operation != "update" {
		t.Fatalf("unsupported zero-row operation %q", operation)
	}
	if err := database.Callback().Update().Before("gorm:update").Register(name, callback); err != nil {
		t.Fatalf("register zero-row callback: %v", err)
	}
	t.Cleanup(func() { _ = database.Callback().Update().Remove(name) })
}

func corruptTenantColumn(t *testing.T, database *gorm.DB, modelValue interface{}, column string) {
	t.Helper()
	if err := database.Model(modelValue).Where(clause.Eq{Column: clause.Column{Name: "tenant_id"}, Value: repositoryTenantID}).Update(column, []byte{1}).Error; err != nil {
		t.Fatalf("corrupt %s: %v", column, err)
	}
}

func deleteTenantSMS(t *testing.T, database *gorm.DB, tenantID string) {
	t.Helper()
	if err := database.Where(&SMSProfile{TenantID: tenantID}).Delete(&SMSProfile{}).Error; err != nil {
		t.Fatalf("delete SMS profile: %v", err)
	}
}

func newRepositoryTestFixture(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tenant.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&Tenant{}, &EmailProfile{}, &SMSProfile{}, &APICredential{}, &IdempotencyRecord{}, &smtpidentity.SenderDomain{}, &smtpidentity.Identity{}, &smtpidentity.ForwardRecipient{}, &model.Notification{}, &model.NotificationAttachment{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	keeper, err := NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("secret keeper: %v", err)
	}
	repository := NewRepository(database, keeper)
	identifiers := []string{repositoryTenantID, repositoryTenantID + "-email", repositoryTenantID + "-sms", repositoryTenantID + "-record", repositorySecondID, repositorySecondID + "-email", repositorySecondID + "-record", "replacement-sms-1", "replacement-sms-2", repositorySecondID + "-sms"}
	repository.newID = func() string {
		identifier := identifiers[0]
		identifiers = identifiers[1:]
		return identifier
	}
	repository.clock = func() time.Time { return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) }
	return repository, database
}

func repositoryCreateInput(t *testing.T, withSMS bool, credentialIDValue string) CreateInput {
	t.Helper()
	owner, _ := NewOwnerUserID(repositoryOwnerID)
	displayName, _ := NewDisplayName("Managed Tenant")
	supportEmail, _ := NewSupportEmail("support@example.com")
	emailProfile, _ := NewEmailProfileInput("smtp.example.com", 587, "smtp-user", "smtp-password", "sender@example.com")
	credentialID, _ := NewCredentialID(credentialIDValue)
	credentialDigest, _ := NewCredentialDigest(bytes.Repeat([]byte{4}, sha256.Size))
	input := CreateInput{OwnerUserID: owner, DisplayName: displayName, SupportEmail: supportEmail, EmailProfile: emailProfile, CredentialID: credentialID, CredentialDigest: credentialDigest}
	if withSMS {
		smsProfile, _ := NewSMSProfileInput("account-sid", "auth-token", "+15551234567")
		input.SMSProfile = &smsProfile
	}
	return input
}

func createRepositoryTenant(t *testing.T, repository *Repository, withSMS bool, expectedTenantID string, credentialIDValue string, requestKey string) (CreateResult, string) {
	t.Helper()
	input := repositoryCreateInput(t, withSMS, credentialIDValue)
	requestDigest, _ := NewRequestDigest(bytes.Repeat([]byte{byte(len(requestKey))}, sha256.Size))
	result, err := repository.Create(context.Background(), input, requestKey, requestDigest)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if result.Resource.ID != expectedTenantID {
		t.Fatalf("expected tenant id %s, got %s", expectedTenantID, result.Resource.ID)
	}
	rawAPIKey := "pgn_1_" + credentialIDValue + "_" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, apiSecretByteCount))
	return result, rawAPIKey
}

func seedTenantOwnedRecords(t *testing.T, database *gorm.DB, tenantID string) {
	t.Helper()
	identity := smtpidentity.Identity{ID: "identity", TenantID: tenantID, EmailAddress: "sender@example.com", Username: "smtp-user", Status: smtpidentity.IdentityStatusActive}
	models := []interface{}{
		&smtpidentity.SenderDomain{TenantID: tenantID, Domain: "example.com", Status: smtpidentity.SenderDomainStatusVerified},
		&identity,
		&smtpidentity.ForwardRecipient{ID: "recipient", IdentityID: identity.ID, EmailAddress: "owner@example.com"},
		&model.Notification{NotificationID: "notification", TenantID: tenantID, NotificationType: model.NotificationEmail, Recipient: "user@example.com", Subject: "subject", Message: "message", Status: model.StatusQueued},
		&model.NotificationAttachment{NotificationID: "notification", TenantID: tenantID, Filename: "file.txt", ContentType: "text/plain", Data: []byte("data")},
	}
	for _, modelValue := range models {
		if err := database.Create(modelValue).Error; err != nil {
			t.Fatalf("seed owned record %T: %v", modelValue, err)
		}
	}
}
