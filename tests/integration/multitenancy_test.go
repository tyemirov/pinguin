package integrationtest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/httpapi"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/tenant"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
	"gorm.io/gorm"
)

const integrationOwnerID = "integration-owner"

type acceptingEmailSender struct{}

func (acceptingEmailSender) SendEmail(context.Context, string, string, string, []model.EmailAttachment) error {
	return nil
}

func TestManagedTenantNotificationIsolation(t *testing.T) {
	database, keeper := setupTestDB(t)
	repository := tenant.NewRepository(database, keeper)
	tenantA := createManagedTenant(t, repository, integrationOwnerID, "Tenant A")
	tenantB := createManagedTenant(t, repository, integrationOwnerID, "Tenant B")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notificationService := service.NewNotificationServiceWithSenders(
		database,
		logger,
		config.Config{MaxRetries: 3, RetryIntervalSec: 1},
		repository,
		acceptingEmailSender{},
		nil,
	)

	contextA := runtimeContext(t, repository, tenantA.ID)
	contextB := runtimeContext(t, repository, tenantB.ID)
	request, requestErr := model.NewNotificationRequest(model.NotificationEmail, "user@example.com", "Subject", "Message", nil, nil)
	if requestErr != nil {
		t.Fatalf("notification request: %v", requestErr)
	}
	response, sendErr := notificationService.SendNotification(contextA, request)
	if sendErr != nil {
		t.Fatalf("send notification: %v", sendErr)
	}
	if _, statusErr := notificationService.GetNotificationStatus(contextB, response.NotificationID); statusErr == nil {
		t.Fatalf("foreign tenant read must fail")
	}
	if listed, listErr := notificationService.ListNotifications(contextB, model.NotificationListFilters{}); listErr != nil || len(listed) != 0 {
		t.Fatalf("foreign tenant list = %v, %v", listed, listErr)
	}
	if _, cancelErr := notificationService.CancelNotification(contextB, response.NotificationID); cancelErr == nil {
		t.Fatalf("foreign tenant cancel must fail")
	}
	if owned, statusErr := notificationService.GetNotificationStatus(contextA, response.NotificationID); statusErr != nil || owned.NotificationID != response.NotificationID {
		t.Fatalf("owner read = %+v, %v", owned, statusErr)
	}
}

type mockSessionValidator struct {
	claims *sessionvalidator.Claims
}

func (validator *mockSessionValidator) ValidateRequest(*http.Request) (*sessionvalidator.Claims, error) {
	if validator.claims != nil {
		return validator.claims, nil
	}
	return &sessionvalidator.Claims{UserID: integrationOwnerID, UserEmail: "owner@example.com"}, nil
}

func TestManagedTenantHTTPCreateAndList(t *testing.T) {
	database, keeper := setupTestDB(t)
	repository := tenant.NewRepository(database, keeper)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notificationService := service.NewNotificationServiceWithSenders(database, logger, config.Config{}, repository, acceptingEmailSender{}, nil)
	address := allocateFreeAddr(t)
	server, serverErr := httpapi.NewServer(httpapi.Config{
		ListenAddr: address, SessionValidator: &mockSessionValidator{}, NotificationService: notificationService,
		TenantRepository: repository, Logger: logger,
	})
	if serverErr != nil {
		t.Fatalf("new HTTP server: %v", serverErr)
	}
	go func() { _ = server.Start() }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	digest := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	payload := map[string]interface{}{
		"display_name":  "Managed Tenant",
		"support_email": "support@example.com",
		"email_profile": map[string]interface{}{
			"host": "smtp.example.com", "port": 587, "username": "smtp-user", "password": "smtp-password", "from_address": "sender@example.com",
		},
		"api_credential": map[string]interface{}{"id": uuid.NewString(), "secret_digest": digest},
	}
	requestBody, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		t.Fatalf("marshal request: %v", marshalErr)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	baseURL := "http://" + address + "/api/tenants"
	createRequest, requestErr := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(requestBody))
	if requestErr != nil {
		t.Fatalf("create request: %v", requestErr)
	}
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("Idempotency-Key", uuid.NewString())
	createResponse := doWhenReady(t, client, createRequest)
	defer createResponse.Body.Close()
	if createResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResponse.Body)
		t.Fatalf("create status %d: %s", createResponse.StatusCode, body)
	}
	if createResponse.Header.Get("Cache-Control") != "private, no-store" || createResponse.Header.Get("ETag") == "" {
		t.Fatalf("missing managed response headers: %v", createResponse.Header)
	}

	listRequest, requestErr := http.NewRequest(http.MethodGet, baseURL, nil)
	if requestErr != nil {
		t.Fatalf("list request: %v", requestErr)
	}
	listResponse, listErr := client.Do(listRequest)
	if listErr != nil {
		t.Fatalf("list request: %v", listErr)
	}
	defer listResponse.Body.Close()
	var listPayload struct {
		Tenants []tenant.Resource `json:"tenants"`
	}
	if decodeErr := json.NewDecoder(listResponse.Body).Decode(&listPayload); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listPayload.Tenants) != 1 || listPayload.Tenants[0].DisplayName != "Managed Tenant" {
		t.Fatalf("unexpected tenant list: %+v", listPayload.Tenants)
	}
	encoded, _ := json.Marshal(listPayload)
	if strings.Contains(string(encoded), "smtp-password") || strings.Contains(string(encoded), digest) {
		t.Fatalf("safe response leaked a secret: %s", encoded)
	}
}

