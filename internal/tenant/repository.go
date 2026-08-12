package tenant

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const tenantCreateOperation = "tenant.create"

var (
	// ErrTenantNotFound indicates an absent or foreign tenant.
	ErrTenantNotFound = errors.New("tenant.not_found")
	// ErrVersionPrecondition indicates a stale resource version.
	ErrVersionPrecondition = errors.New("tenant.version_precondition_failed")
	// ErrIdempotencyConflict indicates reuse of a key with different input.
	ErrIdempotencyConflict = errors.New("tenant.idempotency_conflict")
	// ErrCredentialAuthentication indicates invalid API-key authentication.
	ErrCredentialAuthentication = errors.New("tenant.api_credential.authentication_failed")
)

// RuntimeConfig aggregates tenant data required for notification delivery.
type RuntimeConfig struct {
	Tenant Tenant
	Email  EmailCredentials
	SMS    *SMSCredentials
}

// EmailCredentials contains decrypted external SMTP delivery data.
type EmailCredentials struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
}

// SMSCredentials contains decrypted Twilio delivery data.
type SMSCredentials struct {
	AccountSID string
	AuthToken  string
	FromNumber string
}

// EmailProfileResource is the secret-free email profile representation.
type EmailProfileResource struct {
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	FromAddress string    `json:"from_address"`
	HasUsername bool      `json:"has_username"`
	HasPassword bool      `json:"has_password"`
	Version     uint64    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SMSProfileResource is the secret-free SMS profile representation.
type SMSProfileResource struct {
	FromNumber    string    `json:"from_number"`
	HasAccountSID bool      `json:"has_account_sid"`
	HasAuthToken  bool      `json:"has_auth_token"`
	Version       uint64    `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// APICredentialResource is the secret-free API credential representation.
type APICredentialResource struct {
	ID            string     `json:"id"`
	DisplayPrefix string     `json:"display_prefix"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	Version       uint64     `json:"version"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Resource is the complete safe managed tenant representation.
type Resource struct {
	ID            string                `json:"id"`
	DisplayName   string                `json:"display_name"`
	SupportEmail  string                `json:"support_email,omitempty"`
	Version       uint64                `json:"version"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	EmailProfile  EmailProfileResource  `json:"email_profile"`
	SMSProfile    *SMSProfileResource   `json:"sms_profile,omitempty"`
	APICredential APICredentialResource `json:"api_credential"`
}

// CreateInput contains validated resources for atomic tenant creation.
type CreateInput struct {
	OwnerUserID      OwnerUserID
	DisplayName      DisplayName
	SupportEmail     SupportEmail
	EmailProfile     EmailProfileInput
	SMSProfile       *SMSProfileInput
	CredentialID     CredentialID
	CredentialDigest CredentialDigest
}

// CreateResult contains a first or repeated tenant creation result.
type CreateResult struct {
	Resource Resource
	Repeated bool
}

// MetadataInput contains validated tenant metadata replacement data.
type MetadataInput struct {
	DisplayName  DisplayName
	SupportEmail SupportEmail
}

// EmailProfilePatch contains validated partial external SMTP changes.
type EmailProfilePatch struct {
	Host        *string
	Port        *int
	Username    *string
	Password    *string
	FromAddress *string
}

// SMSProfilePatch contains validated partial Twilio changes.
type SMSProfilePatch struct {
	AccountSID *string
	AuthToken  *string
	FromNumber *string
}

// Repository stores and authorizes managed tenant resources.
type Repository struct {
	database     *gorm.DB
	keeper       *SecretKeeper
	clock        func() time.Time
	newID        func() string
	cacheMutex   sync.RWMutex
	runtimeCache map[string]RuntimeConfig
}

// NewRepository constructs a managed tenant repository.
func NewRepository(database *gorm.DB, keeper *SecretKeeper) *Repository {
	return &Repository{
		database:     database,
		keeper:       keeper,
		clock:        func() time.Time { return time.Now().UTC() },
		newID:        uuid.NewString,
		runtimeCache: make(map[string]RuntimeConfig),
	}
}

// Create creates all required tenant resources in one transaction.
func (repository *Repository) Create(ctx context.Context, input CreateInput, requestKey string, requestDigest RequestDigest) (CreateResult, error) {
	var result CreateResult
	transactionErr := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var existing IdempotencyRecord
		lookupErr := transaction.Where(&IdempotencyRecord{
			OwnerUserID: input.OwnerUserID.String(),
			Operation:   tenantCreateOperation,
			RequestKey:  requestKey,
		}).First(&existing).Error
		if lookupErr == nil {
			if subtle.ConstantTimeCompare(existing.RequestDigest, requestDigest.Bytes()) != 1 {
				return ErrIdempotencyConflict
			}
			if unmarshalErr := json.Unmarshal(existing.ResponseBody, &result.Resource); unmarshalErr != nil {
				return fmt.Errorf("tenant create: decode idempotency result: %w", unmarshalErr)
			}
			result.Repeated = true
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("tenant create: idempotency lookup: %w", lookupErr)
		}

		now := repository.clock()
		tenantID := repository.newID()
		tenantModel := Tenant{
			ID:           tenantID,
			OwnerUserID:  input.OwnerUserID.String(),
			DisplayName:  input.DisplayName.String(),
			SupportEmail: input.SupportEmail.String(),
			Version:      1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if createErr := transaction.Create(&tenantModel).Error; createErr != nil {
			return fmt.Errorf("tenant create: tenant: %w", createErr)
		}
		emailProfile, emailErr := repository.newEmailProfile(tenantID, input.EmailProfile, now)
		if emailErr != nil {
			return emailErr
		}
		if createErr := transaction.Create(&emailProfile).Error; createErr != nil {
			return fmt.Errorf("tenant create: email profile: %w", createErr)
		}
		var smsProfile *SMSProfile
		if input.SMSProfile != nil {
			createdSMSProfile, smsErr := repository.newSMSProfile(tenantID, *input.SMSProfile, now)
			if smsErr != nil {
				return smsErr
			}
			if createErr := transaction.Create(&createdSMSProfile).Error; createErr != nil {
				return fmt.Errorf("tenant create: sms profile: %w", createErr)
			}
			smsProfile = &createdSMSProfile
		}
		credential := APICredential{
			ID:            input.CredentialID.String(),
			TenantID:      tenantID,
			SecretDigest:  input.CredentialDigest.Bytes(),
			DisplayPrefix: input.CredentialID.DisplayPrefix(),
			Version:       1,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if createErr := transaction.Create(&credential).Error; createErr != nil {
			return fmt.Errorf("tenant create: api credential: %w", createErr)
		}
		result.Resource = resourceFromModels(tenantModel, emailProfile, smsProfile, credential)
		responseBody, _ := json.Marshal(result.Resource)
		record := IdempotencyRecord{
			ID:             repository.newID(),
			OwnerUserID:    input.OwnerUserID.String(),
			Operation:      tenantCreateOperation,
			RequestKey:     requestKey,
			RequestDigest:  requestDigest.Bytes(),
			TenantID:       tenantID,
			ResponseStatus: 201,
			ResponseBody:   responseBody,
			CreatedAt:      now,
		}
		if createErr := transaction.Create(&record).Error; createErr != nil {
			return fmt.Errorf("tenant create: idempotency record: %w", createErr)
		}
		return nil
	})
	if transactionErr != nil {
		return CreateResult{}, transactionErr
	}
	return result, nil
}

// ListOwned returns tenants owned by one TAuth user.
func (repository *Repository) ListOwned(ctx context.Context, ownerUserID OwnerUserID) ([]Resource, error) {
	var tenants []Tenant
	if listErr := repository.database.WithContext(ctx).
		Where(&Tenant{OwnerUserID: ownerUserID.String()}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "display_name"}}).
		Find(&tenants).Error; listErr != nil {
		return nil, fmt.Errorf("tenant list: %w", listErr)
	}
	resources := make([]Resource, 0, len(tenants))
	for _, tenantModel := range tenants {
		resource, resourceErr := repository.loadResource(ctx, tenantModel)
		if resourceErr != nil {
			return nil, resourceErr
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// GetOwned returns one tenant only when the owner matches.
func (repository *Repository) GetOwned(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID) (Resource, error) {
	tenantModel, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID)
	if requireErr != nil {
		return Resource{}, requireErr
	}
	return repository.loadResource(ctx, tenantModel)
}

// UpdateMetadata replaces tenant metadata with an optimistic precondition.
func (repository *Repository) UpdateMetadata(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64, input MetadataInput) (Resource, error) {
	var updated Resource
	transactionErr := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		updatedAt := repository.clock()
		updateResult := transaction.Model(&Tenant{}).
			Where(&Tenant{ID: tenantID.String(), OwnerUserID: ownerUserID.String(), Version: expectedVersion}).
			Updates(map[string]interface{}{
				"display_name":  input.DisplayName.String(),
				"support_email": input.SupportEmail.String(),
				"version":       expectedVersion + 1,
				"updated_at":    updatedAt,
			})
		if updateResult.Error != nil {
			return fmt.Errorf("tenant update: save: %w", updateResult.Error)
		}
		if updateResult.RowsAffected != 1 {
			if _, requireErr := repository.requireOwnedTenantWithDatabase(ctx, transaction, ownerUserID, tenantID); requireErr != nil {
				return requireErr
			}
			return ErrVersionPrecondition
		}
		var tenantModel Tenant
		if lookupErr := transaction.Where(&Tenant{ID: tenantID.String(), OwnerUserID: ownerUserID.String()}).First(&tenantModel).Error; lookupErr != nil {
			return fmt.Errorf("tenant update: lookup result: %w", lookupErr)
		}
		resource, resourceErr := repository.loadResourceWithDatabase(ctx, transaction, tenantModel)
		if resourceErr != nil {
			return resourceErr
		}
		updated = resource
		return nil
	})
	if transactionErr != nil {
		return Resource{}, transactionErr
	}
	repository.invalidateRuntime(tenantID.String())
	return updated, nil
}

