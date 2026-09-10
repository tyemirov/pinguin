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
	"gorm.io/gorm"
	"log/slog"
)

const (
	testOwnerID       = "owner-user"
	testTenantID      = "11111111-1111-4111-8111-111111111111"
	testAlphaTenantID = "22222222-2222-4222-8222-222222222222"
	testBravoTenantID = "33333333-3333-4333-8333-333333333333"
)

func TestListNotificationsRequiresAuth(t *testing.T) {
	t.Helper()

	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{err: errors.New("unauthorized")})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/notifications", nil)

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
	request := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/notifications?status=queued&status=errored", nil)

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
	request := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/notifications?status=queued&q=hidden+body&limit=25&cursor="+encodedCursor, nil)

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
		{name: "bad limit", query: "limit=not-a-number"},
		{name: "low limit", query: "limit=0"},
		{name: "high limit", query: "limit=101"},
		{name: "bad cursor", query: "cursor=not-a-cursor"},
		{name: "long search", query: "q=" + strings.Repeat("a", 201)},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			stubSvc := &stubNotificationService{}
			server := newTestHTTPServer(t, stubSvc, &stubValidator{})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/notifications?"+testCase.query, nil)

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
	contextGin.Request = httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/notifications", nil)
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
	request := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/notifications", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if stubSvc.listCalls != 1 || stubSvc.listAllCalls != 0 {
		t.Fatalf("expected selected tenant list to be used, got list=%d listAll=%d", stubSvc.listCalls, stubSvc.listAllCalls)
	}
	if stubSvc.lastTenantID != testTenantID {
		t.Fatalf("expected %s, got %s", testTenantID, stubSvc.lastTenantID)
	}
}

func TestListNotificationsRejectsGlobalRoute(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
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

	alphaReq := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testAlphaTenantID+"/notifications", nil)
	alphaReq.Host = "unknown.localhost"
	alphaRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(alphaRec, alphaReq)
	if alphaRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for alpha, got %d", alphaRec.Code)
	}
	if stubSvc.lastTenantID != testAlphaTenantID {
		t.Fatalf("expected alpha tenant, got %s", stubSvc.lastTenantID)
	}

	bravoReq := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testBravoTenantID+"/notifications", nil)
	bravoReq.Host = "unknown.localhost"
	bravoRec := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(bravoRec, bravoReq)
	if bravoRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bravo, got %d", bravoRec.Code)
	}
	if stubSvc.lastTenantID != testBravoTenantID {
		t.Fatalf("expected bravo tenant, got %s", stubSvc.lastTenantID)
	}

	unknownReq := httptest.NewRequest(http.MethodGet, "/api/tenants/44444444-4444-4444-8444-444444444444/notifications", nil)
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

func TestListNotificationsRejectsForeignOwner(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{
		listResponse: []model.NotificationResponse{},
	}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{userID: "foreign-owner"}, repo)

	deniedRecorder := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testBravoTenantID+"/notifications", nil)
	deniedRequest.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(deniedRecorder, deniedRequest)
	if deniedRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected hidden foreign tenant, got %d body=%s", deniedRecorder.Code, deniedRecorder.Body.String())
	}
	if stubSvc.listCalls != 0 {
		t.Fatalf("service should not be called for unauthorized tenant")
	}
}

func TestListNotificationsReportsTenantAuthorizationStorageError(t *testing.T) {
	t.Helper()

	repo := newClosedTenantRepository(t)
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{email: "member@example.com", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testAlphaTenantID+"/notifications", nil)
	request.Host = "unknown.localhost"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected internal server error, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if stubSvc.listCalls != 0 {
		t.Fatalf("service should not be called")
	}
}

