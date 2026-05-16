package smtpidentity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	credentialSaltBytes     = 16
	credentialPasswordBytes = 24
	credentialUsernameBytes = 12
	credentialNonceBytes    = 12
	domainVerificationBytes = 24
	activeStatusValue       = string(IdentityStatusActive)
	deletedStatusValue      = string(IdentityStatusDeleted)
	identityIDColumn        = "id"
	lastUsedAtColumn        = "last_used_at"
	ownerEmailColumn        = "owner_email"
	statusColumn            = "status"
	updatedAtColumn         = "updated_at"
	usernameColumn          = "username"
)

var (
	// ErrAuthenticationFailed indicates SMTP credentials were rejected.
	ErrAuthenticationFailed = errors.New("smtp_identity.auth_failed")
	// ErrSenderDomainNotAllowed indicates SMTP submission cannot create this sender.
	ErrSenderDomainNotAllowed = errors.New("smtp_identity.sender_domain_not_allowed")
	// ErrIdentityExists indicates an active identity already exists for the sender.
	ErrIdentityExists = errors.New("smtp_identity.exists")
	// ErrIdentityNotFound indicates the identity does not exist.
	ErrIdentityNotFound = errors.New("smtp_identity.not_found")
	// ErrForwardRecipientsRequired indicates a shared identity has no forwarding owner.
	ErrForwardRecipientsRequired = errors.New("smtp_identity.forward_recipients_required")
	// ErrForwardRecipientDuplicate indicates a shared identity repeats a forwarding owner.
	ErrForwardRecipientDuplicate = errors.New("smtp_identity.forward_recipient_duplicate")
	// ErrForwardRecipientSelf indicates a shared identity forwards back to itself.
	ErrForwardRecipientSelf = errors.New("smtp_identity.forward_recipient_self")
)

// IdentityStatus captures SMTP identity lifecycle state.
type IdentityStatus string

const (
	// IdentityStatusActive allows SMTP authentication and relay.
	IdentityStatusActive IdentityStatus = "active"
	// IdentityStatusDeleted prevents authentication while preserving metadata.
	IdentityStatusDeleted IdentityStatus = "deleted"
)

