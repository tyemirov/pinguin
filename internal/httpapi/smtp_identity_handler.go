package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
)

type smtpIdentityHandler struct {
	service    *smtpidentity.Service
	repository *tenant.Repository
	logger     *slog.Logger
}

func newSMTPIdentityHandler(service *smtpidentity.Service, repository *tenant.Repository, logger *slog.Logger) *smtpIdentityHandler {
	return &smtpIdentityHandler{service: service, repository: repository, logger: logger}
}

func (handler *smtpIdentityHandler) listIdentities(contextGin *gin.Context) {
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	identities, err := handler.service.ListForScope(contextGin.Request.Context(), scope)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, gin.H{"identities": identities})
}

func (handler *smtpIdentityHandler) createIdentity(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	var payload struct {
		EmailAddress string   `json:"email_address"`
		ForwardTo    []string `json:"forward_to"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	address, addressErr := smtpidentity.NewAddress(payload.EmailAddress)
	if addressErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.email_address.invalid", "email_address is invalid")
		return
	}
	forwardTo, forwardToErr := parseForwardRecipients(payload.ForwardTo)
	if forwardToErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.forward_to.invalid", forwardToErr.Error())
		return
	}
	credentials, err := handler.service.CreateForScope(contextGin.Request.Context(), scope, address, forwardTo)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusCreated, credentials)
}

func (handler *smtpIdentityHandler) updateForwarding(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	identityID := strings.TrimSpace(contextGin.Param("identity_id"))
	if identityID == "" {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.id.invalid", "identity_id is required")
		return
	}
	var payload struct {
		ForwardTo []string `json:"forward_to"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	forwardTo, forwardToErr := parseForwardRecipients(payload.ForwardTo)
	if forwardToErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.forward_to.invalid", forwardToErr.Error())
		return
	}
	identity, err := handler.service.UpdateForwardingForScope(contextGin.Request.Context(), scope, identityID, forwardTo)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, identity)
}

func (handler *smtpIdentityHandler) getCredentials(contextGin *gin.Context) {
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	identityID := strings.TrimSpace(contextGin.Param("identity_id"))
	if identityID == "" {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.id.invalid", "identity_id is required")
		return
	}
	credentials, err := handler.service.CredentialsForScope(contextGin.Request.Context(), scope, identityID)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, credentials)
}

func (handler *smtpIdentityHandler) rotateIdentity(contextGin *gin.Context) {
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	identityID := strings.TrimSpace(contextGin.Param("identity_id"))
	if identityID == "" {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.id.invalid", "identity_id is required")
		return
	}
	credentials, err := handler.service.RotateForScope(contextGin.Request.Context(), scope, identityID)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, credentials)
}

func (handler *smtpIdentityHandler) deleteIdentity(contextGin *gin.Context) {
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	identityID := strings.TrimSpace(contextGin.Param("identity_id"))
	if identityID == "" {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.id.invalid", "identity_id is required")
		return
	}
	if err := handler.service.DeleteForScope(contextGin.Request.Context(), scope, identityID); err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.Status(http.StatusNoContent)
}

func (handler *smtpIdentityHandler) listSenderDomains(contextGin *gin.Context) {
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	domains, err := handler.service.ListSenderDomains(contextGin.Request.Context(), scope)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, gin.H{"domains": domains})
}

func (handler *smtpIdentityHandler) createSenderDomain(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	domain, err := handler.service.CreateSenderDomain(contextGin.Request.Context(), scope, payload.Domain)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusCreated, domain)
}

func (handler *smtpIdentityHandler) checkSenderDomainDNS(contextGin *gin.Context) {
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	domainID, parseErr := parseSenderDomainID(contextGin.Param("domain_id"))
	if parseErr != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_domain.id.invalid", "sender domain id is required")
		return
	}
	domain, err := handler.service.CheckSenderDomainDNS(contextGin.Request.Context(), scope, domainID)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, domain)
}

func (handler *smtpIdentityHandler) requireAccessScope(contextGin *gin.Context) (smtpidentity.AccessScope, bool) {
	ownerUserID, tenantID, identifiersErr := tenantIdentifiers(contextGin)
	if identifiersErr != nil {
		writeAPIError(contextGin, http.StatusNotFound, "tenant.not_found", "tenant was not found")
		return smtpidentity.AccessScope{}, false
	}
	if _, ownedErr := handler.repository.GetOwned(contextGin.Request.Context(), ownerUserID, tenantID); ownedErr != nil {
		writeAPIError(contextGin, http.StatusNotFound, "tenant.not_found", "tenant was not found")
		return smtpidentity.AccessScope{}, false
	}
	return smtpidentity.AccessScope{TenantID: tenantID.String()}, true
}

func (handler *smtpIdentityHandler) writeError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, smtpidentity.ErrInvalidAddress):
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.email_address.invalid", "email_address is invalid")
	case errors.Is(err, smtpidentity.ErrInvalidSenderDomain):
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_domain.invalid", "sender domain is invalid")
	case errors.Is(err, smtpidentity.ErrSenderDomainNotAllowed):
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "smtp_domain.unverified", "sender domain is not verified")
	case errors.Is(err, smtpidentity.ErrSenderDomainExists):
		writeAPIError(contextGin, http.StatusConflict, "smtp_domain.conflict", "sender domain is already registered")
	case errors.Is(err, smtpidentity.ErrSenderDomainNotFound):
		writeAPIError(contextGin, http.StatusNotFound, "smtp_domain.not_found", "sender domain was not found")
	case errors.Is(err, smtpidentity.ErrIdentityExists):
		writeAPIError(contextGin, http.StatusConflict, "smtp_identity.conflict", "SMTP identity already exists")
	case errors.Is(err, smtpidentity.ErrIdentityNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		writeAPIError(contextGin, http.StatusNotFound, "smtp_identity.not_found", "SMTP identity was not found")
	case errors.Is(err, smtpidentity.ErrForwardRecipientsRequired):
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.forward_to.required", "forward_to is required")
	case errors.Is(err, smtpidentity.ErrForwardRecipientDuplicate):
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.forward_to.duplicate", "forward_to contains duplicate addresses")
	case errors.Is(err, smtpidentity.ErrForwardRecipientSelf):
		writeAPIError(contextGin, http.StatusBadRequest, "smtp_identity.forward_to.self", "forward_to cannot include the shared sender address")
	default:
		handler.logger.Error("smtp_identity_handler_error", "error", err)
		writeAPIError(contextGin, http.StatusInternalServerError, "internal.error", "internal server error")
	}
}

func parseSenderDomainID(rawID string) (uint, error) {
	trimmedID := strings.TrimSpace(rawID)
	if trimmedID == "" {
		return 0, errors.New("sender domain id is required")
	}
	parsedID, parseErr := strconv.ParseUint(trimmedID, 10, 64)
	if parseErr != nil || parsedID == 0 {
		return 0, errors.New("sender domain id is required")
	}
	return uint(parsedID), nil
}

func parseForwardRecipients(values []string) ([]smtpidentity.Address, error) {
	if len(values) == 0 {
		return nil, smtpidentity.ErrForwardRecipientsRequired
	}
	recipients := make([]smtpidentity.Address, 0, len(values))
	for _, value := range values {
		recipient, recipientErr := smtpidentity.NewAddress(value)
		if recipientErr != nil {
			return nil, errors.New("forward_to contains an invalid address")
		}
		recipients = append(recipients, recipient)
	}
	return recipients, nil
}
