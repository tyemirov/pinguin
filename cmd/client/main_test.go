package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/tyemirov/pinguin/cmd/client/internal/command"
	"github.com/tyemirov/pinguin/pkg/client"
	"github.com/tyemirov/pinguin/pkg/grpcapi"
)

type mainRecordingSender struct{}

func (sender mainRecordingSender) SendNotification(context.Context, *grpcapi.NotificationRequest) (*grpcapi.NotificationResponse, error) {
	return &grpcapi.NotificationResponse{
		NotificationId: "notif-main",
		Status:         grpcapi.Status_SENT,
	}, nil
}

func TestRunReturnsZeroForHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--help"}, &stdout, &stderr, command.Dependencies{})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "send") {
		t.Fatalf("expected help to mention send, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestMainReturnsForHelp(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	readPipe, writePipe, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("create stdout pipe: %v", pipeErr)
	}
	os.Args = []string{"pinguin-cli", "--help"}
	os.Stdout = writePipe
	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
		_ = readPipe.Close()
	}()

	main()
	_ = writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if !strings.Contains(string(output), "send") {
		t.Fatalf("expected help output to mention send, got %q", string(output))
	}
}

func TestRunReturnsOneForCommandError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"send"}, &stdout, &stderr, command.Dependencies{})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "grpc-auth-token is required") {
		t.Fatalf("expected auth error, got %q", stderr.String())
	}
}

func TestRunAndExitUsesExitForFailures(t *testing.T) {
	exitCodes := make(chan int, 1)
	runAndExit([]string{"send"}, io.Discard, io.Discard, command.Dependencies{}, func(code int) {
		exitCodes <- code
	})
	select {
	case code := <-exitCodes:
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	default:
		t.Fatalf("expected exit to be called")
	}
}

func TestRunUsesInjectedSender(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{
		"send",
		"--grpc-server-addr", "localhost:50051",
		"--grpc-auth-token", "token",
		"--tenant-id", "tenant",
		"--type", "sms",
		"--recipient", "+15551234567",
		"--message", "hello",
	}, &stdout, &stderr, command.Dependencies{
		NewSender: func(_ *slog.Logger, _ client.Settings) (command.NotificationSender, io.Closer, error) {
			return mainRecordingSender{}, nil, nil
		},
	})
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Notification notif-main sent with status SENT") {
		t.Fatalf("unexpected stdout %q", stdout.String())
	}
}

func TestRunReportsInjectedSenderFailure(t *testing.T) {
	expectedErr := errors.New("dial failed")
	var stderr bytes.Buffer

	exitCode := run([]string{
		"send",
		"--grpc-server-addr", "localhost:50051",
		"--grpc-auth-token", "token",
		"--tenant-id", "tenant",
		"--type", "sms",
		"--recipient", "+15551234567",
		"--message", "hello",
	}, io.Discard, &stderr, command.Dependencies{
		NewSender: func(_ *slog.Logger, _ client.Settings) (command.NotificationSender, io.Closer, error) {
			return nil, nil, expectedErr
		},
	})
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), expectedErr.Error()) {
		t.Fatalf("expected injected error, got %q", stderr.String())
	}
}