// Delete permanently removes a tenant and all tenant-owned records.
func (repository *Repository) Delete(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64) error {
	transactionErr := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		lockResult := transaction.Model(&Tenant{}).
			Where(&Tenant{ID: tenantID.String(), OwnerUserID: ownerUserID.String(), Version: expectedVersion}).
			UpdateColumn("version", expectedVersion)
		if lockResult.Error != nil {
			return fmt.Errorf("tenant delete: lock: %w", lockResult.Error)
		}
		if lockResult.RowsAffected != 1 {
			if _, requireErr := repository.requireOwnedTenantWithDatabase(ctx, transaction, ownerUserID, tenantID); requireErr != nil {
				return requireErr
			}
			return ErrVersionPrecondition
		}
		tenantModel := Tenant{ID: tenantID.String()}
		var identities []smtpidentity.Identity
		if listErr := transaction.Where(&smtpidentity.Identity{TenantID: tenantID.String()}).Find(&identities).Error; listErr != nil {
			return fmt.Errorf("tenant delete: smtp identity list: %w", listErr)
		}
		identityValues := make([]interface{}, 0, len(identities))
		for _, identity := range identities {
			identityValues = append(identityValues, identity.ID)
		}
		if len(identityValues) > 0 {
			if deleteErr := transaction.Where(clause.IN{Column: clause.Column{Name: "identity_id"}, Values: identityValues}).Delete(&smtpidentity.ForwardRecipient{}).Error; deleteErr != nil {
				return fmt.Errorf("tenant delete: forwarding routes: %w", deleteErr)
			}
		}
		deleteModels := []interface{}{
			&smtpidentity.Identity{},
			&smtpidentity.SenderDomain{},
			&model.NotificationAttachment{},
			&model.Notification{},
			&EmailProfile{},
			&SMSProfile{},
			&APICredential{},
			&IdempotencyRecord{},
		}
		for _, modelValue := range deleteModels {
			if deleteErr := transaction.Where(clause.Eq{Column: clause.Column{Name: "tenant_id"}, Value: tenantID.String()}).Delete(modelValue).Error; deleteErr != nil {
				return fmt.Errorf("tenant delete: owned records: %w", deleteErr)
			}
		}
		if deleteErr := transaction.Delete(&tenantModel).Error; deleteErr != nil {
			return fmt.Errorf("tenant delete: tenant: %w", deleteErr)
		}
		return nil
	})
	if transactionErr != nil {
		return transactionErr
	}
	repository.invalidateRuntime(tenantID.String())
	return nil
}

