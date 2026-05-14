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
	"github.com/tyemirov/pinguin/internal/smtpidentity"
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
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-test&status=queued&status=errored", nil)

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

func TestListNotificationsParsesSearchAndPagination(t *testing.T) {
	t.Helper()

	cursor, cursorErr := model.NewNotificationListCursor(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC), 42)
	if cursorErr != nil {
		t.Fatalf("cursor: %v", cursorErr)
	}
	encodedCursor := cursor.Encode()
	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{{NotificationID: "queued", Status: model.StatusQueued}},
		nextCursor:   "next-page",
	}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-test&status=queued&q=hidden+body&limit=25&cursor="+encodedCursor, nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var payload struct {
		Notifications []model.NotificationResponse `json:"notifications"`
		NextCursor    string                       `json:"next_cursor"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response decode error: %v", err)
	}
	if payload.NextCursor != "next-page" || len(payload.Notifications) != 1 {
		t.Fatalf("unexpected payload %+v", payload)
	}
	if stubSvc.lastListFilters.SearchQuery.Value() != "hidden body" {
		t.Fatalf("expected search query, got %q", stubSvc.lastListFilters.SearchQuery.Value())
	}
	if len(stubSvc.lastListFilters.Statuses) != 1 || stubSvc.lastListFilters.Statuses[0] != model.StatusQueued {
		t.Fatalf("unexpected statuses %+v", stubSvc.lastListFilters.Statuses)
	}
	if stubSvc.lastPageRequest.Limit() != 25 {
		t.Fatalf("expected limit 25, got %d", stubSvc.lastPageRequest.Limit())
	}
	parsedCursor := stubSvc.lastPageRequest.Cursor()
	if parsedCursor == nil || parsedCursor.ID() != 42 {
		t.Fatalf("expected parsed cursor id 42, got %+v", parsedCursor)
	}
}

func TestListNotificationsRejectsInvalidListInputs(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name  string
		query string
	}{
		{name: "bad limit", query: "tenant_id=tenant-test&limit=not-a-number"},
		{name: "low limit", query: "tenant_id=tenant-test&limit=0"},
		{name: "high limit", query: "tenant_id=tenant-test&limit=101"},
		{name: "bad cursor", query: "tenant_id=tenant-test&cursor=not-a-cursor"},
		{name: "long search", query: "tenant_id=tenant-test&q=" + strings.Repeat("a", 201)},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			stubSvc := &stubNotificationService{}
			server := newTestHTTPServer(t, stubSvc, &stubValidator{})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/notifications?"+testCase.query, nil)

			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			if stubSvc.listCalls != 0 {
				t.Fatalf("expected service not to be called")
			}
		})
	}
}

func TestWriteNotificationListRequestErrorDefaultsBadRequest(t *testing.T) {
	t.Helper()

	recorder := httptest.NewRecorder()
	contextGin, _ := gin.CreateTestContext(recorder)
	writeNotificationListRequestError(contextGin, errors.New("unexpected"))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestListNotificationsUsesSelectedTenant(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{},
	}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-test", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if stubSvc.listCalls != 1 || stubSvc.listAllCalls != 0 {
		t.Fatalf("expected selected tenant list to be used, got list=%d listAll=%d", stubSvc.listCalls, stubSvc.listAllCalls)
	}
	if stubSvc.lastTenantID != "tenant-test" {
		t.Fatalf("expected tenant-test, got %s", stubSvc.lastTenantID)
	}
}

func TestListNotificationsRequiresTenantID(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if stubSvc.listCalls != 0 {
		t.Fatalf("expected service not to be called")
	}
}

func TestListNotificationsCanSwitchTenants(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{},
	}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{}, repo)

	alphaReq := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-alpha", nil)
	alphaReq.Host = "unknown.localhost"
	alphaRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(alphaRec, alphaReq)
	if alphaRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for alpha, got %d", alphaRec.Code)
	}
	if stubSvc.lastTenantID != "tenant-alpha" {
		t.Fatalf("expected tenant-alpha, got %s", stubSvc.lastTenantID)
	}

	bravoReq := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-bravo", nil)
	bravoReq.Host = "unknown.localhost"
	bravoRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(bravoRec, bravoReq)
	if bravoRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bravo, got %d", bravoRec.Code)
	}
	if stubSvc.lastTenantID != "tenant-bravo" {
		t.Fatalf("expected tenant-bravo, got %s", stubSvc.lastTenantID)
	}

	unknownReq := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-missing", nil)
	unknownReq.Host = "unknown.localhost"
	unknownRec := httptest.NewRecorder()
	currentCalls := stubSvc.listCalls
	server.httpServer.Handler.ServeHTTP(unknownRec, unknownReq)
	if unknownRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown tenant, got %d", unknownRec.Code)
	}
	if stubSvc.listCalls != currentCalls {
		t.Fatalf("service should not be called for unknown tenant")
	}
}

func TestListNotificationsRejectsTenantOutsideUserDomain(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{},
	}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{email: "member@alpha.localhost", roles: []string{"user"}}, repo)

	allowedRecorder := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-alpha", nil)
	allowedRequest.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(allowedRecorder, allowedRequest)
	if allowedRecorder.Code != http.StatusOK {
		t.Fatalf("expected alpha access, got %d body=%s", allowedRecorder.Code, allowedRecorder.Body.String())
	}
	if stubSvc.lastTenantID != "tenant-alpha" {
		t.Fatalf("expected tenant-alpha, got %s", stubSvc.lastTenantID)
	}

	deniedRecorder := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-bravo", nil)
	deniedRequest.Host = "unknown.localhost"
	currentCalls := stubSvc.listCalls
	server.httpServer.Handler.ServeHTTP(deniedRecorder, deniedRequest)
	if deniedRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden bravo access, got %d body=%s", deniedRecorder.Code, deniedRecorder.Body.String())
	}
	if stubSvc.listCalls != currentCalls {
		t.Fatalf("service should not be called for unauthorized tenant")
	}
}

func TestListNotificationsAllowsConfiguredAdminAcrossTenants(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{},
	}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{email: "admin@ops.localhost", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-bravo", nil)
	request.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected configured admin access, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if stubSvc.lastTenantID != "tenant-bravo" {
		t.Fatalf("expected tenant-bravo, got %s", stubSvc.lastTenantID)
	}
}

func TestListNotificationsRejectsSessionWithoutEmailDomain(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{email: "member", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-alpha", nil)
	request.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if stubSvc.listCalls != 0 {
		t.Fatalf("service should not be called")
	}
}

func TestListNotificationsReportsTenantAuthorizationStorageError(t *testing.T) {
	t.Helper()

	repo := newClosedTenantRepository(t)
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{email: "member@example.com", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-alpha", nil)
	request.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected internal server error, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if stubSvc.listCalls != 0 {
		t.Fatalf("service should not be called")
	}
}

func TestListNotificationsReportsDomainAuthorizationStorageError(t *testing.T) {
	t.Helper()

	repo := newTenantRepositoryWithoutDomains(t)
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{email: "member@example.com", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-alpha", nil)
	request.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected internal server error, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if stubSvc.listCalls != 0 {
		t.Fatalf("service should not be called")
	}
}

func TestNotificationMutationsRejectTenantOutsideUserDomain(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{email: "member@alpha.localhost", roles: []string{"user"}}, repo)
	scheduledTime := time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)
	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "reschedule",
			method: http.MethodPatch,
			path:   "/api/notifications/notif-1/schedule?tenant_id=tenant-bravo",
			body:   fmt.Sprintf(`{"scheduled_time":"%s"}`, scheduledTime),
		},
		{
			name:   "cancel",
			method: http.MethodPost,
			path:   "/api/notifications/notif-1/cancel?tenant_id=tenant-bravo",
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.Host = "unknown.localhost"
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
	if stubSvc.rescheduleCalls != 0 || stubSvc.cancelCalls != 0 {
		t.Fatalf("service should not be called, got reschedule=%d cancel=%d", stubSvc.rescheduleCalls, stubSvc.cancelCalls)
	}
}

func TestListTenantsReturnsActiveTenants(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Tenants []runtimeConfigTenant `json:"tenants"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(payload.Tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(payload.Tenants))
	}
	if payload.Tenants[0].ID != "tenant-alpha" || payload.Tenants[1].ID != "tenant-bravo" {
		t.Fatalf("unexpected tenants %+v", payload.Tenants)
	}
}

