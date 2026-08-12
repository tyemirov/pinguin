package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"google.golang.org/grpc"
)

func TestNewSettingsValidation(t *testing.T) {
	t.Helper()
	if _, err := NewSettings("", "key", 1, 1); err == nil {
		t.Fatalf("expected error for empty server address")
	}
	if _, err := NewSettings("addr", "", 1, 1); err == nil {
		t.Fatalf("expected error for empty token")
	}
	if _, err := NewSettings("addr", "key", 0, 1); err == nil {
		t.Fatalf("expected error for invalid connection timeout")
	}
	if _, err := NewSettings("addr", "key", 1, 0); err == nil {
		t.Fatalf("expected error for invalid operation timeout")
	}
	settings, err := NewSettings(" addr ", " key ", 2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ServerAddress() != "addr" {
		t.Fatalf("expected trimmed server address, got %q", settings.ServerAddress())
	}
	if settings.APIKey() != "key" {
		t.Fatalf("expected trimmed API key, got %q", settings.APIKey())
	}
	if settings.ConnectionTimeout() != 2*time.Second || settings.OperationTimeout() != 3*time.Second {
		t.Fatalf("unexpected timeout durations")
	}
}

type fakeNotificationServer struct {
	grpcapi.UnimplementedNotificationServiceServer
	initialStatus grpcapi.Status
	polledStatus  grpcapi.Status
	statusCalls   int
	sendErr       error
	statusErr     error
	lastRequest   *grpcapi.NotificationRequest
}

func (s *fakeNotificationServer) SendNotification(_ context.Context, request *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	if s.sendErr != nil {
		return nil, s.sendErr
	}
	s.lastRequest = request
	return &grpcapi.NotificationResponse{
		NotificationId: "notif-123",
		Status:         s.initialStatus,
	}, nil
}

func (s *fakeNotificationServer) GetNotificationStatus(context.Context, *grpcapi.GetNotificationStatusRequest) (*grpcapi.NotificationResponse, error) {
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	s.statusCalls++
	status := s.polledStatus
	return &grpcapi.NotificationResponse{
		NotificationId: "notif-123",
		Status:         status,
	}, nil
}

func startFakeServer(t *testing.T, srv grpcapi.NotificationServiceServer) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer()
	grpcapi.RegisterNotificationServiceServer(server, srv)
	go server.Serve(listener)
	return listener.Addr().String(), func() {
		server.Stop()
		listener.Close()
	}
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestNotificationClientSendAndWait(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { sendPollInterval = 2 * time.Second })
	sendPollInterval = 10 * time.Millisecond

	server := &fakeNotificationServer{
		initialStatus: grpcapi.Status_QUEUED,
		polledStatus:  grpcapi.Status_SENT,
	}
	address, stop := startFakeServer(t, server)
	defer stop()

	settings, err := NewSettings(address, "key", 5, 5)
	if err != nil {
		t.Fatalf("NewSettings error: %v", err)
	}
	clientInstance, err := NewNotificationClient(newTestLogger(), settings)
	if err != nil {
		t.Fatalf("NewNotificationClient error: %v", err)
	}
	defer clientInstance.Close()

	resp, err := clientInstance.SendNotification(context.Background(), &grpcapi.NotificationRequest{})
	if err != nil || resp.NotificationId == "" {
		t.Fatalf("SendNotification failed: resp=%v err=%v", resp, err)
	}

	status, err := clientInstance.GetNotificationStatus("notif-123")
	if err != nil {
		t.Fatalf("GetNotificationStatus error: %v", err)
	}
	if status.NotificationId != "notif-123" {
		t.Fatalf("unexpected notification id %q", status.NotificationId)
	}

	waitResp, err := clientInstance.SendNotificationAndWait(&grpcapi.NotificationRequest{})
	if err != nil {
		t.Fatalf("SendNotificationAndWait error: %v", err)
	}
	if waitResp.Status != grpcapi.Status_SENT {
		t.Fatalf("expected status SENT, got %v", waitResp.Status)
	}
}

