package smtpidentity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
	activeStatusValue       = string(IdentityStatusActive)
	deletedStatusValue      = string(IdentityStatusDeleted)
	identityIDColumn        = "id"
	lastUsedAtColumn        = "last_used_at"
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
	EmailAddress   string `gorm:"uniqueIndex"`
	Username       string `gorm:"uniqueIndex"`
	PasswordSalt   []byte
	PasswordDigest []byte
	Status         IdentityStatus `gorm:"index"`
	LastUsedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PublicIdentity is the secret-free identity shape exposed to callers.
type PublicIdentity struct {
	ID           string     `json:"id"`
	EmailAddress string     `json:"email_address"`
	Username     string     `json:"username"`
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

// Repository stores and verifies SMTP identities.
type Repository struct {
	db        *gorm.DB
	key       []byte
	random    io.Reader
	clockFunc func() time.Time
}

// NewRepository constructs an SMTP identity repository.
func NewRepository(db *gorm.DB, rawMasterKey string) (*Repository, error) {
	key, decodeErr := hex.DecodeString(strings.TrimSpace(rawMasterKey))
	if decodeErr != nil {
		return nil, fmt.Errorf("smtp identity: invalid master key: %w", decodeErr)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("smtp identity: master key must decode to 32 bytes")
	}
	return &Repository{
		db:        db,
		key:       key,
		random:    rand.Reader,
		clockFunc: func() time.Time { return time.Now().UTC() },
	}, nil
}

// List returns active SMTP identities without secrets.
func (repository *Repository) List(ctx context.Context) ([]PublicIdentity, error) {
	var records []Identity
	if err := repository.db.WithContext(ctx).
		Where(&Identity{Status: IdentityStatusActive}).
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
func (repository *Repository) Create(ctx context.Context, address Address) (PublicIdentity, string, error) {
	if allowedErr := repository.requireSenderDomain(ctx, address.Domain()); allowedErr != nil {
		return PublicIdentity{}, "", allowedErr
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
	username, password, salt, digest, credentialErr := repository.newCredential()
	if credentialErr != nil {
		return PublicIdentity{}, "", credentialErr
	}
	now := repository.clockFunc()
	if findErr == nil {
		existing.Username = username
		existing.PasswordSalt = salt
		existing.PasswordDigest = digest
		existing.Status = IdentityStatusActive
		existing.LastUsedAt = nil
		existing.UpdatedAt = now
		if saveErr := repository.db.WithContext(ctx).Save(&existing).Error; saveErr != nil {
			return PublicIdentity{}, "", fmt.Errorf("smtp identity create: reactivate: %w", saveErr)
		}
		return publicIdentity(existing), password, nil
	}
	record := Identity{
		ID:             uuid.NewString(),
		EmailAddress:   address.String(),
		Username:       username,
		PasswordSalt:   salt,
		PasswordDigest: digest,
		Status:         IdentityStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if createErr := repository.db.WithContext(ctx).Create(&record).Error; createErr != nil {
		return PublicIdentity{}, "", fmt.Errorf("smtp identity create: %w", createErr)
	}
	return publicIdentity(record), password, nil
}

// Rotate replaces credentials for an active identity.
func (repository *Repository) Rotate(ctx context.Context, identityID string) (PublicIdentity, string, error) {
	record, fetchErr := repository.requireIdentity(ctx, identityID)
	if fetchErr != nil {
		return PublicIdentity{}, "", fetchErr
	}
	username, password, salt, digest, credentialErr := repository.newCredential()
	if credentialErr != nil {
		return PublicIdentity{}, "", credentialErr
	}
	record.Username = username
	record.PasswordSalt = salt
	record.PasswordDigest = digest
	record.UpdatedAt = repository.clockFunc()
	if saveErr := repository.db.WithContext(ctx).Save(&record).Error; saveErr != nil {
		return PublicIdentity{}, "", fmt.Errorf("smtp identity rotate: %w", saveErr)
	}
	return publicIdentity(record), password, nil
}

// Delete disables an identity so it can no longer authenticate.
func (repository *Repository) Delete(ctx context.Context, identityID string) error {
	record, fetchErr := repository.requireIdentity(ctx, identityID)
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
	now := repository.clockFunc()
	if markErr := repository.markAuthenticatedIdentityUsed(ctx, record, now); markErr != nil {
		return AuthenticatedIdentity{}, markErr
	}
	address, addressErr := NewAddress(record.EmailAddress)
	if addressErr != nil {
		return AuthenticatedIdentity{}, fmt.Errorf("smtp identity auth: stored address: %w", addressErr)
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

func (repository *Repository) requireIdentity(ctx context.Context, identityID string) (Identity, error) {
	normalizedIdentityID := strings.TrimSpace(identityID)
	var record Identity
	err := repository.db.WithContext(ctx).
		Where(&Identity{ID: normalizedIdentityID, Status: IdentityStatusActive}).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Identity{}, ErrIdentityNotFound
		}
		return Identity{}, fmt.Errorf("smtp identity lookup: identity %s: %w", normalizedIdentityID, err)
	}
	return record, nil
}

func (repository *Repository) requireSenderDomain(ctx context.Context, domain string) error {
	normalizedDomain := strings.ToLower(strings.TrimSpace(domain))
	var domainRecord SenderDomain
	err := repository.db.WithContext(ctx).
		Where(&SenderDomain{Domain: normalizedDomain}).
		First(&domainRecord).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrSenderDomainNotAllowed, normalizedDomain)
		}
		return fmt.Errorf("smtp identity sender domain lookup: domain %s: %w", normalizedDomain, err)
	}
	return nil
}

func (repository *Repository) newCredential() (string, string, []byte, []byte, error) {
	usernameToken, usernameErr := repository.randomToken(credentialUsernameBytes)
	if usernameErr != nil {
		return "", "", nil, nil, usernameErr
	}
	passwordToken, passwordErr := repository.randomToken(credentialPasswordBytes)
	if passwordErr != nil {
		return "", "", nil, nil, passwordErr
	}
	salt := make([]byte, credentialSaltBytes)
	if _, readErr := io.ReadFull(repository.random, salt); readErr != nil {
		return "", "", nil, nil, fmt.Errorf("smtp identity credential: salt: %w", readErr)
	}
	password := "pgsmtp_" + passwordToken
	digest := repository.digest(salt, password)
	return "smtp_" + usernameToken, password, salt, digest, nil
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

func publicIdentity(record Identity) PublicIdentity {
	return PublicIdentity{
		ID:           record.ID,
		EmailAddress: record.EmailAddress,
		Username:     record.Username,
		Status:       string(record.Status),
		LastUsedAt:   record.LastUsedAt,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}
