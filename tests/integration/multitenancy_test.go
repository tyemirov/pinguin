package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/httpapi"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/tenant"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
)

func TestMultitenantIsolation(t *testing.T) {
	// 1. Setup DB and Tenant Config
	db, secretKeeper := setupTestDB(t)
	configFile := setupTenantConfig(t)

	// 2. Bootstrap Tenants
	err := tenant.BootstrapFromFile(context.Background(), db, secretKeeper, configFile)
	if err != nil {
		t.Fatalf("BootstrapFromFile failed: %v", err)
	}

	repo := tenant.NewRepository(db, secretKeeper)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use mock senders to avoid real calls
	// We use a simple config that satisfies requirements
	cfg := config.Config{
		MaxRetries:       3,
		RetryIntervalSec: 1,
		TwilioAccountSID: "mock", TwilioAuthToken: "mock", TwilioFromNumber: "+1555",
	}

	// We initialize with senders to allow SMS even if credentials in DB are mocked
	// But for this test we focus on DB isolation, not dispatch success.
	// Using nil senders means it will try to build them from tenant config (which we provided).
	svc := service.NewNotificationService(db, logger, cfg, repo)

	// 3. Resolve Contexts
	ctxA, err := resolveContext(db, repo, "tenant-a")
	if err != nil {
		t.Fatalf("resolveContext(tenant-a) failed: %v", err)
	}

	ctxB, err := resolveContext(db, repo, "tenant-b")
	if err != nil {
		t.Fatalf("resolveContext(tenant-b) failed: %v", err)
	}

	// 4. Create Notification in Tenant A
	reqA := model.NotificationRequest{
		NotificationType: model.NotificationEmail,
		Recipient:        "user@a.com",
		Subject:          "Subject A",
		Message:          "Message A",
	}
	respA, err := svc.SendNotification(ctxA, reqA)
	if err != nil {
		t.Fatalf("SendNotification(A) failed: %v", err)
	}
	if respA.NotificationID == "" {
		t.Fatal("expected non-empty NotificationID")
	}

	// 5. Verify Isolation: Tenant B cannot see A's notification
	_, err = svc.GetNotificationStatus(ctxB, respA.NotificationID)
	if err == nil {
		t.Fatal("expected error accessing Tenant A notification from Tenant B, got nil")
	}
	if !strings.Contains(err.Error(), "notification not found") && !strings.Contains(err.Error(), "record not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}

	// 6. Verify Isolation: Tenant B List does not show A's notification
	listB, err := svc.ListNotifications(ctxB, model.NotificationListFilters{})
	if err != nil {
		t.Fatalf("ListNotifications(B) failed: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected empty list for Tenant B, got %d items", len(listB))
	}

	// 7. Verify Tenant A CAN see it
	statusA, err := svc.GetNotificationStatus(ctxA, respA.NotificationID)
	if err != nil {
		t.Fatalf("GetNotificationStatus(A) failed: %v", err)
	}
	if statusA.NotificationID != respA.NotificationID {
		t.Fatalf("expected ID %s, got %s", respA.NotificationID, statusA.NotificationID)
	}

	// 8. Verify Tenant B cannot Cancel A's notification
	_, err = svc.CancelNotification(ctxB, respA.NotificationID)
	if err == nil {
		t.Fatal("expected error cancelling Tenant A notification from Tenant B, got nil")
	}
	if !strings.Contains(err.Error(), "notification not found") && !strings.Contains(err.Error(), "record not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

type mockSessionValidator struct{}

func (m *mockSessionValidator) ValidateRequest(r *http.Request) (*sessionvalidator.Claims, error) {
	return nil, nil // Return nil claims, nil error (or error if needed for auth tests)
}

func TestHTTPMultitenantIsolation(t *testing.T) {
	db, secretKeeper := setupTestDB(t)
	configFile := setupTenantConfig(t)
	err := tenant.BootstrapFromFile(context.Background(), db, secretKeeper, configFile)
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}
	repo := tenant.NewRepository(db, secretKeeper)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := service.NewNotificationService(db, logger, config.Config{}, repo)

	addr := allocateFreeAddr(t)
	server, err := httpapi.NewServer(httpapi.Config{
		ListenAddr:          addr,
		SessionValidator:    &mockSessionValidator{},
		NotificationService: svc,
		TenantRepository:    repo,
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// We can't easily use server.httpServer.Handler directly because it's private?
	// No, httpapi.Server struct has `httpServer *http.Server`. `Handler` is public on `http.Server`.
	// But `httpServer` field is private in `httpapi.Server`.
	// Wait, `internal/httpapi/server.go`:
	// type Server struct { httpServer *http.Server ... }
	// It IS private.
	// However, `Start()` runs ListenAndServe.
	// We want to test the Handler.
	// `NewServer` returns `*Server`.
	// We might need to expose Handler or use a real port.
	// Using a real port is flaky (port conflicts).
	// Checking `internal/httpapi/server.go` again...

	// If I cannot access the handler, I have to rely on `Start()` and `Shutdown()`.
	// Or I can modify `httpapi` to verify.
	// OR I can just use `Start()` on a random port.

	go func() {
		_ = server.Start()
	}()
	defer func() { _ = server.Shutdown(context.Background()) }()

	// Give it a moment to start? Or rely on retry?
	time.Sleep(100 * time.Millisecond) // Brittle but simple for now.

	client := &http.Client{}
	runtimeConfigURL := fmt.Sprintf("http://%s/runtime-config", addr)

	// Test 1: Valid Host (Tenant A)
	req, _ := http.NewRequest("GET", runtimeConfigURL, nil)
	req.Host = "a.example.com" // Set Host header
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for tenant-a, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	tenantMap, ok := body["tenant"].(map[string]interface{})
	if !ok {
		t.Fatalf("Response missing 'tenant' object")
	}
	if tenantMap["id"] != "tenant-a" {
		t.Errorf("Expected tenant-a, got %v", tenantMap["id"])
	}

	// Test 2: Invalid Host
	req2, _ := http.NewRequest("GET", runtimeConfigURL, nil)
	req2.Host = "unknown.example.com"
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for unknown host, got %d", resp2.StatusCode)
	}
}

func setupTestDB(t *testing.T) (*gorm.DB, *tenant.SecretKeeper) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open failed: %v", err)
	}

	err = db.AutoMigrate(&model.Notification{}, &model.NotificationAttachment{}, &tenant.Tenant{}, &tenant.TenantDomain{}, &tenant.TenantIdentity{}, &tenant.TenantMember{}, &tenant.EmailProfile{}, &tenant.SMSProfile{})
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Key must be 32 bytes hex -> 64 characters
	key := "000102030405060708090a0b0c0d0e0f000102030405060708090a0b0c0d0e0f"
	keeper, err := tenant.NewSecretKeeper(key)
	if err != nil {
		t.Fatalf("NewSecretKeeper failed: %v", err)
	}

	return db, keeper
}

func setupTenantConfig(t *testing.T) string {
	config := tenant.BootstrapConfig{
		Tenants: []tenant.BootstrapTenant{
			{
				ID: "tenant-a", DisplayName: "Tenant A", Status: "active",
				Domains: []string{"a.example.com"},
				EmailProfile: tenant.BootstrapEmailProfile{
					Host: "smtp.a.com", Port: 587, Username: "userA", Password: "passA", FromAddress: "no@a.com",
				},
				Identity: tenant.BootstrapIdentity{GoogleClientID: "gc-a", TAuthBaseURL: "https://auth.a.com"},
			},
			{
				ID: "tenant-b", DisplayName: "Tenant B", Status: "active",
				Domains: []string{"b.example.com"},
				EmailProfile: tenant.BootstrapEmailProfile{
					Host: "smtp.b.com", Port: 587, Username: "userB", Password: "passB", FromAddress: "no@b.com",
				},
				Identity: tenant.BootstrapIdentity{GoogleClientID: "gc-b", TAuthBaseURL: "https://auth.b.com"},
			},
		},
	}
	bytes, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "tenants.yml")
	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	return path
}

func resolveContext(db *gorm.DB, repo *tenant.Repository, tenantID string) (context.Context, error) {
	rt, err := repo.ResolveByID(context.Background(), tenantID)
	if err != nil {
		return nil, err
	}
	return tenant.WithRuntime(context.Background(), rt), nil
}
