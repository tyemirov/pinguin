package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/tenant"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"log/slog"
)

func ptrBool(value bool) *bool {
	return &value
}

func TestListNotificationsRequiresAuth(t *testing.T) {
	t.Helper()

	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{err: errors.New("unauthorized")})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestListNotificationsReturnsData(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{
			{NotificationID: "queued", Status: model.StatusQueued},
			{NotificationID: "errored", Status: model.StatusErrored},
		},
	}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?status=queued&status=errored", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var payload struct {
		Notifications []model.NotificationResponse `json:"notifications"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response decode error: %v", err)
	}
	if len(payload.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(payload.Notifications))
	}
}

func TestListNotificationsUsesAll(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{},
	}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	request.Host = "example.com"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if stubSvc.listAllCalls != 1 {
		t.Fatalf("expected list all to be used, got %d", stubSvc.listAllCalls)
	}
}

func TestListNotificationsScopesByTenantHost(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{},
	}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{}, repo)

	alphaReq := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	alphaReq.Host = "alpha.localhost"
	alphaRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(alphaRec, alphaReq)
	if alphaRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for alpha, got %d", alphaRec.Code)
	}

	bravoReq := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	bravoReq.Host = "bravo.localhost"
	bravoRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(bravoRec, bravoReq)
	if bravoRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bravo, got %d", bravoRec.Code)
	}

	unknownReq := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	unknownReq.Host = "unknown.localhost"
	unknownRec := httptest.NewRecorder()
	currentCalls := stubSvc.listCalls
	server.httpServer.Handler.ServeHTTP(unknownRec, unknownReq)
	if unknownRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown host, got %d", unknownRec.Code)
	}
	if stubSvc.listCalls != currentCalls {
		t.Fatalf("service should not be called for unknown host")
	}
}

func TestHealthzBypassesTenantLookup(t *testing.T) {
	t.Helper()
	repo := newTestTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for healthz, got %d", recorder.Code)
	}
}

func TestRescheduleValidation(t *testing.T) {
	t.Helper()

	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestRescheduleNotificationRejectsEmptyID(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	requestBody := `{"scheduled_time":"2024-01-02T15:04:05Z"}`
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/%20/schedule", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if stubSvc.rescheduleCalls != 0 {
		t.Fatalf("expected no service invocation, got %d", stubSvc.rescheduleCalls)
	}
}

func TestRescheduleNotificationRejectsPastSchedule(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	pastTime := time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339)
	requestBody := fmt.Sprintf(`{"scheduled_time":"%s"}`, pastTime)
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if stubSvc.rescheduleCalls != 0 {
		t.Fatalf("expected no service invocation, got %d", stubSvc.rescheduleCalls)
	}
}

func TestRescheduleNotificationRequiresTenantID(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	requestBody := fmt.Sprintf(`{"scheduled_time":"%s"}`, time.Now().UTC().Add(5*time.Minute).Format(time.RFC3339))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if stubSvc.rescheduleCalls != 0 {
		t.Fatalf("expected no service invocation, got %d", stubSvc.rescheduleCalls)
	}
}

func TestRescheduleNotificationUsesTenantID(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{}, repo)

	requestBody := fmt.Sprintf(`{"scheduled_time":"%s"}`, time.Now().UTC().Add(5*time.Minute).Format(time.RFC3339))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule?tenant_id=tenant-bravo", bytes.NewBufferString(requestBody))
	request.Host = "alpha.localhost"
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if stubSvc.lastTenantID != "tenant-bravo" {
		t.Fatalf("expected tenant-bravo, got %s", stubSvc.lastTenantID)
	}
}

func TestRescheduleNotificationMapsMissingIDErrorToBadRequest(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{rescheduleErr: fmt.Errorf("missing notification_id")}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	requestBody := `{"scheduled_time":"2024-01-02T15:04:05Z"}`
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule?tenant_id=tenant-test", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestCancelNotificationErrorMapping(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name         string
		cancelError  error
		expectedCode int
	}{
		{
			name:         "MissingNotificationID",
			cancelError:  fmt.Errorf("missing notification_id"),
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Conflict",
			cancelError:  service.ErrNotificationNotEditable,
			expectedCode: http.StatusConflict,
		},
		{
			name:         "NotFound",
			cancelError:  model.ErrNotificationNotFound,
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "Internal",
			cancelError:  errors.New("boom"),
			expectedCode: http.StatusInternalServerError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()

			stubSvc := &stubNotificationService{cancelErr: testCase.cancelError}
			server := newTestHTTPServer(t, stubSvc, &stubValidator{})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/notifications/notif-1/cancel?tenant_id=tenant-test", nil)

			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedCode {
				t.Fatalf("expected %d, got %d", testCase.expectedCode, recorder.Code)
			}
		})
	}
}

func TestCancelNotificationRejectsEmptyID(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/notifications/%20/cancel", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if stubSvc.cancelCalls != 0 {
		t.Fatalf("expected no service invocation, got %d", stubSvc.cancelCalls)
	}
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	t.Helper()

	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	request.Host = "example.com"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown path, got %d", recorder.Code)
	}
}

func TestBuildCORSDefaultDisablesCredentials(t *testing.T) {
	t.Helper()

	engine := gin.New()
	engine.Use(buildCORS(nil))
	engine.GET("/ping", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set("Origin", "https://evil.example")

	engine.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected no credentials header, got %q", got)
	}
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Fatalf("expected wildcard allow origin, got %q", origin)
	}
}

func TestBuildCORSEmitsCredentialsForExplicitAllowList(t *testing.T) {
	t.Helper()

	const allowedOrigin = "https://app.example"

	engine := gin.New()
	engine.Use(buildCORS([]string{allowedOrigin}))
	engine.GET("/ping", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set("Origin", allowedOrigin)

	engine.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header, got %q", got)
	}
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != allowedOrigin {
		t.Fatalf("expected allow origin %q, got %q", allowedOrigin, origin)
	}
}

func TestRuntimeConfigEndpointReturnsValues(t *testing.T) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	tenantRepo := newTestTenantRepository(t)
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: &stubNotificationService{},
		SessionValidator:    &stubValidator{},
		TenantRepository:    tenantRepo,
		TAuthBaseURL:        "https://tauth.example.com",
		TAuthTenantID:       "tauth-test",
		TAuthGoogleClientID: "client-id",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/runtime-config", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var payload struct {
		APIBaseURL     string `json:"apiBaseUrl"`
		TAuthBaseURL   string `json:"tauthBaseUrl"`
		TAuthTenantID  string `json:"tauthTenantId"`
		GoogleClientID string `json:"googleClientId"`
		Tenant         struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"tenant"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.APIBaseURL != "http://example.com/api" {
		t.Fatalf("unexpected api base %q", payload.APIBaseURL)
	}
	if payload.Tenant.ID != "tenant-test" {
		t.Fatalf("unexpected tenant payload %+v", payload.Tenant)
	}
	if payload.TAuthBaseURL != "https://tauth.example.com" || payload.TAuthTenantID != "tauth-test" || payload.GoogleClientID != "client-id" {
		t.Fatalf("unexpected tauth payload %+v", payload)
	}
}

func TestRuntimeConfigResolvesPerHost(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: &stubNotificationService{},
		SessionValidator:    &stubValidator{},
		TenantRepository:    repo,
		TAuthBaseURL:        "https://tauth.example.com",
		TAuthTenantID:       "tauth-test",
		TAuthGoogleClientID: "client-id",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}

	checkHost := func(host string, expectedID string) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/runtime-config", nil)
		request.Host = host
		server.httpServer.Handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200 for host %s, got %d", host, recorder.Code)
		}
		var payload struct {
			Tenant struct {
				ID string `json:"id"`
			} `json:"tenant"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if payload.Tenant.ID != expectedID {
			t.Fatalf("host %s resolved id %s", host, payload.Tenant.ID)
		}
	}

	checkHost("alpha.localhost", "tenant-alpha")
	checkHost("bravo.localhost", "tenant-bravo")
}

