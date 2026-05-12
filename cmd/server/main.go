package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/db"
	"github.com/tyemirov/pinguin/internal/httpapi"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
	"github.com/tyemirov/pinguin/internal/smtpforwarding"
	"github.com/tyemirov/pinguin/internal/smtpidentity"
	"github.com/tyemirov/pinguin/internal/smtpsubmission"
	"github.com/tyemirov/pinguin/internal/tenant"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"github.com/tyemirov/pinguin/pkg/grpcutil"
	"github.com/tyemirov/pinguin/pkg/logging"
	sessionvalidator "github.com/tyemirov/tauth/pkg/sessionvalidator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
	"log/slog"
)

// notificationServiceServer implements grpcapi.NotificationServiceServer.
type notificationServiceServer struct {
	grpcapi.UnimplementedNotificationServiceServer
	notificationService service.NotificationService
	logger              *slog.Logger
}

const (
	tenantMetadataKey                = "x-tenant-id"
	tenantIDRequiredMessage          = "tenant_id is required"
	tenantNotFoundMessage            = "tenant not found"
	tenantRepositoryUnavailableError = "tenant repository unavailable"
	notificationIDRequiredMessage    = "notification_id is required"
	scheduledTimeRequiredMessage     = "scheduled_time is required"
	scheduledTimeFutureMessage       = "scheduled_time must be in the future"
)

func (server *notificationServiceServer) SendNotification(ctx context.Context, req *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	var internalType model.NotificationType
	switch req.NotificationType {
	case grpcapi.NotificationType_EMAIL:
		internalType = model.NotificationEmail
	case grpcapi.NotificationType_SMS:
		internalType = model.NotificationSMS
	default:
		server.logger.Error("Unsupported notification type", "type", req.NotificationType)
		return nil, fmt.Errorf("unsupported notification type: %v", req.NotificationType)
	}

	var scheduledFor *time.Time
	if req.ScheduledTime != nil {
		if err := req.ScheduledTime.CheckValid(); err != nil {
			server.logger.Error("Invalid scheduled timestamp", "error", err)
			return nil, status.Errorf(codes.InvalidArgument, "invalid scheduled_time: %v", err)
		}
		normalizedScheduled := req.ScheduledTime.AsTime().UTC()
		scheduledFor = &normalizedScheduled
	}

	attachments := mapGrpcAttachments(req.GetAttachments())
	modelRequest, requestError := model.NewNotificationRequest(
		internalType,
		req.GetRecipient(),
		req.GetSubject(),
		req.GetMessage(),
		scheduledFor,
		attachments,
	)
	if requestError != nil {
		server.logger.Error("Invalid notification request", "error", requestError)
		return nil, status.Error(codes.InvalidArgument, requestError.Error())
	}

	recipientDigest := digestForLogging(modelRequest.Recipient())
	subjectDigest := digestForLogging(modelRequest.Subject())
	server.logger.Info(
		"notification_request_received",
		"notification_type", req.NotificationType.String(),
		"subject_digest", subjectDigest,
		"recipient_digest", recipientDigest,
		"scheduled", scheduledFor != nil,
		"attachment_count", len(attachments),
	)

	modelResponse, err := server.notificationService.SendNotification(ctx, modelRequest)
	if err != nil {
		server.logger.Error("Service SendNotification error", "error", err)
		return nil, err
	}

	server.logger.Info(
		"notification_request_completed",
		"notification_id", modelResponse.NotificationID,
		"status", modelResponse.Status,
		"recipient_digest", recipientDigest,
	)

	return mapModelToGrpcResponse(modelResponse), nil
}

func (server *notificationServiceServer) GetNotificationStatus(ctx context.Context, req *grpcapi.GetNotificationStatusRequest) (*grpcapi.NotificationResponse, error) {
	notificationID := strings.TrimSpace(req.GetNotificationId())
	if notificationID == "" {
		server.logger.Error("Missing notification ID")
		return nil, status.Error(codes.InvalidArgument, notificationIDRequiredMessage)
	}

	modelResponse, err := server.notificationService.GetNotificationStatus(ctx, notificationID)
	if err != nil {
		server.logger.Error("Service GetNotificationStatus error", "error", err)
		return nil, err
	}
	return mapModelToGrpcResponse(modelResponse), nil
}