// Identity stores SMTP submission credentials.
type Identity struct {
	ID             string `gorm:"primaryKey"`
	OwnerEmail     string `gorm:"index"`
	EmailAddress   string `gorm:"uniqueIndex"`
	Username       string `gorm:"uniqueIndex"`
	PasswordSalt   []byte
	PasswordDigest []byte
	PasswordCipher []byte
	Status         IdentityStatus `gorm:"index"`
	LastUsedAt     *time.Time
	ForwardTo      []ForwardRecipient `gorm:"foreignKey:IdentityID;constraint:OnDelete:CASCADE"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ForwardRecipient stores one mailbox that receives inbound copies for an identity.
type ForwardRecipient struct {
	ID           string `gorm:"primaryKey"`
	IdentityID   string `gorm:"index:idx_smtp_forward_identity_email,unique"`
	EmailAddress string `gorm:"index:idx_smtp_forward_identity_email,unique"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PublicIdentity is the secret-free identity shape exposed to callers.
type PublicIdentity struct {
	ID           string     `json:"id"`
	EmailAddress string     `json:"email_address"`
	Username     string     `json:"username"`
	ForwardTo    []string   `json:"forward_to"`
	Status       string     `json:"status"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AuthenticatedIdentity is the validated SMTP AUTH result.
type AuthenticatedIdentity struct {
	ID           string
	EmailAddress Address
	Username     string
}

// AccessScope constrains authenticated HTTP SMTP relay management to one owner unless admin access is present.
type AccessScope struct {
	OwnerEmail string
	Admin      bool
}

// Repository stores and verifies SMTP identities.
type Repository struct {
	db        *gorm.DB
	key       []byte
	random    io.Reader
	clockFunc func() time.Time
}

// NewRepository constructs an SMTP identity repository.
func NewRepository(db *gorm.DB, rawMasterKey string) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("smtp identity: database is required")
	}
	key, decodeErr := decodeMasterKey(rawMasterKey)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return &Repository{
		db:        db,
		key:       key,
		random:    rand.Reader,
		clockFunc: func() time.Time { return time.Now().UTC() },
	}, nil
}

func decodeMasterKey(rawMasterKey string) ([]byte, error) {
	key, decodeErr := hex.DecodeString(strings.TrimSpace(rawMasterKey))
	if decodeErr != nil {
		return nil, fmt.Errorf("smtp identity: invalid master key: %w", decodeErr)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("smtp identity: master key must decode to 32 bytes")
	}
	return key, nil
}

// List returns active SMTP identities without secrets.
func (repository *Repository) List(ctx context.Context) ([]PublicIdentity, error) {
	return repository.ListForScope(ctx, AccessScope{Admin: true})
}

// ListForScope returns active SMTP identities visible to the authenticated owner scope.
func (repository *Repository) ListForScope(ctx context.Context, scope AccessScope) ([]PublicIdentity, error) {
	var records []Identity
	query := repository.db.WithContext(ctx).
		Preload("ForwardTo", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "email_address"}})
		}).
		Where(&Identity{Status: IdentityStatusActive})
	if !scope.Admin {
		query = query.Where(clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: normalizeOwnerEmail(scope.OwnerEmail)})
	}
	if err := query.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "email_address"}}).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("smtp identity list: %w", err)
	}
	result := make([]PublicIdentity, 0, len(records))
	for _, record := range records {
		result = append(result, publicIdentity(record))
	}
	return result, nil
}

// Create creates or reactivates an exact sender identity.
func (repository *Repository) Create(ctx context.Context, address Address, forwardTo []Address) (PublicIdentity, string, error) {
	return repository.CreateForScope(ctx, AccessScope{Admin: true}, address, forwardTo)
}

// CreateForScope creates or reactivates an exact sender identity owned by a verified sender domain.
func (repository *Repository) CreateForScope(ctx context.Context, scope AccessScope, address Address, forwardTo []Address) (PublicIdentity, string, error) {
	ownerEmail := normalizeOwnerEmail(scope.OwnerEmail)
	if allowedErr := repository.requireSenderDomainForScope(ctx, scope, address.Domain()); allowedErr != nil {
		return PublicIdentity{}, "", allowedErr
	}
	if recipientErr := validateForwardRecipients(forwardTo); recipientErr != nil {
		return PublicIdentity{}, "", recipientErr
	}
	if recipientErr := validateForwardRecipientTarget(address, forwardTo); recipientErr != nil {
		return PublicIdentity{}, "", recipientErr
	}
	var existing Identity
	findErr := repository.db.WithContext(ctx).
		Where(&Identity{EmailAddress: address.String()}).
		First(&existing).Error
	if findErr == nil && existing.Status == IdentityStatusActive {
		return PublicIdentity{}, "", ErrIdentityExists
	}
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return PublicIdentity{}, "", fmt.Errorf("smtp identity create: find existing: %w", findErr)
	}
	username, password, salt, digest, passwordCipher, credentialErr := repository.newCredential()
	if credentialErr != nil {
		return PublicIdentity{}, "", credentialErr
	}
	now := repository.clockFunc()
	if findErr == nil {
		existing.OwnerEmail = ownerEmail
		existing.Username = username
		existing.PasswordSalt = salt
		existing.PasswordDigest = digest
		existing.PasswordCipher = passwordCipher
		existing.Status = IdentityStatusActive
		existing.LastUsedAt = nil
		existing.UpdatedAt = now
		if saveErr := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where(&ForwardRecipient{IdentityID: existing.ID}).Delete(&ForwardRecipient{}).Error; err != nil {
				return fmt.Errorf("smtp identity create: reset forwarding: %w", err)
			}
			if err := tx.Save(&existing).Error; err != nil {
				return fmt.Errorf("smtp identity create: reactivate: %w", err)
			}
			return createForwardRecipients(tx, existing.ID, forwardTo, now)
		}); saveErr != nil {
			return PublicIdentity{}, "", saveErr
		}
		existing.ForwardTo = forwardRecipientRecords(existing.ID, forwardTo, now)
		return publicIdentity(existing), password, nil
	}
	record := Identity{
		ID:             uuid.NewString(),
		OwnerEmail:     ownerEmail,
		EmailAddress:   address.String(),
		Username:       username,
		PasswordSalt:   salt,
		PasswordDigest: digest,
		PasswordCipher: passwordCipher,
		Status:         IdentityStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if createErr := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("smtp identity create: %w", err)
		}
		return createForwardRecipients(tx, record.ID, forwardTo, now)
	}); createErr != nil {
		return PublicIdentity{}, "", createErr
	}
	record.ForwardTo = forwardRecipientRecords(record.ID, forwardTo, now)
	return publicIdentity(record), password, nil
}

// Credentials returns the current active credentials for a sender identity.
func (repository *Repository) Credentials(ctx context.Context, identityID string) (PublicIdentity, string, error) {
	return repository.CredentialsForScope(ctx, AccessScope{Admin: true}, identityID)
}

// CredentialsForScope returns the current active credentials visible to the owner scope.
func (repository *Repository) CredentialsForScope(ctx context.Context, scope AccessScope, identityID string) (PublicIdentity, string, error) {
	record, fetchErr := repository.requireIdentityForScope(ctx, scope, identityID)
	if fetchErr != nil {
		return PublicIdentity{}, "", fetchErr
	}
	password, decryptErr := repository.decryptPassword(record.PasswordCipher)
	if decryptErr != nil {
		return PublicIdentity{}, "", fmt.Errorf("smtp identity credentials: decrypt password: %w", decryptErr)
	}
	return publicIdentity(record), password, nil
}

// UpdateForwarding replaces the forwarding recipients for an active identity.
func (repository *Repository) UpdateForwarding(ctx context.Context, identityID string, forwardTo []Address) (PublicIdentity, error) {
	return repository.UpdateForwardingForScope(ctx, AccessScope{Admin: true}, identityID, forwardTo)
}

// UpdateForwardingForScope replaces forwarding recipients for an identity visible to the owner scope.
func (repository *Repository) UpdateForwardingForScope(ctx context.Context, scope AccessScope, identityID string, forwardTo []Address) (PublicIdentity, error) {
	if recipientErr := validateForwardRecipients(forwardTo); recipientErr != nil {
		return PublicIdentity{}, recipientErr
	}
	record, fetchErr := repository.requireIdentityForScope(ctx, scope, identityID)
	if fetchErr != nil {
		return PublicIdentity{}, fetchErr
	}
	recordAddress, addressErr := NewAddress(record.EmailAddress)
	if addressErr != nil {
		return PublicIdentity{}, fmt.Errorf("smtp identity forwarding update: stored address: %w", addressErr)
	}
	if recipientErr := validateForwardRecipientTarget(recordAddress, forwardTo); recipientErr != nil {
		return PublicIdentity{}, recipientErr
	}
	now := repository.clockFunc()
	record.UpdatedAt = now
	if updateErr := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&ForwardRecipient{IdentityID: record.ID}).Delete(&ForwardRecipient{}).Error; err != nil {
			return fmt.Errorf("smtp identity forwarding update: reset: %w", err)
		}
		if err := createForwardRecipients(tx, record.ID, forwardTo, now); err != nil {
			return err
		}
		if err := tx.Model(&Identity{}).Where(&Identity{ID: record.ID}).Update(updatedAtColumn, now).Error; err != nil {
			return fmt.Errorf("smtp identity forwarding update: timestamp: %w", err)
		}
		return nil
	}); updateErr != nil {
		return PublicIdentity{}, updateErr
	}
	record.ForwardTo = forwardRecipientRecords(record.ID, forwardTo, now)
	return publicIdentity(record), nil
}

// Rotate replaces credentials for an active identity.
func (repository *Repository) Rotate(ctx context.Context, identityID string) (PublicIdentity, string, error) {
	return repository.RotateForScope(ctx, AccessScope{Admin: true}, identityID)
}

// RotateForScope replaces credentials for an active identity visible to the owner scope.
func (repository *Repository) RotateForScope(ctx context.Context, scope AccessScope, identityID string) (PublicIdentity, string, error) {
	record, fetchErr := repository.requireIdentityForScope(ctx, scope, identityID)
	if fetchErr != nil {
		return PublicIdentity{}, "", fetchErr
	}
	username, password, salt, digest, passwordCipher, credentialErr := repository.newCredential()
	if credentialErr != nil {
		return PublicIdentity{}, "", credentialErr
	}
	record.Username = username
	record.PasswordSalt = salt
	record.PasswordDigest = digest
	record.PasswordCipher = passwordCipher
	record.UpdatedAt = repository.clockFunc()
	if saveErr := repository.db.WithContext(ctx).Save(&record).Error; saveErr != nil {
		return PublicIdentity{}, "", fmt.Errorf("smtp identity rotate: %w", saveErr)
	}
	return publicIdentity(record), password, nil
}

// Delete disables an identity so it can no longer authenticate.
func (repository *Repository) Delete(ctx context.Context, identityID string) error {
	return repository.DeleteForScope(ctx, AccessScope{Admin: true}, identityID)
}

// DeleteForScope disables an identity visible to the owner scope so it can no longer authenticate.
func (repository *Repository) DeleteForScope(ctx context.Context, scope AccessScope, identityID string) error {
	record, fetchErr := repository.requireIdentityForScope(ctx, scope, identityID)
	if fetchErr != nil {
		return fetchErr
	}
	record.Status = IdentityStatusDeleted
	record.UpdatedAt = repository.clockFunc()
	if saveErr := repository.db.WithContext(ctx).Save(&record).Error; saveErr != nil {
		return fmt.Errorf("smtp identity delete: %w", saveErr)
	}
	return nil
}

// ResolveForwarding returns the active forwarding route for an inbound address.
func (repository *Repository) ResolveForwarding(ctx context.Context, address Address) (Address, []Address, bool, error) {
	var record Identity
	err := repository.db.WithContext(ctx).
		Preload("ForwardTo", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "email_address"}})
		}).
		Where(&Identity{EmailAddress: address.String(), Status: IdentityStatusActive}).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Address{}, nil, false, nil
		}
		return Address{}, nil, false, fmt.Errorf("smtp identity forwarding lookup: %w", err)
	}
	if len(record.ForwardTo) == 0 {
		return Address{}, nil, false, nil
	}
	if domainErr := repository.requireSenderDomainForIdentity(ctx, record.OwnerEmail, address.Domain()); domainErr != nil {
		if errors.Is(domainErr, ErrSenderDomainNotAllowed) {
			return Address{}, nil, false, nil
		}
		return Address{}, nil, false, domainErr
	}
	recipients := make([]Address, 0, len(record.ForwardTo))
	for _, recipientRecord := range record.ForwardTo {
		recipient, recipientErr := NewAddress(recipientRecord.EmailAddress)
		if recipientErr != nil {
			return Address{}, nil, false, fmt.Errorf("smtp identity forwarding lookup: stored recipient: %w", recipientErr)
		}
		recipients = append(recipients, recipient)
	}
	return address, recipients, true, nil
}

// Authenticate verifies SMTP credentials and returns the exact sender identity.
func (repository *Repository) Authenticate(ctx context.Context, username string, password string) (AuthenticatedIdentity, error) {
	normalizedUsername := strings.TrimSpace(username)
	if normalizedUsername == "" || strings.TrimSpace(password) == "" {
		return AuthenticatedIdentity{}, ErrAuthenticationFailed
	}
	var record Identity
	if err := repository.db.WithContext(ctx).
		Where(&Identity{Username: normalizedUsername, Status: IdentityStatusActive}).
		First(&record).Error; err != nil {
		return AuthenticatedIdentity{}, ErrAuthenticationFailed
	}
	digest := repository.digest(record.PasswordSalt, password)
	if subtle.ConstantTimeCompare(digest, record.PasswordDigest) != 1 {
		return AuthenticatedIdentity{}, ErrAuthenticationFailed
	}
	address, addressErr := NewAddress(record.EmailAddress)
	if addressErr != nil {
		return AuthenticatedIdentity{}, fmt.Errorf("smtp identity auth: stored address: %w", addressErr)
	}
	if domainErr := repository.requireSenderDomainForIdentity(ctx, record.OwnerEmail, address.Domain()); domainErr != nil {
		if errors.Is(domainErr, ErrSenderDomainNotAllowed) {
			return AuthenticatedIdentity{}, ErrAuthenticationFailed
		}
		return AuthenticatedIdentity{}, domainErr
	}
	now := repository.clockFunc()
	if markErr := repository.markAuthenticatedIdentityUsed(ctx, record, now); markErr != nil {
		return AuthenticatedIdentity{}, markErr
	}
	return AuthenticatedIdentity{
		ID:           record.ID,
		EmailAddress: address,
		Username:     record.Username,
	}, nil
}

func (repository *Repository) markAuthenticatedIdentityUsed(ctx context.Context, record Identity, now time.Time) error {
	updateResult := repository.db.WithContext(ctx).
		Model(&Identity{}).
		Where(map[string]interface{}{
			identityIDColumn: record.ID,
			statusColumn:     IdentityStatusActive,
			usernameColumn:   record.Username,
		}).
		Updates(map[string]interface{}{
			lastUsedAtColumn: &now,
			updatedAtColumn:  now,
		})
	if updateResult.Error != nil {
		return fmt.Errorf("smtp identity auth: mark used: %w", updateResult.Error)
	}
	if updateResult.RowsAffected == 0 {
		return ErrAuthenticationFailed
	}
	return nil
}

func (repository *Repository) requireIdentityForScope(ctx context.Context, scope AccessScope, identityID string) (Identity, error) {
	normalizedIdentityID := strings.TrimSpace(identityID)
	var record Identity
	query := repository.db.WithContext(ctx).
		Where(&Identity{ID: normalizedIdentityID, Status: IdentityStatusActive})
	if !scope.Admin {
		query = query.Where(clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: normalizeOwnerEmail(scope.OwnerEmail)})
	}
	err := query.
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Identity{}, ErrIdentityNotFound
		}
		return Identity{}, fmt.Errorf("smtp identity lookup: identity %s: %w", normalizedIdentityID, err)
	}
	return record, nil
}

func (repository *Repository) requireSenderDomainForScope(ctx context.Context, scope AccessScope, domain string) error {
	ownerEmail := normalizeOwnerEmail(scope.OwnerEmail)
	normalizedDomain := strings.ToLower(strings.TrimSpace(domain))
	var domainRecord SenderDomain
	query := repository.db.WithContext(ctx).
		Where(clause.Eq{Column: clause.Column{Name: "domain"}, Value: normalizedDomain})
	if scope.Admin {
		query = query.Where(
			clause.Or(
				clause.And(
					clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: ownerEmail},
					clause.Eq{Column: clause.Column{Name: statusColumn}, Value: senderDomainVerifiedStatus},
				),
				clause.And(
					clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: ""},
					clause.Or(
						clause.Eq{Column: clause.Column{Name: statusColumn}, Value: senderDomainVerifiedStatus},
						clause.Eq{Column: clause.Column{Name: statusColumn}, Value: ""},
					),
				),
			),
		)
	} else {
		query = query.Where(clause.And(
			clause.Eq{Column: clause.Column{Name: ownerEmailColumn}, Value: ownerEmail},
			clause.Eq{Column: clause.Column{Name: statusColumn}, Value: senderDomainVerifiedStatus},
		))
	}
	err := query.
		First(&domainRecord).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrSenderDomainNotAllowed, normalizedDomain)
		}
		return fmt.Errorf("smtp identity sender domain lookup: domain %s: %w", normalizedDomain, err)
	}
	return nil
}

func (repository *Repository) requireSenderDomainForIdentity(ctx context.Context, ownerEmail string, domain string) error {
	return repository.requireSenderDomainForScope(ctx, AccessScope{OwnerEmail: ownerEmail, Admin: true}, domain)
}

func validateForwardRecipients(forwardTo []Address) error {
	if len(forwardTo) == 0 {
		return ErrForwardRecipientsRequired
	}
	seenRecipients := make(map[string]struct{}, len(forwardTo))
	for _, recipient := range forwardTo {
		recipientKey := recipient.String()
		if recipientKey == "" {
			return ErrForwardRecipientsRequired
		}
		if _, exists := seenRecipients[recipientKey]; exists {
			return fmt.Errorf("%w: %s", ErrForwardRecipientDuplicate, recipientKey)
		}
		seenRecipients[recipientKey] = struct{}{}
	}
	return nil
}

func validateForwardRecipientTarget(address Address, forwardTo []Address) error {
	for _, recipient := range forwardTo {
		if recipient.Equals(address) {
			return fmt.Errorf("%w: %s", ErrForwardRecipientSelf, recipient.String())
		}
	}
	return nil
}

func createForwardRecipients(tx *gorm.DB, identityID string, forwardTo []Address, now time.Time) error {
	records := forwardRecipientRecords(identityID, forwardTo, now)
	if err := tx.Create(&records).Error; err != nil {
		return fmt.Errorf("smtp identity forwarding recipients: %w", err)
	}
	return nil
}

func forwardRecipientRecords(identityID string, forwardTo []Address, now time.Time) []ForwardRecipient {
	records := make([]ForwardRecipient, 0, len(forwardTo))
	for _, recipient := range forwardTo {
		records = append(records, ForwardRecipient{
			ID:           uuid.NewString(),
			IdentityID:   identityID,
			EmailAddress: recipient.String(),
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	sort.Slice(records, func(firstIndex int, secondIndex int) bool {
		return records[firstIndex].EmailAddress < records[secondIndex].EmailAddress
	})
	return records
}

func (repository *Repository) newCredential() (string, string, []byte, []byte, []byte, error) {
	usernameToken, usernameErr := repository.randomToken(credentialUsernameBytes)
	if usernameErr != nil {
		return "", "", nil, nil, nil, usernameErr
	}
	passwordToken, passwordErr := repository.randomToken(credentialPasswordBytes)
	if passwordErr != nil {
		return "", "", nil, nil, nil, passwordErr
	}
	salt := make([]byte, credentialSaltBytes)
	if _, readErr := io.ReadFull(repository.random, salt); readErr != nil {
		return "", "", nil, nil, nil, fmt.Errorf("smtp identity credential: salt: %w", readErr)
	}
	password := "pgsmtp_" + passwordToken
	digest := repository.digest(salt, password)
	passwordCipher, cipherErr := repository.encryptPassword(password)
	if cipherErr != nil {
		return "", "", nil, nil, nil, fmt.Errorf("smtp identity credential: password cipher: %w", cipherErr)
	}
	return "smtp_" + usernameToken, password, salt, digest, passwordCipher, nil
}

func (repository *Repository) randomToken(byteCount int) (string, error) {
	rawBytes := make([]byte, byteCount)
	if _, readErr := io.ReadFull(repository.random, rawBytes); readErr != nil {
		return "", fmt.Errorf("smtp identity credential: random: %w", readErr)
	}
	return base64.RawURLEncoding.EncodeToString(rawBytes), nil
}

func (repository *Repository) digest(salt []byte, password string) []byte {
	mac := hmac.New(sha256.New, repository.key)
	mac.Write(salt)
	mac.Write([]byte{0})
	mac.Write([]byte(password))
	return mac.Sum(nil)
}

func (repository *Repository) encryptPassword(password string) ([]byte, error) {
	block, blockErr := aes.NewCipher(repository.key)
	if blockErr != nil {
		return nil, blockErr
	}
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, credentialNonceBytes)
	if _, readErr := io.ReadFull(repository.random, nonce); readErr != nil {
		return nil, readErr
	}
	return gcm.Seal(nonce, nonce, []byte(password), nil), nil
}

func (repository *Repository) decryptPassword(ciphertext []byte) (string, error) {
	block, blockErr := aes.NewCipher(repository.key)
	if blockErr != nil {
		return "", blockErr
	}
	gcm, _ := cipher.NewGCM(block)
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	payload := ciphertext[gcm.NonceSize():]
	plaintext, openErr := gcm.Open(nil, nonce, payload, nil)
	if openErr != nil {
		return "", openErr
	}
	return string(plaintext), nil
}

func publicIdentity(record Identity) PublicIdentity {
	return PublicIdentity{
		ID:           record.ID,
		EmailAddress: record.EmailAddress,
		Username:     record.Username,
		ForwardTo:    publicForwardRecipients(record.ForwardTo),
		Status:       string(record.Status),
		LastUsedAt:   record.LastUsedAt,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}

func publicForwardRecipients(records []ForwardRecipient) []string {
	recipients := make([]string, 0, len(records))
	for _, record := range records {
		recipients = append(recipients, record.EmailAddress)
	}
	sort.Strings(recipients)
	return recipients
}