func TestRuntimeConfigRejectsUnknownHost(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: &stubNotificationService{},
		SessionValidator:    &stubValidator{},
		TenantRepository:    repo,
		TAuthBaseURL:        "https://tauth.example.com",
		TAuthTenantID:       "tauth-test",
		TAuthGoogleClientID: "client-id",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/runtime-config", nil)
	request.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown host, got %d", recorder.Code)
	}
}

func newTestHTTPServer(t *testing.T, svc service.NotificationService, validator SessionValidator) *Server {
	t.Helper()
	repo := newTestTenantRepository(t)
	return newTestHTTPServerWithRepo(t, svc, validator, repo)
}

func newTestHTTPServerWithRepo(t *testing.T, svc service.NotificationService, validator SessionValidator, repo *tenant.Repository) *Server {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: svc,
		SessionValidator:    validator,
		TenantRepository:    repo,
		TAuthBaseURL:        "https://tauth.example.com",
		TAuthTenantID:       "tauth-test",
		TAuthGoogleClientID: "client-id",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}
	return server
}

func newTestTenantRepository(t *testing.T) *tenant.Repository {
	t.Helper()
	cfg := tenant.BootstrapConfig{
		Tenants: []tenant.BootstrapTenant{
			{
				ID:           "tenant-test",
				DisplayName:  "Test Tenant",
				SupportEmail: "support@example.com",
				Enabled:      ptrBool(true),
				Domains:      []string{"example.com"},
				EmailProfile: tenant.BootstrapEmailProfile{
					Host:        "smtp.example.com",
					Port:        587,
					Username:    "smtp-user",
					Password:    "smtp-pass",
					FromAddress: "noreply@example.com",
				},
			},
		},
	}
	return bootstrapTenantRepository(t, cfg)
}