// ReplaceEmailProfile replaces the complete external SMTP delivery profile.
func (repository *Repository) ReplaceEmailProfile(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64, input EmailProfileInput) (EmailProfileResource, error) {
	if _, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID); requireErr != nil {
		return EmailProfileResource{}, requireErr
	}
	var profile EmailProfile
	if lookupErr := repository.database.WithContext(ctx).Where(&EmailProfile{TenantID: tenantID.String()}).First(&profile).Error; lookupErr != nil {
		return EmailProfileResource{}, fmt.Errorf("email profile replace: lookup: %w", lookupErr)
	}
	if profile.Version != expectedVersion {
		return EmailProfileResource{}, ErrVersionPrecondition
	}
	usernameCipher, usernameErr := repository.keeper.Encrypt(input.Username)
	if usernameErr != nil {
		return EmailProfileResource{}, usernameErr
	}
	passwordCipher, passwordErr := repository.keeper.Encrypt(input.Password)
	if passwordErr != nil {
		return EmailProfileResource{}, passwordErr
	}
	updatedAt := repository.clock()
	updateResult := repository.database.WithContext(ctx).Model(&EmailProfile{}).
		Where(&EmailProfile{ID: profile.ID, TenantID: tenantID.String(), Version: expectedVersion}).
		Updates(map[string]interface{}{
			"host":            input.Host,
			"port":            input.Port,
			"username_cipher": usernameCipher,
			"password_cipher": passwordCipher,
			"from_address":    input.FromAddress,
			"version":         expectedVersion + 1,
			"updated_at":      updatedAt,
		})
	if updateResult.Error != nil {
		return EmailProfileResource{}, fmt.Errorf("email profile replace: save: %w", updateResult.Error)
	}
	if updateResult.RowsAffected != 1 {
		return EmailProfileResource{}, ErrVersionPrecondition
	}
	profile.Host = input.Host
	profile.Port = input.Port
	profile.UsernameCipher = usernameCipher
	profile.PasswordCipher = passwordCipher
	profile.FromAddress = input.FromAddress
	profile.Version = expectedVersion + 1
	profile.UpdatedAt = updatedAt
	repository.invalidateRuntime(tenantID.String())
	return emailProfileResource(profile), nil
}

