package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const (
	testTenantID                       = "tenant-test"
	missingTenantRuntimeMessage        = "missing tenant runtime"
	expectedInterceptorSuccessTemplate = "expected interceptor success, got %v"
	expectedTenantIDTemplate           = "expected tenant id %s, got %v"
	expectedHandlerNotCalledMessage    = "expected handler not to be called"
)

func TestDigestForLogging(t *testing.T) {
	t.Helper()
	value := digestForLogging("User@example.com ")
	if value == "" {
		t.Fatalf("expected digest")
	}
	if value != digestForLogging("user@example.com") {
		t.Fatalf("expected normalized digest")
	}
	if digestForLogging("") != "" {
		t.Fatalf("expected empty digest for empty input")
	}
}

func TestMapGrpcAttachments(t *testing.T) {
	t.Helper()
	source := []*grpcapi.EmailAttachment{
		{Filename: "foo.txt", ContentType: "text/plain", Data: []byte("hello")},
	}
	result := mapGrpcAttachments(source)
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
	if len(result[0].Data) == 0 || len(source[0].Data) == 0 {
		t.Fatalf("expected data in attachments")
	}
	if &result[0].Data[0] == &source[0].Data[0] {
		t.Fatalf("expected data copy")
	}
	result[0].Data[0] = 'z'
	if source[0].Data[0] == 'z' {
		t.Fatalf("expected source data unchanged")
	}
	if result[0].Filename != "foo.txt" || result[0].ContentType != "text/plain" {
		t.Fatalf("unexpected attachment contents %+v", result[0])
	}
	if mapGrpcAttachments(nil) != nil {
		t.Fatalf("expected nil when source nil")
	}
}

func TestMapModelAttachments(t *testing.T) {
	t.Helper()
	source := []model.EmailAttachment{
		{Filename: "foo.txt", ContentType: "text/plain", Data: []byte("hello")},
	}
	result := mapModelAttachments(source)
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
	result[0].Data[0] = 'z'
	if source[0].Data[0] == 'z' {
		t.Fatalf("expected copy to avoid aliasing")
	}
	if mapModelAttachments(nil) != nil {
		t.Fatalf("expected nil for empty input")
	}
}

func TestMapGrpcStatuses(t *testing.T) {
	t.Helper()
	statuses := mapGrpcStatuses([]grpcapi.Status{
		grpcapi.Status_SENT,
		grpcapi.Status_FAILED,
		grpcapi.Status_UNKNOWN,
	})
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	if statuses[0] != model.StatusSent {
		t.Fatalf("expected StatusSent, got %v", statuses[0])
	}
}

func TestMapModelToGrpcResponse(t *testing.T) {
	t.Helper()
	now := time.Now().UTC()
	scheduled := now.Add(time.Hour)
	resp := mapModelToGrpcResponse(model.NotificationResponse{
		NotificationID:    "notif-1",
		NotificationType:  model.NotificationEmail,
		Recipient:         "user@example.com",
		Subject:           "subject",
		Message:           "body",
		Status:            model.StatusErrored,
		ProviderMessageID: "provider",
		RetryCount:        3,
		CreatedAt:         now,
		UpdatedAt:         now,
		Attachments: []model.EmailAttachment{
			{Filename: "foo.txt", ContentType: "text/plain", Data: []byte("hello")},
		},
	})
	if resp.Status != grpcapi.Status_ERRORED {
		t.Fatalf("expected ERRORED status, got %s", resp.Status.String())
	}
	if resp.ScheduledTime != nil {
		t.Fatalf("expected nil schedule when model has none")
	}

	withSchedule := model.NotificationResponse{
		NotificationID:   "scheduled",
		NotificationType: model.NotificationSMS,
		Status:           model.StatusSent,
		ScheduledFor:     &scheduled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	resp = mapModelToGrpcResponse(withSchedule)
	if resp.ScheduledTime == nil || resp.ScheduledTime.AsTime().UTC() != scheduled.UTC() {
		t.Fatalf("expected scheduled timestamp to be set")
	}
	if resp.NotificationType != grpcapi.NotificationType_SMS {
		t.Fatalf("expected SMS type, got %v", resp.NotificationType)
	}

	if len(resp.Attachments) != 0 {
		t.Fatalf("unexpected attachments %+v", resp.Attachments)
	}
}

func TestBuildAuthInterceptor(t *testing.T) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	interceptor := buildAuthInterceptor(logger, "token")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token"))
	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	if err != nil || resp != "ok" {
		t.Fatalf("expected successful call, err=%v resp=%v", err, resp)
	}

	t.Run("RejectInvalidToken", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer wrong"))
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated, got %v", err)
		}
	})

	t.Run("MissingMetadata", func(t *testing.T) {
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated for missing metadata, got %v", err)
		}
	})
}