func TestNotificationClientFailurePaths(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { sendPollInterval = 2 * time.Second })
	sendPollInterval = 5 * time.Millisecond

	failureAddr := startServerWithStatuses(t, grpcapi.Status_ERRORED, grpcapi.Status_ERRORED)
	settings, err := NewSettings(failureAddr, "key", 5, 1)
	if err != nil {
		t.Fatalf("NewSettings error: %v", err)
	}
	clientInstance, err := NewNotificationClient(newTestLogger(), settings)
	if err != nil {
		t.Fatalf("NewNotificationClient error: %v", err)
	}
	defer clientInstance.Close()

	resp, err := clientInstance.SendNotificationAndWait(&grpcapi.NotificationRequest{})
	if err == nil || resp.Status != grpcapi.Status_ERRORED {
		t.Fatalf("expected errored status and error, got resp=%v err=%v", resp, err)
	}

	timeoutAddr := startServerWithStatuses(t, grpcapi.Status_QUEUED, grpcapi.Status_QUEUED)
	unresponsiveSettings, err := NewSettings(timeoutAddr, "key", 5, 1)
	if err != nil {
		t.Fatalf("NewSettings error: %v", err)
	}
	timeoutClient, err := NewNotificationClient(newTestLogger(), unresponsiveSettings)
	if err != nil {
		t.Fatalf("NewNotificationClient error: %v", err)
	}
	defer timeoutClient.Close()

	_, timeoutErr := timeoutClient.SendNotificationAndWait(&grpcapi.NotificationRequest{})
	if timeoutErr == nil {
		t.Fatalf("expected timeout error")
	}
}

func TestNotificationClientPropagatesRPCErrors(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { sendPollInterval = 2 * time.Second })
	sendPollInterval = 5 * time.Millisecond

	sendFailureServer := &fakeNotificationServer{sendErr: errors.New("send failed")}
	sendFailureAddr, stopSendFailure := startFakeServer(t, sendFailureServer)
	defer stopSendFailure()
	settings, err := NewSettings(sendFailureAddr, "key", 5, 1)
	if err != nil {
		t.Fatalf("NewSettings error: %v", err)
	}
	clientInstance, err := NewNotificationClient(newTestLogger(), settings)
	if err != nil {
		t.Fatalf("NewNotificationClient error: %v", err)
	}
	defer clientInstance.Close()
	if _, err := clientInstance.SendNotification(context.Background(), &grpcapi.NotificationRequest{}); err == nil {
		t.Fatalf("expected send error")
	}
	if _, err := clientInstance.SendNotificationAndWait(&grpcapi.NotificationRequest{}); err == nil {
		t.Fatalf("expected send-and-wait send error")
	}

	statusFailureServer := &fakeNotificationServer{
		initialStatus: grpcapi.Status_QUEUED,
		statusErr:     errors.New("status failed"),
	}
	statusFailureAddr, stopStatusFailure := startFakeServer(t, statusFailureServer)
	defer stopStatusFailure()
	statusSettings, err := NewSettings(statusFailureAddr, "key", 5, 1)
	if err != nil {
		t.Fatalf("NewSettings error: %v", err)
	}
	statusClient, err := NewNotificationClient(newTestLogger(), statusSettings)
	if err != nil {
		t.Fatalf("NewNotificationClient error: %v", err)
	}
	defer statusClient.Close()
	if _, err := statusClient.GetNotificationStatus("notif-123"); err == nil {
		t.Fatalf("expected status error")
	}
	if _, err := statusClient.SendNotificationAndWait(&grpcapi.NotificationRequest{}); err == nil {
		t.Fatalf("expected poll status error")
	}
}

func TestNewNotificationClientReportsConstructorError(t *testing.T) {
	originalNewClient := newGRPCClient
	t.Cleanup(func() { newGRPCClient = originalNewClient })
	expectedErr := errors.New("constructor failed")
	newGRPCClient = func(string, ...grpc.DialOption) (*grpc.ClientConn, error) {
		return nil, expectedErr
	}
	settings, err := NewSettings("localhost:50051", "key", 5, 5)
	if err != nil {
		t.Fatalf("NewSettings error: %v", err)
	}
	if _, err := NewNotificationClient(newTestLogger(), settings); !errors.Is(err, expectedErr) {
		t.Fatalf("expected constructor error, got %v", err)
	}
}

func startServerWithStatuses(t *testing.T, initial, polled grpcapi.Status) string {
	t.Helper()
	server := &fakeNotificationServer{
		initialStatus: initial,
		polledStatus:  polled,
	}
	address, stop := startFakeServer(t, server)
	t.Cleanup(stop)
	return address
}