// PatchEmailProfile changes supplied external SMTP fields and keeps omitted secrets.
func (repository *Repository) PatchEmailProfile(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64, patch EmailProfilePatch) (EmailProfileResource, error) {
	if _, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID); requireErr != nil {
		return EmailProfileResource{}, requireErr
	}
	var profile EmailProfile
	if lookupErr := repository.database.WithContext(ctx).Where(&EmailProfile{TenantID: tenantID.String()}).First(&profile).Error; lookupErr != nil {
		return EmailProfileResource{}, fmt.Errorf("email profile patch: lookup: %w", lookupErr)
	}
	if profile.Version != expectedVersion {
		return EmailProfileResource{}, ErrVersionPrecondition
	}
	username, usernameErr := repository.keeper.Decrypt(profile.UsernameCipher)
	if usernameErr != nil {
		return EmailProfileResource{}, usernameErr
	}
	password, passwordErr := repository.keeper.Decrypt(profile.PasswordCipher)
	if passwordErr != nil {
		return EmailProfileResource{}, passwordErr
	}
	host := profile.Host
	port := profile.Port
	fromAddress := profile.FromAddress
	if patch.Host != nil {
		host = *patch.Host
	}
	if patch.Port != nil {
		port = *patch.Port
	}
	if patch.Username != nil {
		username = *patch.Username
	}
	if patch.Password != nil {
		password = *patch.Password
	}
	if patch.FromAddress != nil {
		fromAddress = *patch.FromAddress
	}
	input, inputErr := NewEmailProfileInput(host, port, username, password, fromAddress)
	if inputErr != nil {
		return EmailProfileResource{}, inputErr
	}
	return repository.ReplaceEmailProfile(ctx, ownerUserID, tenantID, expectedVersion, input)
}

// ReplaceSMSProfile creates or replaces the complete optional SMS profile.
func (repository *Repository) ReplaceSMSProfile(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64, input SMSProfileInput) (SMSProfileResource, error) {
	if _, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID); requireErr != nil {
		return SMSProfileResource{}, requireErr
	}
	var profile SMSProfile
	lookupErr := repository.database.WithContext(ctx).Where(&SMSProfile{TenantID: tenantID.String()}).First(&profile).Error
	if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return SMSProfileResource{}, fmt.Errorf("sms profile replace: lookup: %w", lookupErr)
	}
	if lookupErr == nil && profile.Version != expectedVersion {
		return SMSProfileResource{}, ErrVersionPrecondition
	}
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) && expectedVersion != 0 {
		return SMSProfileResource{}, ErrVersionPrecondition
	}
	now := repository.clock()
	createdProfile, createErr := repository.newSMSProfile(tenantID.String(), input, now)
	if createErr != nil {
		return SMSProfileResource{}, createErr
	}
	if lookupErr == nil {
		createdProfile.ID = profile.ID
		createdProfile.CreatedAt = profile.CreatedAt
		createdProfile.Version = profile.Version + 1
	}
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		if createErr := repository.database.WithContext(ctx).Create(&createdProfile).Error; createErr != nil {
			return SMSProfileResource{}, fmt.Errorf("sms profile replace: create: %w", createErr)
		}
	} else {
		updateResult := repository.database.WithContext(ctx).Model(&SMSProfile{}).
			Where(&SMSProfile{ID: profile.ID, TenantID: tenantID.String(), Version: expectedVersion}).
			Updates(map[string]interface{}{
				"account_sid_cipher": createdProfile.AccountSIDCipher,
				"auth_token_cipher":  createdProfile.AuthTokenCipher,
				"from_number":        createdProfile.FromNumber,
				"version":            createdProfile.Version,
				"updated_at":         createdProfile.UpdatedAt,
			})
		if updateResult.Error != nil {
			return SMSProfileResource{}, fmt.Errorf("sms profile replace: save: %w", updateResult.Error)
		}
		if updateResult.RowsAffected != 1 {
			return SMSProfileResource{}, ErrVersionPrecondition
		}
	}
	repository.invalidateRuntime(tenantID.String())
	return smsProfileResource(createdProfile), nil
}

