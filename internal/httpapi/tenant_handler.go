package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
)

const (
	idempotencyKeyHeader = "Idempotency-Key"
	ifMatchHeader        = "If-Match"
	etagHeader           = "ETag"
	maxIdempotencyKey    = 200
)

type tenantHandler struct {
	repository *tenant.Repository
	logger     *slog.Logger
}

type emailProfilePayload struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
}

type smsProfilePayload struct {
	AccountSID string `json:"account_sid"`
	AuthToken  string `json:"auth_token"`
	FromNumber string `json:"from_number"`
}

type credentialPayload struct {
	ID           string `json:"id"`
	SecretDigest string `json:"secret_digest"`
}

type createTenantPayload struct {
	DisplayName   string              `json:"display_name"`
	SupportEmail  string              `json:"support_email"`
	EmailProfile  emailProfilePayload `json:"email_profile"`
	SMSProfile    *smsProfilePayload  `json:"sms_profile"`
	APICredential credentialPayload   `json:"api_credential"`
}

func newTenantHandler(repository *tenant.Repository, logger *slog.Logger) *tenantHandler {
	return &tenantHandler{repository: repository, logger: logger}
}

func (handler *tenantHandler) list(contextGin *gin.Context) {
	ownerUserID, ownerErr := ownerFromContext(contextGin)
	if ownerErr != nil {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.owner.invalid", "authenticated user ID is invalid")
		return
	}
	resources, listErr := handler.repository.ListOwned(contextGin.Request.Context(), ownerUserID)
	if listErr != nil {
		handler.writeError(contextGin, listErr)
		return
	}
	contextGin.JSON(http.StatusOK, gin.H{"tenants": resources})
}

func (handler *tenantHandler) create(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	requestKey := strings.TrimSpace(contextGin.GetHeader(idempotencyKeyHeader))
	if requestKey == "" || len(requestKey) > maxIdempotencyKey {
		writeAPIError(contextGin, http.StatusBadRequest, "tenant.idempotency_key.invalid", "Idempotency-Key is required")
		return
	}
	ownerUserID, ownerErr := ownerFromContext(contextGin)
	if ownerErr != nil {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.owner.invalid", "authenticated user ID is invalid")
		return
	}
	var payload createTenantPayload
	if bindErr := contextGin.ShouldBindJSON(&payload); bindErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	input, inputErr := createInputFromPayload(ownerUserID, payload)
	if inputErr != nil {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.create.invalid", "tenant data is invalid or incomplete")
		return
	}
	canonicalBody, _ := json.Marshal(input)
	requestHash := sha256.Sum256(canonicalBody)
	requestDigest, _ := tenant.NewRequestDigest(requestHash[:])
	result, createErr := handler.repository.Create(contextGin.Request.Context(), input, requestKey, requestDigest)
	if createErr != nil {
		handler.writeError(contextGin, createErr)
		return
	}
	contextGin.Header(etagHeader, etag(result.Resource.Version))
	contextGin.JSON(http.StatusCreated, result.Resource)
}

func (handler *tenantHandler) get(contextGin *gin.Context) {
	resource, resourceErr := handler.ownedResource(contextGin)
	if resourceErr != nil {
		handler.writeError(contextGin, resourceErr)
		return
	}
	contextGin.Header(etagHeader, etag(resource.Version))
	contextGin.JSON(http.StatusOK, resource)
}

func (handler *tenantHandler) update(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	ownerUserID, tenantID, identifiersErr := tenantIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeError(contextGin, identifiersErr)
		return
	}
	expectedVersion, versionErr := parseIfMatch(contextGin)
	if versionErr != nil {
		writeAPIError(contextGin, http.StatusPreconditionFailed, "tenant.version.precondition_failed", "If-Match does not contain a current tenant version")
		return
	}
	var payload struct {
		DisplayName  string `json:"display_name"`
		SupportEmail string `json:"support_email"`
	}
	if bindErr := contextGin.ShouldBindJSON(&payload); bindErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	displayName, displayNameErr := tenant.NewDisplayName(payload.DisplayName)
	supportEmail, supportEmailErr := tenant.NewSupportEmail(payload.SupportEmail)
	if displayNameErr != nil || supportEmailErr != nil {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.metadata.invalid", "tenant metadata is invalid")
		return
	}
	resource, updateErr := handler.repository.UpdateMetadata(contextGin.Request.Context(), ownerUserID, tenantID, expectedVersion, tenant.MetadataInput{
		DisplayName: displayName, SupportEmail: supportEmail,
	})
	if updateErr != nil {
		handler.writeError(contextGin, updateErr)
		return
	}
	contextGin.Header(etagHeader, etag(resource.Version))
	contextGin.JSON(http.StatusOK, resource)
}

