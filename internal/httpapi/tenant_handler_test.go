package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/tenant"
	"gorm.io/gorm"
)

const tenantCredentialID = "44444444-4444-4444-8444-444444444444"

func TestManagedTenantHTTPLifecycle(t *testing.T) {
	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})
	digest := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	createBody := map[string]interface{}{
		"display_name":  "Managed Customer",
		"support_email": "support@customer.example",
		"email_profile": map[string]interface{}{
			"host": "smtp.customer.example", "port": 587, "username": "smtp-user", "password": "smtp-password", "from_address": "notify@customer.example",
		},
		"sms_profile": map[string]interface{}{
			"account_sid": "AC123", "auth_token": "twilio-token", "from_number": "+15551234567",
		},
		"api_credential": map[string]interface{}{"id": tenantCredentialID, "secret_digest": digest},
	}

	created := tenantRequest(t, server, http.MethodPost, "/api/tenants", createBody, map[string]string{idempotencyKeyHeader: "create-customer"})
	if created.Code != http.StatusCreated || created.Header().Get(etagHeader) != `"1"` {
		t.Fatalf("create tenant = %d %s", created.Code, created.Body.String())
	}
	var resource tenant.Resource
	decodeTenantResponse(t, created, &resource)
	if resource.ID == "" || resource.SMSProfile == nil || resource.APICredential.ID != tenantCredentialID {
		t.Fatalf("created resource = %+v", resource)
	}
	tenantPath := "/api/tenants/" + resource.ID

	repeated := tenantRequest(t, server, http.MethodPost, "/api/tenants", createBody, map[string]string{idempotencyKeyHeader: "create-customer"})
	if repeated.Code != http.StatusCreated {
		t.Fatalf("repeat create = %d %s", repeated.Code, repeated.Body.String())
	}
	listed := tenantRequest(t, server, http.MethodGet, "/api/tenants", nil, nil)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), resource.ID) {
		t.Fatalf("list tenants = %d %s", listed.Code, listed.Body.String())
	}
	fetched := tenantRequest(t, server, http.MethodGet, tenantPath, nil, nil)
	if fetched.Code != http.StatusOK || fetched.Header().Get(etagHeader) != `"1"` {
		t.Fatalf("get tenant = %d %s", fetched.Code, fetched.Body.String())
	}

	updated := tenantRequest(t, server, http.MethodPut, tenantPath, map[string]interface{}{
		"display_name": "Updated Customer", "support_email": "help@customer.example",
	}, map[string]string{ifMatchHeader: `"1"`})
	if updated.Code != http.StatusOK || updated.Header().Get(etagHeader) != `"2"` {
		t.Fatalf("update tenant = %d %s", updated.Code, updated.Body.String())
	}

	emailPath := tenantPath + "/email-profile"
	if response := tenantRequest(t, server, http.MethodGet, emailPath, nil, nil); response.Code != http.StatusOK || response.Header().Get(etagHeader) != `"1"` {
		t.Fatalf("get email profile = %d %s", response.Code, response.Body.String())
	}
	replacedEmail := tenantRequest(t, server, http.MethodPut, emailPath, map[string]interface{}{
		"host": "smtp.replaced.example", "port": 2525, "username": "replacement-user", "password": "replacement-password", "from_address": "replacement@customer.example",
	}, map[string]string{ifMatchHeader: `"1"`})
	if replacedEmail.Code != http.StatusOK || replacedEmail.Header().Get(etagHeader) != `"2"` {
		t.Fatalf("replace email profile = %d %s", replacedEmail.Code, replacedEmail.Body.String())
	}
	patchedEmail := tenantRequest(t, server, http.MethodPatch, emailPath, map[string]interface{}{
		"host": "smtp.patched.example", "port": 465, "username": "patched-user", "password": "patched-password", "from_address": "patched@customer.example",
	}, map[string]string{ifMatchHeader: `"2"`})
	if patchedEmail.Code != http.StatusOK || patchedEmail.Header().Get(etagHeader) != `"3"` {
		t.Fatalf("patch email profile = %d %s", patchedEmail.Code, patchedEmail.Body.String())
	}

	smsPath := tenantPath + "/sms-profile"
	if response := tenantRequest(t, server, http.MethodGet, smsPath, nil, nil); response.Code != http.StatusOK || response.Header().Get(etagHeader) != `"1"` {
		t.Fatalf("get SMS profile = %d %s", response.Code, response.Body.String())
	}
	replacedSMS := tenantRequest(t, server, http.MethodPut, smsPath, map[string]interface{}{
		"account_sid": "AC456", "auth_token": "replacement-token", "from_number": "+15557654321",
	}, map[string]string{ifMatchHeader: `"1"`})
	if replacedSMS.Code != http.StatusOK || replacedSMS.Header().Get(etagHeader) != `"2"` {
		t.Fatalf("replace SMS profile = %d %s", replacedSMS.Code, replacedSMS.Body.String())
	}
	patchedSMS := tenantRequest(t, server, http.MethodPatch, smsPath, map[string]interface{}{
		"account_sid": "AC789", "auth_token": "patched-token", "from_number": "+15559876543",
	}, map[string]string{ifMatchHeader: `"2"`})
	if patchedSMS.Code != http.StatusOK || patchedSMS.Header().Get(etagHeader) != `"3"` {
		t.Fatalf("patch SMS profile = %d %s", patchedSMS.Code, patchedSMS.Body.String())
	}

	credentialPath := tenantPath + "/api-credential"
	if response := tenantRequest(t, server, http.MethodGet, credentialPath, nil, nil); response.Code != http.StatusOK || response.Header().Get(etagHeader) != `"1"` {
		t.Fatalf("get credential = %d %s", response.Code, response.Body.String())
	}
	rotated := tenantRequest(t, server, http.MethodPut, credentialPath, map[string]interface{}{
		"id": "55555555-5555-4555-8555-555555555555", "secret_digest": base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)),
	}, map[string]string{ifMatchHeader: `"1"`})
	if rotated.Code != http.StatusOK || rotated.Header().Get(etagHeader) != `"2"` {
		t.Fatalf("rotate credential = %d %s", rotated.Code, rotated.Body.String())
	}

	deleted := tenantRequest(t, server, http.MethodDelete, tenantPath, nil, map[string]string{ifMatchHeader: `"2"`})
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete tenant = %d %s", deleted.Code, deleted.Body.String())
	}
	if response := tenantRequest(t, server, http.MethodGet, tenantPath, nil, nil); response.Code != http.StatusNotFound {
		t.Fatalf("get deleted tenant = %d %s", response.Code, response.Body.String())
	}
}

