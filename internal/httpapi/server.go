package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/tenant"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
	"gorm.io/gorm"
	"log/slog"
)

const (
	contextKeyClaims         = "auth_claims"
	defaultTimeout           = 5 * time.Second
	scheduledTimeFutureError = "scheduled_time must be in the future"
	tenantIDQueryParam       = "tenant_id"
)

var (
	errTenantIDRequired = errors.New("tenant_id is required")
)

// SessionValidator exposes the subset of validator behaviour we depend on.
type SessionValidator interface {
	ValidateRequest(request *http.Request) (*sessionvalidator.Claims, error)
}

// Config captures all inputs required to construct the HTTP server.
type Config struct {
	ListenAddr           string
	AllowedOrigins       []string
	SessionValidator     SessionValidator
	NotificationService  service.NotificationService
	TenantRepository     *tenant.Repository
	TAuthBaseURL         string
	TAuthTenantID        string
	TAuthGoogleClientID  string
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
	if strings.TrimSpace(cfg.TAuthBaseURL) == "" {
		return nil, errors.New("httpapi: tauth base url is required")
	}
	if strings.TrimSpace(cfg.TAuthTenantID) == "" {
		return nil, errors.New("httpapi: tauth tenant id is required")
	}
	if strings.TrimSpace(cfg.TAuthGoogleClientID) == "" {
		return nil, errors.New("httpapi: google client id is required")
	}
	if cfg.Logger == nil {
		return nil, errors.New("httpapi: logger is required")
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(cfg.Logger))
	engine.Use(tenantMiddleware(cfg.TenantRepository))
	engine.Use(buildCORS(cfg.AllowedOrigins))

	engine.GET("/runtime-config", serveRuntimeConfig(runtimeConfigTAuth{
		BaseURL:        cfg.TAuthBaseURL,
		TenantID:       cfg.TAuthTenantID,
		GoogleClientID: cfg.TAuthGoogleClientID,
	}))
	engine.GET("/healthz", func(contextGin *gin.Context) {
		contextGin.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	protected := engine.Group("/api")
	protected.Use(sessionMiddleware(cfg.SessionValidator))

	handler := newNotificationHandler(cfg.NotificationService, cfg.TenantRepository, cfg.Logger)
	protected.GET("/notifications", handler.listNotifications)
	protected.PATCH("/notifications/:id/schedule", handler.rescheduleNotification)
	protected.POST("/notifications/:id/cancel", handler.cancelNotification)

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
		)
	}
}

func buildCORS(allowedOrigins []string) gin.HandlerFunc {
	if len(allowedOrigins) == 0 {
		cfg := cors.Config{
			AllowAllOrigins:  true,
			AllowHeaders:     []string{"Content-Type", "X-Requested-With", "X-Client-Data", "X-Client"},
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodOptions},
			AllowCredentials: false,
		}
		return cors.New(cfg)
	}
	cfg := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     []string{"Content-Type", "X-Requested-With", "X-Client-Data", "X-Client"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodOptions},
		AllowCredentials: true,
	}
	return cors.New(cfg)
}

func tenantMiddleware(repo *tenant.Repository) gin.HandlerFunc {
	return func(contextGin *gin.Context) {
		if contextGin.Request != nil && contextGin.Request.URL != nil && contextGin.Request.URL.Path == "/healthz" {
			contextGin.Next()
			return
		}
		runtimeCfg, err := repo.ResolveByHost(contextGin.Request.Context(), contextGin.Request.Host)
		if err != nil {
			contextGin.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "tenant_not_found"})
			return
		}
		ctx := tenant.WithRuntime(contextGin.Request.Context(), runtimeCfg)
		contextGin.Request = contextGin.Request.WithContext(ctx)
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
	if _, ok := tenant.RuntimeFromContext(contextGin.Request.Context()); !ok {
		contextGin.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	statusFilters := contextGin.QueryArray("status")
	filter := model.NotificationListFilters{
		Statuses: parseStatusFilters(statusFilters),
	}
	responses, err := handler.service.ListNotificationsAll(contextGin.Request.Context(), filter)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, gin.H{"notifications": responses})
}