func TestNotificationMutationsRejectForeignOwner(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{userID: "foreign-owner"}, repo)
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
			path:   "/api/tenants/" + testBravoTenantID + "/notifications/notif-1",
			body:   fmt.Sprintf(`{"scheduled_time":"%s"}`, scheduledTime),
		},
		{
			name:   "cancel",
			method: http.MethodPatch,
			path:   "/api/tenants/" + testBravoTenantID + "/notifications/notif-1",
			body:   `{"status":"cancelled"}`,
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
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected hidden foreign tenant, got %d body=%s", recorder.Code, recorder.Body.String())
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
		Tenants []tenant.Resource `json:"tenants"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(payload.Tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(payload.Tenants))
	}
	if payload.Tenants[0].ID != testAlphaTenantID || payload.Tenants[1].ID != testBravoTenantID {
		t.Fatalf("unexpected tenants %+v", payload.Tenants)
	}
}

func TestListTenantsFiltersByOwnerUserID(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{userID: "foreign-owner"}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Tenants []tenant.Resource `json:"tenants"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(payload.Tenants) != 0 {
		t.Fatalf("unexpected tenants %+v", payload.Tenants)
	}
}

func TestListTenantsDoesNotElevateRoleAcrossOwners(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{userID: "foreign-owner", roles: []string{"admin"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Tenants []tenant.Resource `json:"tenants"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(payload.Tenants) != 0 {
		t.Fatalf("expected owner-only tenant list, got %+v", payload.Tenants)
	}
}

func TestListTenantsUsesUserIDWithoutEmail(t *testing.T) {
	t.Helper()

	repo := newMultiTenantRepository(t)
	server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{email: "member", roles: []string{"user"}}, repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	request.Host = "unknown.localhost"

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected owner list, got %d body=%s", recorder.Code, recorder.Body.String())
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

func TestSMTPIdentityLifecycle(t *testing.T) {
	server, identityRepo := newTestHTTPServerWithSMTPIdentities(t)

	createRecorder := httptest.NewRecorder()
	createBody := bytes.NewBufferString(`{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", createBody)
	createRequest.Host = "example.com"
	createRequest.Header.Set("Content-Type", "application/json")
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
	listRequest := httptest.NewRequest(http.MethodGet, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", nil)
	listRequest.Host = "example.com"
	server.httpServer.Handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listRecorder.Code)
	}
	if strings.Contains(listRecorder.Body.String(), createPayload.Password) {
		t.Fatalf("list response leaked stored password")
	}

	credentialsRecorder := httptest.NewRecorder()
	credentialsPath := fmt.Sprintf("/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%s/credential", createPayload.Identity.ID)
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
	updatePath := fmt.Sprintf("/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%s", createPayload.Identity.ID)
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
	rotatePath := fmt.Sprintf("/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%s/credential", createPayload.Identity.ID)
	rotateRequest := httptest.NewRequest(http.MethodPut, rotatePath, nil)
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
	deletePath := fmt.Sprintf("/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%s", createPayload.Identity.ID)
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
	server, _ := newTestHTTPServerWithSMTPIdentitiesValidatorAndResolverSeeded(t, &stubValidator{
		email: "member@example.com",
		roles: []string{"user"},
	}, resolver, false)

	blockedRecorder := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", strings.NewReader(`{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`))
	blockedRequest.Host = "example.com"
	blockedRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(blockedRecorder, blockedRequest)
	if blockedRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unverified domain to block identity create, got %d body=%s", blockedRecorder.Code, blockedRecorder.Body.String())
	}

	createDomainRecorder := httptest.NewRecorder()
	createDomainRequest := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", strings.NewReader(`{"domain":"example.com"}`))
	createDomainRequest.Host = "example.com"
	createDomainRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(createDomainRecorder, createDomainRequest)
	if createDomainRecorder.Code != http.StatusCreated {
		t.Fatalf("expected arbitrary sender domain create 201, got %d body=%s", createDomainRecorder.Code, createDomainRecorder.Body.String())
	}
	var createdExampleDomain smtpidentity.PublicSenderDomain
	if err := json.Unmarshal(createDomainRecorder.Body.Bytes(), &createdExampleDomain); err != nil {
		t.Fatalf("decode arbitrary sender domain: %v", err)
	}
	if createdExampleDomain.Domain != "example.com" || createdExampleDomain.Status != string(smtpidentity.SenderDomainStatusPending) {
		t.Fatalf("unexpected arbitrary sender domain payload: %+v", createdExampleDomain)
	}

	createOwnedDomainRecorder := httptest.NewRecorder()
	createOwnedDomainRequest := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", strings.NewReader(`{"domain":"customer.example"}`))
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
	listDomainsRequest := httptest.NewRequest(http.MethodGet, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", nil)
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
	listedDomains := make(map[string]bool, len(listDomainsPayload.Domains))
	for _, listedDomain := range listDomainsPayload.Domains {
		listedDomains[listedDomain.Domain] = true
	}
	if len(listDomainsPayload.Domains) != 2 || !listedDomains["example.com"] || !listedDomains["customer.example"] {
		t.Fatalf("unexpected sender domain list: %+v", listDomainsPayload.Domains)
	}
	resolver.set(createdDomain.DNSRecords[0].Host, []string{createdDomain.DNSRecords[0].Value})
	resolver.set("customer.example", []string{"v=spf1 include:_spf.example.invalid a:smtp.example.com ~all"})
	resolver.set("_dmarc.customer.example", []string{"v=DMARC1; p=none"})

	checkRecorder := httptest.NewRecorder()
	checkPath := fmt.Sprintf("/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/%d/dns-checks", createdDomain.ID)
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
	createRequest := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", createBody)
	createRequest.Host = "example.com"
	createRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected verified-domain identity create 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
}

func TestSMTPIdentityRoutesAllowTenantOwner(t *testing.T) {
	t.Helper()
	server, _ := newTestHTTPServerWithSMTPIdentitiesAndValidator(t, &stubValidator{
		email: "admin@example.com",
		roles: []string{"user"},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", nil)
	request.Host = "unknown.example.com"
	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected tenant owner SMTP identity access, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSMTPIdentityRejectsOutsideSenderDomain(t *testing.T) {
	server, _ := newTestHTTPServerWithSMTPIdentities(t)

	recorder := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"email_address":"alice@other.example","forward_to":["owner@example.com"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body)
	request.Host = "example.com"
	request.Header.Set("Content-Type", "application/json")
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
		{name: "create invalid json", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body: `{`, expectedCode: http.StatusBadRequest},
		{name: "create invalid address", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body: `{"email_address":"not-an-email","forward_to":["owner@example.com"]}`, expectedCode: http.StatusBadRequest},
		{name: "create missing forwarding", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body: `{"email_address":"alice@example.com"}`, expectedCode: http.StatusBadRequest},
		{name: "create invalid forwarding", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["bad address"]}`, expectedCode: http.StatusBadRequest},
		{name: "create self forwarding", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["alice@example.com"]}`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding empty id", method: http.MethodPatch, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%20", body: `{"forward_to":["owner@example.com"]}`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding invalid json", method: http.MethodPatch, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/missing", body: `{`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding invalid address", method: http.MethodPatch, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/missing", body: `{"forward_to":["bad address"]}`, expectedCode: http.StatusBadRequest},
		{name: "update forwarding missing identity", method: http.MethodPatch, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/missing", body: `{"forward_to":["owner@example.com"]}`, expectedCode: http.StatusNotFound},
		{name: "credentials empty id", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%20/credential", expectedCode: http.StatusBadRequest},
		{name: "credentials missing id", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/missing/credential", expectedCode: http.StatusNotFound},
		{name: "rotate empty id", method: http.MethodPut, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%20/credential", expectedCode: http.StatusBadRequest},
		{name: "rotate missing id", method: http.MethodPut, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/missing/credential", expectedCode: http.StatusNotFound},
		{name: "delete empty id", method: http.MethodDelete, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%20", expectedCode: http.StatusBadRequest},
		{name: "delete missing id", method: http.MethodDelete, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/missing", expectedCode: http.StatusNotFound},
		{name: "create domain invalid json", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", body: `{`, expectedCode: http.StatusBadRequest},
		{name: "create domain invalid", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", body: `{"domain":"bad domain"}`, expectedCode: http.StatusBadRequest},
		{name: "check domain empty id", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/%20/dns-checks", expectedCode: http.StatusBadRequest},
		{name: "check domain bad id", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/not-a-number/dns-checks", expectedCode: http.StatusBadRequest},
		{name: "check domain zero id", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/0/dns-checks", expectedCode: http.StatusBadRequest},
		{name: "check domain missing id", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/404/dns-checks", expectedCode: http.StatusNotFound},
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
	createRequest := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", strings.NewReader(`{"email_address":"dupe@example.com","forward_to":["owner@example.com"]}`))
	createRequest.Host = "example.com"
	createRequest.Header.Set("Content-Type", "application/json")
	server.httpServer.Handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected initial create 201, got %d", createRecorder.Code)
	}
	duplicateRecorder := httptest.NewRecorder()
	duplicateRequest := httptest.NewRequest(http.MethodPost, "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", strings.NewReader(`{"email_address":"dupe@example.com","forward_to":["owner@example.com"]}`))
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
	selfForwardPath := fmt.Sprintf("/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/%s", duplicatePayload.Identity.ID)
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
	invalidAddressContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.writeError(invalidAddressContext, smtpidentity.ErrInvalidAddress)
	if invalidAddressRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct invalid address mapping to 400, got %d", invalidAddressRecorder.Code)
	}
	senderDomainExistsRecorder := httptest.NewRecorder()
	senderDomainExistsContext, _ := gin.CreateTestContext(senderDomainExistsRecorder)
	senderDomainExistsContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.writeError(senderDomainExistsContext, smtpidentity.ErrSenderDomainExists)
	if senderDomainExistsRecorder.Code != http.StatusConflict {
		t.Fatalf("expected direct sender-domain duplicate mapping to 409, got %d", senderDomainExistsRecorder.Code)
	}
	missingForwardRecorder := httptest.NewRecorder()
	missingForwardContext, _ := gin.CreateTestContext(missingForwardRecorder)
	missingForwardContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.writeError(missingForwardContext, smtpidentity.ErrForwardRecipientsRequired)
	if missingForwardRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct missing forwarding mapping to 400, got %d", missingForwardRecorder.Code)
	}
	duplicateForwardRecorder := httptest.NewRecorder()
	duplicateForwardContext, _ := gin.CreateTestContext(duplicateForwardRecorder)
	duplicateForwardContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.writeError(duplicateForwardContext, smtpidentity.ErrForwardRecipientDuplicate)
	if duplicateForwardRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct duplicate forwarding mapping to 400, got %d", duplicateForwardRecorder.Code)
	}
	selfForwardRecorder = httptest.NewRecorder()
	selfForwardContext, _ := gin.CreateTestContext(selfForwardRecorder)
	selfForwardContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.writeError(selfForwardContext, smtpidentity.ErrForwardRecipientSelf)
	if selfForwardRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected direct self forwarding mapping to 400, got %d", selfForwardRecorder.Code)
	}
}

func TestSMTPIdentityRejectsForeignOwner(t *testing.T) {
	t.Helper()
	server, _ := newTestHTTPServerWithSMTPIdentitiesAndValidator(t, &stubValidator{
		userID: "foreign-owner",
	})

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list domains", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains"},
		{name: "create domain", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", body: `{"domain":"customer.example"}`},
		{name: "check domain", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/1/dns-checks"},
		{name: "list identities", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities"},
		{name: "create identity", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`},
		{name: "update forwarding", method: http.MethodPatch, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity", body: `{"forward_to":["owner@example.com"]}`},
		{name: "credentials", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity/credential"},
		{name: "rotate", method: http.MethodPut, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity/credential"},
		{name: "delete", method: http.MethodDelete, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity"},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.Host = "example.com"
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected hidden foreign tenant, got %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSMTPIdentityRejectsTenantLookupFailure(t *testing.T) {
	t.Helper()
	handler := newSMTPIdentityHandler(nil, newClosedTenantRepository(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	contextGin, _ := gin.CreateTestContext(recorder)
	contextGin.Request = httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/smtp-domains", nil)
	contextGin.Params = gin.Params{{Key: "tenant_id", Value: testTenantID}}
	contextGin.Set(contextKeyClaims, &sessionvalidator.Claims{
		UserID:    testOwnerID,
		UserEmail: "member@example.com",
		UserRoles: []string{"user"},
	})

	if _, ok := handler.requireAccessScope(contextGin); ok {
		t.Fatalf("expected tenant lookup failure")
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d", recorder.Code)
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
		{name: "list", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities"},
		{name: "create", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", body: `{"email_address":"alice@example.com","forward_to":["owner@example.com"]}`},
		{name: "update forwarding", method: http.MethodPatch, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity", body: `{"forward_to":["owner@example.com"]}`},
		{name: "credentials", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity/credential"},
		{name: "rotate", method: http.MethodPut, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity/credential"},
		{name: "delete", method: http.MethodDelete, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities/identity"},
		{name: "list domains", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains"},
		{name: "create domain", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", body: `{"domain":"customer.example"}`},
		{name: "check domain", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/1/dns-checks"},
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

	testCases := []struct {
		name         string
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{name: "list identities", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-identities", expectedCode: http.StatusOK},
		{name: "list domains", method: http.MethodGet, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", expectedCode: http.StatusOK},
		{name: "create domain", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains", body: `{"domain":"customer.example"}`, expectedCode: http.StatusCreated},
		{name: "check missing domain", method: http.MethodPost, path: "/api/tenants/11111111-1111-4111-8111-111111111111/smtp-domains/404/dns-checks", expectedCode: http.StatusNotFound},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.Host = "unknown.example.com"
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedCode {
				t.Fatalf("expected %d for tenant-independent SMTP route, got %d body=%s", testCase.expectedCode, recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "tenant_not_found") {
				t.Fatalf("expected SMTP route to bypass tenant lookup, got body=%s", recorder.Body.String())
			}
		})
	}
}

func TestHealthzBypassesTenantLookup(t *testing.T) {
	t.Helper()
	for _, scenario := range []struct {
		name   string
		repo   *tenant.Repository
		status int
	}{
		{"available", newTestTenantRepository(t), http.StatusOK},
		{"unavailable", newClosedTenantRepository(t), http.StatusServiceUnavailable},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{}, scenario.repo)
			listener := httptest.NewServer(server.httpServer.Handler)
			defer listener.Close()
			response, err := listener.Client().Get(listener.URL + "/healthz")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != scenario.status || response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("health response: %d %v", response.StatusCode, response.Header)
			}
		})
	}
}

func TestRescheduleValidation(t *testing.T) {
	t.Helper()

	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", recorder.Code)
	}
}

func TestRescheduleNotificationRejectsEmptyID(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	requestBody := `{"scheduled_time":"2024-01-02T15:04:05Z"}`
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/%20", bytes.NewBufferString(requestBody))
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
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", recorder.Code)
	}
	if stubSvc.rescheduleCalls != 0 {
		t.Fatalf("expected no service invocation, got %d", stubSvc.rescheduleCalls)
	}
}

func TestRescheduleNotificationRejectsInvalidPayloadAndTimestamp(t *testing.T) {
	t.Helper()
	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})

	testCases := []struct {
		name         string
		body         string
		expectedCode int
	}{
		{name: "invalid json", body: `{`, expectedCode: http.StatusBadRequest},
		{name: "invalid timestamp", body: `{"scheduled_time":"not-a-time"}`, expectedCode: http.StatusUnprocessableEntity},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", "application/json")
			server.httpServer.Handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedCode {
				t.Fatalf("expected %d, got %d body=%s", testCase.expectedCode, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRescheduleNotificationRejectsGlobalRoute(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	requestBody := fmt.Sprintf(`{"scheduled_time":"%s"}`, time.Now().UTC().Add(5*time.Minute).Format(time.RFC3339))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/notifications/notif-1/schedule", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
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
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testBravoTenantID+"/notifications/notif-1", bytes.NewBufferString(requestBody))
	request.Host = "unknown.localhost"
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if stubSvc.lastTenantID != testBravoTenantID {
		t.Fatalf("expected bravo tenant, got %s", stubSvc.lastTenantID)
	}
}

func TestRescheduleNotificationMapsMissingIDErrorToBadRequest(t *testing.T) {
	t.Helper()

	stubSvc := &stubNotificationService{rescheduleErr: fmt.Errorf("missing notification_id")}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	requestBody := fmt.Sprintf(`{"scheduled_time":"%s"}`, time.Now().UTC().Add(5*time.Minute).Format(time.RFC3339))
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", bytes.NewBufferString(requestBody))
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
			request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", strings.NewReader(requestBody))
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
			request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", strings.NewReader(`{"status":"cancelled"}`))
			request.Header.Set("Content-Type", "application/json")

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
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", strings.NewReader(`{"status":"cancelled"}`))
	request.Header.Set("Content-Type", "application/json")
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
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/%20", strings.NewReader(`{"status":"cancelled"}`))
	request.Header.Set("Content-Type", "application/json")

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if stubSvc.cancelCalls != 0 {
		t.Fatalf("expected no service invocation, got %d", stubSvc.cancelCalls)
	}
}

func TestCancelNotificationRejectsGlobalRoute(t *testing.T) {
	t.Helper()
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServer(t, stubSvc, &stubValidator{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/notifications/notif-1/cancel", nil)

	server.httpServer.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	if stubSvc.cancelCalls != 0 {
		t.Fatalf("expected no service invocation")
	}
}

func TestCancelNotificationMapsTenantResolutionStorageError(t *testing.T) {
	stubSvc := &stubNotificationService{}
	server := newTestHTTPServerWithRepo(t, stubSvc, &stubValidator{}, newClosedTenantRepository(t))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/tenants/"+testTenantID+"/notifications/notif-1", strings.NewReader(`{"status":"cancelled"}`))
	request.Header.Set("Content-Type", "application/json")
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
	request := httptest.NewRequest(http.MethodGet, "/api/tenants/"+testTenantID+"/notifications", nil)

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

func TestRequestLoggerIncludesAttributionFields(t *testing.T) {
	t.Helper()

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{}))
	engine := gin.New()
	if err := engine.SetTrustedProxies([]string{"198.51.100.10"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}
	engine.Use(requestLogger(logger))
	engine.GET("/probe", func(contextGin *gin.Context) {
		contextGin.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/probe", nil)
	request.RemoteAddr = "198.51.100.10:4231"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.1")
	request.Header.Set("User-Agent", "scanner/1.0")

	engine.ServeHTTP(recorder, request)

	output := logBuffer.String()
	expectedFragments := []string{
		"msg=http_request_completed",
		"method=GET",
		"path=/probe",
		"status=204",
		"source_ip=198.51.100.1",
		"remote_addr=198.51.100.10:4231",
		"user_agent=scanner/1.0",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected log output to contain %q, got %q", fragment, output)
		}
	}
}

func TestRequestLoggerIgnoresForwardedHeadersWithoutTrustedProxy(t *testing.T) {
	t.Helper()

	output := requestLogOutput(t, nil, "198.51.100.10:4231", map[string]string{
		"X-Forwarded-For": "203.0.113.9",
		"X-Real-IP":       "203.0.113.11",
	})
	if !strings.Contains(output, "source_ip=198.51.100.10") {
		t.Fatalf("expected untrusted forwarded headers to be ignored, got %q", output)
	}
}

func TestRequestLoggerUsesTrustedProxyClientIP(t *testing.T) {
	t.Helper()

	output := requestLogOutput(t, []string{"198.51.100.10"}, "198.51.100.10:4231", map[string]string{
		"X-Forwarded-For": "203.0.113.9",
	})
	if !strings.Contains(output, "source_ip=203.0.113.9") {
		t.Fatalf("expected trusted proxy client IP, got %q", output)
	}
}

func TestRequestLoggerStopsForwardedChainAtTrustBoundary(t *testing.T) {
	t.Helper()

	output := requestLogOutput(t, []string{"198.51.100.10"}, "198.51.100.10:4231", map[string]string{
		"X-Forwarded-For": "203.0.113.9, 198.51.100.1",
	})
	if !strings.Contains(output, "source_ip=198.51.100.1") {
		t.Fatalf("expected trust-boundary client IP, got %q", output)
	}
}

func TestSourceIPForContextFallsBackToUnknown(t *testing.T) {
	t.Helper()

	contextGin, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = " "
	contextGin.Request = request
	if got := sourceIPForContext(contextGin); got != unknownSourceIP {
		t.Fatalf("expected unknown source IP, got %q", got)
	}
}

func requestLogOutput(t *testing.T, trustedProxies []string, remoteAddr string, headers map[string]string) string {
	t.Helper()

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{}))
	engine := gin.New()
	if err := engine.SetTrustedProxies(normalizeTrustedProxies(trustedProxies)); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}
	engine.Use(requestLogger(logger))
	engine.GET("/probe", func(contextGin *gin.Context) {
		contextGin.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/probe", nil)
	request.RemoteAddr = remoteAddr
	for headerName, headerValue := range headers {
		request.Header.Set(headerName, headerValue)
	}

	engine.ServeHTTP(recorder, request)
	return logBuffer.String()
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
		TenantURL    string `json:"tenantUrl"`
		EventLogURL  string `json:"eventLogUrl"`
		SMTPRelayURL string `json:"smtpRelayUrl"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.APIBaseURL != "http://example.com/api" {
		t.Fatalf("unexpected api base %q", payload.APIBaseURL)
	}
	if payload.TenantURL != "/tenants.html" || payload.EventLogURL != "/event-log.html" || payload.SMTPRelayURL != "/smtp-relay.html" {
		t.Fatalf("unexpected page links %+v", payload)
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
		{name: "trusted proxy", mutate: func(cfg *Config) { cfg.TrustedProxies = []string{"not-an-ip"} }},
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
	statuses := parseStatusFilters([]string{" queued ", "queued", "", "ERRORED"})
	if len(statuses) != 2 || statuses[0] != model.StatusQueued || statuses[1] != model.StatusErrored {
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
	return newTestHTTPServerWithSMTPIdentitiesValidatorAndResolverSeeded(t, validator, resolver, true)
}

func newTestHTTPServerWithSMTPIdentitiesValidatorAndResolverSeeded(t *testing.T, validator SessionValidator, resolver smtpidentity.DNSResolver, seedVerifiedDomain bool) (*Server, *smtpidentity.Repository) {
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
		&model.Notification{},
		&model.NotificationAttachment{},
		&tenant.Tenant{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&tenant.APICredential{},
		&tenant.IdempotencyRecord{},
		&smtpidentity.SenderDomain{},
		&smtpidentity.Identity{},
		&smtpidentity.ForwardRecipient{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	seedManagedTenant(t, dbInstance, keeper, testTenantID, testOwnerID, "Test Tenant", "support@example.com")
	if seedVerifiedDomain {
		seedHTTPAPISenderDomain(t, dbInstance, testTenantID, "example.com")
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

func seedHTTPAPISenderDomain(t *testing.T, dbInstance *gorm.DB, tenantID string, domain string) {
	t.Helper()
	if err := dbInstance.Create(&smtpidentity.SenderDomain{
		TenantID:          tenantID,
		Domain:            domain,
		Status:            smtpidentity.SenderDomainStatusVerified,
		VerificationToken: "test-token",
	}).Error; err != nil {
		t.Fatalf("seed smtp sender domain: %v", err)
	}
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
	seedHTTPAPISenderDomain(t, dbInstance, testTenantID, "example.com")
	identityRepo, err := smtpidentity.NewRepository(dbInstance, secretKey)
	if err != nil {
		t.Fatalf("identity repository: %v", err)
	}
	sqlDB, err := dbInstance.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
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
	return newSeededTenantRepository(t, []tenantSeed{{
		id: testTenantID, ownerUserID: testOwnerID, displayName: "Test Tenant", supportEmail: "support@example.com",
	}})
}

func newMultiTenantRepository(t *testing.T) *tenant.Repository {
	t.Helper()
	return newSeededTenantRepository(t, []tenantSeed{
		{id: testAlphaTenantID, ownerUserID: testOwnerID, displayName: "Alpha Corp", supportEmail: "alpha@example.com"},
		{id: testBravoTenantID, ownerUserID: testOwnerID, displayName: "Bravo Labs", supportEmail: "bravo@example.com"},
	})
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

type tenantSeed struct {
	id           string
	ownerUserID  string
	displayName  string
	supportEmail string
}

func newSeededTenantRepository(t *testing.T, seeds []tenantSeed) *tenant.Repository {
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
		&model.Notification{},
		&model.NotificationAttachment{},
		&tenant.Tenant{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
		&tenant.APICredential{},
		&tenant.IdempotencyRecord{},
		&smtpidentity.SenderDomain{},
		&smtpidentity.Identity{},
		&smtpidentity.ForwardRecipient{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	for _, seed := range seeds {
		seedManagedTenant(t, dbInstance, keeper, seed.id, seed.ownerUserID, seed.displayName, seed.supportEmail)
	}
	return tenant.NewRepository(dbInstance, keeper)
}

func seedManagedTenant(t *testing.T, database *gorm.DB, keeper *tenant.SecretKeeper, tenantID, ownerUserID, displayName, supportEmail string) {
	t.Helper()
	usernameCipher, usernameErr := keeper.Encrypt("smtp-user")
	if usernameErr != nil {
		t.Fatalf("encrypt SMTP username: %v", usernameErr)
	}
	passwordCipher, passwordErr := keeper.Encrypt("smtp-pass")
	if passwordErr != nil {
		t.Fatalf("encrypt SMTP password: %v", passwordErr)
	}
	now := time.Now().UTC()
	models := []interface{}{
		&tenant.Tenant{ID: tenantID, OwnerUserID: ownerUserID, DisplayName: displayName, SupportEmail: supportEmail, Version: 1, CreatedAt: now, UpdatedAt: now},
		&tenant.EmailProfile{ID: tenantID + "-email", TenantID: tenantID, Host: "smtp.example.com", Port: 587, UsernameCipher: usernameCipher, PasswordCipher: passwordCipher, FromAddress: "noreply@example.com", Version: 1, CreatedAt: now, UpdatedAt: now},
		&tenant.APICredential{ID: strings.Replace(tenantID, "11111111", "aaaaaaaa", 1), TenantID: tenantID, SecretDigest: bytes.Repeat([]byte{1}, 32), DisplayPrefix: "pgn_1_test", Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	for _, modelValue := range models {
		if createErr := database.Create(modelValue).Error; createErr != nil {
			t.Fatalf("seed managed tenant: %v", createErr)
		}
	}
}

type stubValidator struct {
	err    error
	userID string
	email  string
	roles  []string
}

func (validator *stubValidator) ValidateRequest(_ *http.Request) (*sessionvalidator.Claims, error) {
	if validator.err != nil {
		return nil, validator.err
	}
	email := validator.email
	if email == "" {
		email = "user@example.com"
	}
	userID := validator.userID
	if userID == "" {
		userID = testOwnerID
	}
	roles := validator.roles
	if roles == nil {
		roles = []string{"admin"}
	}
	return &sessionvalidator.Claims{UserID: userID, UserEmail: email, UserRoles: roles}, nil
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