// PatchSMSProfile changes supplied Twilio fields and keeps omitted secrets.
func (repository *Repository) PatchSMSProfile(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64, patch SMSProfilePatch) (SMSProfileResource, error) {
	if _, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID); requireErr != nil {
		return SMSProfileResource{}, requireErr
	}
	var profile SMSProfile
	if lookupErr := repository.database.WithContext(ctx).Where(&SMSProfile{TenantID: tenantID.String()}).First(&profile).Error; lookupErr != nil {
		return SMSProfileResource{}, fmt.Errorf("sms profile patch: lookup: %w", lookupErr)
	}
	if profile.Version != expectedVersion {
		return SMSProfileResource{}, ErrVersionPrecondition
	}
	accountSID, accountErr := repository.keeper.Decrypt(profile.AccountSIDCipher)
	if accountErr != nil {
		return SMSProfileResource{}, accountErr
	}
	authToken, tokenErr := repository.keeper.Decrypt(profile.AuthTokenCipher)
	if tokenErr != nil {
		return SMSProfileResource{}, tokenErr
	}
	fromNumber := profile.FromNumber
	if patch.AccountSID != nil {
		accountSID = *patch.AccountSID
	}
	if patch.AuthToken != nil {
		authToken = *patch.AuthToken
	}
	if patch.FromNumber != nil {
		fromNumber = *patch.FromNumber
	}
	input, inputErr := NewSMSProfileInput(accountSID, authToken, fromNumber)
	if inputErr != nil {
		return SMSProfileResource{}, inputErr
	}
	return repository.ReplaceSMSProfile(ctx, ownerUserID, tenantID, expectedVersion, input)
}

// DeleteSMSProfile permanently removes the optional tenant SMS profile.
func (repository *Repository) DeleteSMSProfile(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64) error {
	if _, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID); requireErr != nil {
		return requireErr
	}
	deleteResult := repository.database.WithContext(ctx).
		Where(&SMSProfile{TenantID: tenantID.String(), Version: expectedVersion}).
		Delete(&SMSProfile{})
	if deleteResult.Error != nil {
		return fmt.Errorf("sms profile delete: %w", deleteResult.Error)
	}
	if deleteResult.RowsAffected != 1 {
		var profile SMSProfile
		lookupErr := repository.database.WithContext(ctx).Where(&SMSProfile{TenantID: tenantID.String()}).First(&profile).Error
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return nil
		}
		if lookupErr != nil {
			return fmt.Errorf("sms profile delete: lookup: %w", lookupErr)
		}
		return ErrVersionPrecondition
	}
	repository.invalidateRuntime(tenantID.String())
	return nil
}

// GetCredential returns safe metadata for the tenant API credential.
func (repository *Repository) GetCredential(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID) (APICredentialResource, error) {
	if _, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID); requireErr != nil {
		return APICredentialResource{}, requireErr
	}
	var credential APICredential
	if lookupErr := repository.database.WithContext(ctx).Where(&APICredential{TenantID: tenantID.String()}).First(&credential).Error; lookupErr != nil {
		return APICredentialResource{}, fmt.Errorf("api credential read: %w", lookupErr)
	}
	return apiCredentialResource(credential), nil
}