func (server *notificationServiceServer) ListNotifications(ctx context.Context, req *grpcapi.ListNotificationsRequest) (*grpcapi.ListNotificationsResponse, error) {
	filters := model.NotificationListFilters{}
	if req != nil {
		filters.Statuses = mapGrpcStatuses(req.GetStatuses())
	}

	responses, err := server.notificationService.ListNotifications(ctx, filters)
	if err != nil {
		server.logger.Error("Service ListNotifications error", "error", err)
		return nil, err
	}

	grpcNotifications := make([]*grpcapi.NotificationResponse, 0, len(responses))
	for _, response := range responses {
		grpcNotifications = append(grpcNotifications, mapModelToGrpcResponse(response))
	}

	return &grpcapi.ListNotificationsResponse{Notifications: grpcNotifications}, nil
}

func (server *notificationServiceServer) RescheduleNotification(ctx context.Context, req *grpcapi.RescheduleNotificationRequest) (*grpcapi.NotificationResponse, error) {
	notificationID := strings.TrimSpace(req.GetNotificationId())
	if notificationID == "" {
		server.logger.Error("Missing notification ID for reschedule")
		return nil, status.Error(codes.InvalidArgument, notificationIDRequiredMessage)
	}
	if req.ScheduledTime == nil {
		server.logger.Error("Missing scheduled time for reschedule")
		return nil, status.Error(codes.InvalidArgument, scheduledTimeRequiredMessage)
	}
	if err := req.ScheduledTime.CheckValid(); err != nil {
		server.logger.Error("Invalid scheduled timestamp", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid scheduled_time: %v", err)
	}

	scheduledFor := req.ScheduledTime.AsTime().UTC()
	if scheduledFor.Before(time.Now().UTC()) {
		server.logger.Error("Scheduled time is in the past", "notification_id", notificationID, "scheduled_for", scheduledFor)
		return nil, status.Error(codes.InvalidArgument, scheduledTimeFutureMessage)
	}
	modelResponse, err := server.notificationService.RescheduleNotification(ctx, notificationID, scheduledFor)
	if err != nil {
		server.logger.Error("Service RescheduleNotification error", "error", err)
		return nil, err
	}
	return mapModelToGrpcResponse(modelResponse), nil
}

func (server *notificationServiceServer) CancelNotification(ctx context.Context, req *grpcapi.CancelNotificationRequest) (*grpcapi.NotificationResponse, error) {
	notificationID := strings.TrimSpace(req.GetNotificationId())
	if notificationID == "" {
		server.logger.Error("Missing notification ID for cancel")
		return nil, status.Error(codes.InvalidArgument, notificationIDRequiredMessage)
	}

	modelResponse, err := server.notificationService.CancelNotification(ctx, notificationID)
	if err != nil {
		server.logger.Error("Service CancelNotification error", "error", err)
		return nil, err
	}
	return mapModelToGrpcResponse(modelResponse), nil
}

// mapModelToGrpcResponse converts a model.NotificationResponse to a grpcapi.NotificationResponse.
func mapModelToGrpcResponse(modelResp model.NotificationResponse) *grpcapi.NotificationResponse {
	var grpcNotifType grpcapi.NotificationType
	switch modelResp.NotificationType {
	case model.NotificationEmail:
		grpcNotifType = grpcapi.NotificationType_EMAIL
	case model.NotificationSMS:
		grpcNotifType = grpcapi.NotificationType_SMS
	default:
		grpcNotifType = grpcapi.NotificationType_EMAIL
	}

	var grpcStatus grpcapi.Status
	switch modelResp.Status {
	case model.StatusQueued:
		grpcStatus = grpcapi.Status_QUEUED
	case model.StatusSent:
		grpcStatus = grpcapi.Status_SENT
	case model.StatusCancelled:
		grpcStatus = grpcapi.Status_CANCELLED
	case model.StatusErrored:
		grpcStatus = grpcapi.Status_ERRORED
	case model.StatusFailed:
		grpcStatus = grpcapi.Status_FAILED
	default:
		grpcStatus = grpcapi.Status_UNKNOWN
	}

	var scheduledTime *timestamppb.Timestamp
	if modelResp.ScheduledFor != nil {
		scheduledTime = timestamppb.New(modelResp.ScheduledFor.UTC())
	}

	return &grpcapi.NotificationResponse{
		NotificationId:    modelResp.NotificationID,
		NotificationType:  grpcNotifType,
		Recipient:         modelResp.Recipient,
		Subject:           modelResp.Subject,
		Message:           modelResp.Message,
		Status:            grpcStatus,
		ProviderMessageId: modelResp.ProviderMessageID,
		RetryCount:        int32(modelResp.RetryCount),
		CreatedAt:         modelResp.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         modelResp.UpdatedAt.Format(time.RFC3339),
		ScheduledTime:     scheduledTime,
		Attachments:       mapModelAttachments(modelResp.Attachments),
		TenantId:          modelResp.TenantID,
	}
}

func digestForLogging(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(digest[:8])
}

func mapGrpcAttachments(source []*grpcapi.EmailAttachment) []model.EmailAttachment {
	if len(source) == 0 {
		return nil
	}
	result := make([]model.EmailAttachment, 0, len(source))
	for _, attachment := range source {
		if attachment == nil {
			continue
		}
		clonedData := make([]byte, len(attachment.Data))
		copy(clonedData, attachment.Data)
		result = append(result, model.EmailAttachment{
			Filename:    attachment.GetFilename(),
			ContentType: attachment.GetContentType(),
			Data:        clonedData,
		})
	}
	return result
}

func mapModelAttachments(source []model.EmailAttachment) []*grpcapi.EmailAttachment {
	if len(source) == 0 {
		return nil
	}
	result := make([]*grpcapi.EmailAttachment, 0, len(source))
	for _, attachment := range source {
		clonedData := make([]byte, len(attachment.Data))
		copy(clonedData, attachment.Data)
		result = append(result, &grpcapi.EmailAttachment{
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			Data:        clonedData,
		})
	}
	return result
}

func mapGrpcStatuses(source []grpcapi.Status) []model.NotificationStatus {
	if len(source) == 0 {
		return nil
	}
	result := make([]model.NotificationStatus, 0, len(source))
	for _, statusValue := range source {
		switch statusValue {
		case grpcapi.Status_QUEUED:
			result = append(result, model.StatusQueued)
		case grpcapi.Status_SENT:
			result = append(result, model.StatusSent)
		case grpcapi.Status_FAILED:
			result = append(result, model.StatusFailed)
		case grpcapi.Status_CANCELLED:
			result = append(result, model.StatusCancelled)
		case grpcapi.Status_ERRORED:
			result = append(result, model.StatusErrored)
		case grpcapi.Status_UNKNOWN:
			result = append(result, model.StatusUnknown)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func buildAuthInterceptor(logger *slog.Logger, requiredToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		metadataValues, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			logger.Error("Missing metadata in gRPC request")
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		authorizationHeaders := metadataValues.Get("authorization")
		if len(authorizationHeaders) == 0 {
			logger.Error("Missing authorization header")
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}
		headerValue := authorizationHeaders[0]
		if !strings.HasPrefix(headerValue, "Bearer ") {
			logger.Error("Invalid authorization header format")
			return nil, status.Error(codes.Unauthenticated, "invalid authorization header")
		}
		token := strings.TrimPrefix(headerValue, "Bearer ")
		if token != requiredToken {
			logger.Error("Invalid token provided")
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		return handler(ctx, req)
	}
}

type tenantIDGetter interface {
	GetTenantId() string
}

func buildTenantInterceptor(logger *slog.Logger, repo *tenant.Repository) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if repo == nil {
			logger.Error(tenantRepositoryUnavailableError)
			return nil, status.Error(codes.Internal, tenantRepositoryUnavailableError)
		}
		var tenantID string
		if requestWithTenantID, ok := req.(tenantIDGetter); ok {
			tenantID = strings.TrimSpace(requestWithTenantID.GetTenantId())
		}
		if tenantID == "" {
			if metadataValues, ok := metadata.FromIncomingContext(ctx); ok {
				if values := metadataValues.Get(tenantMetadataKey); len(values) > 0 {
					tenantID = strings.TrimSpace(values[0])
				}
			}
		}
		if tenantID == "" {
			return nil, status.Error(codes.InvalidArgument, tenantIDRequiredMessage)
		}
		runtimeCfg, err := repo.ResolveByID(ctx, tenantID)
		if err != nil {
			logger.Error("tenant_resolution_failed", "tenant_id", tenantID, "error", err)
			return nil, status.Error(codes.NotFound, tenantNotFoundMessage)
		}
		ctxWithTenant := tenant.WithRuntime(ctx, runtimeCfg)
		return handler(ctxWithTenant, req)
	}
}

type smtpSubmissionStarter interface {
	Start(context.Context) error
}

type smtpForwardingStarter interface {
	Start(context.Context) error
}

type httpServerRunner interface {
	Start() error
	Shutdown(context.Context) error
}

type serverDependencies struct {
	loadConfig                func() (config.Config, error)
	newLogger                 func(string) *slog.Logger
	initDB                    func(string, *slog.Logger) (*gorm.DB, error)
	newSecretKeeper           func(string) (*tenant.SecretKeeper, error)
	bootstrapTenants          func(context.Context, *gorm.DB, *tenant.SecretKeeper, tenant.BootstrapConfig) error
	bootstrapTenantsFromFile  func(context.Context, *gorm.DB, *tenant.SecretKeeper, string) error
	newTenantRepository       func(*gorm.DB, *tenant.SecretKeeper) *tenant.Repository
	newSMTPIdentityRepository func(*gorm.DB, string) (*smtpidentity.Repository, error)
	replaceSenderDomains      func(context.Context, *gorm.DB, []string) error
	newSMTPIdentityService    func(*smtpidentity.Repository, smtpidentity.PublicSettings) *smtpidentity.Service
	newNotificationService    func(*gorm.DB, *slog.Logger, config.Config, *tenant.Repository) service.NotificationService
	loadTLSConfig             func(string, string) (*tls.Config, error)
	newSMTPRelay              func(*slog.Logger, config.Config) smtpsubmission.RawRelay
	newSMTPSubmissionServer   func(smtpsubmission.Config) (smtpSubmissionStarter, error)
	newSMTPForwarder          func(*slog.Logger, config.Config) (smtpforwarding.Forwarder, error)
	newSMTPForwardingServer   func(smtpforwarding.Config) (smtpForwardingStarter, error)
	newSessionValidator       func(sessionvalidator.Config) (httpapi.SessionValidator, error)
	newHTTPServer             func(httpapi.Config) (httpServerRunner, error)
	listen                    func(string, string) (net.Listener, error)
	serveGRPC                 func(net.Listener, service.NotificationService, *tenant.Repository, *slog.Logger, string) error
	exit                      func(int)
}

func main() {
	runServerAndExit(os.Args[1:], productionServerDependencies())
}

func runServerAndExit(args []string, dependencies serverDependencies) {
	dependencies = withServerDependencyDefaults(dependencies)
	exitCode := runServer(args, dependencies)
	if exitCode != 0 {
		dependencies.exit(exitCode)
	}
}

func productionServerDependencies() serverDependencies {
	return serverDependencies{
		loadConfig:                config.LoadConfig,
		newLogger:                 logging.NewLogger,
		initDB:                    db.InitDB,
		newSecretKeeper:           tenant.NewSecretKeeper,
		bootstrapTenants:          tenant.Bootstrap,
		bootstrapTenantsFromFile:  tenant.BootstrapFromFile,
		newTenantRepository:       tenant.NewRepository,
		newSMTPIdentityRepository: smtpidentity.NewRepository,
		replaceSenderDomains:      smtpidentity.ReplaceSenderDomains,
		newSMTPIdentityService:    smtpidentity.NewService,
		newNotificationService:    service.NewNotificationService,
		loadTLSConfig:             smtpsubmission.LoadTLSConfig,
		newSMTPRelay: func(logger *slog.Logger, cfg config.Config) smtpsubmission.RawRelay {
			if cfg.SMTPSubmission.DeliveryMode == "direct" {
				return smtpsubmission.NewDirectMXRelay(logger, cfg)
			}
			return smtpsubmission.NewUpstreamRelay(logger, cfg)
		},
		newSMTPSubmissionServer: func(cfg smtpsubmission.Config) (smtpSubmissionStarter, error) {
			return smtpsubmission.NewServer(cfg)
		},
		newSMTPForwarder: func(logger *slog.Logger, cfg config.Config) (smtpforwarding.Forwarder, error) {
			relayProfile := cfg.SMTPForwarding.Relay
			sender := service.NewSMTPEmailSender(service.SMTPConfig{
				Host:     relayProfile.Host,
				Port:     strconv.Itoa(relayProfile.Port),
				Username: relayProfile.Username,
				Password: relayProfile.Password,
				Timeouts: cfg,
			}, logger)
			return smtpforwarding.NewRelayForwarder(sender, logger)
		},
		newSMTPForwardingServer: func(cfg smtpforwarding.Config) (smtpForwardingStarter, error) {
			return smtpforwarding.NewServer(cfg)
		},
		newSessionValidator: func(cfg sessionvalidator.Config) (httpapi.SessionValidator, error) {
			return sessionvalidator.New(cfg)
		},
		newHTTPServer: func(cfg httpapi.Config) (httpServerRunner, error) {
			return httpapi.NewServer(cfg)
		},
		listen:    net.Listen,
		serveGRPC: serveGRPC,
		exit:      os.Exit,
	}
}

func runServer(args []string, dependencies serverDependencies) int {
	dependencies = withServerDependencyDefaults(dependencies)
	flags := flag.NewFlagSet("pinguin-server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	if parseErr := flags.Parse(args); parseErr != nil {
		if errors.Is(parseErr, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	configuration, configErr := dependencies.loadConfig()
	if configErr != nil {
		fallbackLogger := dependencies.newLogger("INFO")
		for _, errMsg := range strings.Split(configErr.Error(), ", ") {
			fallbackLogger.Error("Configuration error", "detail", errMsg)
		}
		return 1
	}

	mainLogger := dependencies.newLogger(configuration.LogLevel)
	mainLogger.Info("Starting gRPC Notification Server on :50051")

	databaseInstance, dbErr := dependencies.initDB(configuration.DatabasePath, mainLogger)
	if dbErr != nil {
		mainLogger.Error("Failed to initialize DB", "error", dbErr)
		return 1
	}

	secretKeeper, keeperErr := dependencies.newSecretKeeper(configuration.MasterEncryptionKey)
	if keeperErr != nil {
		mainLogger.Error("Failed to initialize secret keeper", "error", keeperErr)
		return 1
	}

	bootstrapCfg := configuration.TenantBootstrap
	switch {
	case len(bootstrapCfg.Tenants) > 0:
		if bootstrapErr := dependencies.bootstrapTenants(context.Background(), databaseInstance, secretKeeper, bootstrapCfg); bootstrapErr != nil {
			mainLogger.Error("Failed to bootstrap tenants", "error", bootstrapErr)
			return 1
		}
	case configuration.TenantConfigPath != "":
		if bootstrapErr := dependencies.bootstrapTenantsFromFile(context.Background(), databaseInstance, secretKeeper, configuration.TenantConfigPath); bootstrapErr != nil {
			mainLogger.Error("Failed to bootstrap tenants", "error", bootstrapErr)
			return 1
		}
	default:
		mainLogger.Error("Failed to bootstrap tenants", "error", "no tenant config supplied")
		return 1
	}
	tenantRepo := dependencies.newTenantRepository(databaseInstance, secretKeeper)
	smtpIdentityRepo, smtpIdentityRepoErr := dependencies.newSMTPIdentityRepository(databaseInstance, configuration.MasterEncryptionKey)
	if smtpIdentityRepoErr != nil {
		mainLogger.Error("Failed to initialize SMTP identity repository", "error", smtpIdentityRepoErr)
		return 1
	}
	if len(configuration.SMTPSubmission.SenderDomains) > 0 {
		if senderDomainErr := dependencies.replaceSenderDomains(context.Background(), databaseInstance, configuration.SMTPSubmission.SenderDomains); senderDomainErr != nil {
			mainLogger.Error("Failed to configure SMTP submission sender domains", "error", senderDomainErr)
			return 1
		}
	}
	smtpIdentityService := dependencies.newSMTPIdentityService(smtpIdentityRepo, smtpPublicSettings(configuration.SMTPSubmission))

	notificationSvc := dependencies.newNotificationService(databaseInstance, mainLogger, configuration, tenantRepo)

	// Start the background retry worker.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	go notificationSvc.StartRetryWorker(workerCtx)

	if configuration.SMTPSubmission.Enabled {
		var tlsConfig *tls.Config
		if configuration.SMTPSubmission.TLSCertPath != "" && configuration.SMTPSubmission.TLSKeyPath != "" {
			loadedTLSConfig, tlsErr := dependencies.loadTLSConfig(configuration.SMTPSubmission.TLSCertPath, configuration.SMTPSubmission.TLSKeyPath)
			if tlsErr != nil {
				mainLogger.Error("Failed to load SMTP submission TLS config", "error", tlsErr)
				return 1
			}
			tlsConfig = loadedTLSConfig
		}
		smtpSubmissionServer, smtpServerErr := dependencies.newSMTPSubmissionServer(smtpsubmission.Config{
			Hostname:          configuration.SMTPSubmission.Hostname,
			ListenAddr:        configuration.SMTPSubmission.ListenAddr,
			TLSListenAddr:     configuration.SMTPSubmission.TLSListenAddr,
			TLSConfig:         tlsConfig,
			MaxMessageBytes:   configuration.SMTPSubmission.MaxMessageBytes,
			MaxRecipients:     configuration.SMTPSubmission.MaxRecipients,
			CommandTimeout:    time.Duration(configuration.OperationTimeoutSec) * time.Second,
			AllowInsecureAuth: configuration.SMTPSubmission.AllowInsecureAuth,
			Authenticator:     smtpIdentityRepo,
			Relay:             dependencies.newSMTPRelay(mainLogger, configuration),
			Logger:            mainLogger,
		})
		if smtpServerErr != nil {
			mainLogger.Error("Failed to initialize SMTP submission server", "error", smtpServerErr)
			return 1
		}
		smtpSubmissionCtx, cancelSMTPSubmission := context.WithCancel(context.Background())
		defer cancelSMTPSubmission()
		startSMTPSubmission(smtpSubmissionCtx, mainLogger, smtpSubmissionServer, configuration, dependencies.exit)
	}

	if configuration.SMTPForwarding.Enabled {
		forwarder, forwarderErr := dependencies.newSMTPForwarder(mainLogger, configuration)
		if forwarderErr != nil {
			mainLogger.Error("Failed to initialize SMTP forwarding relay", "error", forwarderErr)
			return 1
		}
		smtpForwardingServer, smtpForwardingErr := dependencies.newSMTPForwardingServer(smtpforwarding.Config{
			Hostname:        configuration.SMTPForwarding.Hostname,
			ListenAddr:      configuration.SMTPForwarding.ListenAddr,
			MaxMessageBytes: configuration.SMTPForwarding.MaxMessageBytes,
			MaxRecipients:   configuration.SMTPForwarding.MaxRecipients,
			CommandTimeout:  time.Duration(configuration.OperationTimeoutSec) * time.Second,
			RouteResolver:   smtpIdentityForwardingResolver{repository: smtpIdentityRepo},
			Forwarder:       forwarder,
			Logger:          mainLogger,
		})
		if smtpForwardingErr != nil {
			mainLogger.Error("Failed to initialize SMTP forwarding server", "error", smtpForwardingErr)
			return 1
		}
		smtpForwardingCtx, cancelSMTPForwarding := context.WithCancel(context.Background())
		defer cancelSMTPForwarding()
		startSMTPForwarding(smtpForwardingCtx, mainLogger, smtpForwardingServer, configuration, dependencies.exit)
	}

	if configuration.WebInterfaceEnabled {
		sessionValidator, validatorErr := dependencies.newSessionValidator(sessionvalidator.Config{
			SigningKey: []byte(configuration.TAuthSigningKey),
			CookieName: configuration.TAuthCookieName,
		})
		if validatorErr != nil {
			mainLogger.Error("Failed to initialize session validator", "error", validatorErr)
			return 1
		}

		httpServer, httpServerErr := dependencies.newHTTPServer(httpapi.Config{
			ListenAddr:          configuration.HTTPListenAddr,
			AllowedOrigins:      configuration.HTTPAllowedOrigins,
			SessionValidator:    sessionValidator,
			NotificationService: notificationSvc,
			SMTPIdentityService: smtpIdentityService,
			TenantRepository:    tenantRepo,
			Logger:              mainLogger,
		})
		if httpServerErr != nil {
			mainLogger.Error("Failed to initialize HTTP server", "error", httpServerErr)
			return 1
		}

		startHTTPServer(mainLogger, httpServer, configuration.HTTPListenAddr, dependencies.exit)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				mainLogger.Error("HTTP server shutdown error", "error", err)
			}
		}()
	} else {
		mainLogger.Info("Web interface disabled; HTTP server not started")
	}

	listener, listenErr := dependencies.listen("tcp", ":50051")
	if listenErr != nil {
		mainLogger.Error("Failed to listen on :50051", "error", listenErr)
		return 1
	}
	mainLogger.Info("gRPC server listening on :50051")

	if serveErr := dependencies.serveGRPC(listener, notificationSvc, tenantRepo, mainLogger, configuration.GRPCAuthToken); serveErr != nil {
		mainLogger.Error("gRPC server crashed", "error", serveErr)
		return 1
	}
	return 0
}

func withServerDependencyDefaults(dependencies serverDependencies) serverDependencies {
	production := productionServerDependencies()
	if dependencies.loadConfig == nil {
		dependencies.loadConfig = production.loadConfig
	}
	if dependencies.newLogger == nil {
		dependencies.newLogger = production.newLogger
	}
	if dependencies.initDB == nil {
		dependencies.initDB = production.initDB
	}
	if dependencies.newSecretKeeper == nil {
		dependencies.newSecretKeeper = production.newSecretKeeper
	}
	if dependencies.bootstrapTenants == nil {
		dependencies.bootstrapTenants = production.bootstrapTenants
	}
	if dependencies.bootstrapTenantsFromFile == nil {
		dependencies.bootstrapTenantsFromFile = production.bootstrapTenantsFromFile
	}
	if dependencies.newTenantRepository == nil {
		dependencies.newTenantRepository = production.newTenantRepository
	}
	if dependencies.newSMTPIdentityRepository == nil {
		dependencies.newSMTPIdentityRepository = production.newSMTPIdentityRepository
	}
	if dependencies.replaceSenderDomains == nil {
		dependencies.replaceSenderDomains = production.replaceSenderDomains
	}
	if dependencies.newSMTPIdentityService == nil {
		dependencies.newSMTPIdentityService = production.newSMTPIdentityService
	}
	if dependencies.newNotificationService == nil {
		dependencies.newNotificationService = production.newNotificationService
	}
	if dependencies.loadTLSConfig == nil {
		dependencies.loadTLSConfig = production.loadTLSConfig
	}
	if dependencies.newSMTPRelay == nil {
		dependencies.newSMTPRelay = production.newSMTPRelay
	}
	if dependencies.newSMTPSubmissionServer == nil {
		dependencies.newSMTPSubmissionServer = production.newSMTPSubmissionServer
	}
	if dependencies.newSMTPForwarder == nil {
		dependencies.newSMTPForwarder = production.newSMTPForwarder
	}
	if dependencies.newSMTPForwardingServer == nil {
		dependencies.newSMTPForwardingServer = production.newSMTPForwardingServer
	}
	if dependencies.newSessionValidator == nil {
		dependencies.newSessionValidator = production.newSessionValidator
	}
	if dependencies.newHTTPServer == nil {
		dependencies.newHTTPServer = production.newHTTPServer
	}
	if dependencies.listen == nil {
		dependencies.listen = production.listen
	}
	if dependencies.serveGRPC == nil {
		dependencies.serveGRPC = production.serveGRPC
	}
	if dependencies.exit == nil {
		dependencies.exit = production.exit
	}
	return dependencies
}

type smtpIdentityForwardingResolver struct {
	repository *smtpidentity.Repository
}

func (resolver smtpIdentityForwardingResolver) Resolve(ctx context.Context, address smtpidentity.Address) (smtpforwarding.Route, bool, error) {
	routeAddress, recipients, exists, err := resolver.repository.ResolveForwarding(ctx, address)
	if err != nil || !exists {
		return smtpforwarding.Route{}, exists, err
	}
	route, routeErr := smtpforwarding.NewRoute(routeAddress, recipients)
	if routeErr != nil {
		return smtpforwarding.Route{}, false, routeErr
	}
	return route, true, nil
}

func startSMTPSubmission(ctx context.Context, logger *slog.Logger, server smtpSubmissionStarter, configuration config.Config, exit func(int)) {
	go func() {
		logger.Info("SMTP submission server listening", "listen_addr", configuration.SMTPSubmission.ListenAddr, "tls_listen_addr", configuration.SMTPSubmission.TLSListenAddr)
		if err := server.Start(ctx); err != nil {
			logger.Error("SMTP submission server crashed", "error", err)
			exit(1)
		}
	}()
}

func startSMTPForwarding(ctx context.Context, logger *slog.Logger, server smtpForwardingStarter, configuration config.Config, exit func(int)) {
	go func() {
		logger.Info("SMTP forwarding server listening", "listen_addr", configuration.SMTPForwarding.ListenAddr)
		if err := server.Start(ctx); err != nil {
			logger.Error("SMTP forwarding server crashed", "error", err)
			exit(1)
		}
	}()
}

func startHTTPServer(logger *slog.Logger, server httpServerRunner, listenAddr string, exit func(int)) {
	go func() {
		logger.Info("HTTP server listening", "addr", listenAddr)
		if err := server.Start(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logger.Error("HTTP server crashed", "error", err)
				exit(1)
			}
		}
	}()
}

func serveGRPC(listener net.Listener, notificationSvc service.NotificationService, tenantRepo *tenant.Repository, logger *slog.Logger, requiredToken string) error {
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(grpcutil.MaxMessageSizeBytes),
		grpc.MaxSendMsgSize(grpcutil.MaxMessageSizeBytes),
		grpc.ChainUnaryInterceptor(
			buildAuthInterceptor(logger, requiredToken),
			buildTenantInterceptor(logger, tenantRepo),
		),
	)
	grpcapi.RegisterNotificationServiceServer(grpcServer, &notificationServiceServer{
		notificationService: notificationSvc,
		logger:              logger,
	})
	return grpcServer.Serve(listener)
}

