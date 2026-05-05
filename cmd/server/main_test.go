package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/httpapi"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/smtpsubmission"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		nil,
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
		grpcapi.Status_QUEUED,
		grpcapi.Status_SENT,
		grpcapi.Status_FAILED,
		grpcapi.Status_CANCELLED,
		grpcapi.Status_ERRORED,
		grpcapi.Status_UNKNOWN,
	})
	if len(statuses) != 6 {
		t.Fatalf("expected 6 statuses, got %d", len(statuses))
	}
	if statuses[0] != model.StatusQueued || statuses[3] != model.StatusCancelled || statuses[4] != model.StatusErrored {
		t.Fatalf("unexpected status mapping %v", statuses)
	}
	if mapGrpcStatuses([]grpcapi.Status{grpcapi.Status(99)}) != nil {
		t.Fatalf("expected nil for unsupported statuses")
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

	cancelled := mapModelToGrpcResponse(model.NotificationResponse{
		NotificationType: model.NotificationEmail,
		Status:           model.StatusCancelled,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if cancelled.Status != grpcapi.Status_CANCELLED {
		t.Fatalf("expected cancelled status, got %v", cancelled.Status)
	}
	failed := mapModelToGrpcResponse(model.NotificationResponse{
		NotificationType: model.NotificationEmail,
		Status:           model.StatusFailed,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if failed.Status != grpcapi.Status_FAILED {
		t.Fatalf("expected failed status, got %v", failed.Status)
	}
	unknown := mapModelToGrpcResponse(model.NotificationResponse{
		NotificationType: "push",
		Status:           "mystery",
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if unknown.NotificationType != grpcapi.NotificationType_EMAIL || unknown.Status != grpcapi.Status_UNKNOWN {
		t.Fatalf("expected default type/status, got %v/%v", unknown.NotificationType, unknown.Status)
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

	t.Run("MissingAuthorization", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-other", "value"))
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated for missing authorization, got %v", err)
		}
	})

	t.Run("InvalidAuthorizationFormat", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic token"))
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected unauthenticated for invalid authorization, got %v", err)
		}
	})
}

func TestBuildTenantInterceptorRejectsMissingRepository(testHandle *testing.T) {
	testHandle.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	interceptor := buildTenantInterceptor(logger, nil)
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return "ok", nil
	}
	_, err := interceptor(context.Background(), &grpcapi.NotificationRequest{TenantId: testTenantID}, &grpc.UnaryServerInfo{}, handler)
	if status.Code(err) != codes.Internal {
		testHandle.Fatalf("expected internal error, got %v", err)
	}
	if handlerCalled {
		testHandle.Fatal(expectedHandlerNotCalledMessage)
	}
}

func TestNotificationServiceServerHandlers(testHandle *testing.T) {
	testHandle.Helper()
	now := time.Now().UTC()
	scheduled := now.Add(time.Hour)
	service := &recordingNotificationService{
		response: model.NotificationResponse{
			NotificationID:    "notif-one",
			NotificationType:  model.NotificationEmail,
			Recipient:         "user@example.com",
			Subject:           "Subject",
			Message:           "Body",
			Status:            model.StatusSent,
			ProviderMessageID: "provider",
			RetryCount:        1,
			CreatedAt:         now,
			UpdatedAt:         now,
			ScheduledFor:      &scheduled,
			TenantID:          testTenantID,
		},
		listResponses: []model.NotificationResponse{
			{
				NotificationID:   "notif-list",
				NotificationType: model.NotificationSMS,
				Recipient:        "+15551234567",
				Message:          "Body",
				Status:           model.StatusQueued,
				CreatedAt:        now,
				UpdatedAt:        now,
				TenantID:         testTenantID,
			},
		},
	}
	server := &notificationServiceServer{
		notificationService: service,
		logger:              slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}
	ctx := context.Background()

	sendResponse, sendErr := server.SendNotification(ctx, &grpcapi.NotificationRequest{
		NotificationType: grpcapi.NotificationType_EMAIL,
		Recipient:        "user@example.com",
		Subject:          "Subject",
		Message:          "Body",
		ScheduledTime:    timestamppb.New(scheduled),
		Attachments: []*grpcapi.EmailAttachment{
			{Filename: "a.txt", ContentType: "text/plain", Data: []byte("hello")},
		},
	})
	if sendErr != nil {
		testHandle.Fatalf("send notification: %v", sendErr)
	}
	if sendResponse.GetNotificationId() != "notif-one" {
		testHandle.Fatalf("unexpected send response %+v", sendResponse)
	}
	if service.sentRequest.Recipient() != "user@example.com" || len(service.sentRequest.Attachments()) != 1 {
		testHandle.Fatalf("unexpected sent request")
	}

	statusResponse, statusErr := server.GetNotificationStatus(ctx, &grpcapi.GetNotificationStatusRequest{NotificationId: "notif-one"})
	if statusErr != nil || statusResponse.GetNotificationId() != "notif-one" {
		testHandle.Fatalf("status response=%+v err=%v", statusResponse, statusErr)
	}
	if service.statusID != "notif-one" {
		testHandle.Fatalf("expected status id recorded")
	}

	listResponse, listErr := server.ListNotifications(ctx, &grpcapi.ListNotificationsRequest{Statuses: []grpcapi.Status{grpcapi.Status_QUEUED}})
	if listErr != nil {
		testHandle.Fatalf("list notifications: %v", listErr)
	}
	if len(listResponse.GetNotifications()) != 1 || service.listFilters.Statuses[0] != model.StatusQueued {
		testHandle.Fatalf("unexpected list response/filter")
	}
	nilListResponse, nilListErr := server.ListNotifications(ctx, nil)
	if nilListErr != nil || len(nilListResponse.GetNotifications()) != 1 {
		testHandle.Fatalf("nil list response=%+v err=%v", nilListResponse, nilListErr)
	}

	rescheduleResponse, rescheduleErr := server.RescheduleNotification(ctx, &grpcapi.RescheduleNotificationRequest{
		NotificationId: "notif-one",
		ScheduledTime:  timestamppb.New(scheduled),
	})
	if rescheduleErr != nil || rescheduleResponse.GetNotificationId() != "notif-one" {
		testHandle.Fatalf("reschedule response=%+v err=%v", rescheduleResponse, rescheduleErr)
	}
	if service.rescheduleID != "notif-one" || !service.rescheduledFor.Equal(scheduled) {
		testHandle.Fatalf("unexpected reschedule capture")
	}

	cancelResponse, cancelErr := server.CancelNotification(ctx, &grpcapi.CancelNotificationRequest{NotificationId: "notif-one"})
	if cancelErr != nil || cancelResponse.GetNotificationId() != "notif-one" {
		testHandle.Fatalf("cancel response=%+v err=%v", cancelResponse, cancelErr)
	}
	if service.cancelID != "notif-one" {
		testHandle.Fatalf("expected cancel id recorded")
	}
}

func TestNotificationServiceServerValidationAndServiceErrors(testHandle *testing.T) {
	testHandle.Helper()
	serviceErr := errors.New("service failed")
	server := &notificationServiceServer{
		notificationService: &recordingNotificationService{
			err:     serviceErr,
			listErr: serviceErr,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}
	ctx := context.Background()

	testCases := []struct {
		name string
		call func() error
		code codes.Code
	}{
		{name: "send unsupported type", call: func() error {
			_, err := server.SendNotification(ctx, &grpcapi.NotificationRequest{NotificationType: grpcapi.NotificationType(99)})
			return err
		}, code: codes.Unknown},
		{name: "send invalid scheduled timestamp", call: func() error {
			_, err := server.SendNotification(ctx, &grpcapi.NotificationRequest{
				NotificationType: grpcapi.NotificationType_EMAIL,
				Recipient:        "user@example.com",
				Subject:          "Subject",
				Message:          "Body",
				ScheduledTime:    &timestamppb.Timestamp{Seconds: math.MaxInt64},
			})
			return err
		}, code: codes.InvalidArgument},
		{name: "send invalid model request", call: func() error {
			_, err := server.SendNotification(ctx, &grpcapi.NotificationRequest{NotificationType: grpcapi.NotificationType_EMAIL})
			return err
		}, code: codes.InvalidArgument},
		{name: "send service error", call: func() error {
			_, err := server.SendNotification(ctx, &grpcapi.NotificationRequest{
				NotificationType: grpcapi.NotificationType_SMS,
				Recipient:        "+15551234567",
				Message:          "Body",
			})
			return err
		}, code: codes.Unknown},
		{name: "status missing id", call: func() error {
			_, err := server.GetNotificationStatus(ctx, &grpcapi.GetNotificationStatusRequest{NotificationId: " "})
			return err
		}, code: codes.InvalidArgument},
		{name: "status service error", call: func() error {
			_, err := server.GetNotificationStatus(ctx, &grpcapi.GetNotificationStatusRequest{NotificationId: "notif"})
			return err
		}, code: codes.Unknown},
		{name: "list service error", call: func() error {
			_, err := server.ListNotifications(ctx, &grpcapi.ListNotificationsRequest{})
			return err
		}, code: codes.Unknown},
		{name: "reschedule missing id", call: func() error {
			_, err := server.RescheduleNotification(ctx, &grpcapi.RescheduleNotificationRequest{ScheduledTime: timestamppb.Now()})
			return err
		}, code: codes.InvalidArgument},
		{name: "reschedule missing time", call: func() error {
			_, err := server.RescheduleNotification(ctx, &grpcapi.RescheduleNotificationRequest{NotificationId: "notif"})
			return err
		}, code: codes.InvalidArgument},
		{name: "reschedule invalid time", call: func() error {
			_, err := server.RescheduleNotification(ctx, &grpcapi.RescheduleNotificationRequest{
				NotificationId: "notif",
				ScheduledTime:  &timestamppb.Timestamp{Seconds: math.MaxInt64},
			})
			return err
		}, code: codes.InvalidArgument},
		{name: "reschedule past time", call: func() error {
			_, err := server.RescheduleNotification(ctx, &grpcapi.RescheduleNotificationRequest{
				NotificationId: "notif",
				ScheduledTime:  timestamppb.New(time.Now().Add(-time.Hour)),
			})
			return err
		}, code: codes.InvalidArgument},
		{name: "reschedule service error", call: func() error {
			_, err := server.RescheduleNotification(ctx, &grpcapi.RescheduleNotificationRequest{
				NotificationId: "notif",
				ScheduledTime:  timestamppb.New(time.Now().Add(time.Hour)),
			})
			return err
		}, code: codes.Unknown},
		{name: "cancel missing id", call: func() error {
			_, err := server.CancelNotification(ctx, &grpcapi.CancelNotificationRequest{NotificationId: " "})
			return err
		}, code: codes.InvalidArgument},
		{name: "cancel service error", call: func() error {
			_, err := server.CancelNotification(ctx, &grpcapi.CancelNotificationRequest{NotificationId: "notif"})
			return err
		}, code: codes.Unknown},
	}
	for _, testCase := range testCases {
		testCase := testCase
		testHandle.Run(testCase.name, func(t *testing.T) {
			err := testCase.call()
			if err == nil {
				t.Fatalf("expected error")
			}
			if testCase.code != codes.Unknown && status.Code(err) != testCase.code {
				t.Fatalf("expected code %s, got %v", testCase.code, err)
			}
		})
	}
}

func TestSMTPPublicSettings(testHandle *testing.T) {
	testHandle.Helper()
	startTLS := smtpPublicSettings(configSMTPSubmission(":2525", ""))
	if startTLS.Port != 2525 || startTLS.SecurityMode != "starttls" {
		testHandle.Fatalf("unexpected starttls settings %+v", startTLS)
	}
	implicitTLS := smtpPublicSettings(configSMTPSubmission("", "127.0.0.1:2465"))
	if implicitTLS.Port != 2465 || implicitTLS.SecurityMode != "ssl" {
		testHandle.Fatalf("unexpected implicit tls settings %+v", implicitTLS)
	}
	caddyTerminated := configSMTPSubmission(":587", "")
	caddyTerminated.PublicPort = 465
	caddyTerminated.PublicSecurityMode = "ssl"
	publicSettings := smtpPublicSettings(caddyTerminated)
	if publicSettings.Port != 465 || publicSettings.SecurityMode != "ssl" {
		testHandle.Fatalf("unexpected caddy-terminated public settings %+v", publicSettings)
	}
	defaultStartTLS := smtpPublicSettings(configSMTPSubmission("bad", ""))
	if defaultStartTLS.Port != 587 {
		testHandle.Fatalf("expected starttls fallback port, got %d", defaultStartTLS.Port)
	}
	defaultImplicitTLS := smtpPublicSettings(configSMTPSubmission("", "bad"))
	if defaultImplicitTLS.Port != 465 {
		testHandle.Fatalf("expected implicit tls fallback port, got %d", defaultImplicitTLS.Port)
	}
	if smtpPortFromAddr(" ", 25) != 25 {
		testHandle.Fatalf("expected blank address fallback")
	}
}

func TestRunServerSuccessWithInlineTenants(testHandle *testing.T) {
	testHandle.Helper()
	cfg := serverTestConfig()
	state, dependencies := newServerTestDependencies(cfg)

	if exitCode := runServer(nil, dependencies); exitCode != 0 {
		testHandle.Fatalf("expected success exit code, got %d", exitCode)
	}
	if !state.bootstrapCalled || state.bootstrapFileCalled {
		testHandle.Fatalf("expected inline bootstrap, state=%+v", state)
	}
	if !state.grpcServed {
		testHandle.Fatalf("expected grpc server to be served")
	}
}

func TestRunServerStartsWebAndSMTPSubmission(testHandle *testing.T) {
	testHandle.Helper()
	cfg := serverTestConfig()
	cfg.TenantBootstrap = tenant.BootstrapConfig{}
	cfg.TenantConfigPath = "tenants.yml"
	cfg.WebInterfaceEnabled = true
	cfg.HTTPListenAddr = "127.0.0.1:8080"
	cfg.TAuthSigningKey = "signing-key"
	cfg.TAuthCookieName = "app_session"
	cfg.TAuthBaseURL = "https://tauth.example.com"
	cfg.TAuthTenantID = "tauth"
	cfg.TAuthGoogleClientID = "client"
	cfg.SMTPSubmission = config.SMTPSubmissionConfig{
		Enabled:           true,
		Hostname:          "smtp.example.com",
		ListenAddr:        ":587",
		TLSListenAddr:     ":465",
		TLSCertPath:       "cert.pem",
		TLSKeyPath:        "key.pem",
		MaxMessageBytes:   1024,
		MaxRecipients:     2,
		AllowInsecureAuth: true,
		SenderDomains:     []string{"example.com"},
	}
	state, dependencies := newServerTestDependencies(cfg)
	state.httpServer.shutdownErr = errors.New("shutdown failed")

	if exitCode := runServer(nil, dependencies); exitCode != 0 {
		testHandle.Fatalf("expected success exit code, got %d", exitCode)
	}
	waitForClosed(testHandle, state.smtpStarter.started)
	waitForClosed(testHandle, state.httpServer.started)
	if !state.bootstrapFileCalled || !state.senderDomainsReplaced || !state.tlsLoaded {
		testHandle.Fatalf("expected file/bootstrap/tls/sender-domain setup, state=%+v", state)
	}
	if !state.httpServer.shutdownCalled {
		testHandle.Fatalf("expected HTTP shutdown")
	}
	if state.smtpConfig.TLSConfig == nil || state.smtpConfig.Relay == nil {
		testHandle.Fatalf("expected SMTP config to include TLS and relay")
	}
}

func TestRunServerErrorPaths(testHandle *testing.T) {
	testHandle.Helper()
	expectedErr := errors.New("boom")
	testCases := []struct {
		name   string
		args   []string
		config func() config.Config
		mutate func(*serverDependencies)
	}{
		{name: "flag parse", args: []string{"-unknown"}, config: serverTestConfig},
		{name: "config", config: serverTestConfig, mutate: func(deps *serverDependencies) {
			deps.loadConfig = func() (config.Config, error) { return config.Config{}, expectedErr }
		}},
		{name: "database", config: serverTestConfig, mutate: func(deps *serverDependencies) {
			deps.initDB = func(string, *slog.Logger) (*gorm.DB, error) { return nil, expectedErr }
		}},
		{name: "secret keeper", config: serverTestConfig, mutate: func(deps *serverDependencies) {
			deps.newSecretKeeper = func(string) (*tenant.SecretKeeper, error) { return nil, expectedErr }
		}},
		{name: "inline bootstrap", config: serverTestConfig, mutate: func(deps *serverDependencies) {
			deps.bootstrapTenants = func(context.Context, *gorm.DB, *tenant.SecretKeeper, tenant.BootstrapConfig) error {
				return expectedErr
			}
		}},
		{name: "file bootstrap", config: func() config.Config {
			cfg := serverTestConfig()
			cfg.TenantBootstrap = tenant.BootstrapConfig{}
			cfg.TenantConfigPath = "tenants.yml"
			return cfg
		}, mutate: func(deps *serverDependencies) {
			deps.bootstrapTenantsFromFile = func(context.Context, *gorm.DB, *tenant.SecretKeeper, string) error {
				return expectedErr
			}
		}},
		{name: "missing tenant config", config: func() config.Config {
			cfg := serverTestConfig()
			cfg.TenantBootstrap = tenant.BootstrapConfig{}
			return cfg
		}},
		{name: "smtp identity repository", config: serverTestConfig, mutate: func(deps *serverDependencies) {
			deps.newSMTPIdentityRepository = func(*gorm.DB, string) (*smtpidentity.Repository, error) { return nil, expectedErr }
		}},
		{name: "sender domains", config: func() config.Config {
			cfg := serverTestConfig()
			cfg.SMTPSubmission.SenderDomains = []string{"example.com"}
			return cfg
		}, mutate: func(deps *serverDependencies) {
			deps.replaceSenderDomains = func(context.Context, *gorm.DB, []string) error { return expectedErr }
		}},
		{name: "tls load", config: func() config.Config {
			cfg := serverTestConfig()
			cfg.SMTPSubmission.Enabled = true
			cfg.SMTPSubmission.TLSCertPath = "cert.pem"
			cfg.SMTPSubmission.TLSKeyPath = "key.pem"
			return cfg
		}, mutate: func(deps *serverDependencies) {
			deps.loadTLSConfig = func(string, string) (*tls.Config, error) { return nil, expectedErr }
		}},
		{name: "smtp server", config: func() config.Config {
			cfg := serverTestConfig()
			cfg.SMTPSubmission.Enabled = true
			return cfg
		}, mutate: func(deps *serverDependencies) {
			deps.newSMTPSubmissionServer = func(smtpsubmission.Config) (smtpSubmissionStarter, error) { return nil, expectedErr }
		}},
		{name: "session validator", config: func() config.Config {
			cfg := serverTestConfig()
			cfg.WebInterfaceEnabled = true
			return cfg
		}, mutate: func(deps *serverDependencies) {
			deps.newSessionValidator = func(sessionvalidator.Config) (httpapi.SessionValidator, error) { return nil, expectedErr }
		}},
		{name: "http server", config: func() config.Config {
			cfg := serverTestConfig()
			cfg.WebInterfaceEnabled = true
			return cfg
		}, mutate: func(deps *serverDependencies) {
			deps.newHTTPServer = func(httpapi.Config) (httpServerRunner, error) { return nil, expectedErr }
		}},
		{name: "listen", config: serverTestConfig, mutate: func(deps *serverDependencies) {
			deps.listen = func(string, string) (net.Listener, error) { return nil, expectedErr }
		}},
		{name: "serve grpc", config: serverTestConfig, mutate: func(deps *serverDependencies) {
			deps.serveGRPC = func(net.Listener, service.NotificationService, *tenant.Repository, *slog.Logger, string) error {
				return expectedErr
			}
		}},
	}
	for _, testCase := range testCases {
		testCase := testCase
		testHandle.Run(testCase.name, func(t *testing.T) {
			state, dependencies := newServerTestDependencies(testCase.config())
			_ = state
			if testCase.mutate != nil {
				testCase.mutate(&dependencies)
			}
			if exitCode := runServer(testCase.args, dependencies); exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d", exitCode)
			}
		})
	}
}

func TestRunServerHelpAndExitWrapper(testHandle *testing.T) {
	testHandle.Helper()
	state, dependencies := newServerTestDependencies(serverTestConfig())
	exitCodes := make(chan int, 1)
	dependencies.exit = func(code int) {
		exitCodes <- code
	}
	if exitCode := runServer([]string{"-h"}, dependencies); exitCode != 0 {
		testHandle.Fatalf("expected help success, got %d", exitCode)
	}
	dependencies.loadConfig = func() (config.Config, error) {
		return config.Config{}, errors.New("config failed")
	}
	runServerAndExit(nil, dependencies)
	select {
	case code := <-exitCodes:
		if code != 1 {
			testHandle.Fatalf("expected exit code 1, got %d", code)
		}
	case <-time.After(time.Second):
		testHandle.Fatalf("expected exit to be called")
	}
	if state.grpcServed {
		testHandle.Fatalf("grpc should not serve after config failure")
	}
}

func TestServerMainReturnsForHelp(testHandle *testing.T) {
	testHandle.Helper()
	oldArgs := os.Args
	oldStderr := os.Stderr
	readPipe, writePipe, pipeErr := os.Pipe()
	if pipeErr != nil {
		testHandle.Fatalf("create stderr pipe: %v", pipeErr)
	}
	os.Args = []string{"pinguin-server", "-h"}
	os.Stderr = writePipe
	defer func() {
		os.Args = oldArgs
		os.Stderr = oldStderr
		_ = readPipe.Close()
	}()

	main()
	_ = writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	if readErr != nil {
		testHandle.Fatalf("read stderr: %v", readErr)
	}
	if !strings.Contains(string(output), "Usage of pinguin-server") {
		testHandle.Fatalf("expected help output, got %q", string(output))
	}
}

func TestServerBackgroundStarters(testHandle *testing.T) {
	testHandle.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	smtpExit := make(chan int, 1)
	smtpStarter := &fakeSMTPStarter{err: errors.New("smtp crashed"), started: make(chan struct{})}
	startSMTPSubmission(context.Background(), logger, smtpStarter, serverTestConfig(), func(code int) {
		smtpExit <- code
	})
	waitForClosed(testHandle, smtpStarter.started)
	select {
	case code := <-smtpExit:
		if code != 1 {
			testHandle.Fatalf("expected smtp exit 1, got %d", code)
		}
	case <-time.After(time.Second):
		testHandle.Fatalf("expected smtp crash exit")
	}

	httpExit := make(chan int, 1)
	httpStarter := &fakeHTTPServer{startErr: errors.New("http crashed"), started: make(chan struct{})}
	startHTTPServer(logger, httpStarter, ":8080", func(code int) {
		httpExit <- code
	})
	waitForClosed(testHandle, httpStarter.started)
	select {
	case code := <-httpExit:
		if code != 1 {
			testHandle.Fatalf("expected http exit 1, got %d", code)
		}
	case <-time.After(time.Second):
		testHandle.Fatalf("expected http crash exit")
	}

	closedHTTP := &fakeHTTPServer{startErr: http.ErrServerClosed, started: make(chan struct{})}
	startHTTPServer(logger, closedHTTP, ":8080", func(int) {
		testHandle.Fatalf("server closed should not exit")
	})
	waitForClosed(testHandle, closedHTTP.started)
}

func TestServerDependencyDefaultsAndProductionWrappers(testHandle *testing.T) {
	testHandle.Helper()
	defaulted := withServerDependencyDefaults(serverDependencies{})
	if defaulted.loadConfig == nil || defaulted.newLogger == nil || defaulted.initDB == nil || defaulted.exit == nil {
		testHandle.Fatalf("expected production defaults to be populated")
	}

	production := productionServerDependencies()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	upstreamRelay := production.newSMTPRelay(logger, serverTestConfig())
	if _, ok := upstreamRelay.(*smtpsubmission.UpstreamRelay); !ok {
		testHandle.Fatalf("expected upstream relay default, got %T", upstreamRelay)
	}
	directConfig := serverTestConfig()
	directConfig.SMTPSubmission.DeliveryMode = "direct"
	directRelay := production.newSMTPRelay(logger, directConfig)
	if _, ok := directRelay.(*smtpsubmission.DirectMXRelay); !ok {
		testHandle.Fatalf("expected direct relay, got %T", directRelay)
	}
	if _, err := production.newSMTPSubmissionServer(smtpsubmission.Config{}); err == nil {
		testHandle.Fatalf("expected invalid SMTP submission config error")
	}
	if _, err := production.newSessionValidator(sessionvalidator.Config{}); err == nil {
		testHandle.Fatalf("expected invalid session validator error")
	}
	if _, err := production.newHTTPServer(httpapi.Config{}); err == nil {
		testHandle.Fatalf("expected invalid HTTP server config error")
	}
}

func TestServeGRPCBuildsServer(testHandle *testing.T) {
	testHandle.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		testHandle.Fatalf("listen: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveGRPC(listener, &recordingNotificationService{}, nil, logger, "token")
	}()
	if err := listener.Close(); err != nil {
		testHandle.Fatalf("close listener: %v", err)
	}
	select {
	case err := <-errCh:
		if err == nil {
			testHandle.Fatalf("expected serve error after listener close")
		}
	case <-time.After(time.Second):
		testHandle.Fatalf("timed out waiting for grpc serve")
	}
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
		&tenant.TenantAdmin{},
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

type recordingNotificationService struct {
	response       model.NotificationResponse
	listResponses  []model.NotificationResponse
	err            error
	listErr        error
	sentRequest    model.NotificationRequest
	statusID       string
	listFilters    model.NotificationListFilters
	rescheduleID   string
	rescheduledFor time.Time
	cancelID       string
}

func (service *recordingNotificationService) SendNotification(_ context.Context, request model.NotificationRequest) (model.NotificationResponse, error) {
	service.sentRequest = request
	if service.err != nil {
		return model.NotificationResponse{}, service.err
	}
	return service.response, nil
}

func (service *recordingNotificationService) GetNotificationStatus(_ context.Context, notificationID string) (model.NotificationResponse, error) {
	service.statusID = notificationID
	if service.err != nil {
		return model.NotificationResponse{}, service.err
	}
	return service.response, nil
}

func (service *recordingNotificationService) ListNotifications(_ context.Context, filters model.NotificationListFilters) ([]model.NotificationResponse, error) {
	service.listFilters = filters
	if service.listErr != nil {
		return nil, service.listErr
	}
	return service.listResponses, nil
}

func (service *recordingNotificationService) ListNotificationsPage(_ context.Context, filters model.NotificationListFilters, _ model.NotificationListPageRequest) (model.NotificationListResponsePage, error) {
	service.listFilters = filters
	if service.listErr != nil {
		return model.NotificationListResponsePage{}, service.listErr
	}
	return model.NotificationListResponsePage{Notifications: service.listResponses}, nil
}

func (service *recordingNotificationService) ListNotificationsAll(_ context.Context, filters model.NotificationListFilters) ([]model.NotificationResponse, error) {
	service.listFilters = filters
	if service.listErr != nil {
		return nil, service.listErr
	}
	return service.listResponses, nil
}

func (service *recordingNotificationService) RescheduleNotification(_ context.Context, notificationID string, scheduledFor time.Time) (model.NotificationResponse, error) {
	service.rescheduleID = notificationID
	service.rescheduledFor = scheduledFor
	if service.err != nil {
		return model.NotificationResponse{}, service.err
	}
	return service.response, nil
}

func (service *recordingNotificationService) CancelNotification(_ context.Context, notificationID string) (model.NotificationResponse, error) {
	service.cancelID = notificationID
	if service.err != nil {
		return model.NotificationResponse{}, service.err
	}
	return service.response, nil
}

func (service *recordingNotificationService) StartRetryWorker(context.Context) {}

func configSMTPSubmission(listenAddr string, tlsListenAddr string) config.SMTPSubmissionConfig {
	return config.SMTPSubmissionConfig{
		Hostname:      "smtp.example.com",
		ListenAddr:    listenAddr,
		TLSListenAddr: tlsListenAddr,
	}
}

type serverTestState struct {
	bootstrapCalled       bool
	bootstrapFileCalled   bool
	senderDomainsReplaced bool
	tlsLoaded             bool
	grpcServed            bool
	smtpConfig            smtpsubmission.Config
	smtpStarter           *fakeSMTPStarter
	httpServer            *fakeHTTPServer
}

func serverTestConfig() config.Config {
	return config.Config{
		DatabasePath:         "pinguin.db",
		GRPCAuthToken:        "token",
		LogLevel:             "INFO",
		MaxRetries:           3,
		RetryIntervalSec:     60,
		MasterEncryptionKey:  strings.Repeat("a", 64),
		ConnectionTimeoutSec: 5,
		OperationTimeoutSec:  30,
		TenantBootstrap: tenant.BootstrapConfig{
			Tenants: []tenant.BootstrapTenant{
				{
					ID:          testTenantID,
					DisplayName: "Test Tenant",
					Domains:     []string{"test.localhost"},
					EmailProfile: tenant.BootstrapEmailProfile{
						Host:        "smtp.localhost",
						Port:        587,
						Username:    "smtp-user",
						Password:    "smtp-pass",
						FromAddress: "admin@example.com",
					},
				},
			},
		},
		SMTPSubmission: config.SMTPSubmissionConfig{
			Hostname:        "smtp.example.com",
			ListenAddr:      ":587",
			MaxMessageBytes: 1024,
			MaxRecipients:   1,
		},
	}
}

func newServerTestDependencies(cfg config.Config) (*serverTestState, serverDependencies) {
	state := &serverTestState{
		smtpStarter: &fakeSMTPStarter{started: make(chan struct{})},
		httpServer:  &fakeHTTPServer{startErr: http.ErrServerClosed, started: make(chan struct{})},
	}
	dependencies := serverDependencies{
		loadConfig: func() (config.Config, error) {
			return cfg, nil
		},
		newLogger: func(string) *slog.Logger {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
		},
		initDB: func(string, *slog.Logger) (*gorm.DB, error) {
			return nil, nil
		},
		newSecretKeeper: func(string) (*tenant.SecretKeeper, error) {
			return &tenant.SecretKeeper{}, nil
		},
		bootstrapTenants: func(context.Context, *gorm.DB, *tenant.SecretKeeper, tenant.BootstrapConfig) error {
			state.bootstrapCalled = true
			return nil
		},
		bootstrapTenantsFromFile: func(context.Context, *gorm.DB, *tenant.SecretKeeper, string) error {
			state.bootstrapFileCalled = true
			return nil
		},
		newTenantRepository: func(*gorm.DB, *tenant.SecretKeeper) *tenant.Repository {
			return nil
		},
		newSMTPIdentityRepository: func(*gorm.DB, string) (*smtpidentity.Repository, error) {
			return &smtpidentity.Repository{}, nil
		},
		replaceSenderDomains: func(context.Context, *gorm.DB, []string) error {
			state.senderDomainsReplaced = true
			return nil
		},
		newSMTPIdentityService: func(repository *smtpidentity.Repository, settings smtpidentity.PublicSettings) *smtpidentity.Service {
			return smtpidentity.NewService(repository, settings)
		},
		newNotificationService: func(*gorm.DB, *slog.Logger, config.Config, *tenant.Repository) service.NotificationService {
			return &recordingNotificationService{}
		},
		loadTLSConfig: func(string, string) (*tls.Config, error) {
			state.tlsLoaded = true
			return &tls.Config{MinVersion: tls.VersionTLS12}, nil
		},
		newSMTPRelay: func(*slog.Logger, config.Config) smtpsubmission.RawRelay {
			return noopRawRelay{}
		},
		newSMTPSubmissionServer: func(smtpConfig smtpsubmission.Config) (smtpSubmissionStarter, error) {
			state.smtpConfig = smtpConfig
			return state.smtpStarter, nil
		},
		newSessionValidator: func(sessionvalidator.Config) (httpapi.SessionValidator, error) {
			return fakeSessionValidator{}, nil
		},
		newHTTPServer: func(httpConfig httpapi.Config) (httpServerRunner, error) {
			_ = httpConfig
			return state.httpServer, nil
		},
		listen: func(string, string) (net.Listener, error) {
			return fakeListener{}, nil
		},
		serveGRPC: func(listener net.Listener, svc service.NotificationService, repo *tenant.Repository, logger *slog.Logger, token string) error {
			_ = listener
			_ = svc
			_ = repo
			_ = logger
			if token != cfg.GRPCAuthToken {
				return errors.New("unexpected token")
			}
			state.grpcServed = true
			return nil
		},
		exit: func(int) {},
	}
	return state, dependencies
}

type fakeSMTPStarter struct {
	err     error
	started chan struct{}
}

func (starter *fakeSMTPStarter) Start(context.Context) error {
	close(starter.started)
	return starter.err
}

type fakeHTTPServer struct {
	startErr       error
	shutdownErr    error
	started        chan struct{}
	shutdownCalled bool
}

func (server *fakeHTTPServer) Start() error {
	close(server.started)
	return server.startErr
}

func (server *fakeHTTPServer) Shutdown(context.Context) error {
	server.shutdownCalled = true
	return server.shutdownErr
}

type noopRawRelay struct{}

func (noopRawRelay) Relay(context.Context, smtpsubmission.RawMessage) error {
	return nil
}

type fakeSessionValidator struct{}

func (fakeSessionValidator) ValidateRequest(*http.Request) (*sessionvalidator.Claims, error) {
	return &sessionvalidator.Claims{UserEmail: "user@example.com"}, nil
}

type fakeListener struct{}

func (fakeListener) Accept() (net.Conn, error) {
	return nil, errors.New("listener closed")
}

func (fakeListener) Close() error {
	return nil
}

func (fakeListener) Addr() net.Addr {
	return fakeAddr("127.0.0.1:0")
}

type fakeAddr string

func (addr fakeAddr) Network() string { return "tcp" }
func (addr fakeAddr) String() string  { return string(addr) }

func waitForClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for goroutine")
	}
}
