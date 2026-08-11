package tenant

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	apiKeyPrefix          = "pgn_1_"
	apiSecretByteCount    = 32
	credentialDigestBytes = sha256.Size
	maxDisplayNameLength  = 120
	maxSupportEmailLength = 320
)

var (
	// ErrInvalidOwnerUserID indicates an absent TAuth owner identity.
	ErrInvalidOwnerUserID = errors.New("tenant.owner_user_id.invalid")
	// ErrInvalidTenantID indicates an invalid tenant UUID.
	ErrInvalidTenantID = errors.New("tenant.id.invalid")
	// ErrInvalidDisplayName indicates an invalid tenant display name.
	ErrInvalidDisplayName = errors.New("tenant.display_name.invalid")
	// ErrInvalidSupportEmail indicates an invalid optional support address.
	ErrInvalidSupportEmail = errors.New("tenant.support_email.invalid")
	// ErrInvalidEmailProfile indicates invalid external SMTP delivery data.
	ErrInvalidEmailProfile = errors.New("tenant.email_profile.invalid")
	// ErrInvalidSMSProfile indicates invalid Twilio delivery data.
	ErrInvalidSMSProfile = errors.New("tenant.sms_profile.invalid")
	// ErrInvalidCredentialID indicates an invalid API credential UUID.
	ErrInvalidCredentialID = errors.New("tenant.api_credential.id.invalid")
	// ErrInvalidCredentialDigest indicates an invalid API secret digest.
	ErrInvalidCredentialDigest = errors.New("tenant.api_credential.digest.invalid")
	// ErrInvalidAPIKey indicates an invalid programmatic bearer value.
	ErrInvalidAPIKey = errors.New("tenant.api_key.invalid")
)

// OwnerUserID is a validated TAuth user ID.
type OwnerUserID string

// NewOwnerUserID validates a TAuth user ID.
func NewOwnerUserID(rawValue string) (OwnerUserID, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return "", ErrInvalidOwnerUserID
	}
	return OwnerUserID(normalized), nil
}

// String returns the validated owner user ID.
func (ownerUserID OwnerUserID) String() string {
	return string(ownerUserID)
}

// TenantID is a validated tenant UUID.
type TenantID string

// NewTenantID validates a tenant UUID.
func NewTenantID(rawValue string) (TenantID, error) {
	normalized := strings.TrimSpace(rawValue)
	parsed, parseErr := uuid.Parse(normalized)
	if parseErr != nil || parsed.String() != strings.ToLower(normalized) {
		return "", fmt.Errorf("%w: %s", ErrInvalidTenantID, normalized)
	}
	return TenantID(normalized), nil
}

// String returns the validated tenant UUID.
func (tenantID TenantID) String() string {
	return string(tenantID)
}

// DisplayName is a validated tenant display name.
type DisplayName string

// NewDisplayName validates a tenant display name.
func NewDisplayName(rawValue string) (DisplayName, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" || utf8.RuneCountInString(normalized) > maxDisplayNameLength {
		return "", ErrInvalidDisplayName
	}
	return DisplayName(normalized), nil
}

// String returns the validated display name.
func (displayName DisplayName) String() string {
	return string(displayName)
}

// SupportEmail is a validated optional support address.
type SupportEmail string

// NewSupportEmail validates an optional support address.
func NewSupportEmail(rawValue string) (SupportEmail, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return "", nil
	}
	if len(normalized) > maxSupportEmailLength || !validEmailAddress(normalized) {
		return "", ErrInvalidSupportEmail
	}
	return SupportEmail(normalized), nil
}

// String returns the validated support address.
func (supportEmail SupportEmail) String() string {
	return string(supportEmail)
}

// CredentialID is a validated API credential UUID.
type CredentialID string

// NewCredentialID validates an API credential UUID.
func NewCredentialID(rawValue string) (CredentialID, error) {
	normalized := strings.TrimSpace(rawValue)
	parsed, parseErr := uuid.Parse(normalized)
	if parseErr != nil || parsed.String() != strings.ToLower(normalized) {
		return "", fmt.Errorf("%w: %s", ErrInvalidCredentialID, normalized)
	}
	return CredentialID(normalized), nil
}

// String returns the validated API credential UUID.
func (credentialID CredentialID) String() string {
	return string(credentialID)
}

// DisplayPrefix returns the safe API key prefix.
func (credentialID CredentialID) DisplayPrefix() string {
	return apiKeyPrefix + credentialID.String()[:8]
}

// CredentialDigest is one SHA-256 API secret digest.
type CredentialDigest [credentialDigestBytes]byte

// RequestDigest identifies the normalized input for one idempotent operation.
type RequestDigest [sha256.Size]byte