func (handler *notificationHandler) rescheduleNotification(contextGin *gin.Context) {
	notificationID := strings.TrimSpace(contextGin.Param("id"))
	if notificationID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "notification_id is required"})
		return
	}
	var payload struct {
		ScheduledTime string `json:"scheduled_time"`
	}
	if err := contextGin.ShouldBindJSON(&payload); err != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if strings.TrimSpace(payload.ScheduledTime) == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "scheduled_time is required"})
		return
	}
	parsedTime, err := time.Parse(time.RFC3339, payload.ScheduledTime)
	if err != nil {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "scheduled_time must be RFC3339"})
		return
	}
	normalizedTime := parsedTime.UTC()
	if normalizedTime.Before(time.Now().UTC()) {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": scheduledTimeFutureError})
		return
	}
	requestContext, resolveErr := handler.resolveNotificationContext(contextGin)
	if resolveErr != nil {
		handler.writeTenantResolutionError(contextGin, resolveErr)
		return
	}
	response, svcErr := handler.service.RescheduleNotification(requestContext, notificationID, normalizedTime)
	if svcErr != nil {
		handler.writeError(contextGin, svcErr)
		return
	}
	contextGin.JSON(http.StatusOK, response)
}

func (handler *notificationHandler) cancelNotification(contextGin *gin.Context) {
	notificationID := strings.TrimSpace(contextGin.Param("id"))
	if notificationID == "" {
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "notification_id is required"})
		return
	}
	requestContext, resolveErr := handler.resolveNotificationContext(contextGin)
	if resolveErr != nil {
		handler.writeTenantResolutionError(contextGin, resolveErr)
		return
	}
	response, err := handler.service.CancelNotification(requestContext, notificationID)
	if err != nil {
		handler.writeError(contextGin, err)
		return
	}
	contextGin.JSON(http.StatusOK, response)
}

func (handler *notificationHandler) writeError(contextGin *gin.Context, err error) {
	switch {
	case isMissingNotificationID(err):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": "notification_id is required"})
	case errors.Is(err, service.ErrNotificationNotEditable):
		contextGin.JSON(http.StatusConflict, gin.H{"error": "notification can only be edited while queued"})
	case errors.Is(err, model.ErrNotificationNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		contextGin.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
	default:
		handler.logger.Error("http_handler_error", "error", err)
		contextGin.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func (handler *notificationHandler) resolveNotificationContext(contextGin *gin.Context) (context.Context, error) {
	if _, ok := tenant.RuntimeFromContext(contextGin.Request.Context()); !ok {
		return nil, errors.New("tenant runtime missing")
	}
	tenantID := strings.TrimSpace(contextGin.Query(tenantIDQueryParam))
	if tenantID == "" {
		return nil, errTenantIDRequired
	}
	targetCfg, err := handler.repository.ResolveByID(contextGin.Request.Context(), tenantID)
	if err != nil {
		return nil, err
	}
	return tenant.WithRuntime(contextGin.Request.Context(), targetCfg), nil
}

func (handler *notificationHandler) writeTenantResolutionError(contextGin *gin.Context, err error) {
	switch {
	case errors.Is(err, errTenantIDRequired):
		contextGin.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, tenant.ErrInvalidTenantID), errors.Is(err, gorm.ErrRecordNotFound):
		contextGin.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
	default:
		handler.logger.Error("http_handler_error", "error", err)
		contextGin.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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

func pickDuration(candidate time.Duration, fallback time.Duration) time.Duration {
	if candidate <= 0 {
		return fallback
	}
	return candidate
}

type runtimeConfigPayload struct {
	APIBaseURL     string              `json:"apiBaseUrl"`
	Tenant         runtimeConfigTenant `json:"tenant"`
	TAuthBaseURL   string              `json:"tauthBaseUrl"`
	TAuthTenantID  string              `json:"tauthTenantId"`
	GoogleClientID string              `json:"googleClientId"`
}

type runtimeConfigTenant struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type runtimeConfigTAuth struct {
	BaseURL        string
	TenantID       string
	GoogleClientID string
}

func serveRuntimeConfig(tauthConfig runtimeConfigTAuth) gin.HandlerFunc {
	return func(contextGin *gin.Context) {
		runtimeCfg, ok := tenant.RuntimeFromContext(contextGin.Request.Context())
		if !ok {
			contextGin.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		payload := runtimeConfigPayload{
			APIBaseURL:     buildAPIBaseURL(contextGin.Request),
			TAuthBaseURL:   tauthConfig.BaseURL,
			TAuthTenantID:  tauthConfig.TenantID,
			GoogleClientID: tauthConfig.GoogleClientID,
			Tenant: runtimeConfigTenant{
				ID:          runtimeCfg.Tenant.ID,
				DisplayName: runtimeCfg.Tenant.DisplayName,
			},
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