func TestListTenantsFiltersByUserDomain(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{email: "member@bravo.localhost", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Tenants []runtimeConfigTenant `json:"tenants"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(payload.Tenants) != 1 || payload.Tenants[0].ID != "tenant-bravo" {
		t.Fatalf("unexpected tenants %+v", payload.Tenants)
	}
}

func TestListTenantsAllowsConfiguredAdmin(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{email: "admin@ops.localhost", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Tenants []runtimeConfigTenant `json:"tenants"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(payload.Tenants) != 2 {
		t.Fatalf("expected all tenants for configured admin, got %+v", payload.Tenants)
	}
}

func TestListTenantsRejectsSessionWithoutEmailDomain(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{email: "member", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestListTenantsRequiresAuth(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{err: errors.New("unauthorized")}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestListTenantsReportsRepositoryError(t *testing.T) {
	t.Helper()
	repo := newClosedTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

func TestListTenantsReportsDomainRepositoryError(t *testing.T) {
	t.Helper()
	repo := newClosedTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{email: "member@example.com", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

func TestSMTPIdentityLifecycle(t *testing.T) {
	server, identityRepo := newTestHTTPServerWithSMTPIdentities(t)

	createRecorder := httptest.NewRecorder()
	createBody := bytes.NewBufferString(`{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/smtp-identities", createBody)
	createRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var createPayload smtpidentity.Credentials
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create payload: %v", err)
	}
	if createPayload.Password == "" || createPayload.Username == "" || createPayload.SMTPSettings.Host != "smtp.example.com" {
		t.Fatalf("unexpected create credentials: %+v", createPayload)
	}
	if strings.Join(createPayload.Identity.ForwardTo, ",") != "owner@example.com" {
		t.Fatalf("unexpected forwarding recipients: %+v", createPayload.Identity.ForwardTo)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/smtp-identities", nil)
	listRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listRecorder.Code)
	}
	if strings.Contains(listRecorder.Body.String(), createPayload.Password) {
		t.Fatalf("list response leaked stored password")
	}

	credentialsRecorder := httptest.NewRecorder()
	credentialsPath := fmt.Sprintf("/api/smtp-identities/%s/credentials", createPayload.Identity.ID)
	credentialsRequest := httptest.NewRequest(http.MethodGet, credentialsPath, nil)
	credentialsRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(credentialsRecorder, credentialsRequest)
	if credentialsRecorder.Code != http.StatusOK {
		t.Fatalf("expected credentials 200, got %d body=%s", credentialsRecorder.Code, credentialsRecorder.Body.String())
	}
	var credentialsPayload smtpidentity.Credentials
	if err := json.Unmarshal(credentialsRecorder.Body.Bytes(), &credentialsPayload); err != nil {
		t.Fatalf("decode credentials payload: %v", err)
	}
	if credentialsPayload.Password != createPayload.Password || credentialsPayload.Username != createPayload.Username {
		t.Fatalf("unexpected credentials payload: %+v", credentialsPayload)
	}

	updateRecorder := httptest.NewRecorder()
	updatePath := fmt.Sprintf("/api/smtp-identities/%s/forwarding", createPayload.Identity.ID)
	updateRequest := httptest.NewRequest(http.MethodPatch, updatePath, strings.NewReader(`{"forward_to":["maria@example.com","owner@example.com"]}`))
	updateRequest.Host = "example.com"
	updateRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected forwarding update 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updatePayload smtpidentity.PublicIdentity
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updatePayload); err != nil {
		t.Fatalf("decode forwarding update payload: %v", err)
	}
	if strings.Join(updatePayload.ForwardTo, ",") != "maria@example.com,owner@example.com" {
		t.Fatalf("unexpected updated forwarding recipients: %+v", updatePayload.ForwardTo)
	}

	rotateRecorder := httptest.NewRecorder()
	rotatePath := fmt.Sprintf("/api/smtp-identities/%s/rotate", createPayload.Identity.ID)
	rotateRequest := httptest.NewRequest(http.MethodPost, rotatePath, nil)
	rotateRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(rotateRecorder, rotateRequest)
	if rotateRecorder.Code != http.StatusOK {
		t.Fatalf("expected rotate 200, got %d", rotateRecorder.Code)
	}
	var rotatePayload smtpidentity.Credentials
	if err := json.Unmarshal(rotateRecorder.Body.Bytes(), &rotatePayload); err != nil {
		t.Fatalf("decode rotate payload: %v", err)
	}
	if rotatePayload.Password == createPayload.Password || rotatePayload.Username == createPayload.Username {
		t.Fatalf("expected rotate credentials to change")
	}

	deleteRecorder := httptest.NewRecorder()
	deletePath := fmt.Sprintf("/api/smtp-identities/%s", createPayload.Identity.ID)
	deleteRequest := httptest.NewRequest(http.MethodDelete, deletePath, nil)
	deleteRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d", deleteRecorder.Code)
	}
	if _, authErr := identityRepo.Authenticate(context.Background(), rotatePayload.Username, rotatePayload.Password); !errors.Is(authErr, smtpidentity.ErrAuthenticationFailed) {
		t.Fatalf("expected deleted credentials to fail, got %v", authErr)
	}
}

func TestSMTPIdentityRoutesAllowAuthenticatedDomainVerification(t *testing.T) {
	t.Helper()
	resolver := fakeDNSResolver{}
	server, _ := newTestHTTPServerWithSMTPIdentitiesValidatorAndResolver(t, &stubValidator{
		email: "member@example.com",
		roles: []string{"user"},
	}, resolver)

	blockedRecorder := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(http.MethodPost, "/api/smtp-identities", strings.NewReader(`{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`))
	blockedRequest.Host = "example.com"
	blockedRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(blockedRecorder, blockedRequest)
	if blockedRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unverified domain to block identity create, got %d body=%s", blockedRecorder.Code, blockedRecorder.Body.String())
	}

	createDomainRecorder := httptest.NewRecorder()
	createDomainRequest := httptest.NewRequest(http.MethodPost, "/api/smtp-domains", strings.NewReader(`{"domain":"example.com"}`))
	createDomainRequest.Host = "example.com"
	createDomainRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(createDomainRecorder, createDomainRequest)
	if createDomainRecorder.Code != http.StatusConflict {
		t.Fatalf("expected configured global domain to remain operator-owned, got %d body=%s", createDomainRecorder.Code, createDomainRecorder.Body.String())
	}

	createOwnedDomainRecorder := httptest.NewRecorder()
	createOwnedDomainRequest := httptest.NewRequest(http.MethodPost, "/api/smtp-domains", strings.NewReader(`{"domain":"customer.example"}`))
	createOwnedDomainRequest.Host = "example.com"
	createOwnedDomainRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(createOwnedDomainRecorder, createOwnedDomainRequest)
	if createOwnedDomainRecorder.Code != http.StatusCreated {
		t.Fatalf("expected domain create 201, got %d body=%s", createOwnedDomainRecorder.Code, createOwnedDomainRecorder.Body.String())
	}
	var createdDomain smtpidentity.PublicSenderDomain
	if err := json.Unmarshal(createOwnedDomainRecorder.Body.Bytes(), &createdDomain); err != nil {
		t.Fatalf("decode sender domain: %v", err)
	}
	if createdDomain.Status != string(smtpidentity.SenderDomainStatusPending) || len(createdDomain.DNSRecords) != 3 {
		t.Fatalf("unexpected sender domain payload: %+v", createdDomain)
	}
	listDomainsRecorder := httptest.NewRecorder()
	listDomainsRequest := httptest.NewRequest(http.MethodGet, "/api/smtp-domains", nil)
	listDomainsRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(listDomainsRecorder, listDomainsRequest)
	if listDomainsRecorder.Code != http.StatusOK {
		t.Fatalf("expected domain list 200, got %d body=%s", listDomainsRecorder.Code, listDomainsRecorder.Body.String())
	}
	var listDomainsPayload struct {
		Domains []smtpidentity.PublicSenderDomain `json:"domains"`
	}
	if err := json.Unmarshal(listDomainsRecorder.Body.Bytes(), &listDomainsPayload); err != nil {
		t.Fatalf("decode sender domain list: %v", err)
	}
	if len(listDomainsPayload.Domains) != 1 || listDomainsPayload.Domains[0].Domain != "customer.example" {
		t.Fatalf("unexpected sender domain list: %+v", listDomainsPayload.Domains)
	}
	resolver.set(createdDomain.DNSRecords[0].Host, []string{createdDomain.DNSRecords[0].Value})
	resolver.set("customer.example", []string{"v=spf1 include:_spf.example.invalid a:smtp.example.com ~all"})
	resolver.set("_dmarc.customer.example", []string{"v=DMARC1; p=none"})

	checkRecorder := httptest.NewRecorder()
	checkPath := fmt.Sprintf("/api/smtp-domains/%d/check-dns", createdDomain.ID)
	checkRequest := httptest.NewRequest(http.MethodPost, checkPath, nil)
	checkRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(checkRecorder, checkRequest)
	if checkRecorder.Code != http.StatusOK {
		t.Fatalf("expected DNS check 200, got %d body=%s", checkRecorder.Code, checkRecorder.Body.String())
	}
	var verifiedDomain smtpidentity.PublicSenderDomain
	if err := json.Unmarshal(checkRecorder.Body.Bytes(), &verifiedDomain); err != nil {
		t.Fatalf("decode checked sender domain: %v", err)
	}
	if verifiedDomain.Status != string(smtpidentity.SenderDomainStatusVerified) {
		t.Fatalf("expected verified domain, got %+v", verifiedDomain)
	}

	createRecorder := httptest.NewRecorder()
	createBody := bytes.NewBufferString(`{"email_address":"alice@customer.example","forward_to":["owner@example.com"]}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/smtp-identities", createBody)
	createRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected verified-domain identity create 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
}

func TestSMTPIdentityRoutesAllowConfiguredTenantAdmin(t *testing.T) {
	t.Helper()
	server, _ := newTestHTTPServerWithSMTPIdentitiesAndValidator(t, &stubValidator{
		email: "admin@example.com",
		roles: []string{"user"},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/smtp-identities", nil)
	request.Host = "unknown.example.com"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected configured admin SMTP identity access, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSMTPIdentityRejectsOutsideSenderDomain(t *testing.T) {
	server, _ := newTestHTTPServerWithSMTPIdentities(t)

	recorder := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"email_address":"alice@other.example","forward_to":["owner@example.com"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/smtp-identities", body)
	request.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSMTPIdentityValidationAndErrorMapping(t *testing.T) {
	t.Helper()
	server, _ := newTestHTTPServerWithSMTPIdentities(t)

	testCases := []struct {
		name         string
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{name: "create invalid json", method: http.MethodPost, path: "/api/smtp-identities", body: `{`, expectedCode: http.StatusBadRequest},
		{name: "create invalid address", method: http.MethodPost, path: "/api/smtp-identities", body: `{"email_address":"not-an-email","forward_to":["owner@example.com"]}`, expectedCode: http.StatusBadRequest},
		{name: "create missing forwarding", method: http.MethodPost, path: "/api/smtp-identities", body: `{"email_address":"alice@example.com"}`, expectedCode: http.StatusBadRequest},
		{name: "create invalid forwarding", method: http.MethodPost, path: "/api/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["bad address"]}`, expectedCode: http.StatusBadRequest},
		{name: "create self forwarding", method: http.MethodPost, path: "/api/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["alice@example.com"]}`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding empty id", method: http.MethodPatch, path: "/api/smtp-identities/%20/forwarding", body: `{"forward_to":["owner@example.com"]}`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding invalid json", method: http.MethodPatch, path: "/api/smtp-identities/missing/forwarding", body: `{`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding invalid address", method: http.MethodPatch, path: "/api/smtp-identities/missing/forwarding", body: `{"forward_to":["bad address"]}`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding missing identity", method: http.MethodPatch, path: "/api/smtp-identities/missing/forwarding", body: `{"forward_to":["owner@example.com"]}`, expectedCode: http.StatusNotFound},
		{name: "credentials empty id", method: http.MethodGet, path: "/api/smtp-identities/%20/credentials", expectedCode: http.StatusBadRequest},
		{name: "credentials missing id", method: http.MethodGet, path: "/api/smtp-identities/missing/credentials", expectedCode: http.StatusNotFound},
		{name: "rotate empty id", method: http.MethodPost, path: "/api/smtp-identities/%20/rotate", expectedCode: http.StatusBadRequest},
		{name: "rotate missing id", method: http.MethodPost, path: "/api/smtp-identities/missing/rotate", expectedCode: http.StatusNotFound},
		{name: "delete empty id", method: http.MethodDelete, path: "/api/smtp-identities/%20", expectedCode: http.StatusBadRequest},
		{name: "delete missing id", method: http.MethodDelete, path: "/api/smtp-identities/missing", expectedCode: http.StatusNotFound},
		{name: "create domain invalid json", method: http.MethodPost, path: "/api/smtp-domains", body: `{`, expectedCode: http.StatusBadRequest},
		{name: "create domain invalid", method: http.MethodPost, path: "/api/smtp-domains", body: `{"domain":"bad domain"}`, expectedCode: http.StatusBadRequest},
		{name: "check domain empty id", method: http.MethodPost, path: "/api/smtp-domains/%20/check-dns", expectedCode: http.StatusBadRequest},
		{name: "check domain bad id", method: http.MethodPost, path: "/api/smtp-domains/not-a-number/check-dns", expectedCode: http.StatusBadRequest},
		{name: "check domain zero id", method: http.MethodPost, path: "/api/smtp-domains/0/check-dns", expectedCode: http.StatusBadRequest},
		{name: "check domain missing id", method: http.MethodPost, path: "/api/smtp-domains/404/check-dns", expectedCode: http.StatusNotFound},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.Host = "example.com"
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedCode {
				t.Fatalf("expected %d, got %d body=%s", testCase.expectedCode, recorder.Code, recorder.Body.String())
			}
		})
	}

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/smtp-identities", strings.NewReader(`{"email_address":"dupe@example.com","forward_to":["owner@example.com"]}`))
	createRequest.Host = "example.com"
	createRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected initial create 201, got %d", createRecorder.Code)
	}
	duplicateRecorder := httptest.NewRecorder()
	duplicateRequest := httptest.NewRequest(http.MethodPost, "/api/smtp-identities", strings.NewReader(`{"email_address":"dupe@example.com","forward_to":["owner@example.com"]}`))
	duplicateRequest.Host = "example.com"
	duplicateRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(duplicateRecorder, duplicateRequest)
	if duplicateRecorder.Code != http.StatusConflict {
		t.Fatalf("expected duplicate conflict, got %d", duplicateRecorder.Code)
	}
	var duplicatePayload smtpidentity.Credentials
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &duplicatePayload); err != nil {
		t.Fatalf("decode duplicate setup payload: %v", err)
	}
	selfForwardRecorder := httptest.NewRecorder()
	selfForwardPath := fmt.Sprintf("/api/smtp-identities/%s/forwarding", duplicatePayload.Identity.ID)
	selfForwardRequest := httptest.NewRequest(http.MethodPatch, selfForwardPath, strings.NewReader(`{"forward_to":["dupe@example.com"]}`))
	selfForwardRequest.Host = "example.com"
	selfForwardRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(selfForwardRecorder, selfForwardRequest)
	if selfForwardRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected update self forwarding 400, got %d body=%s", selfForwardRecorder.Code, selfForwardRecorder.Body.String())
	}

	handler := newSMTPIdentityHandler(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	invalidAddressRecorder := httptest.NewRecorder()
	invalidAddressContext, _ := gin.CreateTestContext(invalidAddressRecorder)
	handler.writeError(invalidAddressContext, smtpidentity.ErrInvalidAddress)
	if invalidAddressRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct invalid address mapping to 400, got %d", invalidAddressRecorder.Code)
	}
	missingForwardRecorder := httptest.NewRecorder()
	missingForwardContext, _ := gin.CreateTestContext(missingForwardRecorder)
	handler.writeError(missingForwardContext, smtpidentity.ErrForwardRecipientsRequired)
	if missingForwardRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct missing forwarding mapping to 400, got %d", missingForwardRecorder.Code)
	}
	duplicateForwardRecorder := httptest.NewRecorder()
	duplicateForwardContext, _ := gin.CreateTestContext(duplicateForwardRecorder)
	handler.writeError(duplicateForwardContext, smtpidentity.ErrForwardRecipientDuplicate)
	if duplicateForwardRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct duplicate forwarding mapping to 400, got %d", duplicateForwardRecorder.Code)
	}
	selfForwardRecorder = httptest.NewRecorder()
	selfForwardContext, _ := gin.CreateTestContext(selfForwardRecorder)
	handler.writeError(selfForwardContext, smtpidentity.ErrForwardRecipientSelf)
	if selfForwardRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct self forwarding mapping to 400, got %d", selfForwardRecorder.Code)
	}
	unavailablePasswordRecorder := httptest.NewRecorder()
	unavailablePasswordContext, _ := gin.CreateTestContext(unavailablePasswordRecorder)
	handler.writeError(unavailablePasswordContext, smtpidentity.ErrPasswordUnavailable)
	if unavailablePasswordRecorder.Code != http.StatusConflict {
		t.Fatalf("expected direct unavailable password mapping to 409, got %d", unavailablePasswordRecorder.Code)
	}
}

func TestSMTPIdentityRejectsSessionWithoutUsableEmail(t *testing.T) {
	t.Helper()
	server, _ := newTestHTTPServerWithSMTPIdentitiesAndValidator(t, &stubValidator{
		email: "not an email",
		roles: []string{"user"},
	})

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list domains", method: http.MethodGet, path: "/api/smtp-domains"},
		{name: "create domain", method: http.MethodPost, path: "/api/smtp-domains", body: `{"domain":"customer.example"}`},
		{name: "check domain", method: http.MethodPost, path: "/api/smtp-domains/1/check-dns"},
		{name: "list identities", method: http.MethodGet, path: "/api/smtp-identities"},
		{name: "create identity", method: http.MethodPost, path: "/api/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`},
		{name: "update forwarding", method: http.MethodPatch, path: "/api/smtp-identities/identity/forwarding", body: `{"forward_to":["owner@example.com"]}`},
		{name: "credentials", method: http.MethodGet, path: "/api/smtp-identities/identity/credentials"},
		{name: "rotate", method: http.MethodPost, path: "/api/smtp-identities/identity/rotate"},
		{name: "delete", method: http.MethodDelete, path: "/api/smtp-identities/identity"},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.Host = "example.com"
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("expected forbidden SMTP access, got %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSMTPIdentityContinuesWhenTenantAdminLookupFails(t *testing.T) {
	t.Helper()
	handler := newSMTPIdentityHandler(nil, newClosedTenantRepository(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	contextGin, _ := gin.CreateTestContext(recorder)
	contextGin.Request = httptest.NewRequest(http.MethodGet, "/api/smtp-domains", nil)
	contextGin.Set(contextKeyClaims, &sessionvalidator.Claims{
		UserEmail: "member@example.com",
		UserRoles: []string{"user"},
	})

	scope, ok := handler.requireAccessScope(contextGin)
	if !ok {
		t.Fatalf("expected SMTP access scope despite tenant admin lookup failure")
	}
	if scope.OwnerEmail != "member@example.com" || scope.Admin {
		t.Fatalf("unexpected SMTP access scope: %+v", scope)
	}
}

func TestSMTPIdentityReportsStorageErrors(t *testing.T) {
	t.Helper()
	server, _ := newTestHTTPServerWithBrokenSMTPIdentities(t)

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/smtp-identities"},
		{name: "create", method: http.MethodPost, path: "/api/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`},
		{name: "update forwarding", method: http.MethodPatch, path: "/api/smtp-identities/identity/forwarding", body: `{"forward_to":["owner@example.com"]}`},
		{name: "credentials", method: http.MethodGet, path: "/api/smtp-identities/identity/credentials"},
		{name: "rotate", method: http.MethodPost, path: "/api/smtp-identities/identity/rotate"},
		{name: "delete", method: http.MethodDelete, path: "/api/smtp-identities/identity"},
		{name: "list domains", method: http.MethodGet, path: "/api/smtp-domains"},
		{name: "create domain", method: http.MethodPost, path: "/api/smtp-domains", body: `{"domain":"customer.example"}`},
		{name: "check domain", method: http.MethodPost, path: "/api/smtp-domains/1/check-dns"},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.Host = "example.com"
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSMTPIdentityRoutesBypassTenantLookup(t *testing.T) {
	server, _ := newTestHTTPServerWithSMTPIdentities(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/smtp-identities", nil)
	request.Host = "unknown.example.com"
	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for tenant-independent SMTP identities, got %d body=%s", recorder.Code, recorder.Body.String())
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

func TestRescheduleNotificationRejectsInvalidPayloadAndTimestamp(t *testing.T) {
	t.Helper()
	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})

	testCases := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{`},
		{name: "invalid timestamp", body: `{"scheduled_time":"not-a-time"}`},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule", strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
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
	request.Host = "unknown.localhost"
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

func TestRescheduleNotificationErrorMapping(t *testing.T) {
	t.Helper()
	testCases := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{name: "Conflict", err: service.ErrNotificationNotEditable, expectedCode: http.StatusConflict},
		{name: "NotFound", err: gorm.ErrRecordNotFound, expectedCode: http.StatusNotFound},
		{name: "Internal", err: errors.New("boom"), expectedCode: http.StatusInternalServerError},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			stubSvc := &stubNotificationService{rescheduleErr: testCase.err}
			server := newTestHTTPServer(t, stubSvc, &stubValidator{})
			recorder := httptest.NewRecorder()
			requestBody := fmt.Sprintf(`{"scheduled_time":"%s"}`, time.Now().UTC().Add(5*time.Minute).Format(time.RFC3339))
			request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule?tenant_id=tenant-test", strings.NewReader(requestBody))
			request.Header.Set("Content-Type", "application/json")

			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedCode {
				t.Fatalf("expected %d, got %d", testCase.expectedCode, recorder.Code)
			}
		})
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

func TestCancelNotificationSuccess(t *testing.T) {
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/notifications/notif-1/cancel?tenant_id=tenant-test", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if stubSvc.cancelCalls != 1 {
		t.Fatalf("expected cancel service call")
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

func TestCancelNotificationRequiresTenantID(t *testing.T) {
	t.Helper()
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/notifications/notif-1/cancel", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if stubSvc.cancelCalls != 0 {
		t.Fatalf("expected no service invocation")
	}
}

func TestCancelNotificationMapsTenantResolutionStorageError(t *testing.T) {
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{}, newClosedTenantRepository(t))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/notifications/notif-1/cancel?tenant_id=tenant-test", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
	if stubSvc.cancelCalls != 0 {
		t.Fatalf("expected no service invocation")
	}
}

func TestListNotificationsMapsServiceError(t *testing.T) {
	t.Helper()
	server := newTestHTTPServer(t, &stubNotificationService{listErr: errors.New("boom")}, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications?tenant_id=tenant-test", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
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
		APIBaseURL   string `json:"apiBaseUrl"`
		EventLogURL  string `json:"eventLogUrl"`
		SMTPRelayURL string `json:"smtpRelayUrl"`
		Tenant       struct {
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
	if payload.EventLogURL != "/event-log.html" || payload.SMTPRelayURL != "/smtp-relay.html" {
		t.Fatalf("unexpected page links %+v", payload)
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

func TestRuntimeConfigMissingRuntimeReturnsInternalServerError(t *testing.T) {
	t.Helper()
	engine := gin.New()
	engine.GET("/runtime-config", serveRuntimeConfig())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/runtime-config", nil)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

func TestNewServerValidation(t *testing.T) {
	t.Helper()
	valid := func() Config {
		return Config{
			ListenAddr:          ":0",
			NotificationService: &stubNotificationService{},
			SessionValidator:    &stubValidator{},
			TenantRepository:    newTestTenantRepository(t),
			Logger:              slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		}
	}
	testCases := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "listen addr", mutate: func(cfg *Config) { cfg.ListenAddr = " " }},
		{name: "validator", mutate: func(cfg *Config) { cfg.SessionValidator = nil }},
		{name: "notification service", mutate: func(cfg *Config) { cfg.NotificationService = nil }},
		{name: "tenant repository", mutate: func(cfg *Config) { cfg.TenantRepository = nil }},
		{name: "logger", mutate: func(cfg *Config) { cfg.Logger = nil }},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			cfg := valid()
			testCase.mutate(&cfg)
			if _, err := NewServer(cfg); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestServerStartAndShutdown(t *testing.T) {
	t.Helper()
	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})
	server.httpServer.Addr = "127.0.0.1:0"
	server.config.ShutdownGraceTimeout = time.Millisecond

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()
	time.Sleep(20 * time.Millisecond)
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("start returned error: %v", err)
	}

	badServer := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})
	badServer.httpServer.Addr = "bad-address"
	if err := badServer.Start(); err == nil {
		t.Fatalf("expected listen error")
	}
}

func TestSmallHelpers(t *testing.T) {
	t.Helper()
	if pickDuration(3*time.Second, time.Second) != 3*time.Second {
		t.Fatalf("expected explicit duration")
	}
	if isMissingNotificationID(nil) {
		t.Fatalf("nil error should not look like missing id")
	}
	statuses := parseStatusFilters([]string{" queued ", "queued", "", "FAILED"})
	if len(statuses) != 2 || statuses[0] != model.StatusQueued || statuses[1] != model.StatusFailed {
		t.Fatalf("unexpected statuses %v", statuses)
	}
	if base := buildAPIBaseURL(nil); base != "/api" {
		t.Fatalf("unexpected nil request base %q", base)
	}
	tlsRequest := httptest.NewRequest(http.MethodGet, "https://api.example/runtime-config", nil)
	if base := buildAPIBaseURL(tlsRequest); base != "https://api.example/api" {
		t.Fatalf("unexpected TLS base %q", base)
	}
	forwardedRequest := httptest.NewRequest(http.MethodGet, "/runtime-config", nil)
	forwardedRequest.Host = ""
	forwardedRequest.Header.Set("X-Forwarded-Proto", "https")
	if base := buildAPIBaseURL(forwardedRequest); base != "https://localhost/api" {
		t.Fatalf("unexpected forwarded base %q", base)
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
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}
	return server
}

func newTestHTTPServerWithSMTPIdentities(t *testing.T) (*Server, *smtpidentity.Repository) {
	t.Helper()
	return newTestHTTPServerWithSMTPIdentitiesAndValidator(t, &stubValidator{})
}

func newTestHTTPServerWithSMTPIdentitiesAndValidator(t *testing.T, validator SessionValidator) (*Server, *smtpidentity.Repository) {
	t.Helper()
	return newTestHTTPServerWithSMTPIdentitiesValidatorAndResolver(t, validator, nil)
}

func newTestHTTPServerWithSMTPIdentitiesValidatorAndResolver(t *testing.T, validator SessionValidator, resolver smtpidentity.DNSResolver) (*Server, *smtpidentity.Repository) {
	t.Helper()
	secretKey := strings.Repeat("a", 64)
	keeper, err := tenant.NewSecretKeeper(secretKey)
	if err != nil {
		t.Fatalf("secret keeper error: %v", err)
	}
	dbInstance, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "httpapi.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbInstance.AutoMigrate(
		&tenant.Tenant{},
		&tenant.TenantDomain{},
		&tenant.TenantAdmin{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&smtpidentity.SenderDomain{},
		&smtpidentity.Identity{},
		&smtpidentity.ForwardRecipient{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	cfg := tenant.BootstrapConfig{
		Tenants: []tenant.BootstrapTenant{
			{
				ID:           "tenant-test",
				DisplayName:  "Test Tenant",
				SupportEmail: "support@example.com",
				Enabled:      ptrBool(true),
				Domains:      []string{"example.com"},
				Admins:       []string{"admin@example.com"},
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
	if err := tenant.Bootstrap(context.Background(), dbInstance, keeper, cfg); err != nil {
		t.Fatalf("bootstrap tenants: %v", err)
	}
	if err := smtpidentity.ReplaceSenderDomains(context.Background(), dbInstance, []string{"example.com"}); err != nil {
		t.Fatalf("bootstrap smtp sender domains: %v", err)
	}
	tenantRepo := tenant.NewRepository(dbInstance, keeper)
	identityRepo, err := smtpidentity.NewRepository(dbInstance, secretKey)
	if err != nil {
		t.Fatalf("identity repository: %v", err)
	}
	publicSettings := smtpidentity.PublicSettings{
		Host:         "smtp.example.com",
		Port:         587,
		SecurityMode: "starttls",
	}
	identityService := smtpidentity.NewService(identityRepo, publicSettings)
	if resolver != nil {
		identityService = smtpidentity.NewServiceWithDNSResolver(identityRepo, publicSettings, resolver)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: &stubNotificationService{},
		SMTPIdentityService: identityService,
		SessionValidator:    validator,
		TenantRepository:    tenantRepo,
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}
	return server, identityRepo
}

func newTestHTTPServerWithBrokenSMTPIdentities(t *testing.T) (*Server, *smtpidentity.Repository) {
	t.Helper()
	return newTestHTTPServerWithBrokenSMTPIdentitiesAndValidator(t, &stubValidator{})
}

func newTestHTTPServerWithBrokenSMTPIdentitiesAndValidator(t *testing.T, validator SessionValidator) (*Server, *smtpidentity.Repository) {
	t.Helper()
	secretKey := strings.Repeat("a", 64)
	dbInstance, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "broken-httpapi.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbInstance.AutoMigrate(&smtpidentity.SenderDomain{}, &smtpidentity.Identity{}, &smtpidentity.ForwardRecipient{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	if err := smtpidentity.ReplaceSenderDomains(context.Background(), dbInstance, []string{"example.com"}); err != nil {
		t.Fatalf("seed sender domains: %v", err)
	}
	sqlDB, err := dbInstance.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}
	identityRepo, err := smtpidentity.NewRepository(dbInstance, secretKey)
	if err != nil {
		t.Fatalf("identity repository: %v", err)
	}
	identityService := smtpidentity.NewService(identityRepo, smtpidentity.PublicSettings{
		Host:         "smtp.example.com",
		Port:         587,
		SecurityMode: "starttls",
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	server, err := NewServer(Config{
		ListenAddr:          ":0",
		NotificationService: &stubNotificationService{},
		SMTPIdentityService: identityService,
		SessionValidator:    validator,
		TenantRepository:    newTestTenantRepository(t),
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("server init error: %v", err)
	}
	return server, identityRepo
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
				Admins:       []string{"admin@ops.localhost"},
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
				Admins:       []string{"admin@ops.localhost"},
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

func newClosedTenantRepository(t *testing.T) *tenant.Repository {
	t.Helper()
	keeper, err := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("secret keeper error: %v", err)
	}
	dbInstance, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "closed.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := dbInstance.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}
	return tenant.NewRepository(dbInstance, keeper)
}

func newTenantRepositoryWithoutDomains(t *testing.T) *tenant.Repository {
	t.Helper()
	keeper, err := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("secret keeper error: %v", err)
	}
	dbInstance, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "missing-domains.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbInstance.AutoMigrate(&tenant.Tenant{}, &tenant.TenantAdmin{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return tenant.NewRepository(dbInstance, keeper)
}

func bootstrapTenantRepository(t *testing.T, cfg tenant.BootstrapConfig) *tenant.Repository {
	t.Helper()
	keeper, err := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("secret keeper error: %v", err)
	}
	databaseName := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "_" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	dbInstance, err := gorm.Open(sqlite.Open(databaseName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := dbInstance.AutoMigrate(
		&tenant.Tenant{},
		&tenant.TenantDomain{},
		&tenant.TenantAdmin{},
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
	roles []string
}

func (validator *stubValidator) ValidateRequest(_ *http.Request) (*sessionvalidator.Claims, error) {
	if validator.err != nil {
		return nil, validator.err
	}
	email := validator.email
	if email == "" {
		email = "user@example.com"
	}
	roles := validator.roles
	if roles == nil {
		roles = []string{"admin"}
	}
	return &sessionvalidator.Claims{UserEmail: email, UserRoles: roles}, nil
}

type fakeDNSResolver map[string][]string

func (resolver fakeDNSResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	values, ok := resolver[name]
	if !ok {
		return nil, errors.New("dns record not found")
	}
	return values, nil
}

func (resolver fakeDNSResolver) set(name string, values []string) {
	resolver[name] = values
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
	lastListFilters    model.NotificationListFilters
	lastPageRequest    model.NotificationListPageRequest
	nextCursor         string
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

func (stub *stubNotificationService) ListNotificationsPage(ctx context.Context, filters model.NotificationListFilters, pageRequest model.NotificationListPageRequest) (model.NotificationListResponsePage, error) {
	stub.listCalls++
	stub.lastListFilters = filters
	stub.lastPageRequest = pageRequest
	if runtimeCfg, ok := tenant.RuntimeFromContext(ctx); ok {
		stub.lastTenantID = runtimeCfg.Tenant.ID
	}
	return model.NotificationListResponsePage{
		Notifications: stub.listResponse,
		NextCursor:    stub.nextCursor,
	}, stub.listErr
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
