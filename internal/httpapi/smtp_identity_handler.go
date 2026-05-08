package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
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
	if !handler.requireAdminSession(contextGin) {
		return
	}
	identities, err := handler.service.List(contextGin.Request.Context())
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, gin.H{"identities": identities})
}

func (handler *smtpIdentityHandler) createIdentity(contextGin *gin.Context) {
	if !handler.requireAdminSession(contextGin) {
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
	credentials, err := handler.service.Create(contextGin.Request.Context(), address, forwardTo)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusCreated, credentials)
}

func (handler *smtpIdentityHandler) updateForwarding(contextGin *gin.Context) {
	if !handler.requireAdminSession(contextGin) {
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
	identity, err := handler.service.UpdateForwarding(contextGin.Request.Context(), identityID, forwardTo)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, identity)
}

func (handler *smtpIdentityHandler) rotateIdentity(contextGin *gin.Context) {
	if !handler.requireAdminSession(contextGin) {
		return
	}
	identityID := strings.TrimSpace(contextGin.Param("id"))
	if identityID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "identity_id is required"})
		return
	}
	credentials, err := handler.service.Rotate(contextGin.Request.Context(), identityID)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, credentials)
}

func (handler *smtpIdentityHandler) deleteIdentity(contextGin *gin.Context) {
	if !handler.requireAdminSession(contextGin) {
		return
	}
	identityID := strings.TrimSpace(contextGin.Param("id"))
	if identityID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "identity_id is required"})
		return
	}
	if err := handler.service.Delete(contextGin.Request.Context(), identityID); err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.Status(http.StatusNoContent)
}

func (handler *smtpIdentityHandler) requireAdminSession(contextGin *gin.Context) bool {
	admin, adminErr := sessionHasAdminAccess(contextGin, handler.repository, claimsFromContextGin(contextGin))
	if adminErr != nil {
		handler.logger.Error("smtp_identity_admin_lookup_error", "error", adminErr)
		contextGin.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return false
	}
	if admin {
		return true
	}
	contextGin.JSON(http.StatusForbidden, gin.H{"error": errAdminAccessRequired.Error()})
	return false
}

func (handler *smtpIdentityHandler) writeError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, smtpidentity.ErrInvalidAddress):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "email_address is invalid"})
	case errors.Is(err, smtpidentity.ErrSenderDomainNotAllowed):
		contextGin.JSON(http.StatusUnprocessableEntity, gin.H{"error": "sender domain is not allowed"})
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