func setupTestDB(t *testing.T) (*gorm.DB, *tenant.SecretKeeper) {
	t.Helper()
	database, openErr := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if openErr != nil {
		t.Fatalf("open database: %v", openErr)
	}
	if migrationErr := database.AutoMigrate(
		&model.Notification{}, &model.NotificationAttachment{}, &tenant.Tenant{}, &tenant.EmailProfile{}, &tenant.SMSProfile{},
		&tenant.APICredential{}, &tenant.IdempotencyRecord{}, &smtpidentity.SenderDomain{}, &smtpidentity.Identity{}, &smtpidentity.ForwardRecipient{},
	); migrationErr != nil {
		t.Fatalf("migrate database: %v", migrationErr)
	}
	keeper, keeperErr := tenant.NewSecretKeeper("000102030405060708090a0b0c0d0e0f000102030405060708090a0b0c0d0e0f")
	if keeperErr != nil {
		t.Fatalf("new secret keeper: %v", keeperErr)
	}
	return database, keeper
}

func createManagedTenant(t *testing.T, repository *tenant.Repository, owner string, displayName string) tenant.Resource {
	t.Helper()
	ownerID, _ := tenant.NewOwnerUserID(owner)
	name, _ := tenant.NewDisplayName(displayName)
	email, _ := tenant.NewEmailProfileInput("smtp.example.com", 587, "user", "password", "sender@example.com")
	credentialID, _ := tenant.NewCredentialID(uuid.NewString())
	digest, _ := tenant.NewCredentialDigest(bytes.Repeat([]byte{5}, 32))
	requestDigest, _ := tenant.NewRequestDigest(bytes.Repeat([]byte{6}, 32))
	result, createErr := repository.Create(context.Background(), tenant.CreateInput{
		OwnerUserID: ownerID, DisplayName: name, EmailProfile: email, CredentialID: credentialID, CredentialDigest: digest,
	}, uuid.NewString(), requestDigest)
	if createErr != nil {
		t.Fatalf("create managed tenant: %v", createErr)
	}
	return result.Resource
}

func runtimeContext(t *testing.T, repository *tenant.Repository, tenantID string) context.Context {
	t.Helper()
	runtimeConfig, resolveErr := repository.ResolveByID(context.Background(), tenantID)
	if resolveErr != nil {
		t.Fatalf("resolve tenant: %v", resolveErr)
	}
	return tenant.WithRuntime(context.Background(), runtimeConfig)
}

func doWhenReady(t *testing.T, client *http.Client, request *http.Request) *http.Response {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, requestErr := client.Do(request.Clone(request.Context()))
		if requestErr == nil {
			return response
		}
		if time.Now().After(deadline) {
			t.Fatalf("HTTP server did not start: %v", requestErr)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