// RotateCredential atomically replaces the tenant API credential.
func (repository *Repository) RotateCredential(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID, expectedVersion uint64, credentialID CredentialID, digest CredentialDigest) (APICredentialResource, error) {
	if _, requireErr := repository.requireOwnedTenant(ctx, ownerUserID, tenantID); requireErr != nil {
		return APICredentialResource{}, requireErr
	}
	var credential APICredential
	if lookupErr := repository.database.WithContext(ctx).Where(&APICredential{TenantID: tenantID.String()}).First(&credential).Error; lookupErr != nil {
		return APICredentialResource{}, fmt.Errorf("api credential rotate: lookup: %w", lookupErr)
	}
	if credential.Version != expectedVersion {
		if credential.Version == expectedVersion+1 && credential.ID == credentialID.String() && subtle.ConstantTimeCompare(credential.SecretDigest, digest.Bytes()) == 1 {
			return apiCredentialResource(credential), nil
		}
		return APICredentialResource{}, ErrVersionPrecondition
	}
	previousCredentialID := credential.ID
	credential.ID = credentialID.String()
	credential.SecretDigest = digest.Bytes()
	credential.DisplayPrefix = credentialID.DisplayPrefix()
	credential.LastUsedAt = nil
	credential.Version++
	credential.UpdatedAt = repository.clock()
	updateResult := repository.database.WithContext(ctx).Model(&APICredential{}).
		Where(&APICredential{ID: previousCredentialID, TenantID: tenantID.String(), Version: expectedVersion}).
		Updates(map[string]interface{}{
			"id":             credentialID.String(),
			"secret_digest":  digest.Bytes(),
			"display_prefix": credentialID.DisplayPrefix(),
			"last_used_at":   nil,
			"version":        credential.Version,
			"updated_at":     credential.UpdatedAt,
		})
	if updateResult.Error != nil {
		return APICredentialResource{}, fmt.Errorf("api credential rotate: save: %w", updateResult.Error)
	}
	if updateResult.RowsAffected != 1 {
		return APICredentialResource{}, ErrVersionPrecondition
	}
	return apiCredentialResource(credential), nil
}

// AuthenticateAPIKey verifies a tenant API key and returns its runtime context.
func (repository *Repository) AuthenticateAPIKey(ctx context.Context, apiKey APIKey) (RuntimeConfig, error) {
	var credential APICredential
	lookupErr := repository.database.WithContext(ctx).Where(&APICredential{ID: apiKey.CredentialID().String()}).First(&credential).Error
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return RuntimeConfig{}, ErrCredentialAuthentication
	}
	if lookupErr != nil {
		return RuntimeConfig{}, fmt.Errorf("api credential authenticate: lookup %s: %w", apiKey.CredentialID().String(), lookupErr)
	}
	if subtle.ConstantTimeCompare(credential.SecretDigest, apiKey.Digest().Bytes()) != 1 {
		return RuntimeConfig{}, ErrCredentialAuthentication
	}
	now := repository.clock()
	credential.LastUsedAt = &now
	if saveErr := repository.database.WithContext(ctx).Model(&credential).Update("last_used_at", now).Error; saveErr != nil {
		return RuntimeConfig{}, fmt.Errorf("api credential authenticate: last use: %w", saveErr)
	}
	return repository.ResolveByID(ctx, credential.TenantID)
}

