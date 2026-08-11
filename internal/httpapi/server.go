package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
	"gorm.io/gorm"
)

const (
	contextKeyClaims         = "auth_claims"
	defaultTimeout           = 5 * time.Second
	scheduledTimeFutureError = "scheduled_time must be in the future"
	notificationSearchParam  = "q"
	notificationLimitParam   = "limit"
	notificationCursorParam  = "cursor"
	unknownSourceIP          = "unknown"
)

// SessionValidator exposes the subset of validator behaviour we depend on.
type SessionValidator interface {
	ValidateRequest(request *http.Request) (*sessionvalidator.Claims, error)
}

// Config captures all inputs required to construct the HTTP server.
type Config struct {
	ListenAddr           string
	AllowedOrigins       []string
	TrustedProxies       []string
	SessionValidator     SessionValidator
	NotificationService  service.NotificationService
	SMTPIdentityService  *smtpidentity.Service
	TenantRepository     *tenant.Repository
	Logger               *slog.Logger
	ReadHeaderTimeout    time.Duration
	ShutdownGraceTimeout time.Duration
}

// Server hosts authenticated HTTP endpoints and static assets for the UI.
type Server struct {
	config     Config
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer wires Gin, middleware, and handlers for the HTTP API.
func NewServer(cfg Config) (*Server, error) {
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return nil, errors.New("httpapi: listen address is required")
	}
	if cfg.SessionValidator == nil {
		return nil, errors.New("httpapi: session validator is required")
	}
	if cfg.NotificationService == nil {
		return nil, errors.New("httpapi: notification service is required")
	}
	if cfg.TenantRepository == nil {
		return nil, errors.New("httpapi: tenant repository is required")
	}
	if cfg.Logger == nil {
		return nil, errors.New("httpapi: logger is required")
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	if err := engine.SetTrustedProxies(normalizeTrustedProxies(cfg.TrustedProxies)); err != nil {
		return nil, fmt.Errorf("httpapi: trusted proxies: %w", err)
	}
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(cfg.Logger))
	engine.Use(buildCORS(cfg.AllowedOrigins))

	engine.GET("/runtime-config", serveRuntimeConfig())
	engine.GET("/healthz", func(contextGin *gin.Context) {
		contextGin.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	protected := engine.Group("/api")
	protected.Use(sessionMiddleware(cfg.SessionValidator))
	protected.Use(noStore())

	handler := newNotificationHandler(cfg.NotificationService, cfg.TenantRepository, cfg.Logger)
	tenantHandler := newTenantHandler(cfg.TenantRepository, cfg.Logger)
	protected.GET("/tenants", tenantHandler.list)
	protected.POST("/tenants", tenantHandler.create)
	protected.GET("/tenants/:tenant_id", tenantHandler.get)
	protected.PUT("/tenants/:tenant_id", tenantHandler.update)
	protected.DELETE("/tenants/:tenant_id", tenantHandler.delete)
	protected.GET("/tenants/:tenant_id/email-profile", tenantHandler.getEmailProfile)
	protected.PUT("/tenants/:tenant_id/email-profile", tenantHandler.putEmailProfile)
	protected.PATCH("/tenants/:tenant_id/email-profile", tenantHandler.patchEmailProfile)
	protected.GET("/tenants/:tenant_id/sms-profile", tenantHandler.getSMSProfile)
	protected.PUT("/tenants/:tenant_id/sms-profile", tenantHandler.putSMSProfile)
	protected.PATCH("/tenants/:tenant_id/sms-profile", tenantHandler.patchSMSProfile)
	protected.GET("/tenants/:tenant_id/api-credential", tenantHandler.getCredential)
	protected.PUT("/tenants/:tenant_id/api-credential", tenantHandler.rotateCredential)
	protected.GET("/tenants/:tenant_id/notifications", handler.listNotifications)
	protected.PATCH("/tenants/:tenant_id/notifications/:notification_id", handler.patchNotification)
	if cfg.SMTPIdentityService != nil {
		identityHandler := newSMTPIdentityHandler(cfg.SMTPIdentityService, cfg.TenantRepository, cfg.Logger)
		protected.GET("/tenants/:tenant_id/smtp-domains", identityHandler.listSenderDomains)
		protected.POST("/tenants/:tenant_id/smtp-domains", identityHandler.createSenderDomain)
		protected.POST("/tenants/:tenant_id/smtp-domains/:domain_id/dns-checks", identityHandler.checkSenderDomainDNS)
		protected.GET("/tenants/:tenant_id/smtp-identities", identityHandler.listIdentities)
		protected.POST("/tenants/:tenant_id/smtp-identities", identityHandler.createIdentity)
		protected.PATCH("/tenants/:tenant_id/smtp-identities/:identity_id", identityHandler.updateForwarding)
		protected.GET("/tenants/:tenant_id/smtp-identities/:identity_id/credential", identityHandler.getCredentials)
		protected.PUT("/tenants/:tenant_id/smtp-identities/:identity_id/credential", identityHandler.rotateIdentity)
		protected.DELETE("/tenants/:tenant_id/smtp-identities/:identity_id", identityHandler.deleteIdentity)
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           engine,
		ReadHeaderTimeout: pickDuration(cfg.ReadHeaderTimeout, defaultTimeout),
	}

	return &Server{
		config:     cfg,
		httpServer: httpServer,
		logger:     cfg.Logger,
	}, nil
}

// Start begins serving HTTP traffic.
func (server *Server) Start() error {
	err := server.httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully terminates the HTTP server.
func (server *Server) Shutdown(ctx context.Context) error {
	timeout := pickDuration(server.config.ShutdownGraceTimeout, defaultTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return server.httpServer.Shutdown(ctx)
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(contextGin *gin.Context) {
		started := time.Now()
		contextGin.Next()
		logger.Info(
			"http_request_completed",
			"method", contextGin.Request.Method,
			"path", contextGin.Request.URL.Path,
			"status", contextGin.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
			"source_ip", sourceIPForContext(contextGin),
			"remote_addr", remoteAddressForRequest(contextGin.Request),
			"user_agent", contextGin.Request.UserAgent(),
		)
	}
}

func normalizeTrustedProxies(trustedProxies []string) []string {
	normalizedTrustedProxies := make([]string, 0, len(trustedProxies))
	for _, trustedProxy := range trustedProxies {
		normalizedTrustedProxy := strings.TrimSpace(trustedProxy)
		if normalizedTrustedProxy != "" {
			normalizedTrustedProxies = append(normalizedTrustedProxies, normalizedTrustedProxy)
		}
	}
	if len(normalizedTrustedProxies) == 0 {
		return nil
	}
	return normalizedTrustedProxies
}

func sourceIPForContext(contextGin *gin.Context) string {
	sourceIP := strings.TrimSpace(contextGin.ClientIP())
	if sourceIP == "" {
		return unknownSourceIP
	}
	return sourceIP
}

func remoteAddressForRequest(request *http.Request) string {
	return remoteAddressForValue(request.RemoteAddr)
}

func remoteAddressForValue(remoteAddress string) string {
	normalizedAddress := strings.TrimSpace(remoteAddress)
	if normalizedAddress == "" {
		return unknownSourceIP
	}
	return normalizedAddress
}

func buildCORS(allowedOrigins []string) gin.HandlerFunc {
	if len(allowedOrigins) == 0 {
		cfg := cors.Config{
			AllowAllOrigins:  true,
			AllowHeaders:     []string{"Content-Type", "Idempotency-Key", "If-Match", "X-Request-ID", "X-Requested-With", "X-Client-Data", "X-Client"},
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
			AllowCredentials: false,
		}
		return cors.New(cfg)
	}
	cfg := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     []string{"Content-Type", "Idempotency-Key", "If-Match", "X-Request-ID", "X-Requested-With", "X-Client-Data", "X-Client"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowCredentials: true,
	}
	return cors.New(cfg)
}

func noStore() gin.HandlerFunc {
	return func(contextGin *gin.Context) {
		contextGin.Header("Cache-Control", "private, no-store")
		contextGin.Next()
	}
}

func sessionMiddleware(validator SessionValidator) gin.HandlerFunc {
	return func(contextGin *gin.Context) {
		claims, err := validator.ValidateRequest(contextGin.Request)
		if err != nil {
			contextGin.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		contextGin.Set(contextKeyClaims, claims)
		contextGin.Next()
	}
}

type notificationHandler struct {
	service    service.NotificationService
	repository *tenant.Repository
	logger     *slog.Logger
}

func newNotificationHandler(svc service.NotificationService, repo *tenant.Repository, logger *slog.Logger) *notificationHandler {
	return &notificationHandler{service: svc, repository: repo, logger: logger}
}

func (handler *notificationHandler) listNotifications(contextGin *gin.Context) {
	requestContext, resolveErr := handler.resolveNotificationContext(contextGin)
	if resolveErr != nil {
		handler.writeTenantResolutionError(contextGin, resolveErr)
		return
	}
	filter, pageRequest, parseErr := parseNotificationListRequest(contextGin)
	if parseErr != nil {
		writeNotificationListRequestError(contextGin, parseErr)
		return
	}
	page, err := handler.service.ListNotificationsPage(requestContext, filter, pageRequest)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, notificationListPayload{
		Notifications: page.Notifications,
		NextCursor:    page.NextCursor,
	})
}

func (handler *notificationHandler) patchNotification(contextGin *gin.Context) {
	if !requireJSON(contextGin) {
		return
	}
	notificationID := strings.TrimSpace(contextGin.Param("notification_id"))
	if notificationID == "" {
		writeAPIError(contextGin, http.StatusBadRequest, "notification.id.invalid", "notification_id is required")
		return
	}
	var payload struct {
		ScheduledTime *string `json:"scheduled_time"`
		Status        *string `json:"status"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		writeAPIError(contextGin, http.StatusBadRequest, "request.json.invalid", "request body is invalid JSON")
		return
	}
	if (payload.ScheduledTime == nil) == (payload.Status == nil) {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "notification.change.invalid", "set scheduled_time or status")
		return
	}
	requestContext, resolveErr := handler.resolveNotificationContext(contextGin)
	if resolveErr != nil {
		handler.writeTenantResolutionError(contextGin, resolveErr)
		return
	}
	var response model.NotificationResponse
	var serviceErr error
	if payload.ScheduledTime != nil {
		parsedTime, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.ScheduledTime))
		if parseErr != nil || !parsedTime.After(time.Now().UTC()) {
			writeAPIError(contextGin, http.StatusUnprocessableEntity, "notification.scheduled_time.invalid", "scheduled_time must be a future RFC3339 value")
			return
		}
		response, serviceErr = handler.service.RescheduleNotification(requestContext, notificationID, parsedTime.UTC())
	} else if strings.TrimSpace(*payload.Status) == string(model.StatusCancelled) {
		response, serviceErr = handler.service.CancelNotification(requestContext, notificationID)
	} else {
		writeAPIError(contextGin, http.StatusUnprocessableEntity, "notification.status.invalid", "status must be cancelled")
		return
	}
	if serviceErr != nil {
		handler.writeError(contextGin, serviceErr)
		return
	}
	contextGin.JSON(http.StatusOK, response)
}

func (handler *notificationHandler) writeError(contextGin *gin.Context, err error) {
	switch {
	case isMissingNotificationID(err):
		writeAPIError(contextGin, http.StatusBadRequest, "notification.id.invalid", "notification_id is required")
	case errors.Is(err, service.ErrNotificationNotEditable):
		writeAPIError(contextGin, http.StatusConflict, "notification.state.conflict", "notification can only be edited while queued")
	case errors.Is(err, model.ErrNotificationNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		writeAPIError(contextGin, http.StatusNotFound, "notification.not_found", "notification was not found")
	default:
		handler.logger.Error("http_handler_error", "error", err)
		writeAPIError(contextGin, http.StatusInternalServerError, "internal.error", "internal server error")
	}
}

func (handler *notificationHandler) resolveNotificationContext(contextGin *gin.Context) (context.Context, error) {
	ownerUserID, ownerErr := ownerFromContext(contextGin)
	if ownerErr != nil {
		return nil, ownerErr
	}
	tenantID, tenantErr := tenant.NewTenantID(contextGin.Param("tenant_id"))
	if tenantErr != nil {
		return nil, tenant.ErrTenantNotFound
	}
	if _, ownedErr := handler.repository.GetOwned(contextGin.Request.Context(), ownerUserID, tenantID); ownedErr != nil {
		return nil, ownedErr
	}
	targetCfg, err := handler.repository.ResolveByID(contextGin.Request.Context(), tenantID.String())
	if err != nil {
		return nil, err
	}
	return tenant.WithRuntime(contextGin.Request.Context(), targetCfg), nil
}

func claimsFromContextGin(contextGin *gin.Context) *sessionvalidator.Claims {
	return contextGin.MustGet(contextKeyClaims).(*sessionvalidator.Claims)
}

func (handler *notificationHandler) writeTenantResolutionError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, tenant.ErrTenantNotFound), errors.Is(err, tenant.ErrInvalidTenantID), errors.Is(err, gorm.ErrRecordNotFound):
		writeAPIError(contextGin, http.StatusNotFound, "tenant.not_found", "tenant was not found")
	default:
		handler.logger.Error("http_handler_error", "error", err)
		writeAPIError(contextGin, http.StatusInternalServerError, "internal.error", "internal server error")
	}
}

func isMissingNotificationID(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "missing notification_id")
}

func parseStatusFilters(values []string) []model.NotificationStatus {
	if len(values) == 0 {
		return nil
	}
	unique := make(map[model.NotificationStatus]struct{}, len(values))
	var statuses []model.NotificationStatus
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		status := model.NotificationStatus(strings.ToLower(trimmed))
		if _, exists := unique[status]; exists {
			continue
		}
		unique[status] = struct{}{}
		statuses = append(statuses, status)
	}
	return statuses
}

type notificationListPayload struct {
	Notifications []model.NotificationResponse `json:"notifications"`
	NextCursor    string                       `json:"next_cursor,omitempty"`
}

func parseNotificationListRequest(contextGin *gin.Context) (model.NotificationListFilters, model.NotificationListPageRequest, error) {
	searchQuery, searchErr := model.NewNotificationSearchQuery(contextGin.Query(notificationSearchParam))
	if searchErr != nil {
		return model.NotificationListFilters{}, model.NotificationListPageRequest{}, searchErr
	}
	cursor, cursorErr := model.ParseNotificationListCursor(contextGin.Query(notificationCursorParam))
	if cursorErr != nil {
		return model.NotificationListFilters{}, model.NotificationListPageRequest{}, cursorErr
	}
	limit, limitErr := parseNotificationListLimit(contextGin.Query(notificationLimitParam))
	if limitErr != nil {
		return model.NotificationListFilters{}, model.NotificationListPageRequest{}, limitErr
	}
	pageRequest, pageErr := model.NewNotificationListPageRequest(limit, cursor)
	if pageErr != nil {
		return model.NotificationListFilters{}, model.NotificationListPageRequest{}, pageErr
	}
	filter := model.NotificationListFilters{
		Statuses:    parseStatusFilters(contextGin.QueryArray("status")),
		SearchQuery: searchQuery,
	}
	return filter, pageRequest, nil
}

func parseNotificationListLimit(rawValue string) (int, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return model.DefaultNotificationListPageRequest().Limit(), nil
	}
	parsed, parseErr := strconv.Atoi(normalized)
	if parseErr != nil {
		return 0, fmt.Errorf("%w: parse", model.ErrInvalidNotificationLimit)
	}
	return parsed, nil
}

func writeNotificationListRequestError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrInvalidNotificationSearch):
		writeAPIError(contextGin, http.StatusBadRequest, "notification.query.invalid", "q must be 200 characters or fewer")
	case errors.Is(err, model.ErrInvalidNotificationCursor):
		writeAPIError(contextGin, http.StatusBadRequest, "notification.cursor.invalid", "cursor is invalid")
	case errors.Is(err, model.ErrInvalidNotificationLimit):
		writeAPIError(contextGin, http.StatusBadRequest, "notification.limit.invalid", "limit must be between 1 and 100")
	default:
		writeAPIError(contextGin, http.StatusBadRequest, "notification.list.invalid", "notification list request is invalid")
	}
}

func pickDuration(candidate time.Duration, fallback time.Duration) time.Duration {
	if candidate <= 0 {
		return fallback
	}
	return candidate
}

type runtimeConfigPayload struct {
	APIBaseURL   string `json:"apiBaseUrl"`
	TenantURL    string `json:"tenantUrl"`
	EventLogURL  string `json:"eventLogUrl"`
	SMTPRelayURL string `json:"smtpRelayUrl"`
}

func serveRuntimeConfig() gin.HandlerFunc {
	return func(contextGin *gin.Context) {
		payload := runtimeConfigPayload{
			APIBaseURL:   buildAPIBaseURL(contextGin.Request),
			TenantURL:    "/tenants.html",
			EventLogURL:  "/event-log.html",
			SMTPRelayURL: "/smtp-relay.html",
		}
		contextGin.JSON(http.StatusOK, payload)
	}
}

func buildAPIBaseURL(request *http.Request) string {
	if request == nil {
		return "/api"
	}
	scheme := "http"
	if proto := request.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if request.TLS != nil {
		scheme = "https"
	}
	host := request.Host
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s://%s/api", scheme, host)
}
