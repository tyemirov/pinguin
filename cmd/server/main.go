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
	tenantRepo          *tenant.Repository
	logger              *slog.Logger
}

func (server *notificationServiceServer) SendNotification(ctx context.Context, req *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	ctxWithTenant, tenantErr := server.attachTenantRuntime(ctx, req.GetTenantId())
	if tenantErr != nil {
		return nil, tenantErr
	}
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

	recipientDigest := digestForLogging(req.Recipient)
	subjectDigest := digestForLogging(req.Subject)
	attachments := mapGrpcAttachments(req.GetAttachments())
	server.logger.Info(
		"notification_request_received",
		"notification_type", req.NotificationType.String(),
		"subject_digest", subjectDigest,
		"recipient_digest", recipientDigest,
		"scheduled", scheduledFor != nil,
		"attachment_count", len(attachments),
	)

	modelRequest := model.NotificationRequest{
		NotificationType: internalType,
		Recipient:        req.Recipient,
		Subject:          req.Subject,
		Message:          req.Message,
		ScheduledFor:     scheduledFor,
		Attachments:      attachments,
	}

	modelResponse, err := server.notificationService.SendNotification(ctxWithTenant, modelRequest)
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
	if req.NotificationId == "" {
		server.logger.Error("Missing notification ID")
		return nil, fmt.Errorf("missing notification ID")
	}

	ctxWithTenant, tenantErr := server.attachTenantRuntime(ctx, req.GetTenantId())
	if tenantErr != nil {
		return nil, tenantErr
	}

	modelResponse, err := server.notificationService.GetNotificationStatus(ctxWithTenant, req.NotificationId)
	if err != nil {
		server.logger.Error("Service GetNotificationStatus error", "error", err)
		return nil, err
	}
	return mapModelToGrpcResponse(modelResponse), nil
}

func (server *notificationServiceServer) ListNotifications(ctx context.Context, req *grpcapi.ListNotificationsRequest) (*grpcapi.ListNotificationsResponse, error) {
	ctxWithTenant, tenantErr := server.attachTenantRuntime(ctx, req.GetTenantId())
	if tenantErr != nil {
		return nil, tenantErr
	}
	filters := model.NotificationListFilters{}
	if req != nil {
		filters.Statuses = mapGrpcStatuses(req.GetStatuses())
	}

	responses, err := server.notificationService.ListNotifications(ctxWithTenant, filters)
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
	if req.GetNotificationId() == "" {
		server.logger.Error("Missing notification ID for reschedule")
		return nil, status.Error(codes.InvalidArgument, "notification_id is required")
	}
	if req.ScheduledTime == nil {
		server.logger.Error("Missing scheduled time for reschedule")
		return nil, status.Error(codes.InvalidArgument, "scheduled_time is required")
	}
	if err := req.ScheduledTime.CheckValid(); err != nil {
		server.logger.Error("Invalid scheduled timestamp", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid scheduled_time: %v", err)
	}

	ctxWithTenant, tenantErr := server.attachTenantRuntime(ctx, req.GetTenantId())
	if tenantErr != nil {
		return nil, tenantErr
	}

	scheduledFor := req.ScheduledTime.AsTime().UTC()
	modelResponse, err := server.notificationService.RescheduleNotification(ctxWithTenant, req.GetNotificationId(), scheduledFor)
	if err != nil {
		server.logger.Error("Service RescheduleNotification error", "error", err)
		return nil, err
	}
	return mapModelToGrpcResponse(modelResponse), nil
}

func (server *notificationServiceServer) CancelNotification(ctx context.Context, req *grpcapi.CancelNotificationRequest) (*grpcapi.NotificationResponse, error) {
	if req.GetNotificationId() == "" {
		server.logger.Error("Missing notification ID for cancel")
		return nil, status.Error(codes.InvalidArgument, "notification_id is required")
	}

	ctxWithTenant, tenantErr := server.attachTenantRuntime(ctx, req.GetTenantId())
	if tenantErr != nil {
		return nil, tenantErr
	}

	modelResponse, err := server.notificationService.CancelNotification(ctxWithTenant, req.GetNotificationId())
	if err != nil {
		server.logger.Error("Service CancelNotification error", "error", err)
		return nil, err
	}
	return mapModelToGrpcResponse(modelResponse), nil
}

func (server *notificationServiceServer) attachTenantRuntime(ctx context.Context, explicitTenantID string) (context.Context, error) {
	if server.tenantRepo == nil {
		server.logger.Error("tenant repository unavailable")
		return ctx, status.Error(codes.Internal, "tenant repository unavailable")
	}
	tenantID := strings.TrimSpace(explicitTenantID)
	if tenantID == "" {
		if metadataValues, ok := metadata.FromIncomingContext(ctx); ok {
			if values := metadataValues.Get("x-tenant-id"); len(values) > 0 {
				tenantID = strings.TrimSpace(values[0])
			}
		}
	}
	if tenantID == "" {
		return ctx, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	runtimeCfg, err := server.tenantRepo.ResolveByID(ctx, tenantID)
	if err != nil {
		server.logger.Error("tenant_resolution_failed", "tenant_id", tenantID, "error", err)
		return ctx, status.Error(codes.NotFound, "tenant not found")
	}
	return tenant.WithRuntime(ctx, runtimeCfg), nil
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
			Issuer:     configuration.TAuthIssuer,
			CookieName: configuration.TAuthCookieName,
		})
		if validatorErr != nil {
			mainLogger.Error("Failed to initialize session validator", "error", validatorErr)
			os.Exit(1)
		}

		httpServer, httpServerErr := httpapi.NewServer(httpapi.Config{
			ListenAddr:          configuration.HTTPListenAddr,
			StaticRoot:          configuration.HTTPStaticRoot,
			AllowedOrigins:      configuration.HTTPAllowedOrigins,
			SessionValidator:    sessionValidator,
			NotificationService: notificationSvc,
			TenantRepository:    tenantRepo,
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
		grpc.UnaryInterceptor(buildAuthInterceptor(mainLogger, configuration.GRPCAuthToken)),
	)
	grpcapi.RegisterNotificationServiceServer(grpcServer, &notificationServiceServer{
		notificationService: notificationSvc,
		tenantRepo:          tenantRepo,
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