// ResolveByID returns decrypted runtime data for an internal tenant ID.
func (repository *Repository) ResolveByID(ctx context.Context, rawTenantID string) (RuntimeConfig, error) {
	tenantID, tenantErr := NewTenantID(rawTenantID)
	if tenantErr != nil {
		return RuntimeConfig{}, tenantErr
	}
	if cached, available := repository.cachedRuntime(tenantID.String()); available {
		return cached, nil
	}
	var tenantModel Tenant
	if lookupErr := repository.database.WithContext(ctx).Where(&Tenant{ID: tenantID.String()}).First(&tenantModel).Error; lookupErr != nil {
		return RuntimeConfig{}, fmt.Errorf("tenant runtime: tenant: %w", lookupErr)
	}
	var emailProfile EmailProfile
	if lookupErr := repository.database.WithContext(ctx).Where(&EmailProfile{TenantID: tenantID.String()}).First(&emailProfile).Error; lookupErr != nil {
		return RuntimeConfig{}, fmt.Errorf("tenant runtime: email profile: %w", lookupErr)
	}
	username, usernameErr := repository.keeper.Decrypt(emailProfile.UsernameCipher)
	if usernameErr != nil {
		return RuntimeConfig{}, usernameErr
	}
	password, passwordErr := repository.keeper.Decrypt(emailProfile.PasswordCipher)
	if passwordErr != nil {
		return RuntimeConfig{}, passwordErr
	}
	runtimeConfig := RuntimeConfig{
		Tenant: tenantModel,
		Email: EmailCredentials{
			Host:        emailProfile.Host,
			Port:        emailProfile.Port,
			Username:    username,
			Password:    password,
			FromAddress: emailProfile.FromAddress,
		},
	}
	var smsProfile SMSProfile
	lookupErr := repository.database.WithContext(ctx).Where(&SMSProfile{TenantID: tenantID.String()}).First(&smsProfile).Error
	if lookupErr == nil {
		accountSID, accountErr := repository.keeper.Decrypt(smsProfile.AccountSIDCipher)
		if accountErr != nil {
			return RuntimeConfig{}, accountErr
		}
		authToken, tokenErr := repository.keeper.Decrypt(smsProfile.AuthTokenCipher)
		if tokenErr != nil {
			return RuntimeConfig{}, tokenErr
		}
		runtimeConfig.SMS = &SMSCredentials{AccountSID: accountSID, AuthToken: authToken, FromNumber: smsProfile.FromNumber}
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return RuntimeConfig{}, fmt.Errorf("tenant runtime: sms profile: %w", lookupErr)
	}
	repository.cacheRuntime(tenantID.String(), runtimeConfig)
	return cloneRuntimeConfig(runtimeConfig), nil
}

// ListTenantIDs returns all current tenant IDs for retry processing.
func (repository *Repository) ListTenantIDs(ctx context.Context) ([]string, error) {
	var tenants []Tenant
	if listErr := repository.database.WithContext(ctx).Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}}).Find(&tenants).Error; listErr != nil {
		return nil, fmt.Errorf("tenant id list: %w", listErr)
	}
	identifiers := make([]string, 0, len(tenants))
	for _, tenantModel := range tenants {
		identifiers = append(identifiers, tenantModel.ID)
	}
	return identifiers, nil
}

func (repository *Repository) requireOwnedTenant(ctx context.Context, ownerUserID OwnerUserID, tenantID TenantID) (Tenant, error) {
	return repository.requireOwnedTenantWithDatabase(ctx, repository.database, ownerUserID, tenantID)
}

func (repository *Repository) requireOwnedTenantWithDatabase(ctx context.Context, database *gorm.DB, ownerUserID OwnerUserID, tenantID TenantID) (Tenant, error) {
	var tenantModel Tenant
	lookupErr := database.WithContext(ctx).Where(&Tenant{ID: tenantID.String(), OwnerUserID: ownerUserID.String()}).First(&tenantModel).Error
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return Tenant{}, ErrTenantNotFound
	}
	if lookupErr != nil {
		return Tenant{}, fmt.Errorf("tenant ownership lookup: %w", lookupErr)
	}
	return tenantModel, nil
}

func (repository *Repository) loadResource(ctx context.Context, tenantModel Tenant) (Resource, error) {
	return repository.loadResourceWithDatabase(ctx, repository.database, tenantModel)
}

func (repository *Repository) loadResourceWithDatabase(ctx context.Context, database *gorm.DB, tenantModel Tenant) (Resource, error) {
	var emailProfile EmailProfile
	if lookupErr := database.WithContext(ctx).Where(&EmailProfile{TenantID: tenantModel.ID}).First(&emailProfile).Error; lookupErr != nil {
		return Resource{}, fmt.Errorf("tenant resource: email profile: %w", lookupErr)
	}
	var smsProfile *SMSProfile
	var storedSMSProfile SMSProfile
	lookupErr := database.WithContext(ctx).Where(&SMSProfile{TenantID: tenantModel.ID}).First(&storedSMSProfile).Error
	if lookupErr == nil {
		smsProfile = &storedSMSProfile
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return Resource{}, fmt.Errorf("tenant resource: sms profile: %w", lookupErr)
	}
	var credential APICredential
	if lookupErr := database.WithContext(ctx).Where(&APICredential{TenantID: tenantModel.ID}).First(&credential).Error; lookupErr != nil {
		return Resource{}, fmt.Errorf("tenant resource: api credential: %w", lookupErr)
	}
	return resourceFromModels(tenantModel, emailProfile, smsProfile, credential), nil
}

