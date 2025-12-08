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
	"testing"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
)

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

func TestSessionMiddlewareRejectsNonAdmins(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: stubSvc,
		SessionValidator:    &stubValidator{email: "guest@example.com"},
		Logger:              logger,
		AdminEmails:         []string{"admin@example.com"},
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", recorder.Code)
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

func TestRescheduleNotificationMapsMissingIDErrorToBadRequest(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{rescheduleErr: fmt.Errorf("missing notification_id")}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	requestBody := `{"scheduled_time":"2024-01-02T15:04:05Z"}`
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule", bytes.NewBufferString(requestBody))
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
			request := httptest.NewRequest(http.MethodPost, "/api/notifications/notif-1/cancel", nil)

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

func TestNewServerSupportsStaticRootAfterAPIRoutes(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	assetPath := filepath.Join(tempDir, "app.js")
	if writeErr := os.WriteFile(assetPath, []byte("console.log('ok');"), 0o644); writeErr != nil {
		t.Fatalf("failed to write static file: %v", writeErr)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		StaticRoot:          tempDir,
		NotificationService: &stubNotificationService{},
		SessionValidator:    &stubValidator{},
		Logger:              logger,
		AdminEmails:         []string{"user@example.com"},
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app.js", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 when serving static content, got %d", recorder.Code)
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
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: &stubNotificationService{},
		SessionValidator:    &stubValidator{},
		Logger:              logger,
		AdminEmails:         []string{"user@example.com"},
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
		APIBaseURL string `json:"apiBaseUrl"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.APIBaseURL != "http://example.com/api" {
		t.Fatalf("unexpected api base %q", payload.APIBaseURL)
	}
}

func newTestHTTPServer(t *testing.T, svc service.NotificationService, validator SessionValidator) *Server {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: svc,
		SessionValidator:    validator,
		Logger:              logger,
		AdminEmails:         []string{"user@example.com"},
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}
	return server
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
}

func (stub *stubNotificationService) SendNotification(context.Context, model.NotificationRequest) (model.NotificationResponse, error) {
	return model.NotificationResponse{}, errors.New("not implemented")
}

func (stub *stubNotificationService) GetNotificationStatus(context.Context, string) (model.NotificationResponse, error) {
	return model.NotificationResponse{}, errors.New("not implemented")
}

func (stub *stubNotificationService) ListNotifications(context.Context, model.NotificationListFilters) ([]model.NotificationResponse, error) {
	return stub.listResponse, stub.listErr
}

func (stub *stubNotificationService) RescheduleNotification(_ context.Context, notificationID string, scheduledFor time.Time) (model.NotificationResponse, error) {
	stub.rescheduleCalls++
	stub.lastRescheduleID = notificationID
	_ = scheduledFor
	if stub.rescheduleErr != nil {
		return model.NotificationResponse{}, stub.rescheduleErr
	}
	return stub.rescheduleResponse, nil
}

func (stub *stubNotificationService) CancelNotification(_ context.Context, notificationID string) (model.NotificationResponse, error) {
	stub.cancelCalls++
	stub.lastCancelID = notificationID
	if stub.cancelErr != nil {
		return model.NotificationResponse{}, stub.cancelErr
	}
	return stub.cancelResponse, nil
}

func (stub *stubNotificationService) StartRetryWorker(context.Context) {}
