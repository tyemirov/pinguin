package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pingwin/internal/delivery"
	"github.com/tyemirov/pingwin/internal/model"
	"github.com/tyemirov/pingwin/internal/smtpidentity"
	"github.com/tyemirov/pingwin/internal/tenant"
	"gorm.io/gorm"
)

type deliveryHTTPFixture struct {
	server         *Server
	database       *gorm.DB
	now            time.Time
	providerStatus int
	calls          int
}

func newDeliveryHTTPFixture(t *testing.T) *deliveryHTTPFixture {
	t.Helper()
	fixture := &deliveryHTTPFixture{now: time.Now().UTC().Truncate(time.Second)}
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "delivery.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	fixture.database = database
	if err := database.AutoMigrate(&tenant.Tenant{}, &tenant.EmailProfile{}, &tenant.SMSProfile{}, &tenant.APICredential{}, &tenant.IdempotencyRecord{}, &smtpidentity.SenderDomain{}, &smtpidentity.Identity{}, &smtpidentity.ForwardRecipient{}, &model.Notification{}, &model.NotificationAttachment{}); err != nil {
		t.Fatal(err)
	}
	keeper, _ := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	seedManagedTenant(t, database, keeper, testTenantID, testOwnerID, "Delivery tenant", "support@example.com")
	seedManagedTenant(t, database, keeper, testBravoTenantID, "other-owner", "Other tenant", "support@other.com")
	for _, domain := range []smtpidentity.SenderDomain{{TenantID: testTenantID, Domain: "example.com", Status: smtpidentity.SenderDomainStatusVerified, CreatedAt: fixture.now.Add(-2 * time.Hour)}, {TenantID: testTenantID, Domain: "pending.com", Status: smtpidentity.SenderDomainStatusPending, CreatedAt: fixture.now.Add(-2 * time.Hour)}, {TenantID: testBravoTenantID, Domain: "other.com", Status: smtpidentity.SenderDomainStatusVerified, CreatedAt: fixture.now.Add(-2 * time.Hour)}} {
		if err := database.Create(&domain).Error; err != nil {
			t.Fatal(err)
		}
	}
	// A freely editable notification profile cannot authorize another domain's history.
	if err := database.Model(&tenant.EmailProfile{}).Where(&tenant.EmailProfile{TenantID: testTenantID}).Update("from_address", "secret@other.com").Error; err != nil {
		t.Fatal(err)
	}
	identityRepo, _ := smtpidentity.NewRepository(database, strings.Repeat("a", 64))
	identityService := smtpidentity.NewService(identityRepo, smtpidentity.PublicSettings{})
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fixture.calls++
		writer.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if fixture.providerStatus != 0 {
			writer.WriteHeader(fixture.providerStatus)
			_, _ = io.WriteString(writer, `{"__type":"AccessDeniedException","message":"private provider detail"}`)
			return
		}
		events := []map[string]interface{}{}
		for _, record := range []struct {
			id, sender string
			at         time.Time
		}{{"visible", "sender@example.com", fixture.now.Add(-time.Hour)}, {"older-visible", "sender@example.com", fixture.now.Add(-90 * time.Minute)}, {"secret", "secret@other.com", fixture.now.Add(-time.Hour)}, {"pending-secret", "secret@pending.com", fixture.now.Add(-time.Hour)}, {"expired", "sender@example.com", fixture.now.Add(-25 * time.Hour)}} {
			raw, _ := json.Marshal(map[string]interface{}{"source": "aws.ses", "account": "123456789012", "region": "us-east-1", "time": record.at, "detail": map[string]interface{}{"eventType": "Send", "mail": map[string]interface{}{"timestamp": record.at, "source": record.sender, "messageId": record.id, "destination": []string{"recipient@gmail.com"}, "commonHeaders": map[string]interface{}{"subject": record.id}, "tags": map[string]interface{}{"ses:configuration-set": []string{"delivery"}}, "headers": []map[string]string{{"name": "Authorization", "value": "never expose raw headers"}}}}})
			events = append(events, map[string]interface{}{"message": string(raw)})
		}
		_ = json.NewEncoder(writer).Encode(map[string]interface{}{"events": events})
	}))
	t.Cleanup(provider.Close)
	reader, err := delivery.NewCloudWatchReader(delivery.CloudWatchConfig{Region: "us-east-1", AccountID: "123456789012", LogGroup: "/delivery", ConfigurationSet: "delivery", AccessKeyID: "key", SecretAccessKey: "secret"}, func(options *cloudwatchlogs.Options) {
		options.BaseEndpoint = aws.String(provider.URL)
		options.RetryMaxAttempts = 1
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := delivery.NewService(reader, func() time.Time { return fixture.now })
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(Config{ListenAddr: ":0", NotificationService: &stubNotificationService{}, SessionValidator: &stubValidator{}, TenantRepository: tenant.NewRepository(database, keeper), SMTPIdentityService: identityService, DeliveryService: service, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	fixture.server = server
	return fixture
}

func deliveryHTTPGet(t *testing.T, fixture *deliveryHTTPFixture, path string) (int, http.Header, string) {
	t.Helper()
	listener := httptest.NewServer(fixture.server.httpServer.Handler)
	defer listener.Close()
	response, err := http.Get(listener.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, string(body)
}

func TestEmailDeliveryHTTPIsolationSearchAndPagination(t *testing.T) {
	fixture := newDeliveryHTTPFixture(t)
	path := "/api/tenants/" + testTenantID + "/email-deliveries"
	status, headers, body := deliveryHTTPGet(t, fixture, path+"?limit=1")
	if status != 200 || headers.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("response=%d,%s", status, body)
	}
	var page delivery.Page
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].ID != "visible" || page.NextCursor == "" || strings.Contains(body, "never expose") || strings.Contains(body, "secret") {
		t.Fatalf("page=%s", body)
	}
	status, _, body = deliveryHTTPGet(t, fixture, path+"?limit=1&cursor="+url.QueryEscape(page.NextCursor))
	if status != 200 || !strings.Contains(body, "older-visible") {
		t.Fatalf("next=%d,%s", status, body)
	}
	status, _, body = deliveryHTTPGet(t, fixture, path+"?q=secret")
	if status != 200 || strings.Contains(body, "secret") {
		t.Fatalf("foreign query=%d,%s", status, body)
	}
	status, _, body = deliveryHTTPGet(t, fixture, "/api/tenants/"+testBravoTenantID+"/email-deliveries")
	if status != 404 || strings.Contains(body, "secret") {
		t.Fatal("foreign tenant available")
	}
	for _, query := range []string{"?limit=0", "?q=" + strings.Repeat("a", 201), "?cursor=!"} {
		status, _, body = deliveryHTTPGet(t, fixture, path+query)
		if status != 400 {
			t.Fatalf("invalid query=%d,%s", status, body)
		}
	}
	fixture.providerStatus = 400
	status, _, body = deliveryHTTPGet(t, fixture, path)
	if status != 502 || strings.Contains(body, "private provider detail") {
		t.Fatalf("provider failure=%d,%s", status, body)
	}
}

func TestEmailDeliveryHTTPAuthorizationAndUnavailable(t *testing.T) {
	for _, testCase := range []struct {
		name, path string
		change     func(*deliveryHTTPFixture)
		status     int
	}{
		{name: "invalid tenant", path: "invalid", status: 404},
		{name: "unconfigured", path: testTenantID, status: 503, change: func(f *deliveryHTTPFixture) { f.server.config.DeliveryService = nil }},
		{name: "missing identity service", path: testTenantID, status: 503, change: func(f *deliveryHTTPFixture) { f.server.config.SMTPIdentityService = nil }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newDeliveryHTTPFixture(t)
			if testCase.change != nil {
				testCase.change(fixture)
			}
			// Registered handlers hold the immutable server configuration from construction.
			fixture.server = mustDeliveryServer(t, fixture.server.config)
			status, _, body := deliveryHTTPGet(t, fixture, "/api/tenants/"+testCase.path+"/email-deliveries")
			if status != testCase.status {
				t.Fatalf("response=%d,%s", status, body)
			}
		})
	}
	fixture := newDeliveryHTTPFixture(t)
	cfg := fixture.server.config
	cfg.SessionValidator = &stubValidator{err: errors.New("unauthorized")}
	fixture.server = mustDeliveryServer(t, cfg)
	status, _, _ := deliveryHTTPGet(t, fixture, "/api/tenants/"+testTenantID+"/email-deliveries")
	if status != 401 || fixture.calls != 0 {
		t.Fatal("unauthenticated provider access")
	}
}

func TestEmailDeliveryHTTPDomainRevocationAndBoundaryFailures(t *testing.T) {
	fixture := newDeliveryHTTPFixture(t)
	path := "/api/tenants/" + testTenantID + "/email-deliveries"
	if err := fixture.database.Model(&smtpidentity.SenderDomain{}).Where(&smtpidentity.SenderDomain{Domain: "example.com"}).Update("status", smtpidentity.SenderDomainStatusPending).Error; err != nil {
		t.Fatal(err)
	}
	status, _, body := deliveryHTTPGet(t, fixture, path)
	if status != 200 || strings.Contains(body, "visible") || fixture.calls != 0 {
		t.Fatal("revoked domain exposed history")
	}
	if err := fixture.database.Model(&smtpidentity.SenderDomain{}).Where(&smtpidentity.SenderDomain{Domain: "example.com"}).Updates(map[string]interface{}{"status": smtpidentity.SenderDomainStatusVerified, "domain": "bad domain"}).Error; err != nil {
		t.Fatal(err)
	}
	status, _, body = deliveryHTTPGet(t, fixture, path)
	if status != 500 {
		t.Fatalf("invalid trusted scope=%d,%s", status, body)
	}
	if err := fixture.database.Callback().Query().Before("gorm:query").Register("delivery_domain_failure", func(database *gorm.DB) {
		if database.Statement.Table == "sender_domains" {
			database.AddError(errors.New("domain failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	status, _, body = deliveryHTTPGet(t, fixture, path)
	if status != 500 {
		t.Fatalf("domain read failure=%d,%s", status, body)
	}
}

func mustDeliveryServer(t *testing.T, config Config) *Server {
	t.Helper()
	server, err := NewServer(config)
	if err != nil {
		t.Fatal(err)
	}
	return server
}