func (repository *Repository) newEmailProfile(tenantID string, input EmailProfileInput, now time.Time) (EmailProfile, error) {
	usernameCipher, usernameErr := repository.keeper.Encrypt(input.Username)
	if usernameErr != nil {
		return EmailProfile{}, usernameErr
	}
	passwordCipher, passwordErr := repository.keeper.Encrypt(input.Password)
	if passwordErr != nil {
		return EmailProfile{}, passwordErr
	}
	return EmailProfile{
		ID:             repository.newID(),
		TenantID:       tenantID,
		Host:           input.Host,
		Port:           input.Port,
		UsernameCipher: usernameCipher,
		PasswordCipher: passwordCipher,
		FromAddress:    input.FromAddress,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (repository *Repository) newSMSProfile(tenantID string, input SMSProfileInput, now time.Time) (SMSProfile, error) {
	accountSIDCipher, accountErr := repository.keeper.Encrypt(input.AccountSID)
	if accountErr != nil {
		return SMSProfile{}, accountErr
	}
	authTokenCipher, tokenErr := repository.keeper.Encrypt(input.AuthToken)
	if tokenErr != nil {
		return SMSProfile{}, tokenErr
	}
	return SMSProfile{
		ID:               repository.newID(),
		TenantID:         tenantID,
		AccountSIDCipher: accountSIDCipher,
		AuthTokenCipher:  authTokenCipher,
		FromNumber:       input.FromNumber,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func resourceFromModels(tenantModel Tenant, emailProfile EmailProfile, smsProfile *SMSProfile, credential APICredential) Resource {
	resource := Resource{
		ID:            tenantModel.ID,
		DisplayName:   tenantModel.DisplayName,
		SupportEmail:  tenantModel.SupportEmail,
		Version:       tenantModel.Version,
		CreatedAt:     tenantModel.CreatedAt,
		UpdatedAt:     tenantModel.UpdatedAt,
		EmailProfile:  emailProfileResource(emailProfile),
		APICredential: apiCredentialResource(credential),
	}
	if smsProfile != nil {
		smsResource := smsProfileResource(*smsProfile)
		resource.SMSProfile = &smsResource
	}
	return resource
}

func emailProfileResource(profile EmailProfile) EmailProfileResource {
	return EmailProfileResource{
		Host:        profile.Host,
		Port:        profile.Port,
		FromAddress: profile.FromAddress,
		HasUsername: len(profile.UsernameCipher) > 0,
		HasPassword: len(profile.PasswordCipher) > 0,
		Version:     profile.Version,
		CreatedAt:   profile.CreatedAt,
		UpdatedAt:   profile.UpdatedAt,
	}
}

func smsProfileResource(profile SMSProfile) SMSProfileResource {
	return SMSProfileResource{
		FromNumber:    profile.FromNumber,
		HasAccountSID: len(profile.AccountSIDCipher) > 0,
		HasAuthToken:  len(profile.AuthTokenCipher) > 0,
		Version:       profile.Version,
		CreatedAt:     profile.CreatedAt,
		UpdatedAt:     profile.UpdatedAt,
	}
}

func apiCredentialResource(credential APICredential) APICredentialResource {
	return APICredentialResource{
		ID:            credential.ID,
		DisplayPrefix: credential.DisplayPrefix,
		LastUsedAt:    credential.LastUsedAt,
		Version:       credential.Version,
		CreatedAt:     credential.CreatedAt,
		UpdatedAt:     credential.UpdatedAt,
	}
}

func (repository *Repository) cachedRuntime(tenantID string) (RuntimeConfig, bool) {
	repository.cacheMutex.RLock()
	cached, available := repository.runtimeCache[tenantID]
	repository.cacheMutex.RUnlock()
	if !available {
		return RuntimeConfig{}, false
	}
	return cloneRuntimeConfig(cached), true
}

func (repository *Repository) cacheRuntime(tenantID string, runtimeConfig RuntimeConfig) {
	repository.cacheMutex.Lock()
	repository.runtimeCache[tenantID] = cloneRuntimeConfig(runtimeConfig)
	repository.cacheMutex.Unlock()
}

func (repository *Repository) invalidateRuntime(tenantID string) {
	repository.cacheMutex.Lock()
	delete(repository.runtimeCache, tenantID)
	repository.cacheMutex.Unlock()
}

func cloneRuntimeConfig(runtimeConfig RuntimeConfig) RuntimeConfig {
	cloned := runtimeConfig
	if runtimeConfig.SMS != nil {
		smsCopy := *runtimeConfig.SMS
		cloned.SMS = &smsCopy
	}
	return cloned
}