// NewRequestDigest validates request digest bytes.
func NewRequestDigest(rawValue []byte) (RequestDigest, error) {
	if len(rawValue) != sha256.Size {
		return RequestDigest{}, ErrInvalidCredentialDigest
	}
	var digest RequestDigest
	copy(digest[:], rawValue)
	return digest, nil
}

// Bytes returns a copy of the request digest.
func (digest RequestDigest) Bytes() []byte {
	return append([]byte(nil), digest[:]...)
}

// ParseCredentialDigest validates a base64url API secret digest.
func ParseCredentialDigest(rawValue string) (CredentialDigest, error) {
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(strings.TrimSpace(rawValue))
	if decodeErr != nil || len(decoded) != credentialDigestBytes {
		return CredentialDigest{}, ErrInvalidCredentialDigest
	}
	var digest CredentialDigest
	copy(digest[:], decoded)
	return digest, nil
}

// NewCredentialDigest validates digest bytes.
func NewCredentialDigest(rawValue []byte) (CredentialDigest, error) {
	if len(rawValue) != credentialDigestBytes {
		return CredentialDigest{}, ErrInvalidCredentialDigest
	}
	var digest CredentialDigest
	copy(digest[:], rawValue)
	return digest, nil
}

// Bytes returns a copy of the digest.
func (digest CredentialDigest) Bytes() []byte {
	return append([]byte(nil), digest[:]...)
}

// String returns base64url without padding.
func (digest CredentialDigest) String() string {
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

// APIKey is a validated programmatic bearer key.
type APIKey struct {
	credentialID CredentialID
	digest       CredentialDigest
}

// ParseAPIKey validates the canonical bearer format.
func ParseAPIKey(rawValue string) (APIKey, error) {
	parts := strings.SplitN(strings.TrimSpace(rawValue), "_", 4)
	if len(parts) != 4 || parts[0] != "pgn" || parts[1] != "1" {
		return APIKey{}, ErrInvalidAPIKey
	}
	credentialID, credentialErr := NewCredentialID(parts[2])
	if credentialErr != nil {
		return APIKey{}, ErrInvalidAPIKey
	}
	secret, decodeErr := base64.RawURLEncoding.DecodeString(parts[3])
	if decodeErr != nil || len(secret) != apiSecretByteCount {
		return APIKey{}, ErrInvalidAPIKey
	}
	digestBytes := sha256.Sum256(secret)
	digest := CredentialDigest(digestBytes)
	return APIKey{credentialID: credentialID, digest: digest}, nil
}

// CredentialID returns the key credential ID.
func (apiKey APIKey) CredentialID() CredentialID {
	return apiKey.credentialID
}

// Digest returns the key secret digest.
func (apiKey APIKey) Digest() CredentialDigest {
	return apiKey.digest
}

// EmailProfileInput is validated external SMTP delivery data.
type EmailProfileInput struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
}

// NewEmailProfileInput validates complete external SMTP delivery data.
func NewEmailProfileInput(host string, port int, username string, password string, fromAddress string) (EmailProfileInput, error) {
	normalizedHost := strings.TrimSpace(host)
	normalizedUsername := strings.TrimSpace(username)
	normalizedPassword := strings.TrimSpace(password)
	normalizedFromAddress := strings.TrimSpace(fromAddress)
	if normalizedHost == "" || port < 1 || port > 65535 || normalizedUsername == "" || normalizedPassword == "" || !validEmailAddress(normalizedFromAddress) {
		return EmailProfileInput{}, ErrInvalidEmailProfile
	}
	return EmailProfileInput{
		Host:        normalizedHost,
		Port:        port,
		Username:    normalizedUsername,
		Password:    normalizedPassword,
		FromAddress: normalizedFromAddress,
	}, nil
}

// SMSProfileInput is validated complete Twilio delivery data.
type SMSProfileInput struct {
	AccountSID string
	AuthToken  string
	FromNumber string
}

// NewSMSProfileInput validates complete Twilio delivery data.
func NewSMSProfileInput(accountSID string, authToken string, fromNumber string) (SMSProfileInput, error) {
	normalizedAccountSID := strings.TrimSpace(accountSID)
	normalizedAuthToken := strings.TrimSpace(authToken)
	normalizedFromNumber := strings.TrimSpace(fromNumber)
	if normalizedAccountSID == "" || normalizedAuthToken == "" || len(normalizedFromNumber) < 2 || normalizedFromNumber[0] != '+' {
		return SMSProfileInput{}, ErrInvalidSMSProfile
	}
	return SMSProfileInput{
		AccountSID: normalizedAccountSID,
		AuthToken:  normalizedAuthToken,
		FromNumber: normalizedFromNumber,
	}, nil
}

func validEmailAddress(value string) bool {
	parsed, parseErr := mail.ParseAddress(value)
	return parseErr == nil && parsed.Address == value
}
