package integrationtest

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/pinguin/pkg/grpcapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type recordingNotificationServer struct {
	grpcapi.UnimplementedNotificationServiceServer
	lastRequest  *grpcapi.NotificationRequest
	lastMetadata metadata.MD
}

func (s *recordingNotificationServer) SendNotification(ctx context.Context, req *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	s.lastRequest = req
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.lastMetadata = md
	}
	return &grpcapi.NotificationResponse{
		NotificationId: "test-id",
		Status:         grpcapi.Status_SENT,
	}, nil
}

func TestCLISendUsesFlagsAndToAlias(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	grpcServer := grpc.NewServer()
	t.Cleanup(grpcServer.Stop)

	recorder := &recordingNotificationServer{}
	grpcapi.RegisterNotificationServiceServer(grpcServer, recorder)

	go grpcServer.Serve(listener)

	cliBinary := buildCLIBinary(t)

	cmd := exec.Command(
		cliBinary,
		"send",
		"--grpc-server-addr", listener.Addr().String(),
		"--grpc-auth-token", "token-123",
		"--tenant-id", "tenant-123",
		"--type", "email",
		"--to", "user@example.com",
		"--subject", "Hello",
		"--message", "World",
	)
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("expected CLI to succeed: %v\n%s", runErr, string(output))
	}
	if !strings.Contains(string(output), "Notification test-id sent") {
		t.Fatalf("unexpected CLI output:\n%s", string(output))
	}

	deadline := time.Now().Add(2 * time.Second)
	for recorder.lastRequest == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if recorder.lastRequest == nil {
		t.Fatalf("expected gRPC request to be recorded")
	}
	if got := recorder.lastRequest.GetTenantId(); got != "tenant-123" {
		t.Fatalf("expected tenant id tenant-123, got %q", got)
	}
	if got := recorder.lastRequest.GetRecipient(); got != "user@example.com" {
		t.Fatalf("expected recipient user@example.com, got %q", got)
	}
	if got := recorder.lastRequest.GetSubject(); got != "Hello" {
		t.Fatalf("expected subject Hello, got %q", got)
	}
	if got := recorder.lastRequest.GetMessage(); got != "World" {
		t.Fatalf("expected message World, got %q", got)
	}

	if got := recorder.lastMetadata.Get("authorization"); len(got) == 0 || got[0] != "Bearer token-123" {
		t.Fatalf("expected authorization metadata, got %v", got)
	}
	if got := recorder.lastMetadata.Get("x-tenant-id"); len(got) == 0 || got[0] != "tenant-123" {
		t.Fatalf("expected x-tenant-id metadata, got %v", got)
	}
}

func TestCLISendReadsUnprefixedEnv(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	grpcServer := grpc.NewServer()
	t.Cleanup(grpcServer.Stop)

	recorder := &recordingNotificationServer{}
	grpcapi.RegisterNotificationServiceServer(grpcServer, recorder)

	go grpcServer.Serve(listener)

	cliBinary := buildCLIBinary(t)

	cmd := exec.Command(
		cliBinary,
		"send",
		"--type", "sms",
		"--recipient", "+15551234567",
		"--message", "OTP",
	)
	cmd.Env = append(os.Environ(),
		"GRPC_SERVER_ADDR="+listener.Addr().String(),
		"GRPC_AUTH_TOKEN=token-456",
		"TENANT_ID=tenant-456",
	)

	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("expected CLI to succeed: %v\n%s", runErr, string(output))
	}
	if recorder.lastRequest == nil {
		t.Fatalf("expected gRPC request to be recorded")
	}
	if got := recorder.lastRequest.GetTenantId(); got != "tenant-456" {
		t.Fatalf("expected tenant id tenant-456, got %q", got)
	}
	if got := recorder.lastRequest.GetNotificationType(); got != grpcapi.NotificationType_SMS {
		t.Fatalf("expected sms type, got %v", got)
	}
}

func TestCLISendRejectsAttachmentsForSMS(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	grpcServer := grpc.NewServer()
	t.Cleanup(grpcServer.Stop)

	recorder := &recordingNotificationServer{}
	grpcapi.RegisterNotificationServiceServer(grpcServer, recorder)

	go grpcServer.Serve(listener)

	attachmentPath := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(attachmentPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	cliBinary := buildCLIBinary(t)

	cmd := exec.Command(
		cliBinary,
		"send",
		"--grpc-server-addr", listener.Addr().String(),
		"--grpc-auth-token", "token-123",
		"--tenant-id", "tenant-123",
		"--type", "sms",
		"--recipient", "+15551234567",
		"--message", "OTP",
		"--attachment", attachmentPath,
	)
	output, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected CLI to fail; output:\n%s", string(output))
	}
	if !strings.Contains(string(output), "attachments are only supported for email") {
		t.Fatalf("unexpected output:\n%s", string(output))
	}
	if recorder.lastRequest != nil {
		t.Fatalf("expected gRPC request to be skipped on validation failure")
	}
}

func TestCLISendRequiresSubjectForEmail(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	grpcServer := grpc.NewServer()
	t.Cleanup(grpcServer.Stop)

	recorder := &recordingNotificationServer{}
	grpcapi.RegisterNotificationServiceServer(grpcServer, recorder)

	go grpcServer.Serve(listener)

	cliBinary := buildCLIBinary(t)

	cmd := exec.Command(
		cliBinary,
		"send",
		"--grpc-server-addr", listener.Addr().String(),
		"--grpc-auth-token", "token-123",
		"--tenant-id", "tenant-123",
		"--type", "email",
		"--recipient", "user@example.com",
		"--message", "Body",
	)
	output, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected CLI to fail; output:\n%s", string(output))
	}
	if !strings.Contains(string(output), "subject is required") {
		t.Fatalf("unexpected output:\n%s", string(output))
	}
	if recorder.lastRequest != nil {
		t.Fatalf("expected gRPC request to be skipped on validation failure")
	}
}

func buildCLIBinary(t *testing.T) string {
	t.Helper()

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	repositoryRoot := filepath.Dir(filepath.Dir(workingDirectory))
	temporaryBinaryDirectory := t.TempDir()
	temporaryBinaryPath := filepath.Join(temporaryBinaryDirectory, "pinguin-cli")

	buildCommand := exec.Command("go", "build", "-o", temporaryBinaryPath, "./cmd/client")
	buildCommand.Dir = repositoryRoot
	commandOutput, buildErr := buildCommand.CombinedOutput()
	if buildErr != nil {
		t.Fatalf("go build failed: %v\n%s", buildErr, string(commandOutput))
	}

	return temporaryBinaryPath
}