func (handler *tenantHandler) delete(contextGin *gin.Context) {
	ownerUserID, tenantID, identifiersErr := tenantIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeError(contextGin, identifiersErr)
		return
	}
	expectedVersion, versionErr := parseIfMatch(contextGin)
	if versionErr != nil {
		writeAPIError(contextGin, http.StatusPreconditionFailed, "tenant.version.precondition_failed", "If-Match does not contain a current tenant version")
		return
	}
	if deleteErr := handler.repository.Delete(contextGin.Request.Context(), ownerUserID, tenantID, expectedVersion); deleteErr != nil {
		handler.writeError(contextGin, deleteErr)
		return
	}
	contextGin.Status(http.StatusNoContent)
}

func (handler *tenantHandler) getEmailProfile(contextGin *gin.Context) {
	resource, resourceErr := handler.ownedResource(contextGin)
	if resourceErr != nil {
		handler.writeError(contextGin, resourceErr)
		return
	}
	contextGin.Header(etagHeader, etag(resource.EmailProfile.Version))
	contextGin.JSON(http.StatusOK, resource.EmailProfile)
}

func (handler *tenantHandler) putEmailProfile(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	ownerUserID, tenantID, expectedVersion, identifiersErr := profileIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeProfileError(contextGin, identifiersErr)
		return
	}
	var payload emailProfilePayload
	if bindErr := contextGin.ShouldBindJSON(&payload); bindErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	input, inputErr := emailInput(payload)
	if inputErr != nil {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.email_profile.invalid", "email profile is invalid or incomplete")
		return
	}
	profile, replaceErr := handler.repository.ReplaceEmailProfile(contextGin.Request.Context(), ownerUserID, tenantID, expectedVersion, input)
	if replaceErr != nil {
		handler.writeProfileError(contextGin, replaceErr)
		return
	}
	contextGin.Header(etagHeader, etag(profile.Version))
	contextGin.JSON(http.StatusOK, profile)
}

func (handler *tenantHandler) patchEmailProfile(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	ownerUserID, tenantID, expectedVersion, identifiersErr := profileIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeProfileError(contextGin, identifiersErr)
		return
	}
	var payload struct {
		Host        *string `json:"host"`
		Port        *int    `json:"port"`
		Username    *string `json:"username"`
		Password    *string `json:"password"`
		FromAddress *string `json:"from_address"`
	}
	if bindErr := contextGin.ShouldBindJSON(&payload); bindErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	if payload.Host == nil && payload.Port == nil && payload.Username == nil && payload.Password == nil && payload.FromAddress == nil || masked(payload.Username) || masked(payload.Password) {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.email_profile.patch.invalid", "email profile changes are invalid")
		return
	}
	profile, patchErr := handler.repository.PatchEmailProfile(contextGin.Request.Context(), ownerUserID, tenantID, expectedVersion, tenant.EmailProfilePatch{
		Host: payload.Host, Port: payload.Port, Username: payload.Username, Password: payload.Password, FromAddress: payload.FromAddress,
	})
	if patchErr != nil {
		handler.writeProfileError(contextGin, patchErr)
		return
	}
	contextGin.Header(etagHeader, etag(profile.Version))
	contextGin.JSON(http.StatusOK, profile)
}

func (handler *tenantHandler) getSMSProfile(contextGin *gin.Context) {
	resource, resourceErr := handler.ownedResource(contextGin)
	if resourceErr != nil {
		handler.writeError(contextGin, resourceErr)
		return
	}
	if resource.SMSProfile == nil {
		writeAPIError(contextGin, http.StatusNotFound, "tenant.sms_profile.not_found", "SMS profile was not found")
		return
	}
	contextGin.Header(etagHeader, etag(resource.SMSProfile.Version))
	contextGin.JSON(http.StatusOK, resource.SMSProfile)
}

func (handler *tenantHandler) putSMSProfile(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	ownerUserID, tenantID, expectedVersion, identifiersErr := profileIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeProfileError(contextGin, identifiersErr)
		return
	}
	var payload smsProfilePayload
	if bindErr := contextGin.ShouldBindJSON(&payload); bindErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	input, inputErr := smsInput(payload)
	if inputErr != nil {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.sms_profile.invalid", "SMS profile is invalid or incomplete")
		return
	}
	profile, replaceErr := handler.repository.ReplaceSMSProfile(contextGin.Request.Context(), ownerUserID, tenantID, expectedVersion, input)
	if replaceErr != nil {
		handler.writeProfileError(contextGin, replaceErr)
		return
	}
	contextGin.Header(etagHeader, etag(profile.Version))
	contextGin.JSON(http.StatusOK, profile)
}

