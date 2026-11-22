package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tyemirov/pingwin/internal/delivery"
	"github.com/tyemirov/pingwin/internal/smtpidentity"
)

const emailDeliveriesPath = "/tenants/:tenant_id/email-deliveries"

func (server *Server) listEmailDeliveries(contextGin *gin.Context) {
	identityHandler := newSMTPIdentityHandler(server.config.SMTPIdentityService, server.config.TenantRepository, server.logger)
	scope, authorized := identityHandler.requireAccessScope(contextGin)
	if !authorized {
		return
	}
	if server.config.DeliveryService == nil || server.config.SMTPIdentityService == nil {
		writeAPIError(contextGin, http.StatusServiceUnavailable, "delivery.unavailable", "Email delivery history is not configured.")
		return
	}
	query, err := delivery.NewQuery(contextGin.Query("q"), contextGin.Query("limit"), contextGin.Query("cursor"))
	if err != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "delivery.query.invalid", "Search must be 200 characters or fewer, limit must be between 1 and 100, and cursor must be valid.")
		return
	}
	ctx, cancel := context.WithTimeout(contextGin.Request.Context(), 15*time.Second)
	defer cancel()
	domains, err := server.config.SMTPIdentityService.ListSenderDomains(ctx, scope)
	if err != nil {
		server.logger.Error("delivery_domain_read_failed", "error", err)
		writeAPIError(contextGin, http.StatusInternalServerError, "internal.error", "Unable to check sender domain access.")
		return
	}
	verified := make([]delivery.Domain, 0, len(domains))
	for _, domain := range domains {
		if domain.Status == string(smtpidentity.SenderDomainStatusVerified) {
			verified = append(verified, delivery.Domain{Name: domain.Domain, RegisteredAt: domain.CreatedAt})
		}
	}
	deliveryScope, err := delivery.NewScope(scope.TenantID, verified)
	if err != nil {
		server.logger.Error("delivery_scope_failed", "error", err)
		writeAPIError(contextGin, http.StatusInternalServerError, "internal.error", "Unable to check sender domain access.")
		return
	}
	page, err := server.config.DeliveryService.List(ctx, deliveryScope, query)
	if err != nil {
		if errors.Is(err, delivery.ErrInvalidQuery) {
			writeAPIError(contextGin, http.StatusBadRequest, "delivery.query.invalid", "The delivery cursor is invalid or expired.")
			return
		}
		server.logger.Error("delivery_history_read_failed", "error", err)
		writeAPIError(contextGin, http.StatusBadGateway, "delivery.provider_unavailable", "Email delivery history is temporarily unavailable.")
		return
	}
	contextGin.JSON(http.StatusOK, page)
}