func newMultiTenantRepository(t *testing.T) *tenant.Repository {
	t.Helper()
	cfg := tenant.BootstrapConfig{
		Tenants: []tenant.BootstrapTenant{
			{
				ID:           "tenant-alpha",
				DisplayName:  "Alpha Corp",
				SupportEmail: "alpha@example.com",
				Enabled:      ptrBool(true),
				Domains:      []string{"alpha.localhost"},
				EmailProfile: tenant.BootstrapEmailProfile{
					Host:        "smtp.alpha.localhost",
					Port:        587,
					Username:    "alpha-smtp",
					Password:    "alpha-secret",
					FromAddress: "noreply@alpha.localhost",
				},
			},
			{
				ID:           "tenant-bravo",
				DisplayName:  "Bravo Labs",
				SupportEmail: "bravo@example.com",
				Enabled:      ptrBool(true),
				Domains:      []string{"bravo.localhost"},
				EmailProfile: tenant.BootstrapEmailProfile{
					Host:        "smtp.bravo.localhost",
					Port:        2525,
					Username:    "bravo-smtp",
					Password:    "bravo-secret",
					FromAddress: "noreply@bravo.localhost",
				},
			},
		},
	}
	return bootstrapTenantRepository(t, cfg)
}

func bootstrapTenantRepository(t *testing.T, cfg tenant.BootstrapConfig) *tenant.Repository {
	t.Helper()
	keeper, err := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("secret keeper error: %v", err)
	}
	dbInstance, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbInstance.AutoMigrate(
		&tenant.Tenant{},
		&tenant.TenantDomain{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	payload, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal bootstrap config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "tenants.yml")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write bootstrap config: %v", err)
	}
	if err := tenant.BootstrapFromFile(context.Background(), dbInstance, keeper, path); err != nil {
		t.Fatalf("bootstrap tenants: %v", err)
	}
	return tenant.NewRepository(dbInstance, keeper)
}

type stubValidator struct {
	err   error
	email string
}

func (validator *stubValidator) ValidateRequest(_ *http.Request) (*sessionvalidator.Claims, error) {
	if validator.err != nil {
		return nil, validator.err
	}
	email := validator.email
	if email == "" {
		email = "user@example.com"
	}
	return &sessionvalidator.Claims{UserEmail: email}, nil
}

type stubNotificationService struct {
	listResponse       []model.NotificationResponse
	listErr            error
	rescheduleResponse model.NotificationResponse
	rescheduleErr      error
	rescheduleCalls    int
	lastRescheduleID   string
	cancelResponse     model.NotificationResponse
	cancelErr          error
	cancelCalls        int
	lastCancelID       string
	lastTenantID       string
	listCalls          int
	listAllCalls       int
}

func (stub *stubNotificationService) SendNotification(context.Context, model.NotificationRequest) (model.NotificationResponse, error) {
	return model.NotificationResponse{}, errors.New("not implemented")
}

func (stub *stubNotificationService) GetNotificationStatus(context.Context, string) (model.NotificationResponse, error) {
	return model.NotificationResponse{}, errors.New("not implemented")
}

func (stub *stubNotificationService) ListNotifications(ctx context.Context, _ model.NotificationListFilters) ([]model.NotificationResponse, error) {
	stub.listCalls++
	if runtimeCfg, ok := tenant.RuntimeFromContext(ctx); ok {
		stub.lastTenantID = runtimeCfg.Tenant.ID
	}
	return stub.listResponse, stub.listErr
}

func (stub *stubNotificationService) ListNotificationsAll(_ context.Context, _ model.NotificationListFilters) ([]model.NotificationResponse, error) {
	stub.listCalls++
	stub.listAllCalls++
	return stub.listResponse, stub.listErr
}

func (stub *stubNotificationService) RescheduleNotification(requestContext context.Context, notificationID string, scheduledFor time.Time) (model.NotificationResponse, error) {
	stub.rescheduleCalls++
	stub.lastRescheduleID = notificationID
	_ = scheduledFor
	if runtimeCfg, ok := tenant.RuntimeFromContext(requestContext); ok {
		stub.lastTenantID = runtimeCfg.Tenant.ID
	}
	if stub.rescheduleErr != nil {
		return model.NotificationResponse{}, stub.rescheduleErr
	}
	return stub.rescheduleResponse, nil
}

func (stub *stubNotificationService) CancelNotification(requestContext context.Context, notificationID string) (model.NotificationResponse, error) {
	stub.cancelCalls++
	stub.lastCancelID = notificationID
	if runtimeCfg, ok := tenant.RuntimeFromContext(requestContext); ok {
		stub.lastTenantID = runtimeCfg.Tenant.ID
	}
	if stub.cancelErr != nil {
		return model.NotificationResponse{}, stub.cancelErr
	}
	return stub.cancelResponse, nil
}

func (stub *stubNotificationService) StartRetryWorker(context.Context) {}