func (handler *tenantHandler) patchSMSProfile(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	ownerUserID, tenantID, expectedVersion, identifiersErr := profileIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeProfileError(contextGin, identifiersErr)
		return
	}
	var payload struct {
		AccountSID *string `json:"account_sid"`
		AuthToken  *string `json:"auth_token"`
		FromNumber *string `json:"from_number"`
	}
	if bindErr := contextGin.ShouldBindJSON(&payload); bindErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	if payload.AccountSID == nil && payload.AuthToken == nil && payload.FromNumber == nil || masked(payload.AccountSID) || masked(payload.AuthToken) {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.sms_profile.patch.invalid", "SMS profile changes are invalid")
		return
	}
	profile, patchErr := handler.repository.PatchSMSProfile(contextGin.Request.Context(), ownerUserID, tenantID, expectedVersion, tenant.SMSProfilePatch{
		AccountSID: payload.AccountSID, AuthToken: payload.AuthToken, FromNumber: payload.FromNumber,
	})
	if patchErr != nil {
		handler.writeProfileError(contextGin, patchErr)
		return
	}
	contextGin.Header(etagHeader, etag(profile.Version))
	contextGin.JSON(http.StatusOK, profile)
}

func (handler *tenantHandler) getCredential(contextGin *gin.Context) {
	ownerUserID, tenantID, identifiersErr := tenantIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeError(contextGin, identifiersErr)
		return
	}
	credential, credentialErr := handler.repository.GetCredential(contextGin.Request.Context(), ownerUserID, tenantID)
	if credentialErr != nil {
		handler.writeError(contextGin, credentialErr)
		return
	}
	contextGin.Header(etagHeader, etag(credential.Version))
	contextGin.JSON(http.StatusOK, credential)
}

func (handler *tenantHandler) rotateCredential(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	ownerUserID, tenantID, expectedVersion, identifiersErr := profileIdentifiers(contextGin)
	if identifiersErr != nil {
		handler.writeProfileError(contextGin, identifiersErr)
		return
	}
	var payload credentialPayload
	if bindErr := contextGin.ShouldBindJSON(&payload); bindErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	credentialID, credentialIDErr := tenant.NewCredentialID(payload.ID)
	digest, digestErr := tenant.ParseCredentialDigest(payload.SecretDigest)
	if credentialIDErr != nil || digestErr != nil {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.api_credential.invalid", "API credential is invalid")
		return
	}
	credential, rotateErr := handler.repository.RotateCredential(contextGin.Request.Context(), ownerUserID, tenantID, expectedVersion, credentialID, digest)
	if rotateErr != nil {
		handler.writeProfileError(contextGin, rotateErr)
		return
	}
	contextGin.Header(etagHeader, etag(credential.Version))
	contextGin.JSON(http.StatusOK, credential)
}

func (handler *tenantHandler) ownedResource(contextGin *gin.Context) (tenant.Resource, error) {
	ownerUserID, tenantID, identifiersErr := tenantIdentifiers(contextGin)
	if identifiersErr != nil {
		return tenant.Resource{}, identifiersErr
	}
	return handler.repository.GetOwned(contextGin.Request.Context(), ownerUserID, tenantID)
}

func (handler *tenantHandler) writeProfileError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, tenant.ErrVersionPrecondition):
		writeAPIError(contextGin, http.StatusPreconditionFailed, "tenant.version.precondition_failed", "If-Match does not contain the current resource version")
	case errors.Is(err, tenant.ErrInvalidEmailProfile):
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.email_profile.invalid", "email profile is invalid")
	case errors.Is(err, tenant.ErrInvalidSMSProfile):
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "tenant.sms_profile.invalid", "SMS profile is invalid")
	default:
		handler.writeError(contextGin, err)
	}
}

func (handler *tenantHandler) writeError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, tenant.ErrTenantNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		writeAPIError(contextGin, http.StatusNotFound, "tenant.not_found", "tenant was not found")
	case errors.Is(err, tenant.ErrVersionPrecondition):
		writeAPIError(contextGin, http.StatusPreconditionFailed, "tenant.version.precondition_failed", "If-Match does not contain the current resource version")
	case errors.Is(err, tenant.ErrIdempotencyConflict):
		writeAPIError(contextGin, http.StatusConflict, "tenant.idempotency_conflict", "Idempotency-Key was already used for a different request")
	default:
		handler.logger.Error("tenant_http_handler_failed", "error", err)
		writeAPIError(contextGin, http.StatusInternalServerError, "internal.error", "internal server error")
	}
}