func TestBuildTenantInterceptorAttachesRuntime(testHandle *testing.T) {
	testHandle.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	repo := newTestTenantRepository(testHandle, testTenantID)
	interceptor := buildTenantInterceptor(logger, repo)
	request := &grpcapi.NotificationRequest{TenantId: testTenantID}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		runtimeCfg, ok := tenant.RuntimeFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Internal, missingTenantRuntimeMessage)
		}
		return runtimeCfg.Tenant.ID, nil
	}
	response, err := interceptor(context.Background(), request, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		testHandle.Fatalf(expectedInterceptorSuccessTemplate, err)
	}
	if response != testTenantID {
		testHandle.Fatalf(expectedTenantIDTemplate, testTenantID, response)
	}
}

func TestBuildTenantInterceptorUsesMetadata(testHandle *testing.T) {
	testHandle.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	repo := newTestTenantRepository(testHandle, testTenantID)
	interceptor := buildTenantInterceptor(logger, repo)
	request := &grpcapi.GetNotificationStatusRequest{}
	metadataContext := metadata.NewIncomingContext(context.Background(), metadata.Pairs(tenantMetadataKey, testTenantID))
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		runtimeCfg, ok := tenant.RuntimeFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Internal, missingTenantRuntimeMessage)
		}
		return runtimeCfg.Tenant.ID, nil
	}
	response, err := interceptor(metadataContext, request, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		testHandle.Fatalf(expectedInterceptorSuccessTemplate, err)
	}
	if response != testTenantID {
		testHandle.Fatalf(expectedTenantIDTemplate, testTenantID, response)
	}
}

func TestBuildTenantInterceptorRejectsMissingTenantID(testHandle *testing.T) {
	testHandle.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	repo := newTestTenantRepository(testHandle, testTenantID)
	interceptor := buildTenantInterceptor(logger, repo)
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return "ok", nil
	}
	_, err := interceptor(context.Background(), &grpcapi.ListNotificationsRequest{}, &grpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.InvalidArgument {
		testHandle.Fatalf("expected invalid argument, got %v", err)
	}
	if handlerCalled {
		testHandle.Fatal(expectedHandlerNotCalledMessage)
	}
}

func TestBuildTenantInterceptorRejectsUnknownTenant(testHandle *testing.T) {
	testHandle.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	repo := newTestTenantRepository(testHandle, testTenantID)
	interceptor := buildTenantInterceptor(logger, repo)
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return "ok", nil
	}
	request := &grpcapi.CancelNotificationRequest{TenantId: "missing-tenant"}
	_, err := interceptor(context.Background(), request, &grpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.NotFound {
		testHandle.Fatalf("expected not found, got %v", err)
	}
	if handlerCalled {
		testHandle.Fatal(expectedHandlerNotCalledMessage)
	}
}

func newTestTenantRepository(testHandle *testing.T, tenantID string) *tenant.Repository {
	testHandle.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		testHandle.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(
		&tenant.Tenant{},
		&tenant.TenantDomain{},
		&tenant.TenantMember{},
		&tenant.EmailProfile{},
		&tenant.SMSProfile{},
	); err != nil {
		testHandle.Fatalf("auto migrate: %v", err)
	}
	secretKeeper, err := tenant.NewSecretKeeper(strings.Repeat("a", 64))
	if err != nil {
		testHandle.Fatalf("init secret keeper: %v", err)
	}
	enabled := true
	bootstrapCfg := tenant.BootstrapConfig{
		Tenants: []tenant.BootstrapTenant{
			{
				ID:          tenantID,
				DisplayName: "Test Tenant",
				Enabled:     &enabled,
				Domains:     []string{"test.localhost"},
				Admins:      tenant.BootstrapAdmins{"admin@example.com"},
				EmailProfile: tenant.BootstrapEmailProfile{
					Host:        "smtp.localhost",
					Port:        587,
					Username:    "smtp-user",
					Password:    "smtp-pass",
					FromAddress: "admin@example.com",
				},
			},
		},
	}
	if err := tenant.Bootstrap(context.Background(), database, secretKeeper, bootstrapCfg); err != nil {
		testHandle.Fatalf("bootstrap tenants: %v", err)
	}
	return tenant.NewRepository(database, secretKeeper)
}
