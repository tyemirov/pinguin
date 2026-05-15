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
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	var payload struct {
		EmailAddress string   `json:"email_address"`
		ForwardTo    []string `json:"forward_to"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	address, addressErr := smtpidentity.NewAddress(payload.EmailAddress)
	if addressErr != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "email_address is invalid"})
		return
	}
	forwardTo, forwardToErr := parseForwardRecipients(payload.ForwardTo)
	if forwardToErr != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": forwardToErr.Error()})
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
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	identityID := strings.TrimSpace(contextGin.Param("id"))
	if identityID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "identity_id is required"})
		return
	}
	var payload struct {
		ForwardTo []string `json:"forward_to"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	forwardTo, forwardToErr := parseForwardRecipients(payload.ForwardTo)
	if forwardToErr != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": forwardToErr.Error()})
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
	identityID := strings.TrimSpace(contextGin.Param("id"))
	if identityID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "identity_id is required"})
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
	identityID := strings.TrimSpace(contextGin.Param("id"))
	if identityID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "identity_id is required"})
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
	identityID := strings.TrimSpace(contextGin.Param("id"))
	if identityID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "identity_id is required"})
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
	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		return
	}
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
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
	domainID, parseErr := parseSenderDomainID(contextGin.Param("id"))
	if parseErr != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "sender domain id is required"})
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
	claims := claimsFromContextGin(contextGin)
	ownerEmail, ownerErr := smtpidentity.NewAddress(claims.GetUserEmail())
	if ownerErr != nil {
		contextGin.JSON(http.StatusForbidden, gin.H{"error": "authenticated email is required"})
		return smtpidentity.AccessScope{}, false
	}
	admin := sessionHasAdminRole(claims)
	if !admin && handler.repository != nil {
		configuredAdmin, adminErr := handler.repository.IsActiveTenantAdmin(contextGin.Request.Context(), claims.GetUserEmail())
		if adminErr != nil {
			handler.logger.Warn("smtp_identity_admin_lookup_unavailable", "error", adminErr)
		} else {
			admin = configuredAdmin
		}
	}
	return smtpidentity.AccessScope{OwnerEmail: ownerEmail.String(), Admin: admin}, true
}

func (handler *smtpIdentityHandler) writeError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, smtpidentity.ErrInvalidAddress):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "email_address is invalid"})
	case errors.Is(err, smtpidentity.ErrInvalidSenderDomain):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "sender domain is invalid"})
	case errors.Is(err, smtpidentity.ErrSenderDomainNotAllowed):
		contextGin.JSON(http.StatusUnprocessableEntity, gin.H{"error": "sender domain is not verified"})
	case errors.Is(err, smtpidentity.ErrSenderDomainExists):
		contextGin.JSON(http.StatusConflict, gin.H{"error": "sender domain is already registered"})
	case errors.Is(err, smtpidentity.ErrSenderDomainNotFound):
		contextGin.JSON(http.StatusNotFound, gin.H{"error": "sender domain not found"})
	case errors.Is(err, smtpidentity.ErrIdentityExists):
		contextGin.JSON(http.StatusConflict, gin.H{"error": "smtp identity already exists"})
	case errors.Is(err, smtpidentity.ErrIdentityNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		contextGin.JSON(http.StatusNotFound, gin.H{"error": "smtp identity not found"})
	case errors.Is(err, smtpidentity.ErrForwardRecipientsRequired):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "forward_to is required"})
	case errors.Is(err, smtpidentity.ErrForwardRecipientDuplicate):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "forward_to contains duplicate addresses"})
	case errors.Is(err, smtpidentity.ErrForwardRecipientSelf):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "forward_to cannot include the shared sender address"})
	default:
		handler.logger.Error("smtp_identity_handler_error", "error", err)
		contextGin.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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
