package client

import (
	"context"
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
	if _, err := NewSettings("", "token", 1, 1); err == nil {
		t.Fatalf("expected error for empty server address")
	}
	if _, err := NewSettings("addr", "", 1, 1); err == nil {
		t.Fatalf("expected error for empty token")
	}
	if _, err := NewSettings("addr", "token", 0, 1); err == nil {
		t.Fatalf("expected error for invalid connection timeout")
	}
	if _, err := NewSettings("addr", "token", 1, 0); err == nil {
		t.Fatalf("expected error for invalid operation timeout")
	}
	settings, err := NewSettings(" addr ", " token ", 2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.ServerAddress() != "addr" {
		t.Fatalf("expected trimmed server address, got %q", settings.ServerAddress())
	}
	if settings.AuthToken() != "token" {
		t.Fatalf("expected trimmed token, got %q", settings.AuthToken())
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
}

func (s *fakeNotificationServer) SendNotification(context.Context, *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	return &grpcapi.NotificationResponse{
		NotificationId: "notif-123",
		Status:         s.initialStatus,
	}, nil
}

func (s *fakeNotificationServer) GetNotificationStatus(context.Context, *grpcapi.GetNotificationStatusRequest) (*grpcapi.NotificationResponse, error) {
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

	settings, err := NewSettings(address, "token", 5, 5)
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

	failureAddr := startServerWithStatuses(t, grpcapi.Status_FAILED, grpcapi.Status_FAILED)
	settings, err := NewSettings(failureAddr, "token", 5, 1)
	if err != nil {
		t.Fatalf("NewSettings error: %v", err)
	}
	clientInstance, err := NewNotificationClient(newTestLogger(), settings)
	if err != nil {
		t.Fatalf("NewNotificationClient error: %v", err)
	}
	defer clientInstance.Close()

	resp, err := clientInstance.SendNotificationAndWait(&grpcapi.NotificationRequest{})
	if err == nil || resp.Status != grpcapi.Status_FAILED {
		t.Fatalf("expected failure status and error, got resp=%v err=%v", resp, err)
	}

	timeoutAddr := startServerWithStatuses(t, grpcapi.Status_QUEUED, grpcapi.Status_QUEUED)
	unresponsiveSettings, err := NewSettings(timeoutAddr, "token", 5, 1)
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