func smtpPublicSettings(cfg config.SMTPSubmissionConfig) smtpidentity.PublicSettings {
	port := smtpPortFromAddr(cfg.ListenAddr, 587)
	securityMode := "starttls"
	if strings.TrimSpace(cfg.ListenAddr) == "" && strings.TrimSpace(cfg.TLSListenAddr) != "" {
		port = smtpPortFromAddr(cfg.TLSListenAddr, 465)
		securityMode = "ssl"
	}
	if cfg.PublicPort > 0 {
		port = cfg.PublicPort
	}
	if strings.TrimSpace(cfg.PublicSecurityMode) != "" {
		securityMode = strings.ToLower(strings.TrimSpace(cfg.PublicSecurityMode))
	}
	return smtpidentity.PublicSettings{
		Host:         cfg.Hostname,
		Port:         port,
		SecurityMode: securityMode,
	}
}

func smtpPortFromAddr(address string, fallback int) int {
	trimmedAddress := strings.TrimSpace(address)
	if trimmedAddress == "" {
		return fallback
	}
	_, portValue, splitErr := net.SplitHostPort(trimmedAddress)
	if splitErr != nil {
		portValue = strings.TrimPrefix(trimmedAddress, ":")
	}
	port, parseErr := strconv.Atoi(portValue)
	if parseErr != nil || port <= 0 {
		return fallback
	}
	return port
}