func TestManagedTenantHTTPValidationAndFailures(t *testing.T) {
	digest := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	validCreate := map[string]interface{}{
		"display_name": "Customer", "support_email": "support@example.com",
		"email_profile":  map[string]interface{}{"host": "smtp.example.com", "port": 587, "username": "user", "password": "password", "from_address": "sender@example.com"},
		"api_credential": map[string]interface{}{"id": tenantCredentialID, "secret_digest": digest},
	}
	validEmail := map[string]interface{}{"host": "smtp.example.com", "port": 587, "username": "user", "password": "password", "from_address": "sender@example.com"}
	validSMS := map[string]interface{}{"account_sid": "AC123", "auth_token": "token", "from_number": "+15551234567"}
	validCredential := map[string]interface{}{"id": "66666666-6666-4666-8666-666666666666", "secret_digest": digest}
	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})

	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		body       interface{}
		headers    map[string]string
		statusCode int
	}{
		{name: "create media type", method: http.MethodPost, path: "/api/tenants", body: validCreate, headers: map[string]string{idempotencyKeyHeader: "key", "Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "create missing key", method: http.MethodPost, path: "/api/tenants", body: validCreate, statusCode: http.StatusBadRequest},
		{name: "create long key", method: http.MethodPost, path: "/api/tenants", body: validCreate, headers: map[string]string{idempotencyKeyHeader: strings.Repeat("a", maxIdempotencyKey+1)}, statusCode: http.StatusBadRequest},
		{name: "create malformed JSON", method: http.MethodPost, path: "/api/tenants", body: json.RawMessage(`{"broken"`), headers: map[string]string{idempotencyKeyHeader: "bad-json"}, statusCode: http.StatusBadRequest},
		{name: "create invalid required data", method: http.MethodPost, path: "/api/tenants", body: map[string]interface{}{}, headers: map[string]string{idempotencyKeyHeader: "bad-data"}, statusCode: http.StatusUnprocessableEntity},
		{name: "create invalid SMS", method: http.MethodPost, path: "/api/tenants", body: mergeTenantPayload(validCreate, "sms_profile", map[string]interface{}{}), headers: map[string]string{idempotencyKeyHeader: "bad-sms"}, statusCode: http.StatusUnprocessableEntity},
		{name: "update media type", method: http.MethodPut, path: "/api/tenants/" + testTenantID, body: map[string]string{}, headers: map[string]string{"Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "update missing version", method: http.MethodPut, path: "/api/tenants/" + testTenantID, body: map[string]string{}, statusCode: http.StatusPreconditionFailed},
		{name: "update invalid tenant", method: http.MethodPut, path: "/api/tenants/invalid", body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusNotFound},
		{name: "update invalid version", method: http.MethodPut, path: "/api/tenants/" + testTenantID, body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"bad"`}, statusCode: http.StatusPreconditionFailed},
		{name: "update malformed JSON", method: http.MethodPut, path: "/api/tenants/" + testTenantID, body: json.RawMessage(`{"broken"`), headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusBadRequest},
		{name: "update invalid metadata", method: http.MethodPut, path: "/api/tenants/" + testTenantID, body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "update stale version", method: http.MethodPut, path: "/api/tenants/" + testTenantID, body: map[string]string{"display_name": "Name", "support_email": "support@example.com"}, headers: map[string]string{ifMatchHeader: `"9"`}, statusCode: http.StatusPreconditionFailed},
		{name: "delete missing version", method: http.MethodDelete, path: "/api/tenants/" + testTenantID, statusCode: http.StatusPreconditionFailed},
		{name: "delete invalid tenant", method: http.MethodDelete, path: "/api/tenants/invalid", headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusNotFound},
		{name: "delete stale version", method: http.MethodDelete, path: "/api/tenants/" + testTenantID, headers: map[string]string{ifMatchHeader: `"9"`}, statusCode: http.StatusPreconditionFailed},
		{name: "email get invalid tenant", method: http.MethodGet, path: "/api/tenants/invalid/email-profile", statusCode: http.StatusNotFound},
		{name: "email put media type", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]string{}, headers: map[string]string{"Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "email put invalid tenant", method: http.MethodPut, path: "/api/tenants/invalid/email-profile", body: validEmail, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusNotFound},
		{name: "email put malformed JSON", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/email-profile", body: json.RawMessage(`{"broken"`), headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusBadRequest},
		{name: "email put invalid", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "email put masked", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]interface{}{"host": "smtp.example.com", "port": 587, "username": "******", "password": "password", "from_address": "sender@example.com"}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "email put stale version", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/email-profile", body: validEmail, headers: map[string]string{ifMatchHeader: `"9"`}, statusCode: http.StatusPreconditionFailed},
		{name: "email patch media type", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]string{}, headers: map[string]string{"Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "email patch missing version", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]string{}, statusCode: http.StatusPreconditionFailed},
		{name: "email patch malformed JSON", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/email-profile", body: json.RawMessage(`{"broken"`), headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusBadRequest},
		{name: "email patch empty", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "email patch masked", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]string{"password": "••••"}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "email patch invalid profile", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/email-profile", body: map[string]string{"host": ""}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "SMS missing", method: http.MethodGet, path: "/api/tenants/" + testTenantID + "/sms-profile", statusCode: http.StatusNotFound},
		{name: "SMS get invalid tenant", method: http.MethodGet, path: "/api/tenants/invalid/sms-profile", statusCode: http.StatusNotFound},
		{name: "SMS create", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/sms-profile", body: validSMS, headers: map[string]string{ifMatchHeader: `"0"`}, statusCode: http.StatusOK},
		{name: "SMS put media type", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/sms-profile", body: map[string]string{}, headers: map[string]string{"Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "SMS put invalid tenant", method: http.MethodPut, path: "/api/tenants/invalid/sms-profile", body: validSMS, headers: map[string]string{ifMatchHeader: `"0"`}, statusCode: http.StatusNotFound},
		{name: "SMS put malformed JSON", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/sms-profile", body: json.RawMessage(`{"broken"`), headers: map[string]string{ifMatchHeader: `"0"`}, statusCode: http.StatusBadRequest},
		{name: "SMS put invalid", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/sms-profile", body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"0"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "SMS put masked", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/sms-profile", body: map[string]string{"account_sid": "***", "auth_token": "token", "from_number": "+1"}, headers: map[string]string{ifMatchHeader: `"0"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "SMS put stale version", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/sms-profile", body: validSMS, headers: map[string]string{ifMatchHeader: `"9"`}, statusCode: http.StatusPreconditionFailed},
		{name: "SMS patch media type", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/sms-profile", body: map[string]string{}, headers: map[string]string{"Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "SMS patch malformed JSON", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/sms-profile", body: json.RawMessage(`{"broken"`), headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusBadRequest},
		{name: "SMS patch empty", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/sms-profile", body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "SMS patch masked", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/sms-profile", body: map[string]string{"auth_token": "***"}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "SMS patch invalid tenant", method: http.MethodPatch, path: "/api/tenants/invalid/sms-profile", body: map[string]string{"from_number": "+15551234567"}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusNotFound},
		{name: "SMS patch invalid profile", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/sms-profile", body: map[string]string{"from_number": "invalid"}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "credential get invalid tenant", method: http.MethodGet, path: "/api/tenants/invalid/api-credential", statusCode: http.StatusNotFound},
		{name: "credential put media type", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/api-credential", body: map[string]string{}, headers: map[string]string{"Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "credential put invalid tenant", method: http.MethodPut, path: "/api/tenants/invalid/api-credential", body: validCredential, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusNotFound},
		{name: "credential malformed JSON", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/api-credential", body: json.RawMessage(`{"broken"`), headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusBadRequest},
		{name: "credential invalid", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/api-credential", body: map[string]string{}, headers: map[string]string{ifMatchHeader: `"1"`}, statusCode: http.StatusUnprocessableEntity},
		{name: "credential stale version", method: http.MethodPut, path: "/api/tenants/" + testTenantID + "/api-credential", body: validCredential, headers: map[string]string{ifMatchHeader: `"9"`}, statusCode: http.StatusPreconditionFailed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := tenantRequest(t, server, testCase.method, testCase.path, testCase.body, testCase.headers)
			if response.Code != testCase.statusCode {
				t.Fatalf("status = %d, want %d, body=%s", response.Code, testCase.statusCode, response.Body.String())
			}
		})
	}

	t.Run("idempotency conflict", func(t *testing.T) {
		first := tenantRequest(t, server, http.MethodPost, "/api/tenants", validCreate, map[string]string{idempotencyKeyHeader: "conflict"})
		if first.Code != http.StatusCreated {
			t.Fatalf("first create = %d %s", first.Code, first.Body.String())
		}
		changed := mergeTenantPayload(validCreate, "display_name", "Different")
		response := tenantRequest(t, server, http.MethodPost, "/api/tenants", changed, map[string]string{idempotencyKeyHeader: "conflict"})
		if response.Code != http.StatusConflict {
			t.Fatalf("conflicting create = %d %s", response.Code, response.Body.String())
		}
	})

	invalidOwnerServer := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{userID: " "})
	for _, requestSpec := range []struct{ method, path string }{
		{method: http.MethodGet, path: "/api/tenants"},
		{method: http.MethodPost, path: "/api/tenants"},
		{method: http.MethodGet, path: "/api/tenants/" + testTenantID},
	} {
		headers := map[string]string{}
		body := interface{}(nil)
		if requestSpec.method == http.MethodPost {
			headers[idempotencyKeyHeader] = "invalid-owner"
			body = validCreate
		}
		response := tenantRequest(t, invalidOwnerServer, requestSpec.method, requestSpec.path, body, headers)
		if response.Code != http.StatusUnprocessableEntity && response.Code != http.StatusInternalServerError {
			t.Fatalf("invalid owner response = %d %s", response.Code, response.Body.String())
		}
	}

	closedServer := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{}, newClosedTenantRepository(t))
	for _, requestSpec := range []struct {
		method  string
		path    string
		body    interface{}
		headers map[string]string
	}{
		{method: http.MethodGet, path: "/api/tenants"},
		{method: http.MethodPost, path: "/api/tenants", body: validCreate, headers: map[string]string{idempotencyKeyHeader: "closed"}},
		{method: http.MethodGet, path: "/api/tenants/" + testTenantID},
		{method: http.MethodPut, path: "/api/tenants/" + testTenantID, body: map[string]string{"display_name": "Name", "support_email": "support@example.com"}, headers: map[string]string{ifMatchHeader: `"1"`}},
		{method: http.MethodDelete, path: "/api/tenants/" + testTenantID, headers: map[string]string{ifMatchHeader: `"1"`}},
		{method: http.MethodGet, path: "/api/tenants/" + testTenantID + "/api-credential"},
	} {
		response := tenantRequest(t, closedServer, requestSpec.method, requestSpec.path, requestSpec.body, requestSpec.headers)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("closed repository response = %d %s", response.Code, response.Body.String())
		}
	}
}

func TestManagedRoutesCoverSharedHTTPValidation(t *testing.T) {
	if remoteAddressForValue(" \t") != unknownSourceIP {
		t.Fatal("blank remote address must be unknown")
	}
	server := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{})
	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		body       interface{}
		headers    map[string]string
		statusCode int
	}{
		{name: "notification media type", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/notifications/notification", body: map[string]string{"status": "cancelled"}, headers: map[string]string{"Content-Type": "text/plain"}, statusCode: http.StatusUnsupportedMediaType},
		{name: "notification invalid status", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/notifications/notification", body: map[string]string{"status": "queued"}, statusCode: http.StatusUnprocessableEntity},
		{name: "notification invalid tenant", method: http.MethodGet, path: "/api/tenants/invalid/notifications", statusCode: http.StatusNotFound},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := tenantRequest(t, server, testCase.method, testCase.path, testCase.body, testCase.headers)
			if response.Code != testCase.statusCode {
				t.Fatalf("status = %d, want %d, body=%s", response.Code, testCase.statusCode, response.Body.String())
			}
		})
	}
	invalidOwnerServer := newTestHTTPServer(t, &stubNotificationService{}, &stubValidator{userID: " "})
	if response := tenantRequest(t, invalidOwnerServer, http.MethodGet, "/api/tenants/"+testTenantID+"/notifications", nil, nil); response.Code != http.StatusInternalServerError {
		t.Fatalf("notification invalid owner = %d %s", response.Code, response.Body.String())
	}
	seedKeeper, _ := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	runtimeKeeper, _ := tenant.NewSecretKeeper(strings.Repeat("b", 64))
	runtimeDatabase, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "broken-runtime.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open broken runtime database: %v", err)
	}
	if err := runtimeDatabase.AutoMigrate(&tenant.Tenant{}, &tenant.EmailProfile{}, &tenant.SMSProfile{}, &tenant.APICredential{}); err != nil {
		t.Fatalf("migrate broken runtime database: %v", err)
	}
	seedManagedTenant(t, runtimeDatabase, seedKeeper, testTenantID, testOwnerID, "Broken Runtime", "support@example.com")
	brokenRuntimeServer := newTestHTTPServerWithRepo(t, &stubNotificationService{}, &stubValidator{}, tenant.NewRepository(runtimeDatabase, runtimeKeeper))
	if response := tenantRequest(t, brokenRuntimeServer, http.MethodGet, "/api/tenants/"+testTenantID+"/notifications", nil, nil); response.Code != http.StatusInternalServerError {
		t.Fatalf("broken tenant runtime = %d %s", response.Code, response.Body.String())
	}

	smtpServer, _ := newTestHTTPServerWithSMTPIdentities(t)
	for _, testCase := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "identity create media type", method: http.MethodPost, path: "/api/tenants/" + testTenantID + "/smtp-identities"},
		{name: "identity update media type", method: http.MethodPatch, path: "/api/tenants/" + testTenantID + "/smtp-identities/identity"},
		{name: "domain create media type", method: http.MethodPost, path: "/api/tenants/" + testTenantID + "/smtp-domains"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := tenantRequest(t, smtpServer, testCase.method, testCase.path, map[string]string{}, map[string]string{"Content-Type": "text/plain"})
			if response.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
		})
	}
	if response := tenantRequest(t, smtpServer, http.MethodGet, "/api/tenants/invalid/smtp-identities", nil, nil); response.Code != http.StatusNotFound {
		t.Fatalf("SMTP invalid tenant = %d %s", response.Code, response.Body.String())
	}
}

func tenantRequest(t *testing.T, server *Server, method string, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var encodedBody []byte
	if body != nil {
		if rawBody, ok := body.(json.RawMessage); ok {
			encodedBody = rawBody
		} else {
			var err error
			encodedBody, err = json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encodedBody))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeTenantResponse(t *testing.T, recorder *httptest.ResponseRecorder, destination interface{}) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), destination); err != nil {
		t.Fatalf("decode response: %v, body=%s", err, recorder.Body.String())
	}
}

func mergeTenantPayload(source map[string]interface{}, key string, value interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source)+1)
	for sourceKey, sourceValue := range source {
		result[sourceKey] = sourceValue
	}
	result[key] = value
	return result
}
