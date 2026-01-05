package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tyemirov/pinguin/internal/config"
	"github.com/tyemirov/pinguin/internal/db"
	"github.com/tyemirov/pinguin/internal/httpapi"
	"github.com/tyemirov/pinguin/internal/model"
	"github.com/tyemirov/pinguin/internal/service"
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

func main() {
	disableWebFlag := flag.Bool("disable-web-interface", false, "disable the HTTP web interface and static asset server (env: DISABLE_WEB_INTERFACE)")
	flag.Parse()

	configuration, configErr := config.LoadConfig(*disableWebFlag)
	if configErr != nil {
		fallbackLogger := logging.NewLogger("INFO")
		for _, errMsg := range strings.Split(configErr.Error(), ", ") {
			fallbackLogger.Error("Configuration error", "detail", errMsg)
		}
		os.Exit(1)
	}

	mainLogger := logging.NewLogger(configuration.LogLevel)
	mainLogger.Info("Starting gRPC Notification Server on :50051")

	databaseInstance, dbErr := db.InitDB(configuration.DatabasePath, mainLogger)
	if dbErr != nil {
		mainLogger.Error("Failed to initialize DB", "error", dbErr)
		os.Exit(1)
	}

	secretKeeper, keeperErr := tenant.NewSecretKeeper(configuration.MasterEncryptionKey)
	if keeperErr != nil {
		mainLogger.Error("Failed to initialize secret keeper", "error", keeperErr)
		os.Exit(1)
	}

	bootstrapCfg := configuration.TenantBootstrap
	switch {
	case len(bootstrapCfg.Tenants) > 0:
		if bootstrapErr := tenant.Bootstrap(context.Background(), databaseInstance, secretKeeper, bootstrapCfg); bootstrapErr != nil {
			mainLogger.Error("Failed to bootstrap tenants", "error", bootstrapErr)
			os.Exit(1)
		}
	case configuration.TenantConfigPath != "":
		if bootstrapErr := tenant.BootstrapFromFile(context.Background(), databaseInstance, secretKeeper, configuration.TenantConfigPath); bootstrapErr != nil {
			mainLogger.Error("Failed to bootstrap tenants", "error", bootstrapErr)
			os.Exit(1)
		}
	default:
		mainLogger.Error("Failed to bootstrap tenants", "error", "no tenant config supplied")
		os.Exit(1)
	}
	tenantRepo := tenant.NewRepository(databaseInstance, secretKeeper)

	notificationSvc := service.NewNotificationService(databaseInstance, mainLogger, configuration, tenantRepo)

	// Start the background retry worker.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	go notificationSvc.StartRetryWorker(workerCtx)

	if configuration.WebInterfaceEnabled {
		sessionValidator, validatorErr := sessionvalidator.New(sessionvalidator.Config{
			SigningKey: []byte(configuration.TAuthSigningKey),
			CookieName: configuration.TAuthCookieName,
		})
		if validatorErr != nil {
			mainLogger.Error("Failed to initialize session validator", "error", validatorErr)
			os.Exit(1)
		}

		httpServer, httpServerErr := httpapi.NewServer(httpapi.Config{
			ListenAddr:          configuration.HTTPListenAddr,
			AllowedOrigins:      configuration.HTTPAllowedOrigins,
			SessionValidator:    sessionValidator,
			NotificationService: notificationSvc,
			TenantRepository:    tenantRepo,
			TAuthBaseURL:        configuration.TAuthBaseURL,
			TAuthTenantID:       configuration.TAuthTenantID,
			TAuthGoogleClientID: configuration.TAuthGoogleClientID,
			Logger:              mainLogger,
		})
		if httpServerErr != nil {
			mainLogger.Error("Failed to initialize HTTP server", "error", httpServerErr)
			os.Exit(1)
		}

		go func() {
			mainLogger.Info("HTTP server listening", "addr", configuration.HTTPListenAddr)
			if err := httpServer.Start(); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					mainLogger.Error("HTTP server crashed", "error", err)
					os.Exit(1)
				}
			}
		}()
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

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(grpcutil.MaxMessageSizeBytes),
		grpc.MaxSendMsgSize(grpcutil.MaxMessageSizeBytes),
		grpc.ChainUnaryInterceptor(
			buildAuthInterceptor(mainLogger, configuration.GRPCAuthToken),
			buildTenantInterceptor(mainLogger, tenantRepo),
		),
	)
	grpcapi.RegisterNotificationServiceServer(grpcServer, &notificationServiceServer{
		notificationService: notificationSvc,
		logger:              mainLogger,
	})

	listener, listenErr := net.Listen("tcp", ":50051")
	if listenErr != nil {
		mainLogger.Error("Failed to listen on :50051", "error", listenErr)
		os.Exit(1)
	}
	mainLogger.Info("gRPC server listening on :50051")

	if serveErr := grpcServer.Serve(listener); serveErr != nil {
		mainLogger.Error("gRPC server crashed", "error", serveErr)
		os.Exit(1)
	}
}