func createInputFromPayload(ownerUserID tenant.OwnerUserID, payload createTenantPayload) (tenant.CreateInput, error) {
	displayName, displayNameErr := tenant.NewDisplayName(payload.DisplayName)
	supportEmail, supportEmailErr := tenant.NewSupportEmail(payload.SupportEmail)
	emailProfile, emailProfileErr := emailInput(payload.EmailProfile)
	credentialID, credentialIDErr := tenant.NewCredentialID(payload.APICredential.ID)
	credentialDigest, credentialDigestErr := tenant.ParseCredentialDigest(payload.APICredential.SecretDigest)
	if displayNameErr != nil || supportEmailErr != nil || emailProfileErr != nil || credentialIDErr != nil || credentialDigestErr != nil {
		return tenant.CreateInput{}, errors.New("invalid tenant create data")
	}
	var smsProfile *tenant.SMSProfileInput
	if payload.SMSProfile != nil {
		validatedSMSProfile, smsProfileErr := smsInput(*payload.SMSProfile)
		if smsProfileErr != nil {
			return tenant.CreateInput{}, smsProfileErr
		}
		smsProfile = &validatedSMSProfile
	}
	return tenant.CreateInput{
		OwnerUserID: ownerUserID, DisplayName: displayName, SupportEmail: supportEmail,
		EmailProfile: emailProfile, SMSProfile: smsProfile,
		CredentialID: credentialID, CredentialDigest: credentialDigest,
	}, nil
}

func emailInput(payload emailProfilePayload) (tenant.EmailProfileInput, error) {
	if masked(&payload.Username) || masked(&payload.Password) {
		return tenant.EmailProfileInput{}, tenant.ErrInvalidEmailProfile
	}
	return tenant.NewEmailProfileInput(payload.Host, payload.Port, payload.Username, payload.Password, payload.FromAddress)
}

func smsInput(payload smsProfilePayload) (tenant.SMSProfileInput, error) {
	if masked(&payload.AccountSID) || masked(&payload.AuthToken) {
		return tenant.SMSProfileInput{}, tenant.ErrInvalidSMSProfile
	}
	return tenant.NewSMSProfileInput(payload.AccountSID, payload.AuthToken, payload.FromNumber)
}

func ownerFromContext(contextGin *gin.Context) (tenant.OwnerUserID, error) {
	return tenant.NewOwnerUserID(claimsFromContextGin(contextGin).GetUserID())
}

func tenantIdentifiers(contextGin *gin.Context) (tenant.OwnerUserID, tenant.TenantID, error) {
	ownerUserID, ownerErr := ownerFromContext(contextGin)
	if ownerErr != nil {
		return "", "", ownerErr
	}
	tenantID, tenantErr := tenant.NewTenantID(contextGin.Param("tenant_id"))
	if tenantErr != nil {
		return "", "", tenant.ErrTenantNotFound
	}
	return ownerUserID, tenantID, nil
}

func profileIdentifiers(contextGin *gin.Context) (tenant.OwnerUserID, tenant.TenantID, uint64, error) {
	ownerUserID, tenantID, identifiersErr := tenantIdentifiers(contextGin)
	if identifiersErr != nil {
		return "", "", 0, identifiersErr
	}
	expectedVersion, versionErr := parseIfMatch(contextGin)
	if versionErr != nil {
		return "", "", 0, tenant.ErrVersionPrecondition
	}
	return ownerUserID, tenantID, expectedVersion, nil
}

func parseIfMatch(contextGin *gin.Context) (uint64, error) {
	rawVersion := strings.TrimSpace(contextGin.GetHeader(ifMatchHeader))
	if len(rawVersion) < 3 || rawVersion[0] != '"' || rawVersion[len(rawVersion)-1] != '"' {
		return 0, errors.New("invalid If-Match")
	}
	version, parseErr := strconv.ParseUint(rawVersion[1:len(rawVersion)-1], 10, 64)
	if parseErr != nil {
		return 0, errors.New("invalid If-Match")
	}
	return version, nil
}

func etag(version uint64) string {
	return fmt.Sprintf("\"%d\"", version)
}

func requireJSON(contextGin *gin.Context) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contextGin.GetHeader("Content-Type"), ";")[0]))
	if mediaType != "application/json" {
		writeAPIError(contextGin, http.StatusUnsupportedMediaType, "request.media_type.unsupported", "Content-Type must be application/json")
		return false
	}
	return true
}

func masked(value *string) bool {
	if value == nil {
		return false
	}
	normalized := strings.TrimSpace(*value)
	return normalized != "" && strings.Trim(normalized, "*•") == ""
}

func writeAPIError(contextGin *gin.Context, statusCode int, code string, message string) {
	requestID := strings.TrimSpace(contextGin.GetHeader("X-Request-ID"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	contextGin.Header("X-Request-ID", requestID)
	contextGin.JSON(statusCode, gin.H{"error": gin.H{"code": code, "message": message, "request_id": requestID}})
}
