package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"log/slog"

	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"github.com/tyemirov/pinguin/pkg/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// ErrInvalidSettings indicates the provided Settings inputs do not meet
// required invariants (address, token, or timeout configuration).
var ErrInvalidSettings = errors.New("invalid_client_settings")

// Settings captures the reusable connection/authentication parameters for
// NotificationClient instances. Use NewSettings to construct a validated copy.
type Settings struct {
	serverAddress     string
	authToken         string
	connectionTimeout time.Duration
	operationTimeout  time.Duration
}

// NewSettings validates and normalizes connection/authentication parameters
// used by NotificationClient.
func NewSettings(serverAddress string, authToken string, connectionTimeoutSeconds int, operationTimeoutSeconds int) (Settings, error) {
	address := strings.TrimSpace(serverAddress)
	if address == "" {
		return Settings{}, fmt.Errorf("%w: empty server address", ErrInvalidSettings)
	}
	token := strings.TrimSpace(authToken)
	if token == "" {
		return Settings{}, fmt.Errorf("%w: empty auth token", ErrInvalidSettings)
	}
	if connectionTimeoutSeconds <= 0 {
		return Settings{}, fmt.Errorf("%w: invalid connection timeout %d", ErrInvalidSettings, connectionTimeoutSeconds)
	}
	if operationTimeoutSeconds <= 0 {
		return Settings{}, fmt.Errorf("%w: invalid operation timeout %d", ErrInvalidSettings, operationTimeoutSeconds)
	}
	return Settings{
		serverAddress:     address,
		authToken:         token,
		connectionTimeout: time.Duration(connectionTimeoutSeconds) * time.Second,
		operationTimeout:  time.Duration(operationTimeoutSeconds) * time.Second,
	}, nil
}

// ServerAddress returns the normalized gRPC endpoint for this client.
func (s Settings) ServerAddress() string {
	return s.serverAddress
}

// AuthToken returns the Bearer token that will be attached to outgoing RPCs.
func (s Settings) AuthToken() string {
	return s.authToken
}

// ConnectionTimeout exposes the maximum time allowed to establish the gRPC
// connection.
func (s Settings) ConnectionTimeout() time.Duration {
	return s.connectionTimeout
}

// OperationTimeout exposes the per-RPC timeout used when a context is not
// provided by the caller.
func (s Settings) OperationTimeout() time.Duration {
	return s.operationTimeout
}

// NotificationClient is a thin wrapper around the generated gRPC client that
// automatically wires authentication metadata, call sizing, and optional
// polling helpers.
type NotificationClient struct {
	conn       *grpc.ClientConn
	grpcClient grpcapi.NotificationServiceClient
	authToken  string
	logger     *slog.Logger
	settings   Settings
}

// NewNotificationClient dials the configured server and returns a ready-to-use
// NotificationClient.
func NewNotificationClient(logger *slog.Logger, settings Settings) (*NotificationClient, error) {
	dialCtx, cancel := context.WithTimeout(context.Background(), settings.ConnectionTimeout())
	defer cancel()

	conn, err := grpc.NewClient(
		settings.ServerAddress(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, "tcp", addr)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(grpcutil.MaxMessageSizeBytes),
			grpc.MaxCallSendMsgSize(grpcutil.MaxMessageSizeBytes),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	if dialCtx.Err() != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("dialing gRPC server timed out: %w", dialCtx.Err())
	}

	grpcClient := grpcapi.NewNotificationServiceClient(conn)
	return &NotificationClient{
		conn:       conn,
		grpcClient: grpcClient,
		authToken:  settings.AuthToken(),
		logger:     logger,
		settings:   settings,
	}, nil
}

// Close releases the underlying gRPC connection.
func (clientInstance *NotificationClient) Close() error {
	return clientInstance.conn.Close()
}

// SendNotification invokes the SendNotification RPC with the provided context.
func (clientInstance *NotificationClient) SendNotification(ctx context.Context, req *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+clientInstance.authToken)
	resp, err := clientInstance.grpcClient.SendNotification(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNotificationStatus fetches the latest server status for the supplied
// notification identifier, applying the client's default timeout.
func (clientInstance *NotificationClient) GetNotificationStatus(notificationID string) (*grpcapi.NotificationResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), clientInstance.settings.OperationTimeout())
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+clientInstance.authToken)
	req := &grpcapi.GetNotificationStatusRequest{
		NotificationId: notificationID,
	}
	resp, err := clientInstance.grpcClient.GetNotificationStatus(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

var sendPollInterval = 2 * time.Second

// SendNotificationAndWait issues a SendNotification RPC and polls for its
// terminal status until it is either sent, fails, or the client's timeout
// elapses.
func (clientInstance *NotificationClient) SendNotificationAndWait(req *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), clientInstance.settings.OperationTimeout())
	defer cancel()

	resp, err := clientInstance.SendNotification(ctx, req)
	if err != nil {
		clientInstance.logger.Error("SendNotification failed", "error", err)
		return nil, err
	}
	pollTimeout := clientInstance.settings.OperationTimeout()
	startTime := time.Now()

	for {
		switch resp.Status {
		case grpcapi.Status_SENT:
			return resp, nil
		case grpcapi.Status_FAILED:
			return resp, fmt.Errorf("notification failed")
		}

		if time.Since(startTime) > pollTimeout {
			return resp, fmt.Errorf("timeout waiting for notification to be sent")
		}

		time.Sleep(sendPollInterval)
		statusResp, statusErr := clientInstance.GetNotificationStatus(resp.NotificationId)
		if statusErr != nil {
			clientInstance.logger.Error("GetNotificationStatus failed", "notificationID", resp.NotificationId, "error", statusErr)
			return nil, statusErr
		}
		resp = statusResp
	}
}
